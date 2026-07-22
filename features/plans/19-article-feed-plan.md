# Plan: Article Feed (Feature 19)

## Overview

Implement `GET /api/articles/feed` — a protected endpoint returning articles authored by users that
the authenticated user follows, ordered by most recent first. Supports `limit` (default 20) and
`offset` (default 0) query parameters. The `body` field is omitted from article results.

---

## SQL Sharing Analysis

`ListArticles` and `FeedArticles` share an identical base query (SELECT columns, FROM/JOINs, GROUP
BY, ORDER BY). The only difference is the WHERE conditions:

- `ListArticles` has optional conditions for tag, author, and favorited filters.
- `FeedArticles` has one required condition: `f.follower_id IS NOT NULL` (the viewer already LEFT
  JOINed as `f`, so this condition restricts results to articles whose author is followed by the
  authenticated user).

**Decision**: Extract a private helper `buildArticleListQuery` in `postgres.go` that constructs the
base query given a `viewerID`, a slice of condition strings, an args slice, and limit/offset values.
Both `ListArticles` and the new `FeedArticles` call this helper.

`GetArticleBySlug` is intentionally kept separate: it fetches a single row by slug, includes the
`body` column, and uses `QueryRowxContext`. The structural differences make sharing impractical.

---

## Steps

### 1. `internal/domain/models.go` — Add `ArticleFeedFilter`

Add a new filter struct for the feed endpoint, following the same pattern as `ListArticlesFilter`:

```go
type ArticleFeedFilter struct {
    Limit  int
    Offset int
}
```

### 2. `internal/domain/article.go` — Extend repo interface and controller

**In `articleRepo` interface**, add:
```go
FeedArticles(ctx context.Context, filter ArticleFeedFilter, viewerID int) (*ArticleList, error)
```

**Add controller method**:
```go
func (c *ArticleController) FeedArticles(ctx context.Context, filter ArticleFeedFilter, viewerID int) (*ArticleList, error) {
    return c.repo.FeedArticles(ctx, filter, viewerID)
}
```

### 3. `internal/adapters/out/db/postgres.go` — Shared query helper + `FeedArticles`

**Extract private helper** `buildArticleListQuery`:

```go
func buildArticleListQuery(viewerID int, conditions []string, args []any, limit, offset int) (string, []any) {
    // args always starts with viewerID as $1
    nextArg := func(v any) string {
        args = append(args, v)
        return fmt.Sprintf("$%d", len(args))
    }

    query := `
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
        LEFT JOIN article_favorites fav ON fav.article_id = a.id AND fav.user_id = $1`

    if len(conditions) > 0 {
        query += " WHERE " + strings.Join(conditions, " AND ")
    }

    limitArg := nextArg(limit)
    offsetArg := nextArg(offset)
    query += fmt.Sprintf(`
        GROUP BY a.id, u.username, u.bio, u.image, f.follower_id, fav.user_id
        ORDER BY a.created_at DESC
        LIMIT %s OFFSET %s`, limitArg, offsetArg)

    return query, args
}
```

**Refactor `ListArticles`** to use this helper (the dynamic condition-building logic stays in
`ListArticles`, it just delegates the query construction to `buildArticleListQuery`).

**Add `FeedArticles`**:

```go
func (p *Postgres) FeedArticles(ctx context.Context, filter domain.ArticleFeedFilter, viewerID int) (*domain.ArticleList, error) {
    args := []any{viewerID}
    conditions := []string{"f.follower_id IS NOT NULL"}
    query, args := buildArticleListQuery(viewerID, conditions, args, filter.Limit, filter.Offset)

    // scan using same listRow type and conversion logic as ListArticles
    ...
}
```

Note: the `listRow` type (currently declared inside `ListArticles`) must be moved to package-level
scope so it can be reused by both `ListArticles` and `FeedArticles`.

### 4. `internal/adapters/in/webserver/handlers.go` — Add `GetArticleFeed` handler

**Extend `articleService` interface**:
```go
FeedArticles(ctx context.Context, filter domain.ArticleFeedFilter, viewerID int) (*domain.ArticleList, error)
```

**Add handler** `GetArticleFeed`:
- Reads `userID` from context (required — this is a protected route).
- Parses `limit` (default 20) and `offset` (default 0) from query parameters.
- Calls `h.articleService.FeedArticles(...)`.
- Builds and returns an `ArticlesResponse` (same shape as `ListArticles` response, omitting `body`).

### 5. `internal/adapters/in/webserver/server.go` — Register route

**Add `GetArticleFeed` to `ServerHandlers` interface**:
```go
GetArticleFeed(http.ResponseWriter, *http.Request)
```

**Register in `protected` subrouter** (requires authentication):
```go
protected.HandleFunc("/api/articles/feed", h.GetArticleFeed).Methods("GET")
```

This must be registered **before** the `optionalAuth` subrouter's `/api/articles/{slug}` route so
that gorilla/mux prefers the literal path `/api/articles/feed` over the parametric `{slug}` pattern.
In practice, since gorilla/mux gives priority to more specific (literal) paths, and the feed route is
in `protected` which is created before `optionalAuth`, the ordering is correct.

---

## Bug Fixes (discovered during integration tests)

### Fix A: 403 Forbidden for unauthorized resource access

Tests expected HTTP 403 with a resource-specific `"forbidden"` error body when an authenticated user
tries to modify/delete another user's article or comment. The server was returning 401.

**Root cause**: `DeleteArticle`, `UpdateArticle`, and `DeleteComment` returned `*domain.CredentialsError`
for ownership failures, and handlers mapped that to 401. But 401 means "not authenticated"; 403 means
"authenticated but not permitted".

**Fix**:
- Add `ForbiddenError` to `internal/domain/errors.go`.
- Change `DeleteArticle`, `UpdateArticle` (ownership check), and `DeleteComment` (ownership check) in
  `postgres.go` to return `*domain.ForbiddenError` instead of `*domain.CredentialsError`.
- In `handlers.go`, replace `CredentialsError → 401` with `ForbiddenError → 403` in the
  `UpdateArticle`, `DeleteArticle`, and `DeleteArticleComment` handlers. Error body uses the
  resource-specific key: `{"errors":{"article":["forbidden"]}}` or `{"errors":{"comment":["forbidden"]}}`.

### Fix B: Duplicate article titles should be allowed (unique slugs)

Tests expected that two articles with the same title can both be created successfully, each receiving
a unique slug (e.g., `my-title` and `my-title-2`). The server was returning 409.

**Root cause**: The `articles` table had a `UNIQUE` constraint on `title`, and `InsertArticle` mapped
the resulting `23505` violation to a `DuplicateError`.

**Fix**:
- Add migration `008_allow_duplicate_article_titles.sql` to drop `articles_title_unique`.
- In `InsertArticle`: before the INSERT, loop with a SELECT EXISTS check to find the first available
  slug (`slug`, `slug-2`, `slug-3`, …). A retry-after-error approach does **not** work inside a
  Postgres transaction — the transaction is aborted after a constraint violation and all subsequent
  commands fail. The pre-check avoids that.
- In `UpdateArticle`: same pre-check loop when the title (and thus slug) changes, excluding the
  current article's own ID (`WHERE slug = $1 AND id != $2`) so an article can be updated without
  changing its title without triggering a false collision.
- Remove the `articles_title_unique` case from all `pq.Error` constraint switch statements.

---

## File Change Summary

| File | Change |
|------|--------|
| `internal/domain/models.go` | Add `ArticleFeedFilter` struct |
| `internal/domain/errors.go` | Add `ForbiddenError` type |
| `internal/domain/article.go` | Add `FeedArticles` to `articleRepo` interface and `ArticleController` |
| `internal/adapters/out/db/migrations/008_allow_duplicate_article_titles.sql` | Drop `articles_title_unique` constraint |
| `internal/adapters/out/db/postgres.go` | Extract `buildArticleListQuery` helper; refactor `ListArticles` to use it; add `FeedArticles`; promote `listRow` to package-level; fix slug uniqueness via pre-check loop in `InsertArticle` and `UpdateArticle`; return `ForbiddenError` from `DeleteArticle`, `UpdateArticle`, `DeleteComment` ownership checks |
| `internal/adapters/in/webserver/handlers.go` | Add `FeedArticles` to `articleService` interface; add `GetArticleFeed` handler; map `ForbiddenError` → 403 in `UpdateArticle`, `DeleteArticle`, `DeleteArticleComment` handlers |
| `internal/adapters/in/webserver/server.go` | Add `GetArticleFeed` to `ServerHandlers`; register `GET /api/articles/feed` in protected subrouter |
