# Plan: Get User (Feature 4)

## Overview

Implement `GET /api/user` — returns the authenticated user's data when a valid JWT is provided in the `Authorization` header.

## Error responses

| Condition | Status | Body |
|-----------|--------|------|
| `Authorization` header missing | 401 | `{"errors": {"token": ["is missing"]}}` |
| Token invalid / user not found | 401 | `{"errors": {"credentials": ["invalid"]}}` |
| Success | 200 | `UserResponse` JSON |

## Changes required

### 1. `internal/domain/user.go`

**Extend `userRepo` interface:**
```go
GetUserByUsername(ctx context.Context, username string) (*User, error)
```

**Add `GetUser` method to `UserController`:**
- Accept the raw JWT string.
- Parse and verify the JWT using `jwtSecret` (HS256). On failure return `*CredentialsError`.
- Extract the `sub` claim (username). On failure return `*CredentialsError`.
- Call `repo.GetUserByUsername`. On failure propagate the error (repo returns `*CredentialsError` when no row found).
- Generate a fresh token via `generateToken`, set `user.Token`, return `*User`.

### 2. `internal/adapters/out/db/postgres.go`

**Add `GetUserByUsername` method to `Postgres`:**
- Query `SELECT username, email, bio, image FROM users WHERE username = $1`.
- Return `*domain.CredentialsError` when no row is found (`sql.ErrNoRows`).
- Scan into the existing `user` struct; convert and return via `convertUser`.

### 3. `internal/adapters/in/webserver/handlers.go`

**Extend `userService` interface:**
```go
GetUser(ctx context.Context, token string) (*domain.User, error)
```

**Add `GetUser` handler to `Handler`:**
- Read the `Authorization` header. If missing or empty, immediately return 401 with `{"errors": {"token": ["is missing"]}}`.
- Strip the `"Token "` prefix to extract the raw JWT. If the prefix is absent, return 401 with `{"errors": {"token": ["is missing"]}}`.
- Call `h.service.GetUser(r.Context(), rawToken)`.
- On `*domain.CredentialsError`: return 401 `{"errors": {"credentials": ["invalid"]}}`.
- On success: return 200 `UserResponse`.

### 4. `internal/adapters/in/webserver/server.go`

**Extend `ServerHandlers` interface:**
```go
GetUser(http.ResponseWriter, *http.Request)
```

**Register new route:**
```go
r.HandleFunc("/api/user", h.GetUser).Methods("GET")
```

### 5. `arch.md`

- Add `GET /api/user` to the routes table.
- Update Current State section to note that authenticated user retrieval is implemented.

## Order of implementation

1. Add `GetUserByUsername` to `userRepo` interface and `Postgres` adapter.
2. Add `GetUser` to `UserController` in the domain layer.
3. Add `GetUser` handler and extend `userService` interface in the webserver adapter.
4. Register the route in `server.go`.
5. Run `make lint` and fix any errors.
6. Update `arch.md`.
