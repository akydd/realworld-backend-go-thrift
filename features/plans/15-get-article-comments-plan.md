# Plan: Feature 15 — Get Article Comments

## Overview

Implement `GET /api/articles/{slug}/comments`. Authentication is optional. Returns all comments for the article, each with an `author.following` value that reflects the viewer's follow status. Returns 404 if the slug doesn't match any article.

## Steps

### 1. Domain — `internal/domain/comment.go`

**Extend `commentRepo` interface:**
```go
GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error)
```

**Add `GetComments` method to `CommentController`:**
```go
func (c *CommentController) GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error) {
    return c.repo.GetCommentsByArticleSlug(ctx, articleSlug, viewerID)
}
```

No new domain models are needed — returns `[]*domain.Comment`.

### 2. DB Adapter — `internal/adapters/out/db/postgres.go`

**Add `GetCommentsByArticleSlug` method:**

Two-step approach:
1. Check article existence: `SELECT id FROM articles WHERE slug = $1`. If `sql.ErrNoRows`, return `&domain.ArticleNotFoundError{}`.
2. Query comments with author and viewer's following status:

```sql
SELECT
    c.id, c.body, c.created_at, c.updated_at,
    u.username, u.bio, u.image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
FROM comments c
JOIN users u ON u.id = c.author_id
LEFT JOIN follows f ON f.followee_id = c.author_id AND f.follower_id = $2
WHERE c.article_id = $1
ORDER BY c.created_at ASC
```

Use `db.SelectContext` into a slice of a local row struct. Map each row to `*domain.Comment`. Return `[]*domain.Comment{}` (never nil) when there are no comments.

The row struct:
```go
struct {
    ID        int            `db:"id"`
    Body      string         `db:"body"`
    CreatedAt time.Time      `db:"created_at"`
    UpdatedAt time.Time      `db:"updated_at"`
    Username  string         `db:"username"`
    Bio       sql.NullString `db:"bio"`
    Image     sql.NullString `db:"image"`
    Following bool           `db:"following"`
}
```

### 3. HTTP Handler — `internal/adapters/in/webserver/handlers.go`

**Extend `commentService` interface:**
```go
GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error)
```

**Add `CommentsResponse` DTO:**
```go
type CommentsResponse struct {
    Comments []CommentResponseInner `json:"comments"`
}
```

**Add `GetArticleComments` handler:**
- Extract `slug` from path vars.
- Extract `viewerID` from context (0 if unauthenticated, using the `_, _ = value.(type)` optional pattern already used in `GetArticle`/`GetProfile`).
- Call `h.commentService.GetComments(r.Context(), slug, viewerID)`.
- On `ArticleNotFoundError` → 404. On other error → 500.
- On success → 200 with `CommentsResponse`.

### 4. Router — `internal/adapters/in/webserver/server.go`

**Add `GetArticleComments` to `ServerHandlers` interface.**

**Register route on the `optionalAuth` subrouter:**
```go
optionalAuth.HandleFunc("/api/articles/{slug}/comments", h.GetArticleComments).Methods("GET")
```

### 5. Update `arch.md`

- Add `GetCommentsByArticleSlug` to the `commentRepo` description and `CommentController` methods.
- Add `GetCommentsByArticleSlug` to the DB adapter descriptions.
- Add the route `GET /api/articles/{slug}/comments` to the routes table.
- Update the Current State section.
