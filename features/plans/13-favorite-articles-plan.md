# Plan: Favorite Articles (Feature 13)

## Overview

Two pieces of work:
1. Add `POST /api/articles/{slug}/favorite` and `DELETE /api/articles/{slug}/favorite` endpoints.
2. Make `favorited` and `favoritesCount` real computed values in **all** single-article responses (currently hardcoded to `false`/`0` in `GetArticleBySlug`).

`InsertArticle` may keep hardcoding `false`/`0` — a newly created article cannot have any favorites yet, so the values are always correct at creation time.

## Database Schema (4NF)

Add one table in migration `006_create_article_favorites.sql`:

- **`article_favorites`**: `user_id INTEGER FK→users.id ON DELETE CASCADE`, `article_id INTEGER FK→articles.id ON DELETE CASCADE`, composite PK `(user_id, article_id)`

Same pattern as `follows` and `article_tags` — a pure junction table, 4NF.

## Step-by-Step Changes

### 1. Migration — `internal/adapters/out/db/migrations/006_create_article_favorites.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS article_favorites (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, article_id)
);

-- +goose Down
DROP TABLE IF EXISTS article_favorites;
```

### 2. DB adapter — `internal/adapters/out/db/postgres.go`

**Update `articleWithTagsRow`** to add two new scan fields:

```go
type articleWithTagsRow struct {
    // ... existing fields ...
    Favorited      bool           `db:"favorited"`
    FavoritesCount int            `db:"favorites_count"`
}
```

**Update `GetArticleBySlug` query** to compute real `favorited` and `favorites_count`:

```sql
SELECT
    a.slug,
    a.title,
    a.description,
    a.body,
    a.created_at,
    a.updated_at,
    u.username  AS author_username,
    u.bio       AS author_bio,
    u.image     AS author_image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following,
    COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list,
    CASE WHEN fav.user_id IS NOT NULL THEN true ELSE false END AS favorited,
    (SELECT COUNT(*) FROM article_favorites WHERE article_id = a.id) AS favorites_count
FROM articles a
JOIN users u ON u.id = a.author_id
LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $2
LEFT JOIN article_tags at ON at.article_id = a.id
LEFT JOIN tags t ON t.id = at.tag_id
LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $2
WHERE a.slug = $1
GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id
```

Key notes:
- `LEFT JOIN article_favorites fav ... AND fav.user_id = $2` — viewer-specific; when `viewerID=0`, no row matches (user IDs start at 1) so `favorited` is always `false` for unauthenticated requests.
- The `favorites_count` uses a correlated subquery to avoid interference with the `ARRAY_AGG` grouping that collects tags.
- `fav.user_id` is added to `GROUP BY` since it appears in a non-aggregate `CASE` expression.

**Update `GetArticleBySlug` return value** to use the real fields:

```go
return &domain.Article{
    // ... existing fields ...
    Favorited:      row.Favorited,
    FavoritesCount: int(row.FavoritesCount),
    // ...
}, nil
```

**Add `FavoriteArticle`**:

```go
func (p *Postgres) FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
    _, err := p.db.ExecContext(ctx,
        `INSERT INTO article_favorites (user_id, article_id)
         SELECT $1, id FROM articles WHERE slug = $2
         ON CONFLICT DO NOTHING`,
        userID, slug)
    if err != nil {
        return nil, err
    }
    return p.GetArticleBySlug(ctx, slug, userID)
}
```

If the article doesn't exist, the INSERT inserts 0 rows (no error), and `GetArticleBySlug` returns `*domain.ArticleNotFoundError`. Favoriting an already-favorited article is idempotent (`ON CONFLICT DO NOTHING`).

**Add `UnfavoriteArticle`**:

```go
func (p *Postgres) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error) {
    _, err := p.db.ExecContext(ctx,
        `DELETE FROM article_favorites
         WHERE user_id = $1 AND article_id = (SELECT id FROM articles WHERE slug = $2)`,
        userID, slug)
    if err != nil {
        return nil, err
    }
    return p.GetArticleBySlug(ctx, slug, userID)
}
```

Unfavoriting an article that isn't favorited is idempotent (DELETE deletes 0 rows). If the article doesn't exist, `GetArticleBySlug` returns `*domain.ArticleNotFoundError`.

### 3. Domain — `internal/domain/article.go`

**Add `FavoriteArticle` and `UnfavoriteArticle` to the `articleRepo` interface**:

```go
type articleRepo interface {
    InsertArticle(ctx context.Context, authorID int, slug string, a *CreateArticle) (*Article, error)
    GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error)
    UpdateArticle(ctx context.Context, callerID int, slug string, u *UpdateArticle) (*Article, error)
    FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
    UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error)
}
```

**Add methods to `ArticleController`**:

```go
func (c *ArticleController) FavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
    return c.repo.FavoriteArticle(ctx, userID, slug)
}

func (c *ArticleController) UnfavoriteArticle(ctx context.Context, userID int, slug string) (*Article, error) {
    return c.repo.UnfavoriteArticle(ctx, userID, slug)
}
```

No domain validation needed — the repo handles all cases.

### 4. HTTP handler — `internal/adapters/in/webserver/handlers.go`

**Extend `articleService` interface**:

```go
type articleService interface {
    CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
    GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error)
    UpdateArticle(ctx context.Context, callerID int, slug string, u *domain.UpdateArticle) (*domain.Article, error)
    FavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
    UnfavoriteArticle(ctx context.Context, userID int, slug string) (*domain.Article, error)
}
```

**Add `FavoriteArticle` handler**:

```go
func (h *Handler) FavoriteArticle(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    w.Header().Set("Content-Type", "application/json")

    article, err := h.articleService.FavoriteArticle(r.Context(), userID, slug)
    if err != nil {
        writeArticleErr(w, err)
        return
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(articleResponse(article))
}
```

**Add `UnfavoriteArticle` handler**:

```go
func (h *Handler) UnfavoriteArticle(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    w.Header().Set("Content-Type", "application/json")

    article, err := h.articleService.UnfavoriteArticle(r.Context(), userID, slug)
    if err != nil {
        writeArticleErr(w, err)
        return
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(articleResponse(article))
}
```

Both handlers reuse `writeArticleErr` and `articleResponse` — no new DTOs needed.

### 5. HTTP server — `internal/adapters/in/webserver/server.go`

**Add to `ServerHandlers` interface**:

```go
FavoriteArticle(http.ResponseWriter, *http.Request)
UnfavoriteArticle(http.ResponseWriter, *http.Request)
```

**Register on the protected subrouter**:

```go
protected.HandleFunc("/api/articles/{slug}/favorite", h.FavoriteArticle).Methods("POST")
protected.HandleFunc("/api/articles/{slug}/favorite", h.UnfavoriteArticle).Methods("DELETE")
```

### 6. `cmd/server/server.go` — no changes required

### 7. Update `arch.md`

- Add `006_create_article_favorites.sql` to migration list.
- Add `article_favorites` schema table.
- Update `ArticleController` and `articleRepo` entries to include `FavoriteArticle` and `UnfavoriteArticle`.
- Update `GetArticleBySlug` DB description to mention real `favorited`/`favoritesCount`.
- Document `FavoriteArticle` and `UnfavoriteArticle` in the DB adapter section.
- Add the two new routes to the routes table.
- Update Current State.

## Order of Implementation

1. Migration `006_create_article_favorites.sql`
2. `internal/adapters/out/db/postgres.go` — update `articleWithTagsRow`, update `GetArticleBySlug` query, add `FavoriteArticle` and `UnfavoriteArticle`
3. `internal/domain/article.go` — add to `articleRepo` interface and `ArticleController`
4. `internal/adapters/in/webserver/handlers.go` — extend `articleService`, add handlers
5. `internal/adapters/in/webserver/server.go` — add to interface, register routes
6. `make lint` — iterate until clean
7. `arch.md` — update
