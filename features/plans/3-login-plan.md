# Plan: User Login

## Overview

Six files require changes across all three layers to add `POST /api/users/login`. No new dependencies are needed — password verification uses the existing `argon2id` package and token generation reuses `generateToken`.

---

## 1. Domain — `internal/domain/models.go`

Add a new request model for the login payload:

```go
type LoginUser struct {
    Email    string
    Password string
}
```

---

## 2. Domain — `internal/domain/errors.go`

Add a new `CredentialsError` type for the 401 invalid-credentials case:

```go
type CredentialsError struct{}

func (c *CredentialsError) Error() string {
    return "invalid credentials"
}
```

The HTTP handler will use `createErrResponse("credentials", []string{"invalid"})` to produce the required response body.

---

## 3. Domain — `internal/domain/user.go`

### Extend `userRepo` interface

Add `GetUserByEmail` so the domain can retrieve a user record (including hashed password) for verification:

```go
type userRepo interface {
    InsertUser(ctx context.Context, u *RegisterUser) (*User, error)
    GetUserByEmail(ctx context.Context, email string) (*User, string, error) // returns user, hashedPassword, error
}
```

### Add `LoginUser` method to `UserController`

```go
func (c *UserController) LoginUser(ctx context.Context, u *LoginUser) (*User, error) {
    if u.Email == "" {
        return nil, NewValidationError("email", blankFieldErrMsg)
    }
    if u.Password == "" {
        return nil, NewValidationError("password", blankFieldErrMsg)
    }

    user, hashedPassword, err := c.repo.GetUserByEmail(ctx, u.Email)
    if err != nil {
        return nil, err
    }

    match, err := argon2id.ComparePasswordAndHash(u.Password, hashedPassword)
    if err != nil {
        return nil, err
    }
    if !match {
        return nil, &CredentialsError{}
    }

    token, err := generateToken(user.Username, c.jwtSecret)
    if err != nil {
        return nil, err
    }
    user.Token = token

    return user, nil
}
```

**Notes:**
- Blank field validation uses the existing `NewValidationError` + `blankFieldErrMsg`, matching the pattern used by `RegisterUser`.
- `argon2id.ComparePasswordAndHash` is already available via the existing dependency.
- When the email is not found, `GetUserByEmail` returns `*CredentialsError` (see DB adapter below), preventing user enumeration.

---

## 4. DB Adapter — `internal/adapters/out/db/postgres.go`

### Add internal struct for row scanning

A new internal struct is needed to scan the `password` column alongside the existing user fields:

```go
type userWithPassword struct {
    user
    Password string `db:"password"`
}
```

### Add `GetUserByEmail` method

```go
func (p *Postgres) GetUserByEmail(ctx context.Context, email string) (*domain.User, string, error) {
    query := "SELECT username, email, bio, image, password FROM users WHERE email = $1"
    var dbUser userWithPassword

    err := p.db.QueryRowxContext(ctx, query, email).StructScan(&dbUser)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, "", &domain.CredentialsError{}
        }
        return nil, "", err
    }

    user := convertUser(dbUser.user)
    return &user, dbUser.Password, nil
}
```

Returning `*domain.CredentialsError` on `sql.ErrNoRows` is consistent with the existing pattern of the DB adapter returning domain-level errors (e.g. `NewDuplicateError`), and ensures a missing email is indistinguishable from a wrong password at the HTTP layer.

---

## 5. HTTP Adapter — `internal/adapters/in/webserver/handlers.go`

### Extend `userService` interface

```go
type userService interface {
    RegisterUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error)
    LoginUser(ctx context.Context, u *domain.LoginUser) (*domain.User, error)
}
```

### Add request DTO

```go
type LoginUserInner struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginUserRequest struct {
    User LoginUserInner `json:"user"`
}
```

### Add `LoginUser` handler

```go
func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
    var req LoginUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    d := domain.LoginUser(req.User)

    w.Header().Set("Content-Type", "application/json")

    user, err := h.service.LoginUser(r.Context(), &d)
    if err != nil {
        var errResp []byte
        var validationErr *domain.ValidationError
        var credErr *domain.CredentialsError
        if errors.As(err, &validationErr) {
            errResp = createErrResponse(validationErr.Field, validationErr.Errors)
            w.WriteHeader(http.StatusUnprocessableEntity)
        } else if errors.As(err, &credErr) {
            errResp = createErrResponse("credentials", []string{"invalid"})
            w.WriteHeader(http.StatusUnauthorized)
        } else {
            fmt.Println(err.Error())
            errResp = createErrResponse("unknown_error", []string{err.Error()})
            w.WriteHeader(http.StatusInternalServerError)
        }
        _, _ = w.Write(errResp)
        return
    }

    resp := UserResponse{
        User: UserResponseInner(*user),
    }
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(resp)
}
```

Note: `domain.LoginUser(req.User)` works because `LoginUserInner` and `domain.LoginUser` have the same fields (`Email`, `Password`), following the same casting pattern used for `RegisterUser`.

---

## 6. HTTP Adapter — `internal/adapters/in/webserver/server.go`

### Extend `ServerHandlers` interface

```go
type ServerHandlers interface {
    RegisterUser(http.ResponseWriter, *http.Request)
    LoginUser(http.ResponseWriter, *http.Request)
}
```

### Register the new route

```go
r.HandleFunc("/api/users/login", h.LoginUser).Methods("POST")
```

---

## Summary of Files Changed

| File | Change |
|------|--------|
| `internal/domain/models.go` | Add `LoginUser` struct |
| `internal/domain/errors.go` | Add `CredentialsError` type |
| `internal/domain/user.go` | Add `GetUserByEmail` to `userRepo`; add `LoginUser` method |
| `internal/adapters/out/db/postgres.go` | Add `userWithPassword` struct; add `GetUserByEmail` method |
| `internal/adapters/in/webserver/handlers.go` | Add `LoginUser` to `userService`; add DTOs; add `LoginUser` handler |
| `internal/adapters/in/webserver/server.go` | Add `LoginUser` to `ServerHandlers`; register route |

No new dependencies required.
