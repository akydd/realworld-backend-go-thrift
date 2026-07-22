# Plan: Tags (Feature 10)

## Overview

Two pieces of work:
1. Wire up `tagList` during article creation (tags are stored and returned).
2. Add `GET /api/tags` endpoint returning all tags.

## Database Schema (4NF)

Add two tables in migration `005_create_tags.sql`:

- **`tags`**: `id SERIAL PK`, `name VARCHAR(255) NOT NULL UNIQUE`
- **`article_tags`**: `article_id INTEGER FK→articles.id ON DELETE CASCADE`, `tag_id INTEGER FK→tags.id ON DELETE CASCADE`, composite PK `(article_id, tag_id)`

Both tables are in 4NF: every non-key attribute depends on the whole key and there are no multi-valued dependencies.

## Step-by-Step Changes

### 1. Migration — `internal/adapters/out/db/migrations/005_create_tags.sql`

Create the `tags` and `article_tags` tables.

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS tags (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
ALTER TABLE tags ADD CONSTRAINT tags_name_unique UNIQUE (name);

CREATE TABLE IF NOT EXISTS article_tags (
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS article_tags;
DROP TABLE IF EXISTS tags;
```

### 2. Domain model — `internal/domain/models.go`

Add `TagList []string` to `CreateArticle`:

```go
type CreateArticle struct {
    Title       string
    Description string
    Body        string
    TagList     []string
}
```

### 3. Article domain — `internal/domain/article.go`

Deduplicate `TagList` in `CreateArticle` before calling the repo. Deduplication preserves order (first occurrence wins):

```go
func deduplicateTags(tags []string) []string {
    seen := make(map[string]bool)
    result := make([]string, 0, len(tags))
    for _, t := range tags {
        if !seen[t] {
            seen[t] = true
            result = append(result, t)
        }
    }
    return result
}
```

Call `a.TagList = deduplicateTags(a.TagList)` inside `CreateArticle` before `repo.InsertArticle`.

### 4. Tag domain — `internal/domain/tag.go` (new file)

```go
package domain

import "context"

type tagRepo interface {
    GetAllTags(ctx context.Context) ([]string, error)
}

type TagController struct {
    repo tagRepo
}

func NewTagController(r tagRepo) *TagController {
    return &TagController{repo: r}
}

func (c *TagController) GetTags(ctx context.Context) ([]string, error) {
    return c.repo.GetAllTags(ctx)
}
```

### 5. DB adapter — `internal/adapters/out/db/postgres.go`

**Update `InsertArticle`**: wrap all queries in a single transaction so a failure at any step rolls everything back atomically — no orphaned article rows and no partially-linked tags.

```go
tx, err := p.db.BeginTxx(ctx, nil)
if err != nil {
    return nil, err
}
defer tx.Rollback()

// Insert article
var row articleRow
err = tx.QueryRowxContext(ctx, `
    INSERT INTO articles (slug, title, description, body, author_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, slug, title, description, body, author_id, created_at, updated_at`,
    slug, a.Title, a.Description, a.Body, authorID).StructScan(&row)
if err != nil {
    // map unique-violation errors as before
    ...
    return nil, err
}

// Upsert tags and link them
if len(a.TagList) > 0 {
    _, err = tx.ExecContext(ctx,
        `INSERT INTO tags (name) SELECT unnest($1::text[]) ON CONFLICT (name) DO NOTHING`,
        pq.Array(a.TagList))
    if err != nil {
        return nil, err
    }

    var tagIDs []int
    err = tx.SelectContext(ctx, &tagIDs,
        `SELECT id FROM tags WHERE name = ANY($1) ORDER BY array_position($1::text[], name)`,
        pq.Array(a.TagList))
    if err != nil {
        return nil, err
    }

    for _, tagID := range tagIDs {
        _, err = tx.ExecContext(ctx,
            `INSERT INTO article_tags (article_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
            row.ID, tagID)
        if err != nil {
            return nil, err
        }
    }
}

// Fetch author profile
var authorRow struct { ... }
err = tx.QueryRowxContext(ctx, `SELECT username, bio, image FROM users WHERE id = $1`, authorID).StructScan(&authorRow)
if err != nil {
    return nil, err
}

if err = tx.Commit(); err != nil {
    return nil, err
}
```

Return `TagList: a.TagList` (the deduplicated input) in the returned `*domain.Article`.

Note: `sqlx.Tx` exposes the same `QueryRowxContext`, `ExecContext`, and `SelectContext` methods as `sqlx.DB`, so all existing query calls are unchanged except they go through `tx` instead of `p.db`.

**Add `GetAllTags`**:

```go
func (p *Postgres) GetAllTags(ctx context.Context) ([]string, error) {
    var tags []string
    err := p.db.SelectContext(ctx, &tags, `SELECT name FROM tags ORDER BY name`)
    if err != nil {
        return nil, err
    }
    if tags == nil {
        tags = []string{}
    }
    return tags, nil
}
```

### 6. HTTP handler — `internal/adapters/in/webserver/handlers.go`

**Add `tagService` interface and field**:

```go
type tagService interface {
    GetTags(ctx context.Context) ([]string, error)
}

type Handler struct {
    service        userService
    profileService profileService
    articleService articleService
    tagService     tagService
}

func NewHandler(s userService, ps profileService, as articleService, ts tagService) *Handler {
    return &Handler{service: s, profileService: ps, articleService: as, tagService: ts}
}
```

**Update `CreateArticle` handler**: pass `req.Article.TagList` into `domain.CreateArticle`:

```go
d := domain.CreateArticle{
    Title:       req.Article.Title,
    Description: req.Article.Description,
    Body:        req.Article.Body,
    TagList:     req.Article.TagList,
}
```

**Add `GetTags` handler**:

```go
type TagsResponse struct {
    Tags []string `json:"tags"`
}

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    tags, err := h.tagService.GetTags(r.Context())
    if err != nil {
        fmt.Println(err.Error())
        w.WriteHeader(http.StatusInternalServerError)
        _, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
        return
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(TagsResponse{Tags: tags})
}
```

### 7. HTTP server — `internal/adapters/in/webserver/server.go`

Add `GetTags` to the `ServerHandlers` interface and register the route (no auth required):

```go
type ServerHandlers interface {
    // ... existing methods ...
    GetTags(http.ResponseWriter, *http.Request)
}
```

Register in `NewServer`:
```go
r.HandleFunc("/api/tags", h.GetTags).Methods("GET")
```

### 8. Entry point — `cmd/server/server.go`

Create `TagController` and pass it to `NewHandler`:

```go
tagController := domain.NewTagController(database)
handlers := webserver.NewHandler(userController, profileController, articleController, tagController)
```

### 9. Update `arch.md`

- Add `tag.go` to domain layer file listing.
- Document `TagController` and `tagRepo` interface.
- Add `GetAllTags` to the DB adapter section.
- Add the new tables to the schema section.
- Add `GET /api/tags` to the routes table.
- Update "Current State" note about `tagList` being stored.

## Order of Implementation

1. Migration file
2. `models.go` (add `TagList` to `CreateArticle`)
3. `article.go` (deduplication)
4. `tag.go` (new file)
5. `postgres.go` (update `InsertArticle`, add `GetAllTags`)
6. `handlers.go` (tagService interface, update CreateArticle handler, add GetTags handler)
7. `server.go` (add GetTags to interface + route)
8. `cmd/server/server.go` (wire TagController)
9. `arch.md` update
10. `make lint` — iterate until clean
