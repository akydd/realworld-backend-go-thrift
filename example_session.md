# Example Session

This document captures a representative session implementing features 15–17 of the RealWorld backend, plus the tail end of feature 14. It shows the full planning → implementation → testing → PR workflow.

---

## Feature 14 Wrap-Up — Commit, Push, Create PR

**User:** Commit, push, create PR.

**Assistant:**

```
git add arch.md cmd/server/server.go internal/adapters/in/webserver/handlers.go \
  internal/adapters/in/webserver/server.go internal/adapters/out/db/postgres.go \
  internal/domain/models.go features/14-create-article-comment.md \
  features/plans/14-create-article-comment-plan.md \
  internal/adapters/out/db/migrations/007_create_comments.sql \
  internal/domain/comment.go

git commit -m "Implement POST /api/articles/{slug}/comments (create article comment)."

git push -u origin 14-create-article-comment

gh pr create --title "Implement POST /api/articles/{slug}/comments (create article comment)"
# → https://github.com/akydd/realworld-backend-go/pull/18
```

---

## Feature 15 — Get Article Comments

### Create Plan

**User:** Create a plan for @features/15-get-article-comments.md.

**Feature spec:**
- `GET /api/articles/:slug/comments` — auth optional
- Returns array of comments; 404 if article not found
- `author.following` reflects viewer's follow status when authenticated

**Assistant:** Created `features/plans/15-get-article-comments-plan.md`:

1. **Domain** — add `GetCommentsByArticleSlug` to `commentRepo` interface; add `GetComments` to `CommentController`.
2. **DB adapter** — check article existence first (→ `ArticleNotFoundError`), then query comments with author JOIN and viewer `follows` LEFT JOIN; ordered by `created_at ASC`.
3. **HTTP handler** — extend `commentService`, add `CommentsResponse` DTO, add `GetArticleComments` handler.
4. **Router** — register `GET /api/articles/{slug}/comments` on `optionalAuth` subrouter.
5. **`arch.md`** — updated last.

### Implement

**User:** Implement the plan.

**Changes made:**

`internal/domain/comment.go`:
```go
type commentRepo interface {
    InsertComment(ctx context.Context, authorID int, articleSlug string, c *CreateComment) (*Comment, error)
    GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error)
}

func (c *CommentController) GetComments(ctx context.Context, articleSlug string, viewerID int) ([]*Comment, error) {
    return c.repo.GetCommentsByArticleSlug(ctx, articleSlug, viewerID)
}
```

`internal/adapters/out/db/postgres.go`:
```go
func (p *Postgres) GetCommentsByArticleSlug(ctx context.Context, articleSlug string, viewerID int) ([]*domain.Comment, error) {
    var articleID int
    err := p.db.QueryRowxContext(ctx, `SELECT id FROM articles WHERE slug = $1`, articleSlug).Scan(&articleID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, &domain.ArticleNotFoundError{}
        }
        return nil, err
    }

    query := `
        SELECT
            c.id, c.body, c.created_at, c.updated_at,
            u.username, u.bio, u.image,
            CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
        FROM comments c
        JOIN users u ON u.id = c.author_id
        LEFT JOIN follows f ON f.followee_id = c.author_id AND f.follower_id = $2
        WHERE c.article_id = $1
        ORDER BY c.created_at ASC`

    var rows []struct {
        ID        int            `db:"id"`
        Body      string         `db:"body"`
        CreatedAt time.Time      `db:"created_at"`
        UpdatedAt time.Time      `db:"updated_at"`
        Username  string         `db:"username"`
        Bio       sql.NullString `db:"bio"`
        Image     sql.NullString `db:"image"`
        Following bool           `db:"following"`
    }

    if err := p.db.SelectContext(ctx, &rows, query, articleID, viewerID); err != nil {
        return nil, err
    }

    comments := make([]*domain.Comment, 0, len(rows))
    for _, row := range rows {
        author := domain.Profile{Username: row.Username, Following: row.Following}
        if row.Bio.Valid {
            s := row.Bio.String
            author.Bio = &s
        }
        if row.Image.Valid {
            s := row.Image.String
            author.Image = &s
        }
        comments = append(comments, &domain.Comment{
            ID:        row.ID,
            CreatedAt: row.CreatedAt,
            UpdatedAt: row.UpdatedAt,
            Body:      row.Body,
            Author:    author,
        })
    }
    return comments, nil
}
```

`internal/adapters/in/webserver/handlers.go` (new DTO and handler):
```go
type CommentsResponse struct {
    Comments []CommentResponseInner `json:"comments"`
}

func (h *Handler) GetArticleComments(w http.ResponseWriter, r *http.Request) {
    slug := mux.Vars(r)["slug"]
    viewerID, _ := r.Context().Value(userIDKey).(int)

    w.Header().Set("Content-Type", "application/json")

    comments, err := h.commentService.GetComments(r.Context(), slug, viewerID)
    if err != nil {
        var notFoundErr *domain.ArticleNotFoundError
        if errors.As(err, &notFoundErr) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("article", []string{"not found"}))
        } else {
            fmt.Println(err.Error())
            w.WriteHeader(http.StatusInternalServerError)
            _, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
        }
        return
    }

    resp := CommentsResponse{Comments: make([]CommentResponseInner, 0, len(comments))}
    for _, c := range comments {
        resp.Comments = append(resp.Comments, CommentResponseInner{
            ID:        c.ID,
            CreatedAt: c.CreatedAt,
            UpdatedAt: c.UpdatedAt,
            Body:      c.Body,
            Author: CommentAuthor{
                Username:  c.Author.Username,
                Bio:       c.Author.Bio,
                Image:     c.Author.Image,
                Following: c.Author.Following,
            },
        })
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(resp)
}
```

`internal/adapters/in/webserver/server.go`:
```go
optionalAuth.HandleFunc("/api/articles/{slug}/comments", h.GetArticleComments).Methods("GET")
```

`make lint` → 0 issues.

### Integration Tests

**User:** Run int-tests.

**Result:** `GET /api/articles/{slug}/comments` → 200 ✓. All other failures are pre-existing unimplemented endpoints (article listing, feed, etc.). No regressions.

### Commit, Push, PR

**User:** Commit, push, create PR.

```
git commit -m "Implement GET /api/articles/{slug}/comments (get article comments)."
git push
gh pr create → https://github.com/akydd/realworld-backend-go/pull/19
```

---

## Feature 16 — Delete Comment

### Create Plan

**User:** Create a plan for @features/16-delete-comment.md.

**Feature spec:**
- `DELETE /api/articles/:slug/comments/:id` — auth required
- 404 if article not found
- 404 if comment not found or doesn't belong to the article
- 401 if caller is not the comment author
- 204 on success

**Assistant:** Created `features/plans/16-delete-comment-plan.md`:

1. **`errors.go`** — add `CommentNotFoundError`.
2. **Domain** — add `DeleteComment` to `commentRepo` and `CommentController`.
3. **DB adapter** — check article exists (→ `ArticleNotFoundError`), check comment exists with matching `article_id` (→ `CommentNotFoundError`), check ownership (→ `CredentialsError`), then DELETE.
4. **HTTP handler** — extend `commentService`, add `DeleteArticleComment` (parses `{id}` as int).
5. **Router** — register `DELETE /api/articles/{slug}/comments/{id}` on `protected` subrouter.

### Implement

**User:** Implement the plan.

**Changes made:**

`internal/domain/errors.go`:
```go
type CommentNotFoundError struct{}

func (c *CommentNotFoundError) Error() string {
    return "comment not found"
}
```

`internal/domain/comment.go`:
```go
type commentRepo interface {
    InsertComment(...)
    GetCommentsByArticleSlug(...)
    DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error
}

func (c *CommentController) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
    return c.repo.DeleteComment(ctx, callerID, articleSlug, commentID)
}
```

`internal/adapters/out/db/postgres.go`:
```go
func (p *Postgres) DeleteComment(ctx context.Context, callerID int, articleSlug string, commentID int) error {
    var articleID int
    err := p.db.QueryRowxContext(ctx, `SELECT id FROM articles WHERE slug = $1`, articleSlug).Scan(&articleID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return &domain.ArticleNotFoundError{}
        }
        return err
    }

    var authorID int
    err = p.db.QueryRowxContext(ctx,
        `SELECT author_id FROM comments WHERE id = $1 AND article_id = $2`,
        commentID, articleID).Scan(&authorID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return &domain.CommentNotFoundError{}
        }
        return err
    }

    if authorID != callerID {
        return &domain.CredentialsError{}
    }

    _, err = p.db.ExecContext(ctx, `DELETE FROM comments WHERE id = $1`, commentID)
    return err
}
```

`internal/adapters/in/webserver/handlers.go`:
```go
func (h *Handler) DeleteArticleComment(w http.ResponseWriter, r *http.Request) {
    callerID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    commentID, err := strconv.Atoi(mux.Vars(r)["id"])
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        _, _ = w.Write(createErrResponse("id", []string{"must be an integer"}))
        return
    }

    w.Header().Set("Content-Type", "application/json")

    if err := h.commentService.DeleteComment(r.Context(), callerID, slug, commentID); err != nil {
        var notFoundArticle *domain.ArticleNotFoundError
        var notFoundComment *domain.CommentNotFoundError
        var credErr *domain.CredentialsError
        if errors.As(err, &notFoundArticle) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("article", []string{"not found"}))
        } else if errors.As(err, &notFoundComment) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("comment", []string{"not found"}))
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

    w.WriteHeader(http.StatusNoContent)
}
```

`internal/adapters/in/webserver/server.go`:
```go
protected.HandleFunc("/api/articles/{slug}/comments/{id}", h.DeleteArticleComment).Methods("DELETE")
```

`make lint` → 0 issues.

### Integration Tests

**User:** Run int-tests.

**Result:** `DELETE /api/articles/{slug}/comments/{id}` → 204 ✓. Comments test suite now passes create, get, and delete. No regressions.

### Commit, Push, PR

**User:** Commit, push, create PR.

```
git commit -m "Implement DELETE /api/articles/{slug}/comments/{id} (delete comment)."
git push
gh pr create → https://github.com/akydd/realworld-backend-go/pull/20
```

---

## Feature 17 — Delete Article

### Create Plan

**User:** Create a plan for @features/17-delete-article.md.

**Feature spec:**
- `DELETE /api/articles/:slug` — auth required
- 404 if article not found
- 401 if caller is not the author
- 204 on success; related data deleted via existing `ON DELETE CASCADE` constraints

**Assistant:** Created `features/plans/17-delete-article-plan.md`:

1. **Domain** — add `DeleteArticle(ctx, callerID, slug)` to `articleRepo` and `ArticleController`.
2. **DB adapter** — fetch `author_id` by slug (→ `ArticleNotFoundError`), check ownership (→ `CredentialsError`), then `DELETE FROM articles WHERE slug = $1`.
3. **HTTP handler** — extend `articleService`, add `DeleteArticle` handler.
4. **Router** — register `DELETE /api/articles/{slug}` on `protected` subrouter.

### Implement

**User:** Implement the plan.

**Changes made:**

`internal/domain/article.go`:
```go
type articleRepo interface {
    // ... existing methods ...
    DeleteArticle(ctx context.Context, callerID int, slug string) error
}

func (c *ArticleController) DeleteArticle(ctx context.Context, callerID int, slug string) error {
    return c.repo.DeleteArticle(ctx, callerID, slug)
}
```

`internal/adapters/out/db/postgres.go`:
```go
func (p *Postgres) DeleteArticle(ctx context.Context, callerID int, slug string) error {
    var authorID int
    err := p.db.QueryRowxContext(ctx, `SELECT author_id FROM articles WHERE slug = $1`, slug).Scan(&authorID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return &domain.ArticleNotFoundError{}
        }
        return err
    }

    if authorID != callerID {
        return &domain.CredentialsError{}
    }

    _, err = p.db.ExecContext(ctx, `DELETE FROM articles WHERE slug = $1`, slug)
    return err
}
```

`internal/adapters/in/webserver/handlers.go`:
```go
func (h *Handler) DeleteArticle(w http.ResponseWriter, r *http.Request) {
    callerID := r.Context().Value(userIDKey).(int)
    slug := mux.Vars(r)["slug"]

    w.Header().Set("Content-Type", "application/json")

    if err := h.articleService.DeleteArticle(r.Context(), callerID, slug); err != nil {
        var notFoundErr *domain.ArticleNotFoundError
        var credErr *domain.CredentialsError
        if errors.As(err, &notFoundErr) {
            w.WriteHeader(http.StatusNotFound)
            _, _ = w.Write(createErrResponse("article", []string{"not found"}))
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

    w.WriteHeader(http.StatusNoContent)
}
```

`internal/adapters/in/webserver/server.go`:
```go
protected.HandleFunc("/api/articles/{slug}", h.DeleteArticle).Methods("DELETE")
```

`make lint` → 0 issues.

### Integration Tests

**User:** Run int-tests.

**Result:** Succeeded files improved from 4 → **7 (53.8%)**. `comments.hurl`, `errors_comments.hurl`, and `tags.hurl` now all pass. Remaining failures are unimplemented endpoints (article listing, feed, pagination). No regressions.

### Commit, Push, PR

**User:** Commit, push, create PR.

```
git commit -m "Implement DELETE /api/articles/{slug} (delete article)."
git push
gh pr create → https://github.com/akydd/realworld-backend-go/pull/21
```
