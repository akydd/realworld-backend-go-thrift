# Plan: Unique User Feature

## Overview

Prevent duplicate user registration by enforcing unique constraints at the database level and returning structured errors up through the domain and HTTP layers.

---

## 1. Migration

**New file**: `internal/adapters/out/db/migrations/002_unique_users.sql`

Add unique constraints on `email` and `username`. Name the constraints explicitly so the Postgres adapter can identify which column caused a violation.

```sql
-- +goose Up
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
ALTER TABLE users ADD CONSTRAINT users_username_unique UNIQUE (username);

-- +goose Down
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_unique;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_unique;
```

Goose will run this migration automatically on next application startup.

---

## 2. Domain Layer

**File**: `internal/domain/errors.go`

Add the new constant and error type alongside the existing `ValidationError`.

### New constant

```go
const (
    blankFieldErrMsg = "can't be blank"
    DuplicateErrMsg  = "has already been taken"
)
```

### New error type

> **Note on field naming**: The spec names one field `Error string`. In Go, a struct field and a method cannot share the same name — defining both `Error string` and `Error() string` on the same type is a compile error. Since `DuplicateError` must implement the `error` interface (so it can be returned as `error` and detected with `errors.As`), the message field will be named `Msg string` instead. It will always be set to `DuplicateErrMsg`. Please advise if a different approach is preferred.

```go
type DuplicateError struct {
    Field string
    Msg   string // always DuplicateErrMsg
}

func (d *DuplicateError) Error() string {
    return fmt.Sprintf("field %s: %s", d.Field, d.Msg)
}

func NewDuplicateError(field string) *DuplicateError {
    return &DuplicateError{
        Field: field,
        Msg:   DuplicateErrMsg,
    }
}
```

---

## 3. Postgres Adapter

**File**: `internal/adapters/out/db/postgres.go`

### Import change

The `lib/pq` package is currently blank-imported (`_ "github.com/lib/pq"`). Change it to a named import so the `pq.Error` type is accessible:

```go
"github.com/lib/pq"
```

### Updated `InsertUser`

After the query fails, check whether the error is a PostgreSQL unique constraint violation (`pq` error code `23505`). Map the constraint name to the appropriate field and return a `*domain.DuplicateError`.

```go
func (p *Postgres) InsertUser(ctx context.Context, u *domain.RegisterUser) (*domain.User, error) {
    query := "insert into users (username, email, password) values ($1, $2, $3) returning username, email, bio, image"
    var dbUser user

    err := p.db.QueryRowxContext(ctx, query, u.Username, u.Email, u.Password).StructScan(&dbUser)
    if err != nil {
        var pqErr *pq.Error
        if errors.As(err, &pqErr) && pqErr.Code == "23505" {
            switch pqErr.Constraint {
            case "users_email_unique":
                return nil, domain.NewDuplicateError("email")
            case "users_username_unique":
                return nil, domain.NewDuplicateError("username")
            }
        }
        return nil, err
    }

    user := convertUser(dbUser)
    return &user, nil
}
```

---

## 4. HTTP Adapter

**File**: `internal/adapters/in/webserver/handlers.go`

### Updated `RegisterUser` handler

Add a `DuplicateError` check in the error-handling block, before the fallthrough to the generic 500 handler. Use the existing `createErrResponse` helper, passing `dupErr.Field` and `[]string{dupErr.Msg}` to produce the required response body.

```go
user, err := h.service.RegisterUser(r.Context(), &d)
if err != nil {
    var errResp []byte
    var validationErr *domain.ValidationError
    var dupErr *domain.DuplicateError
    if errors.As(err, &validationErr) {
        errResp = createErrResponse(validationErr.Field, validationErr.Errors)
        w.WriteHeader(http.StatusUnprocessableEntity)
    } else if errors.As(err, &dupErr) {
        errResp = createErrResponse(dupErr.Field, []string{dupErr.Msg})
        w.WriteHeader(http.StatusConflict) // 409
    } else {
        fmt.Println(err.Error())
        errResp = createErrResponse("unknown_error", []string{err.Error()})
        w.WriteHeader(http.StatusInternalServerError)
    }

    w.Write(errResp)
    return
}
```

**Response for duplicate email (409):**
```json
{ "errors": { "email": ["has already been taken"] } }
```

**Response for duplicate username (409):**
```json
{ "errors": { "username": ["has already been taken"] } }
```

---

## Summary of Files Changed

| File | Change |
|------|--------|
| `internal/adapters/out/db/migrations/002_unique_users.sql` | **New** — unique constraints on `email` and `username` |
| `internal/domain/errors.go` | Add `DuplicateErrMsg` constant, `DuplicateError` struct, `NewDuplicateError` constructor |
| `internal/adapters/out/db/postgres.go` | Change blank `pq` import to named; detect `23505` errors in `InsertUser` |
| `internal/adapters/in/webserver/handlers.go` | Add `DuplicateError` branch (409) in `RegisterUser` error handling |

No new external dependencies are required. `github.com/lib/pq` is already in `go.mod`.
