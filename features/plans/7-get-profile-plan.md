# Plan: Get Profile (Feature 7)

## Overview

Implement `GET /api/profiles/{username}` — returns a user's public profile. Authentication is **optional**: an authenticated viewer's identity is available to the domain for future use (e.g. determining `following` status), but is unused for now. `following` is always `false` in this iteration.

Note: the feature spec writes the path as `/api/profile/:username` but the integration tests use `/api/profiles/{username}` (plural). The plural form is correct.

## Response shapes

**Success (200):**
```json
{
  "profile": {
    "username": "jake",
    "bio": "I work at statefarm",
    "image": "https://...",
    "following": false
  }
}
```

**Not found (404):**
```json
{"errors": {"profile": ["not found"]}}
```

## Design

### Optional auth middleware

A second middleware, `optionalAuthMiddleware(jwtSecret)`, is needed for routes where a token may or may not be present. It attempts JWT validation when the `Authorization` header is present, silently ignoring failures, and stores the authenticated username in context if successful. If the header is absent or the token is invalid, the request proceeds unauthenticated (no username in context, no error response).

```
request arrives
│
├── Authorization header absent or missing "Token " prefix?
│     └── call next (no username in context)
│
├── JWT invalid?
│     └── call next (no username in context — ignore silently)
│
└── JWT valid → store username in context, call next
```

Routes using `optionalAuthMiddleware` form a third subrouter alongside the existing public and protected groups.

### New `ProfileController`

A separate `ProfileController` (new file `internal/domain/profile.go`) keeps profile logic isolated from user account logic, in line with the hexagonal architecture. It depends on a `profileRepo` interface.

The `GetProfile` method accepts both the target `profileUsername` (from the URL) and the `viewerUsername` (from context, empty string if unauthenticated). The viewer identity is plumbed through now so future follow/unfollow logic can use it without changing the signature.

### Handler wiring

The existing `Handler` struct gains a `profileService` field. `NewHandler` is updated to accept a `profileService`. In `cmd/server/server.go`, a `ProfileController` is constructed and passed alongside the `UserController`.

## Changes required

### 1. `internal/domain/errors.go`

Add `ProfileNotFoundError`:
```go
type ProfileNotFoundError struct{}
func (p *ProfileNotFoundError) Error() string { return "profile not found" }
```

### 2. `internal/domain/models.go`

Add `Profile` struct:
```go
type Profile struct {
    Username  string
    Bio       *string
    Image     *string
    Following bool
}
```

### 3. New file `internal/domain/profile.go`

```go
type profileRepo interface {
    GetProfileByUsername(ctx context.Context, username string) (*Profile, error)
}

type ProfileController struct {
    repo profileRepo
}

func NewProfileController(r profileRepo) *ProfileController

func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string, viewerUsername string) (*Profile, error)
    // calls repo.GetProfileByUsername; propagates errors (incl. ProfileNotFoundError)
    // viewerUsername is unused for now but accepted for future follow support
```

### 4. `internal/adapters/out/db/postgres.go`

Add `GetProfileByUsername(ctx, username string) (*domain.Profile, error)`:
- Query: `SELECT username, bio, image FROM users WHERE username = $1`
- Scan into a new local struct; convert Bio/Image from `sql.NullString`; set `Following: false`.
- Return `*domain.ProfileNotFoundError` on `sql.ErrNoRows`.

### 5. `internal/adapters/in/webserver/middleware.go`

Add `optionalAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler`:
- If `Authorization` header is absent or lacks `"Token "` prefix: call `next` immediately.
- Parse and validate the JWT. If valid, store the `sub` claim in context under `usernameKey`. If invalid, call `next` without storing anything.
- No error responses are ever written by this middleware.

### 6. `internal/adapters/in/webserver/handlers.go`

**Add `profileService` interface:**
```go
type profileService interface {
    GetProfile(ctx context.Context, profileUsername string, viewerUsername string) (*domain.Profile, error)
}
```

**Update `Handler` struct and `NewHandler`:**
```go
type Handler struct {
    service        userService
    profileService profileService
}

func NewHandler(s userService, ps profileService) *Handler
```

**Add DTOs:**
```go
type ProfileResponseInner struct {
    Username  string  `json:"username"`
    Bio       *string `json:"bio"`
    Image     *string `json:"image"`
    Following bool    `json:"following"`
}

type ProfileResponse struct {
    Profile ProfileResponseInner `json:"profile"`
}
```

**Add `GetProfile` handler:**
- Read `profileUsername` from path via `mux.Vars(r)["username"]`.
- Read `viewerUsername` from context: `r.Context().Value(usernameKey).(string)` — empty string if unauthenticated (safe type assertion with `, ok` idiom).
- Set `Content-Type: application/json`.
- Call `h.profileService.GetProfile(r.Context(), profileUsername, viewerUsername)`.
- On `*domain.ProfileNotFoundError`: 404 `{"errors": {"profile": ["not found"]}}`.
- On success: 200 + `ProfileResponse`.

### 7. `internal/adapters/in/webserver/server.go`

**Extend `ServerHandlers` interface:**
```go
GetProfile(http.ResponseWriter, *http.Request)
```

**Add optional-auth subrouter:**
```go
optionalAuth := r.NewRoute().Subrouter()
optionalAuth.Use(optionalAuthMiddleware(jwtSecret))
optionalAuth.HandleFunc("/api/profiles/{username}", h.GetProfile).Methods("GET")
```

### 8. `cmd/server/server.go`

```go
profileController := domain.NewProfileController(database)
handlers := webserver.NewHandler(userController, profileController)
```

### 9. `arch.md`

- Add `GET /api/profiles/{username}` to the routes table.
- Document `ProfileController`, `profileRepo`, `optionalAuthMiddleware`, and `ProfileNotFoundError`.
- Update Current State.

## Order of implementation

1. Add `ProfileNotFoundError` to `internal/domain/errors.go`.
2. Add `Profile` to `internal/domain/models.go`.
3. Create `internal/domain/profile.go` with `ProfileController`.
4. Add `GetProfileByUsername` to the Postgres adapter.
5. Add `optionalAuthMiddleware` to `middleware.go`.
6. Update `handlers.go`: add `profileService` interface, update `Handler`/`NewHandler`, add DTOs and `GetProfile` handler.
7. Update `server.go`: extend `ServerHandlers`, add optional-auth subrouter.
8. Update `cmd/server/server.go`: wire `ProfileController`.
9. Run `make lint` and fix any errors.
10. Update `arch.md`.
