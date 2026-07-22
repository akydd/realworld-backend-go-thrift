# Plan: Feature 16 — Delete Comment

## Overview

Implement `DELETE /api/articles/{slug}/comments/{id}` (auth required). Validates article existence, comment existence + article membership, and author ownership before deleting. Returns 204 on success.

## Steps

### 1. New error type — `internal/domain/errors.go`

Add `CommentNotFoundError`:
```go
type CommentNotFoundError struct{}

func (c *CommentNotFoundError) Error() string {
    return "comment not found"
}
```

### 2. Domain — `internal/domain/comment.go`

**Extend `commentRepo` interface:**
```go
DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
```

**Add `DeleteComment` method to `CommentController`:**
```go
func (c *CommentController) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
    return c.repo.DeleteComment(ctx, callerID, articleSlug, commentID)
}
```

No domain-level validation needed — all checks are persistence concerns (existence and ownership).

### 3. DB Adapter — `internal/adapters/out/db/postgres.go`

**Add `DeleteComment` method** using a sequential approach:

1. Check article exists:
   ```sql
   SELECT id FROM articles WHERE slug = $1
   ```
   → `&domain.ArticleNotFoundError{}` if `sql.ErrNoRows`.

2. Check comment exists and belongs to the article:
   ```sql
   SELECT author_id FROM comments WHERE id = $2 AND article_id = $1
   ```
   → `&domain.CommentNotFoundError{}` if `sql.ErrNoRows`.

3. Check ownership: if `authorID != callerID` → `&domain.CredentialsError{}`.

4. Delete:
   ```sql
   DELETE FROM comments WHERE id = $1
   ```

### 4. HTTP Handler — `internal/adapters/in/webserver/handlers.go`

**Extend `commentService` interface:**
```go
DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
```

**Add `DeleteArticleComment` handler:**
- Extract `slug` from path vars.
- Extract `id` from path vars; parse as int (400 if not a valid integer).
- Extract `callerID` from context (protected route).
- Call `h.commentService.DeleteComment(r.Context(), callerID, slug, commentID)`.
- Error mapping:
  - `ArticleNotFoundError` → 404, `{"errors": {"article": ["not found"]}}`
  - `CommentNotFoundError` → 404, `{"errors": {"comment": ["not found"]}}`
  - `CredentialsError` → 401, `{"errors": {"credentials": ["invalid"]}}`
  - other → 500
- Success → 204, no body.

### 5. Router — `internal/adapters/in/webserver/server.go`

**Add `DeleteArticleComment` to `ServerHandlers` interface.**

**Register route on the `protected` subrouter:**
```go
protected.HandleFunc("/api/articles/{slug}/comments/{id}", h.DeleteArticleComment).Methods("DELETE")
```

### 6. Update `arch.md`

- Add `CommentNotFoundError` to domain error types.
- Add `DeleteComment` to `commentRepo` and `CommentController` descriptions.
- Add `DeleteComment` to the DB adapter descriptions.
- Add `DELETE /api/articles/{slug}/comments/{id}` to the routes table.
- Update Current State section.
