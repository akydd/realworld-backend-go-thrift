# Plan: JWT Tokens Feature

## Overview

Replace `user.Token = "fake token"` in `UserController.RegisterUser` with a real JWT token signed with a secret key loaded from the environment.

---

## New Dependency

A JWT library is required. The recommended choice is **`github.com/golang-jwt/jwt/v5`** — the standard, actively maintained JWT library for Go, with no transitive dependencies beyond the standard library.

```
go get github.com/golang-jwt/jwt/v5
```

This will update `go.mod` and `go.sum`.

---

## Changes

### 1. Environment config — `.env` and `.env_test`

Add a `JWT_SECRET` variable to both files. This is the HMAC signing key used to sign tokens.

```
JWT_SECRET=some-secret-key
```

Use distinct values in each file (production vs. test).

---

### 2. Domain layer — `internal/domain/user.go`

**Goal**: `UserController` generates a signed JWT instead of the hardcoded string.

#### Add `jwtSecret` field to `UserController`

```go
type UserController struct {
    repo      userRepo
    jwtSecret string
}
```

#### Update `New()` constructor

```go
func New(r userRepo, jwtSecret string) *UserController {
    return &UserController{
        repo:      r,
        jwtSecret: jwtSecret,
    }
}
```

#### Replace fake token in `RegisterUser`

After `InsertUser` succeeds, call a helper to generate the token:

```go
token, err := generateToken(user.Username, c.jwtSecret)
if err != nil {
    return nil, err
}
user.Token = token
```

#### New `generateToken` helper (unexported)

Uses HS256 with a `sub` claim (username) and a 72-hour expiry:

```go
import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

func generateToken(username string, secret string) (string, error) {
    claims := jwt.RegisteredClaims{
        Subject:   username,
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}
```

---

### 3. Entry point — `cmd/server/server.go`

Read `JWT_SECRET` from the environment and pass it to `domain.New()`:

```go
userController := domain.New(database, os.Getenv("JWT_SECRET"))
```

---

## Summary of Files Changed

| File | Change |
|------|--------|
| `go.mod` / `go.sum` | Add `github.com/golang-jwt/jwt/v5` |
| `.env` | Add `JWT_SECRET=<value>` |
| `.env_test` | Add `JWT_SECRET=<value>` |
| `internal/domain/user.go` | Add `jwtSecret` to `UserController`; update `New()`; replace fake token with `generateToken()` |
| `cmd/server/server.go` | Pass `os.Getenv("JWT_SECRET")` to `domain.New()` |

No changes are needed to the HTTP adapter, DB adapter, or domain models.

---

## Design Notes

- **Algorithm**: HS256 (HMAC-SHA256). Symmetric, no key infrastructure needed, consistent with typical RealWorld implementations.
- **Claims**: `sub` (username) and `exp` (72 hours). Additional claims (e.g. `iat`) can be added later.
- **Secret source**: Environment variable, consistent with the existing config approach.
- **Error handling**: `generateToken` can fail (e.g. if the secret is empty), so the error is propagated back to the caller. The HTTP layer will return 500 for this case via the existing fallthrough handler.
