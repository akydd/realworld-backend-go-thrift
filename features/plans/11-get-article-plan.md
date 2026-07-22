# Plan: Get Article (Feature 11)

## Overview

Add one new optionally-authenticated endpoint:

- `GET /api/articles/{slug}` — returns a single article by slug

Returns `200` with full article JSON including real `tagList`, `favorited: false`, `favoritesCount: 0`, and `author.following` computed from the viewer's follow relationship. Returns `404` when the slug does not match any article.

## Step-by-Step Changes

### 1. Domain — `internal/domain/article.go`

Add one method to the `articleRepo` interface:

```go
GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error)
```

Add one method to `ArticleController` (no validation logic — delegates directly):

```go
func (c *ArticleController) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*Article, error) {
    return c.repo.GetArticleBySlug(ctx, slug, viewerID)
}
```

The `viewerID=0` convention for unauthenticated callers is already established by `GetProfileByUsername`.

### 2. DB adapter — `internal/adapters/out/db/postgres.go`

Add a new scan struct:

```go
type articleWithTagsRow struct {
    Slug           string         `db:"slug"`
    Title          string         `db:"title"`
    Description    string         `db:"description"`
    Body           string         `db:"body"`
    CreatedAt      time.Time      `db:"created_at"`
    UpdatedAt      time.Time      `db:"updated_at"`
    AuthorUsername string         `db:"author_username"`
    AuthorBio      sql.NullString `db:"author_bio"`
    AuthorImage    sql.NullString `db:"author_image"`
    Following      bool           `db:"following"`
    TagList        pq.StringArray `db:"tag_list"`
}
```

Implement `GetArticleBySlug` with a single query:

```go
func (p *Postgres) GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error) {
    query := `
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
            COALESCE(ARRAY_AGG(t.name ORDER BY t.name) FILTER (WHERE t.name IS NOT NULL), '{}') AS tag_list
        FROM articles a
        JOIN users u ON u.id = a.author_id
        LEFT JOIN follows f ON f.followee_id = a.author_id AND f.follower_id = $2
        LEFT JOIN article_tags at ON at.article_id = a.id
        LEFT JOIN tags t ON t.id = at.tag_id
        WHERE a.slug = $1
        GROUP BY a.id, u.username, u.bio, u.image, f.follower_id`

    var row articleWithTagsRow
    err := p.db.QueryRowxContext(ctx, query, slug, viewerID).StructScan(&row)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, &domain.ArticleNotFoundError{}
        }
        return nil, err
    }

    author := domain.Profile{
        Username:  row.AuthorUsername,
        Following: row.Following,
    }
    if row.AuthorBio.Valid {
        s := row.AuthorBio.String
        author.Bio = &s
    }
    if row.AuthorImage.Valid {
        s := row.AuthorImage.String
        author.Image = &s
    }

    tagList := []string(row.TagList)
    if tagList == nil {
        tagList = []string{}
    }

    return &domain.Article{
        Slug:           row.Slug,
        Title:          row.Title,
        Description:    row.Description,
        Body:           row.Body,
        TagList:        tagList,
        CreatedAt:      row.CreatedAt,
        UpdatedAt:      row.UpdatedAt,
        Favorited:      false,
        FavoritesCount: 0,
        Author:         author,
    }, nil
}
```

Key notes on the SQL:
- `JOIN users` (inner) — safe because `author_id` is `NOT NULL` in the schema.
- `LEFT JOIN follows` with `f.follower_id = $2` — when `viewerID=0`, no follows row matches (user IDs start at 1), so `following` is always `false` for unauthenticated requests.
- `LEFT JOIN article_tags` + `LEFT JOIN tags` — articles with no tags still return a row.
- `ARRAY_AGG(...) FILTER (WHERE t.name IS NOT NULL)` excludes the NULL produced by the LEFT JOIN when there are no tags. `COALESCE(..., '{}')` ensures an empty array rather than NULL.
- `GROUP BY a.id, u.username, u.bio, u.image, f.follower_id` — `a.id` anchors the group; the user and follow columns are listed explicitly as PostgreSQL requires.
- `pq.StringArray` (from `github.com/lib/pq`, already imported) handles PostgreSQL array scanning.

### 3. HTTP handler — `internal/adapters/in/webserver/handlers.go`

Extend the `articleService` interface:

```go
type articleService interface {
    CreateArticle(ctx context.Context, authorID int, a *domain.CreateArticle) (*domain.Article, error)
    GetArticleBySlug(ctx context.Context, slug string, viewerID int) (*domain.Article, error)
}
```

Add an error-writing helper (mirrors `writeProfileErr`):

```go
func writeArticleErr(w http.ResponseWriter, err error) {
    var notFoundErr *domain.ArticleNotFoundError
    if errors.As(err, &notFoundErr) {
        w.WriteHeader(http.StatusNotFound)
        _, _ = w.Write(createErrResponse("article", []string{"not found"}))
    } else {
        fmt.Println(err.Error())
        w.WriteHeader(http.StatusInternalServerError)
        _, _ = w.Write(createErrResponse("unknown_error", []string{err.Error()}))
    }
}
```

Add the `GetArticle` handler:

```go
func (h *Handler) GetArticle(w http.ResponseWriter, r *http.Request) {
    slug := mux.Vars(r)["slug"]
    viewerID, _ := r.Context().Value(userIDKey).(int)

    w.Header().Set("Content-Type", "application/json")

    article, err := h.articleService.GetArticleBySlug(r.Context(), slug, viewerID)
    if err != nil {
        writeArticleErr(w, err)
        return
    }

    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(articleResponse(article))
}
```

No new DTOs needed — `ArticleResponse`, `ArticleResponseInner`, `ArticleAuthor`, and `articleResponse()` are already shared with `CreateArticle`.

### 4. HTTP server — `internal/adapters/in/webserver/server.go`

Add `GetArticle` to the `ServerHandlers` interface:

```go
GetArticle(http.ResponseWriter, *http.Request)
```

Register the route on the `optionalAuth` subrouter (same subrouter as `GetProfile`):

```go
optionalAuth.HandleFunc("/api/articles/{slug}", h.GetArticle).Methods("GET")
```

### 5. Entry point — `cmd/server/server.go`

No changes required. `database` is already passed to `domain.NewArticleController(database)`, which will satisfy the expanded `articleRepo` interface automatically.

### 6. Update `arch.md`

- Add `GET /api/articles/{slug}` to the routes table (auth optional).
- Add `GetArticleBySlug(ctx, slug, viewerID)` to the `articleRepo` interface description and `ArticleController` entry.
- Document `GetArticleBySlug` in the DB adapter section (single-query JOIN approach, `viewerID=0` convention, `pq.StringArray` for tags).
- Update Current State.

## Order of Implementation

1. `internal/domain/article.go` — add `GetArticleBySlug` to interface and controller
2. `internal/adapters/out/db/postgres.go` — add `articleWithTagsRow` struct and `GetArticleBySlug`
3. `internal/adapters/in/webserver/handlers.go` — extend `articleService`, add `writeArticleErr` and `GetArticle`
4. `internal/adapters/in/webserver/server.go` — add to interface, register route on `optionalAuth`
5. `make lint` — iterate until clean
6. `arch.md` — update
