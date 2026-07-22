# Plan: Create Article Comment (Feature 14)

## Overview

Add one new protected endpoint:

- `POST /api/articles/{slug}/comments` — create a comment on an article, authored by the authenticated user

Returns `201` with the created comment. Returns `422` if body is blank, `404` if the article slug doesn't match.

## Database Schema (4NF)

Add one table in migration `007_create_comments.sql`:

- **`comments`**: `id SERIAL PK`, `body TEXT NOT NULL`, `author_id INTEGER FK→users.id ON DELETE CASCADE`, `article_id INTEGER FK→articles.id ON DELETE CASCADE`, `created_at TIMESTAMPTZ DEFAULT now()`, `updated_at TIMESTAMPTZ DEFAULT now()`

4NF: every non-key attribute depends on the full key `id`; no multi-valued dependencies.

## Step-by-Step Changes

### 1. Migration — `internal/adapters/out/db/migrations/007_create_comments.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS comments (
    id         SERIAL PRIMARY KEY,
    body       TEXT NOT NULL,
    author_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS comments;
```

### 2. Domain models — `internal/domain/models.go`

Add two new types:

```go
type Comment struct {
    ID        int
    CreatedAt time.Time
    UpdatedAt time.Time
    Body      string
    Author    Profile
}

type CreateComment struct {
    Body string
}
```

### 3. Domain — `internal/domain/comment.go` (new file)

```go
package domain

import "context"

type commentRepo interface {
    InsertComment(ctx context.Context, authorID int, articleSlug string, c *CreateComment) (*Comment, error)
}

type CommentController struct {
    repo commentRepo
}

func NewCommentController(r commentRepo) *CommentController {
    return &CommentController{repo: r}
}

func (c *CommentController) CreateComment(ctx context.Context, authorID int, articleSlug string, comment *CreateComment) (*Comment, error) {
    if comment.Body == "" {
        return nil, NewValidationError("body", blankFieldErrMsg)
    }
    return c.repo.InsertComment(ctx, authorID, articleSlug, comment)
}
```

### 4. DB adapter — `internal/adapters/out/db/postgres.go`

Add `InsertComment` using a single CTE query that inserts the comment and joins the author in one round trip:

```go
func (p *Postgres) InsertComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error) {
    query := `
        WITH ins AS (
            INSERT INTO comments (body, author_id, article_id)
            SELECT $1, $2, id FROM articles WHERE slug = $3
            RETURNING id, body, author_id, created_at, updated_at
        )
        SELECT ins.id, ins.body, ins.created_at, ins.updated_at,
               u.username, u.bio, u.image
        FROM ins
        JOIN users u ON u.id = ins.author_id`

    var row struct {
        ID        int            `db:"id"`
        Body      string         `db:"body"`
        CreatedAt time.Time      `db:"created_at"`
        UpdatedAt time.Time      `db:"updated_at"`
        Username  string         `db:"username"`
        Bio       sql.NullString `db:"bio"`
        Image     sql.NullString `db:"image"`
    }

    err := p.db.QueryRowxContext(ctx, query, c.Body, authorID, articleSlug).StructScan(&row)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, &domain.ArticleNotFoundError{}
        }
        return nil, err
    }

    author := domain.Profile{Username: row.Username, Following: false}
    if row.Bio.Valid {
        s := row.Bio.String
        author.Bio = &s
    }
    if row.Image.Valid {
        s := row.Image.String
        author.Image = &s
    }

    return &domain.Comment{
        ID:        row.ID,
        CreatedAt: row.CreatedAt,
        UpdatedAt: row.UpdatedAt,
        Body:      row.Body,
        Author:    author,
    }, nil
}
```

Key notes:
- The CTE `INSERT ... SELECT ... FROM articles WHERE slug = $3` inserts 0 rows if the article doesn't exist. With `RETURNING`, `QueryRowxContext` then yields `sql.ErrNoRows` → `ArticleNotFoundError`.
- `author.Following` is always `false` — a comment author's follow relationship with the viewer is not part of the comment response per the spec.
- No transaction needed — it's a single atomic query.

### 5. HTTP handler — `internal/adapters/in/webserver/handlers.go`

**Add `commentService` interface**:

```go
type commentService interface {
    CreateComment(ctx context.Context, authorID int, articleSlug string, c *domain.CreateComment) (*domain.Comment, error)
}
```

**Add `commentService` field to `Handler` and update `NewHandler`**:

```go
type Handler struct {
    service        userService
    profileService profileService
    articleService articleService
    tagService     tagService
    commentService commentService
}

func NewHandler(s userService, ps profileService, as articleService, ts tagService, cs commentService) *Handler {
    return &Handler{
        service:        s,
        profileService: ps,
        articleService: as,
        tagService:     ts,
        commentService: cs,
    }
}
```

**Add response DTOs**:

```go
type CommentAuthor struct {
    Username  string  `json:"username"`
    Bio       *string `json:"bio"`
    Image     *string `json:"image"`
    Following bool    `json:"following"`
}

type CommentResponseInner struct {
    ID        int           `json:"id"`
    CreatedAt time.Time     `json:"createdAt"`
    UpdatedAt time.Time     `json:"updatedAt"`
    Body      string        `json:"body"`
    Author    CommentAuthor `json:"author"`
}

type CommentResponse struct {
    Comment CommentResponseInner `json:"comment"`
}
```

**Add `CreateArticleComment` handler**:

```go
func (h *Handler) CreateArticleComment(w http.ResponseWriter, r *http.Request) {
    authorID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    var req struct {
        Comment struct {
            Body string `json:"body"`
        } `json:"comment"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")

    comment, err := h.commentService.CreateComment(r.Context(), authorID, slug, &domain.CreateComment{Body: req.Comment.Body})
    if err != nil {
        var validationErr *domain.ValidationError
        var notFoundErr *domain.ArticleNotFoundError
        if errors.As(err, &validationErr) {
            w.WriteHeader(http.StatusUnprocessableEntity)
            _, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
        } else if errors.As(err, &notFoundErr) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("article", []string{"not found"}))
        } else {
            fmt.Println(err.Error())
            w.WriteHeader(http.StatusInternalServerError)
            _, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
        }
        return
    }

    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(CommentResponse{
        Comment: CommentResponseInner{
            ID:        comment.ID,
            CreatedAt: comment.CreatedAt,
            UpdatedAt: comment.UpdatedAt,
            Body:      comment.Body,
            Author: CommentAuthor{
                Username:  comment.Author.Username,
                Bio:       comment.Author.Bio,
                Image:     comment.Author.Image,
                Following: comment.Author.Following,
            },
        },
    })
}
```

### 6. HTTP server — `internal/adapters/in/webserver/server.go`

**Add `CreateArticleComment` to `ServerHandlers`**:

```go
CreateArticleComment(http.ResponseWriter, *http.Request)
```

**Register on the protected subrouter**:

```go
protected.HandleFunc("/api/articles/{slug}/comments", h.CreateArticleComment).Methods("POST")
```

### 7. Entry point — `cmd/server/server.go`

Add `CommentController` and pass as 5th arg to `NewHandler`:

```go
commentController := domain.NewCommentController(database)
handlers := webserver.NewHandler(userController, profileController, articleController, tagController, commentController)
```

### 8. Update `arch.md`

- Add `comment.go` to domain layer listing.
- Document `CommentController` and `commentRepo` interface.
- Add `InsertComment` to DB adapter section.
- Add `comments` schema table.
- Add `POST /api/articles/{slug}/comments` to routes table.
- Update Current State.

## Order of Implementation

1. `internal/adapters/out/db/migrations/007_create_comments.sql`
2. `internal/domain/models.go` — add `Comment` and `CreateComment`
3. `internal/domain/comment.go` — new file
4. `internal/adapters/out/db/postgres.go` — add `InsertComment`
5. `internal/adapters/in/webserver/handlers.go` — add interface, field, `NewHandler` update, DTOs, handler
6. `internal/adapters/in/webserver/server.go` — add to interface, register route
7. `cmd/server/server.go` — wire `CommentController`
8. `make lint` — iterate until clean
9. `arch.md` — update
