# Plan: Protected Routes (Feature 6)

## Overview

Extract the duplicated `Authorization` header parsing and JWT validation from `GetUser` and `UpdateUser` into a reusable middleware. Future protected routes will use the same middleware without any additional boilerplate.

## Current duplication

Both `GetUser` and `UpdateUser` in the domain layer contain identical JWT parsing and validation logic:

```go
// In handlers.go — duplicated header extraction
authHeader := r.Header.Get("Authorization")
w.Header().Set("Content-Type", "application/json")
const prefix = "Token "
if authHeader == "" || !strings.HasPrefix(authHeader, prefix) {
    w.WriteHeader(http.StatusUnauthorized)
    _, _ = w.Write(createErrResponse("token", []string{"is missing"}))
    return
}
rawToken := strings.TrimPrefix(authHeader, prefix)

// In domain/user.go — duplicated JWT validation
token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, &CredentialsError{}
    }
    return []byte(c.jwtSecret), nil
})
if err != nil || !token.Valid { return nil, &CredentialsError{} }
claims, ok := token.Claims.(*jwt.RegisteredClaims)
if !ok || claims.Subject == "" { return nil, &CredentialsError{} }
currentUsername := claims.Subject
```

## Design

The middleware consolidates both steps — header extraction **and** JWT validation — into a single place. After successful validation, the authenticated username (from the JWT `sub` claim) is stored in the request context. Protected handlers read the username from context and pass it directly to the domain service, which no longer needs to touch the token at all.

### Why move JWT validation into middleware

JWT validation is a cross-cutting concern of the HTTP transport layer: it determines whether a request is authenticated before it reaches business logic. Keeping it in the domain service couples the domain to a specific auth mechanism and forces each new protected domain method to repeat the same token-parsing boilerplate. Moving it to the middleware keeps the domain focused on business logic and makes authentication a single, testable unit.

### Context key

A package-private type is used for the context key to avoid collisions with other packages:

```go
type contextKey string
const usernameKey contextKey = "username"
```

### Middleware behaviour

```
request arrives
│
├── Authorization header missing or lacks "Token " prefix?
│     └── 401  {"errors": {"token": ["is missing"]}}
│
├── JWT invalid (bad signature, expired, wrong algorithm)?
│     └── 401  {"errors": {"credentials": ["invalid"]}}
│
└── Extract username from "sub" claim, store in context, call next handler
```

### Passing jwtSecret to the middleware

The middleware needs the JWT secret to validate signatures. It is implemented as a constructor that captures the secret in a closure:

```go
func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler { ... }
}
```

`NewServer` receives `jwtSecret` as a new parameter and passes it to `authMiddleware`. The call site in `cmd/server/server.go` is updated accordingly.

### Router setup

```go
protected := r.NewRoute().Subrouter()
protected.Use(authMiddleware(jwtSecret))
protected.HandleFunc("/api/user", h.GetUser).Methods("GET")
protected.HandleFunc("/api/user", h.UpdateUser).Methods("PUT")
```

Adding a future protected route requires only one line in the `protected` subrouter.

## Changes required

### 1. `internal/adapters/in/webserver/middleware.go`

- Define `contextKey` type and `usernameKey` constant.
- Implement `authMiddleware(jwtSecret string) func(http.Handler) http.Handler`:
  - Read the `Authorization` header; if absent or missing `"Token "` prefix → 401 `{"errors": {"token": ["is missing"]}}`.
  - Parse and validate the JWT using `jwtSecret` (HS256). If invalid → 401 `{"errors": {"credentials": ["invalid"]}}`.
  - Extract the `sub` claim; if empty → 401 credentials invalid.
  - Store the username in context and call `next.ServeHTTP`.

### 2. `internal/adapters/in/webserver/handlers.go`

- **`GetUser`**: read username from context with `r.Context().Value(usernameKey).(string)`; pass it to `h.service.GetUser(ctx, username)`.
- **`UpdateUser`**: same — read username from context, pass to `h.service.UpdateUser(ctx, username, &d)`.
- Update the `userService` interface: `GetUser` and `UpdateUser` now take `username string` instead of `token string`.

### 3. `internal/adapters/in/webserver/server.go`

- Add `jwtSecret string` parameter to `NewServer`.
- Apply `authMiddleware(jwtSecret)` to the protected subrouter.

### 4. `internal/domain/user.go`

- **`GetUser(ctx, username string)`**: remove all JWT parsing. Look up the user directly by username via `repo.GetUserByUsername`, generate a fresh token, and return.
- **`UpdateUser(ctx, username string, u *UpdateUser)`**: remove all JWT parsing. Use the passed `username` as `currentUsername` directly.
- The `jwtSecret` field is still needed on `UserController` for `generateToken` calls.

### 5. `cmd/server/server.go`

- Update `webserver.NewServer(port, handlers, os.Getenv("JWT_SECRET"))` to pass the JWT secret.

### 6. `arch.md`

- Update the middleware description to reflect full JWT validation.
- Update Current State.

## Order of implementation

1. Update `middleware.go`: change to constructor form, add JWT validation, store username in context.
2. Update `handlers.go`: read username from context, update `userService` interface signatures.
3. Update `domain/user.go`: remove JWT parsing from `GetUser` and `UpdateUser`.
4. Update `server.go`: add `jwtSecret` parameter, pass to `authMiddleware`.
5. Update `cmd/server/server.go`: pass `JWT_SECRET` to `NewServer`.
6. Run `make lint` and fix any errors.
7. Update `arch.md`.
