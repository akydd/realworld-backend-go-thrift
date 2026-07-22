# Plan: Update Article (Feature 12)

## Overview

Add one new protected endpoint:

- `PUT /api/articles/{slug}` — allows the article's author to update title, description, and/or body

All three fields are optional, but at least one must be present. A title update also regenerates the slug. Returns the updated article in the same shape as create/get.

## Error cases

| Condition | Status | Body |
|-----------|--------|------|
| No fields in payload | 422 | `{"errors": {"article": ["can't be blank"]}}` |
| Title present but blank | 422 | `{"errors": {"title": ["can't be blank"]}}` |
| Article not found | 404 | `{"errors": {"article": ["not found"]}}` |
| Caller is not the author | 401 | `{"errors": {"credentials": ["invalid"]}}` |
| New title duplicates an existing one | 409 | `{"errors": {"title": ["has already been taken"]}}` |

## Step-by-Step Changes

### 1. Domain model — `internal/domain/models.go`

Add `UpdateArticle` with pointer fields (nil = not provided):

```go
type UpdateArticle struct {
    Title       *string
    Description *string
    Body        *string
}
```

### 2. Domain — `internal/domain/article.go`

**Export `generateSlug`** by renaming it to `GenerateSlug` (capital G). The DB adapter needs it to recompute the slug when a title is provided. Update the internal call in `CreateArticle` accordingly.

**Add `validateUpdateArticle`**:

```go
func validateUpdateArticle(u *UpdateArticle) error {
    if u.Title == nil && u.Description == nil && u.Body == nil {
        return NewValidationError("article", blankFieldErrMsg)
    }
    if u.Title != nil && *u.Title == "" {
        return NewValidationError("title", blankFieldErrMsg)
    }
    return nil
}
```

**Add `UpdateArticle` to the `articleRepo` interface**:

```go
UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error)
```

**Add `UpdateArticle` method to `ArticleController`**:

```go
func (c *ArticleController) UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error) {
    if err := validateUpdateArticle(u); err != nil {
        return nil, err
    }
    return c.repo.UpdateArticle(ctx, callerID, slug, u)
}
```

### 3. DB adapter — `internal/adapters/out/db/postgres.go`

Implement `UpdateArticle` on `*Postgres`. All DB work runs in a transaction:

```go
func (p *Postgres) UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error) {
    tx, err := p.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback() //nolint:errcheck

    // 1. Fetch current article fields + author_id
    var cur struct {
        ID          int    `db:"id"`
        AuthorID    int    `db:"author_id"`
        Title       string `db:"title"`
        Description string `db:"description"`
        Body        string `db:"body"`
    }
    err = tx.QueryRowxContext(ctx,
        `SELECT id, author_id, title, description, body FROM articles WHERE slug = $1`,
        slug).StructScan(&cur)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, &domain.ArticleNotFoundError{}
        }
        return nil, err
    }

    // 2. Auth check
    if cur.AuthorID != callerID {
        return nil, &domain.CredentialsError{}
    }

    // 3. Merge provided fields over current values
    newTitle := cur.Title
    newDescription := cur.Description
    newBody := cur.Body
    newSlug := slug

    if u.Title != nil {
        newTitle = *u.Title
        newSlug = domain.GenerateSlug(newTitle)
    }
    if u.Description != nil {
        newDescription = *u.Description
    }
    if u.Body != nil {
        newBody = *u.Body
    }

    // 4. UPDATE the article row
    _, err = tx.ExecContext(ctx,
        `UPDATE articles SET slug=$1, title=$2, description=$3, body=$4, updated_at=now() WHERE id=$5`,
        newSlug, newTitle, newDescription, newBody, cur.ID)
    if err != nil {
        var pqErr *pq.Error
        if errors.As(err, &pqErr) && pqErr.Code == "23505" {
            switch pqErr.Constraint {
            case "articles_title_unique", "articles_slug_unique":
                return nil, domain.NewDuplicateError("title")
            }
        }
        return nil, err
    }

    if err = tx.Commit(); err != nil {
        return nil, err
    }

    // 5. Return the full article using the existing GetArticleBySlug method
    return p.GetArticleBySlug(ctx, newSlug, callerID)
}
```

Key notes:
- The transaction covers the SELECT + auth check + UPDATE atomically; the final read via `GetArticleBySlug` happens after commit (consistent because the slug is now stable).
- `domain.GenerateSlug` (exported) is called for slug recomputation when a title is provided — keeps the slug algorithm in one place.
- The `callerID` is passed as `viewerID` to `GetArticleBySlug`; a user cannot follow themselves so `author.following` is always `false`.
- Unique-violation mapping mirrors `InsertArticle`.

### 4. HTTP handler — `internal/adapters/in/webserver/handlers.go`

**Add DTOs**:

```go
type UpdateArticleInner struct {
    Title       *string `json:"title"`
    Description *string `json:"description"`
    Body        *string `json:"body"`
}

type UpdateArticleRequest struct {
    Article UpdateArticleInner `json:"article"`
}
```

**Extend `articleService` interface**:

```go
type articleService interface {
    CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
    GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error)
    UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error)
}
```

**Add `UpdateArticle` handler**:

```go
func (h *Handler) UpdateArticle(w http.ResponseWriter, r *http.Request) {
    callerID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    var req UpdateArticleRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    u := domain.UpdateArticle{
        Title:       req.Article.Title,
        Description: req.Article.Description,
        Body:        req.Article.Body,
    }

    w.Header().Set("Content-Type", "application/json")

    article, err := h.articleService.UpdateArticle(r.Context(), callerID, slug, &u)
    if err != nil {
        var validationErr *domain.ValidationError
        var notFoundErr *domain.ArticleNotFoundError
        var dupErr *domain.DuplicateError
        var credErr *domain.CredentialsError
        if errors.As(err, &validationErr) {
            w.WriteHeader(http.StatusUnprocessableEntity)
            _, _ = w.Write(createErrResponse(validationErr.Field, validationErr.Errors))
        } else if errors.As(err, &notFoundErr) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("article", []string{"not found"}))
        } else if errors.As(err, &dupErr) {
            w.WriteHeader(http.StatusConflict)
            _, _ = w.Write(createErrResponse(dupErr.Field, []string{dupErr.Msg}))
        } else if errors.As(err, &credErr) {
            w.WriteHeader(http.StatusUnauthorized)
            _, _ = w.Write(createErrResponse("credentials", []string{"invalid"}))
        } else {
            fmt.Println(err.Error())
            w.WriteHeader(http.StatusInternalServerError)
            _, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
        }
        return
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(articleResponse(article))
}
```

No new response DTOs needed — `articleResponse()` is reused.

### 5. HTTP server — `internal/adapters/in/webserver/server.go`

**Add `UpdateArticle` to `ServerHandlers` interface**:

```go
UpdateArticle(http.ResponseWriter, *http.Request)
```

**Register on the protected subrouter**:

```go
protected.HandleFunc("/api/articles/{slug}", h.UpdateArticle).Methods("PUT")
```

### 6. `cmd/server/server.go` — no changes required

### 7. Update `arch.md`

- Update `ArticleController` entry to include `UpdateArticle`.
- Update `articleRepo` interface entry to include `UpdateArticle`.
- Document `UpdateArticle` in the DB adapter section.
- Add `PUT /api/articles/{slug}` to the routes table.
- Note that `GenerateSlug` is now exported.
- Update Current State.

## Order of Implementation

1. `internal/domain/models.go` — add `UpdateArticle` struct
2. `internal/domain/article.go` — export `GenerateSlug`, add `validateUpdateArticle`, add `UpdateArticle` to interface and controller
3. `internal/adapters/out/db/postgres.go` — implement `UpdateArticle`
4. `internal/adapters/in/webserver/handlers.go` — add DTOs, extend interface, add handler
5. `internal/adapters/in/webserver/server.go` — add to interface, register route
6. `make lint` — iterate until clean
7. `arch.md` — update
