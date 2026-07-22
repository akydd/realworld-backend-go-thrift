# Plan: Create Article (Feature 9)

## Overview

Add one new **protected** endpoint:

- `POST /api/articles` — authenticated user creates an article

Required fields: `title`, `description`, `body`. `tagList` is accepted in the request but not stored or returned for now (always responds with `[]`). `favorited` is always `false`, `favoritesCount` is always `0`.

The response includes an `author` object which is the `Profile` of the authenticated user (the creator). Since a user does not follow themselves, `following` in the author object is always `false`.

## Integration test scope note

`articles.hurl` and `errors_articles.hurl` both cover many endpoints beyond create (list, get, update, delete, feed, favorites). Both test files will continue to fail until those endpoints are implemented in future features.

## Database schema (4NF analysis)

New `articles` table:

```
articles(id PK, slug, title, description, body, author_id FK → users.id, created_at, updated_at)
```

All non-key attributes depend only on `id`. `slug` and `title` are unique (candidate keys). No non-trivial multi-valued dependencies. ✓ In 4NF.

The existing `TRUNCATE TABLE users CASCADE` teardown step will cascade to `articles` automatically via the FK — no Makefile change required.

## Response shapes

**Success (201):**
```json
{
  "article": {
    "slug": "how-to-train-your-dragon",
    "title": "How to train your dragon",
    "description": "Ever wonder how?",
    "body": "You have to believe",
    "tagList": [],
    "createdAt": "2016-02-18T03:22:56.637Z",
    "updatedAt": "2016-02-18T03:48:35.824Z",
    "favorited": false,
    "favoritesCount": 0,
    "author": {
      "username": "jake",
      "bio": "I work at statefarm",
      "image": "https://i.stack.imgur.com/xHWG8.jpg",
      "following": false
    }
  }
}
```

**Validation error (422):**
```json
{"errors": {"title": ["can't be blank"]}}
```

**Duplicate title (409):**
```json
{"errors": {"title": ["has already been taken"]}}
```

## Changes required

### 1. New migration `internal/adapters/out/db/migrations/004_create_articles.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS articles (
    id          SERIAL PRIMARY KEY,
    slug        VARCHAR(255) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    body        TEXT NOT NULL,
    author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE articles ADD CONSTRAINT articles_slug_unique UNIQUE (slug);
ALTER TABLE articles ADD CONSTRAINT articles_title_unique UNIQUE (title);

-- +goose Down
DROP TABLE IF EXISTS articles;
```

### 2. `internal/domain/errors.go`

Add `ArticleNotFoundError` (needed for future get/update/delete endpoints):

```go
type ArticleNotFoundError struct{}
func (a *ArticleNotFoundError) Error() string { return "article not found" }
```

### 3. `internal/domain/models.go`

Add `Article` and `CreateArticle`:

```go
type Article struct {
    Slug           string
    Title          string
    Description    string
    Body           string
    TagList        []string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    Favorited      bool
    FavoritesCount int
    Author         Profile
}

type CreateArticle struct {
    Title       string
    Description string
    Body        string
}
```

### 4. New file `internal/domain/article.go`

```go
type articleRepo interface {
    InsertArticle(ctx context.Context, authorID int, a *CreateArticle) (*Article, error)
}

type ArticleController struct {
    repo articleRepo
}

func NewArticleController(r articleRepo) *ArticleController

func (c *ArticleController) CreateArticle(ctx context.Context, authorID int, a *CreateArticle) (*Article, error)
    // 1. Validate: title, description, body must be non-empty (return ValidationError for first blank field)
    // 2. Generate slug from title
    // 3. Call repo.InsertArticle
```

**Slug generation** (package-level helper):

```go
var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(title string) string {
    lower := strings.ToLower(title)
    slug := nonAlphanumRe.ReplaceAllString(lower, "-")
    return strings.Trim(slug, "-")
}
```

**Validation** (package-level helper):

```go
func validateCreateArticle(a *CreateArticle) error {
    if a.Title == "" {
        return NewValidationError("title", blankFieldErrMsg)
    }
    if a.Description == "" {
        return NewValidationError("description", blankFieldErrMsg)
    }
    if a.Body == "" {
        return NewValidationError("body", blankFieldErrMsg)
    }
    return nil
}
```

### 5. `internal/adapters/out/db/postgres.go`

Add `InsertArticle`:

1. Insert the article and return the created row:
```sql
INSERT INTO articles (slug, title, description, body, author_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, slug, title, description, body, author_id, created_at, updated_at
```
Map PostgreSQL unique-violation errors (code `23505`) on `articles_title_unique` or `articles_slug_unique` to `*domain.DuplicateError{Field: "title"}`.

2. Fetch the author profile:
```sql
SELECT username, bio, image FROM users WHERE id = $1
```
Set `Following: false` (creator never follows themselves).

3. Return `*domain.Article` with `TagList: []string{}`, `Favorited: false`, `FavoritesCount: 0`, and the populated `Author`.

Use a local `articleRow` struct with `db` tags for scanning:

```go
type articleRow struct {
    ID          int            `db:"id"`
    Slug        string         `db:"slug"`
    Title       string         `db:"title"`
    Description string         `db:"description"`
    Body        string         `db:"body"`
    AuthorID    int            `db:"author_id"`
    CreatedAt   time.Time      `db:"created_at"`
    UpdatedAt   time.Time      `db:"updated_at"`
}
```

### 6. `internal/adapters/in/webserver/handlers.go`

**Add `articleService` interface:**

```go
type articleService interface {
    CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
}
```

**Update `Handler` struct and `NewHandler`:**

```go
type Handler struct {
    service        userService
    profileService profileService
    articleService articleService
}

func NewHandler(s userService, ps profileService, as articleService) *Handler
```

**Add DTOs:**

```go
type CreateArticleInner struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Body        string   `json:"body"`
    TagList     []string `json:"tagList"`
}

type CreateArticleRequest struct {
    Article CreateArticleInner `json:"article"`
}

type ArticleAuthor struct {
    Username  string  `json:"username"`
    Bio       *string `json:"bio"`
    Image     *string `json:"image"`
    Following bool    `json:"following"`
}

type ArticleResponseInner struct {
    Slug           string        `json:"slug"`
    Title          string        `json:"title"`
    Description    string        `json:"description"`
    Body           string        `json:"body"`
    TagList        []string      `json:"tagList"`
    CreatedAt      time.Time     `json:"createdAt"`
    UpdatedAt      time.Time     `json:"updatedAt"`
    Favorited      bool          `json:"favorited"`
    FavoritesCount int           `json:"favoritesCount"`
    Author         ArticleAuthor `json:"author"`
}

type ArticleResponse struct {
    Article ArticleResponseInner `json:"article"`
}
```

**Add `CreateArticle` handler:**

- Read `authorID` from context via `userIDKey`.
- Decode request body into `CreateArticleRequest`.
- Build `domain.CreateArticle` from request (ignoring `TagList`).
- Call `h.articleService.CreateArticle(r.Context(), authorID, &d)`.
- On `*domain.ValidationError`: 422 with field/message.
- On `*domain.DuplicateError`: 409 with field/message.
- On success: 201 + `ArticleResponse`.

**Add helper:**

```go
func articleResponse(a *domain.Article) ArticleResponse
```

### 7. `internal/adapters/in/webserver/server.go`

**Extend `ServerHandlers` interface:**

```go
CreateArticle(http.ResponseWriter, *http.Request)
```

**Register on the protected subrouter:**

```go
protected.HandleFunc("/api/articles", h.CreateArticle).Methods("POST")
```

### 8. `cmd/server/server.go`

```go
articleController := domain.NewArticleController(database)
handlers := webserver.NewHandler(userController, profileController, articleController)
```

### 9. `arch.md`

- Add `004_create_articles.sql` to project structure.
- Document `ArticleController`, `articleRepo`, `ArticleNotFoundError` in the domain section.
- Add `POST /api/articles` to the routes table.
- Document `InsertArticle` in the DB adapter section.
- Add `articles` table schema.
- Update Current State.

## Order of implementation

1. Add migration `004_create_articles.sql`.
2. Add `ArticleNotFoundError` to `internal/domain/errors.go`.
3. Add `Article` and `CreateArticle` to `internal/domain/models.go`.
4. Create `internal/domain/article.go` with `ArticleController`, `articleRepo`, `generateSlug`, and `validateCreateArticle`.
5. Add `InsertArticle` to `internal/adapters/out/db/postgres.go`.
6. Update `internal/adapters/in/webserver/handlers.go`: add `articleService` interface, update `Handler`/`NewHandler`, add DTOs and `CreateArticle` handler.
7. Update `internal/adapters/in/webserver/server.go`: extend `ServerHandlers`, register route.
8. Update `cmd/server/server.go`: wire `ArticleController`.
9. Run `make lint` and fix any errors.
10. Update `arch.md`.
