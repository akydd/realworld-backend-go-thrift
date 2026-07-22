# Plan: Follow / Unfollow User (Feature 8)

## Overview

Add two new **protected** endpoints:

- `POST   /api/profiles/{username}/follow`   — authenticated user follows the given profile
- `DELETE /api/profiles/{username}/follow`   — authenticated user unfollows the given profile

Both return the same `ProfileResponse` shape as `GET /api/profiles/{username}`.

As a side effect, `GET /api/profiles/{username}` must now return the **real** `following` value for an authenticated viewer instead of always `false`.

## Database schema (4NF analysis)

A new `follows` table records follow relationships:

```
follows(follower_id FK → users.id, followee_id FK → users.id)
PK: (follower_id, followee_id)
```

There are no non-key attributes, so the table is trivially in 4NF.

## Response shapes

**Success (200):**
```json
{
  "profile": {
    "username": "jake",
    "bio": "I work at statefarm",
    "image": "https://...",
    "following": true
  }
}
```

**Profile not found (404):**
```json
{"errors": {"profile": ["not found"]}}
```

## Changes required

### 1. New migration `internal/adapters/out/db/migrations/003_create_follows.sql`

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS follows (
    follower_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (follower_id, followee_id)
);

-- +goose Down
DROP TABLE IF EXISTS follows;
```

### 2. `internal/domain/profile.go`

Update the `profileRepo` interface to accept a viewer ID and add follow/unfollow methods:

```go
type profileRepo interface {
    GetProfileByUsername(ctx context.Context, profileUsername string, viewerID int) (*Profile, error)
    FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
    UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
}
```

Update `ProfileController`:

```go
func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string, viewerID int) (*Profile, error)
    // calls repo.GetProfileByUsername(ctx, profileUsername, viewerID)

func (c *ProfileController) FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
    // calls repo.FollowUser(ctx, followerID, followeeUsername)

func (c *ProfileController) UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
    // calls repo.UnfollowUser(ctx, followerID, followeeUsername)
```

### 3. `internal/adapters/out/db/postgres.go`

**Update `GetProfileByUsername`** to accept `viewerID int` and compute `following` via a LEFT JOIN:

```sql
SELECT u.username, u.bio, u.image,
    CASE WHEN f.follower_id IS NOT NULL THEN true ELSE false END AS following
FROM users u
LEFT JOIN follows f ON f.followee_id = u.id AND f.follower_id = $2
WHERE u.username = $1
```

Pass `viewerID = 0` when the viewer is unauthenticated — no row in `follows` will ever have `follower_id = 0`, so `following` will correctly be `false`.

Use a new local DB scan struct that includes a `Following bool` field with `db:"following"` tag.

**Add `FollowUser`**:
1. Look up followee ID by username — return `&domain.ProfileNotFoundError{}` on `sql.ErrNoRows`.
2. `INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING` (idempotent).
3. Call `GetProfileByUsername(ctx, followeeUsername, followerID)` to return the full profile (all fields including `bio` and `image`) with `following: true`.

**Add `UnfollowUser`**:
1. Look up followee ID by username — return `&domain.ProfileNotFoundError{}` on `sql.ErrNoRows`.
2. `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`.
3. Call `GetProfileByUsername(ctx, followeeUsername, followerID)` to return the full profile (all fields including `bio` and `image`) with `following: false`.

### 4. `internal/adapters/in/webserver/handlers.go`

**Update `profileService` interface:**

```go
type profileService interface {
    GetProfile(ctx context.Context, profileUsername string, viewerID int) (*domain.Profile, error)
    FollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
    UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*domain.Profile, error)
}
```

**Update `GetProfile` handler** to read the optional viewer ID from context (0 if unauthenticated):

```go
viewerID, _ := r.Context().Value(userIDKey).(int)
profile, err := h.profileService.GetProfile(r.Context(), profileUsername, viewerID)
```

**Add `FollowUser` handler** (protected route → `userIDKey` is always present):
- Read `followerID` from context via `userIDKey`.
- Read `followeeUsername` from `mux.Vars(r)["username"]`.
- Call `h.profileService.FollowUser(r.Context(), followerID, followeeUsername)`.
- On `*domain.ProfileNotFoundError`: 404.
- On success: 200 + `ProfileResponse`.

**Add `UnfollowUser` handler** (same structure, calls `UnfollowUser`).

### 5. `internal/adapters/in/webserver/server.go`

**Extend `ServerHandlers` interface:**

```go
FollowUser(http.ResponseWriter, *http.Request)
UnfollowUser(http.ResponseWriter, *http.Request)
```

**Register on the protected subrouter:**

```go
protected.HandleFunc("/api/profiles/{username}/follow", h.FollowUser).Methods("POST")
protected.HandleFunc("/api/profiles/{username}/follow", h.UnfollowUser).Methods("DELETE")
```

### 6. `compose.test.yaml`

Remove the named volume declaration. The volume was mounted at `/var/lib/postgres`, but PostgreSQL stores its data at `$PGDATA=/var/lib/postgresql/18/docker`. The mount path was wrong so the volume never captured any DB data — all state (including `goose_db_version`) lived in the container's ephemeral writable layer and was already destroyed on `docker compose down`. Removing the volume makes this ephemerality explicit.

The `TRUNCATE TABLE users CASCADE` in the Makefile teardown must use `CASCADE` because the new `follows` table has a foreign key referencing `users`.

### 7. `compose.yaml`

The production DB has the same wrong volume mount path (`/var/lib/postgres`). Fix it so data persists between container restarts by pinning `PGDATA` to a known, version-independent path and mounting the volume there:

```yaml
environment:
  PGDATA: /var/lib/postgresql/data
volumes:
  - app-data:/var/lib/postgresql/data
```

Setting `PGDATA` explicitly makes the mount path stable across Postgres image upgrades (the version-specific default `/var/lib/postgresql/18/docker` would break on a major version bump).

### 8. `arch.md`

- Add the two new routes to the routes table.
- Document `FollowUser`/`UnfollowUser` on `ProfileController` and `profileRepo`.
- Add `follows` table to the DB schema section.
- Update Current State.

## Order of implementation

1. Add migration `003_create_follows.sql`.
2. Update `internal/domain/profile.go`: extend `profileRepo` interface and `ProfileController`.
3. Update `internal/adapters/out/db/postgres.go`: update `GetProfileByUsername` (new signature + LEFT JOIN), add `FollowUser` and `UnfollowUser`.
4. Update `internal/adapters/in/webserver/handlers.go`: update `profileService` interface, update `GetProfile` handler, add `FollowUser` and `UnfollowUser` handlers.
5. Update `internal/adapters/in/webserver/server.go`: extend `ServerHandlers`, register new routes.
6. Fix `compose.test.yaml`: remove the named volume; update `Makefile` teardown to `TRUNCATE TABLE users CASCADE`.
7. Fix `compose.yaml`: set `PGDATA=/var/lib/postgresql/data` and mount the volume at that path.
8. Run `make lint` and fix any errors.
9. Update `arch.md`.
