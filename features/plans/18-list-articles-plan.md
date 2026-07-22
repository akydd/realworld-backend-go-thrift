# Plan: Feature 18 — List Articles

## Overview

Implement `GET /api/articles` (auth optional). Supports filtering by `tag`, `author`, and `favorited` (all case-insensitive), plus `limit` (default 20) and `offset` (default 0). Returns articles ordered by most recent first. Response omits the `body` field of each article. Returns `{ "articles": [...], "articlesCount": N }`.

## Post-implementation corrections

Two issues discovered during integration testing:

### Issue 1 — `articlesCount` must be the total matching count, not the returned count

The test creates 2 articles, queries with `limit=1`, and expects `articlesCount=2`. The spec's "number of returned articles" is misleading — `articlesCount` must reflect the total count of all matching articles before `limit`/`offset` is applied.

**Fix:** Added `COUNT(*) OVER() AS total_count` to the SELECT. PostgreSQL evaluates window functions after `GROUP BY` but before `LIMIT`/`OFFSET`, so this gives the full pre-limit match count. Added `ArticleList { Articles []*Article, TotalCount int }` domain model; `articleRepo.ListArticles` and `ArticleController.ListArticles` now return `(*ArticleList, error)`. Handler uses `list.TotalCount` for `articlesCount`.

### Issue 2 — `PUT /api/articles/{slug}` must support `tagList` updates

The integration test for article listing also covers tag updates via `PUT /api/articles/{slug}`:
- `{"article": {"tagList": []}}` → 200, clears all tags
- `{"article": {"tagList": null}}` → 422
- `tagList` absent → existing tags preserved

**Fix:**
- Added `NullableStringSlice` DTO (with `Present`, `IsNull`, `Value` fields and custom `UnmarshalJSON`) to `handlers.go`.
- Added `TagList NullableStringSlice` to `UpdateArticleInner`.
- Handler rejects `null` tagList with 422 before calling the service; converts present non-null value to `*[]string`.
- Added `TagList *[]string` to `domain.UpdateArticle` (`nil` = preserve, non-nil = replace).
- Updated `validateUpdateArticle` to accept updates with only `tagList` (previously rejected all-nil as blank).
- `Postgres.UpdateArticle` transaction: if `u.TagList != nil`, deletes all existing `article_tags` for the article then re-inserts the new set (upsert tags, then link).

## Steps

### 1. Domain — `internal/domain/models.go`

Add `ListArticlesFilter` struct:
```go
type ListArticlesFilter struct {
    Tag       *string
    Author    *string
    Favorited *string
    Limit     int
    Offset    int
}
```

### 2. Domain — `internal/domain/article.go`

Also add `ArticleList` struct to `models.go` (see Issue 1 above):
```go
type ArticleList struct {
    Articles   []*Article
    TotalCount int
}
```

**Extend `articleRepo` interface:**
```go
ListArticles(ctx context.Context, filter ListArticlesFilter, viewerID int) (*ArticleList, error)
```

**Add `ListArticles` method to `ArticleController`:**
```go
func (c *ArticleController) ListArticles(ctx context.Context, filter ListArticlesFilter, viewerID int) (*ArticleList, error) {
    return c.repo.ListArticles(ctx, filter, viewerID)
}
```

### 3. DB Adapter — `internal/adapters/out/db/postgres.go`

**Add `ListArticles` method** using a dynamic query builder.

Base query (note: no `body` column selected, matching the spec):
```sql
SELECT
    a.slug,
    a.title,
    a.description,
    a.created_at,
    a.updated_at,
    u.username  AS author_username,
    u.bio       AS author_bio,
    u.image     AS author_image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following,
    COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list,
    CASE WHEN fav.user_id IS NOT NULL THEN true ELSE false END AS favorited,
    (SELECT COUNT(*) FROM article_favorites WHERE article_id = a.id) AS favorites_count,
    COUNT(*) OVER() AS total_count
FROM articles a
JOIN users u ON u.id = a.author_id
LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $1
LEFT JOIN article_tags at ON at.article_id = a.id
LEFT JOIN tags t ON t.id = at.tag_id
LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $1
```

Dynamic WHERE clauses accumulated into a `[]string` conditions slice and `[]any` args slice (starting with `$1`=viewerID, then appending filter args):

- `filter.Tag != nil`:
  ```sql
  EXISTS (SELECT 1 FROM article_tags at2 JOIN tags t2 ON t2.id = at2.tag_id
          WHERE at2.article_id = a.id AND lower(t2.name) = lower($N))
  ```
- `filter.Author != nil`:
  ```sql
  lower(u.username) = lower($N)
  ```
- `filter.Favorited != nil`:
  ```sql
  EXISTS (SELECT 1 FROM article_favorites af2 JOIN users u2 ON u2.id = af2.user_id
          WHERE af2.article_id = a.id AND lower(u2.username) = lower($N))
  ```

Append to query:
```sql
[WHERE <conditions joined by " AND ">]
GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id
ORDER BY a.created_at DESC
LIMIT $N OFFSET $N
```

Use a local row struct (same as `articleWithTagsRow` but without `Body`, and with `TotalCount int`) and `db.SelectContext`. Map to `*domain.ArticleList`; set `TotalCount` from the first row's `total_count` (0 when no rows). Return a non-nil `*ArticleList` with empty `Articles` slice when there are no results.

### 4. HTTP Handler — `internal/adapters/in/webserver/handlers.go`

**Extend `articleService` interface:**
```go
ListArticles(ctx context.Context, filter domain.ListArticlesFilter, viewerID int) (*domain.ArticleList, error)
```

**Add DTOs:**
```go
type ArticleListItemInner struct {
    Slug           string        `json:"slug"`
    Title          string        `json:"title"`
    Description    string        `json:"description"`
    TagList        []string      `json:"tagList"`
    CreatedAt      time.Time     `json:"createdAt"`
    UpdatedAt      time.Time     `json:"updatedAt"`
    Favorited      bool          `json:"favorited"`
    FavoritesCount int           `json:"favoritesCount"`
    Author         ArticleAuthor `json:"author"`
}

type ArticlesResponse struct {
    Articles      []ArticleListItemInner `json:"articles"`
    ArticlesCount int                    `json:"articlesCount"`
}
```

**Add `ListArticles` handler:**
- Extract `viewerID` from context (0 if unauthenticated).
- Parse query params:
  - `tag`, `author`, `favorited` — convert non-empty string to `*string`, else `nil`.
  - `limit` — `strconv.Atoi`, default 20 if absent or invalid.
  - `offset` — `strconv.Atoi`, default 0 if absent or invalid.
- Call `h.articleService.ListArticles(r.Context(), filter, viewerID)` → `*ArticleList`.
- On error → 500.
- On success → 200 with `ArticlesResponse`; use `list.TotalCount` for `articlesCount`.

### 5. Router — `internal/adapters/in/webserver/server.go`

**Add `ListArticles` to `ServerHandlers` interface.**

**Register route on the `optionalAuth` subrouter:**
```go
optionalAuth.HandleFunc("/api/articles", h.ListArticles).Methods("GET")
```

### 6. Update `arch.md`

- Add `ListArticlesFilter`, `ArticleList`, and updated `UpdateArticle` to domain models.
- Add `ListArticles` to `articleRepo` interface and `ArticleController` descriptions.
- Add `ListArticles` and updated `UpdateArticle` to DB adapter descriptions.
- Add `GET /api/articles` to routes table; note `PUT /api/articles/{slug}` tagList support.
- Update Current State section.
