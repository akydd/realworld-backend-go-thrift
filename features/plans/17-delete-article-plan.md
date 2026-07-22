# Plan: Feature 17 — Delete Article

## Overview

Implement `DELETE /api/articles/{slug}` (auth required). Validates article existence and author ownership before deleting. Cascade deletes of related data (`article_tags`, `article_favorites`, `comments`) are already handled by the existing `ON DELETE CASCADE` constraints in the schema. Returns 204 on success.

## Steps

### 1. Domain — `internal/domain/article.go`

**Extend `articleRepo` interface:**
```go
DeleteArticle(ctx context.Context, callerID int, slug string) error
```

**Add `DeleteArticle` method to `ArticleController`:**
```go
func (a *ArticleController) DeleteArticle(ctx context.Context, callerID int, slug string) error {
    return a.repo.DeleteArticle(ctx, callerID, slug)
}
```

No domain-level validation needed — all checks are persistence concerns.

### 2. DB Adapter — `internal/adapters/out/db/postgres.go`

**Add `DeleteArticle` method:**

1. Fetch article author:
   ```sql
   SELECT author_id FROM articles WHERE slug = $1
   ```
   → `&domain.ArticleNotFoundError{}` if `sql.ErrNoRows`.

2. Check ownership: if `authorID != callerID` → `&domain.CredentialsError{}`.

3. Delete:
   ```sql
   DELETE FROM articles WHERE slug = $1
   ```
   Cascade constraints handle related rows in `article_tags`, `article_favorites`, and `comments`.

### 3. HTTP Handler — `internal/adapters/in/webserver/handlers.go`

**Extend `articleService` interface:**
```go
DeleteArticle(ctx context.Context, callerID int, slug string) error
```

**Add `DeleteArticle` handler:**
- Extract `slug` from path vars.
- Extract `callerID` from context (protected route).
- Call `h.articleService.DeleteArticle(r.Context(), callerID, slug)`.
- Error mapping:
  - `ArticleNotFoundError` → 404, `{"errors": {"article": ["not found"]}}`
  - `CredentialsError` → 401, `{"errors": {"credentials": ["invalid"]}}`
  - other → 500
- Success → 204, no body.

### 4. Router — `internal/adapters/in/webserver/server.go`

**Add `DeleteArticle` to `ServerHandlers` interface.**

**Register route on the `protected` subrouter:**
```go
protected.HandleFunc("/api/articles/{slug}", h.DeleteArticle).Methods("DELETE")
```

### 5. Update `arch.md`

- Add `DeleteArticle` to `articleRepo` interface and `ArticleController` descriptions.
- Add `DeleteArticle` to the DB adapter descriptions.
- Add `DELETE /api/articles/{slug}` to the routes table.
- Update Current State section.
