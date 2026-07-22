# Plan: Update User (Feature 5)

## Overview

Implement `PUT /api/user` — allows an authenticated user to update their own account fields. All request fields are optional, but at least one must be present.

## Error responses

| Condition | Status | Body |
|-----------|--------|------|
| `Authorization` header missing | 401 | `{"errors": {"token": ["is missing"]}}` |
| Token invalid / user not found | 401 | `{"errors": {"credentials": ["invalid"]}}` |
| No fields provided, or non-nullable field blank | 422 | `{"errors": {"<field>": ["can't be blank"]}}` |
| Username or email already taken | 409 | `{"errors": {"<field>": ["has already been taken"]}}` |
| Success | 200 | `UserResponse` JSON |

## Partial update strategy

Rather than building a dynamic SQL `SET` clause, the domain layer will:
1. Fetch the current user record (including password hash) from the DB.
2. Apply the non-nil updates to form a complete `UpdateUserData` struct.
3. Write all fields back with a single `UPDATE ... RETURNING` query.

This avoids complex SQL and handles bio/image (which can be set to `""`) cleanly.

## Changes required

### 1. `internal/domain/models.go`

Add two new structs:

```go
// Input from the HTTP layer — all fields optional.
// Bio and Image use **string to represent three states:
//   nil           = not provided (keep current value)
//   &nil          = explicitly set to null
//   &&"text"      = set to a new value
// Email, Username, Password use *string (nil = not provided; "" = validation error).
type UpdateUser struct {
    Email    *string
    Bio      **string
    Image    **string
    Username *string
    Password *string
}

// Resolved input for the DB layer — all fields populated
type UpdateUserData struct {
    Email    string
    Username string
    Password string // hashed
    Bio      *string
    Image    *string
}
```

### 2. `internal/adapters/out/db/postgres.go`

**Add `GetFullUserByUsername`:**
- Query `SELECT username, email, bio, image, password FROM users WHERE username = $1`.
- Scan into existing `userWithPassword` struct.
- Return `(*domain.User, string, error)`, returning `*domain.CredentialsError` on no row.

**Add `UpdateUser(ctx, currentUsername string, u *domain.UpdateUserData) (*domain.User, error)`:**
- Query: `UPDATE users SET email=$1, username=$2, password=$3, bio=$4, image=$5 WHERE username=$6 RETURNING username, email, bio, image`
- Use `currentUsername` in the `WHERE` clause (in case the username itself is being changed).
- Scan result into the `user` struct and return via `convertUser`.
- Map pq unique-violation errors (code `23505`) to `*domain.DuplicateError` by constraint name, same as `InsertUser`.

### 3. `internal/domain/user.go`

**Extend `userRepo` interface:**
```go
GetFullUserByUsername(ctx context.Context, username string) (*User, string, error)
UpdateUser(ctx context.Context, currentUsername string, u *UpdateUserData) (*User, error)
```

**Add `UpdateUser` method to `UserController`:**
1. Parse and validate JWT (same approach as `GetUser`). Return `*CredentialsError` on failure.
2. Extract current username from the `sub` claim.
3. Validate the update request:
   - At least one field must be non-nil → 422 `{"user": ["can't be blank"]}` if all nil.
   - `Email`, `Username`, `Password`: if non-nil, must not be `""` → 422 with field name.
4. Fetch current user + password hash via `repo.GetFullUserByUsername`. Returns `*CredentialsError` if not found.
5. Build `UpdateUserData` by starting from current values and overwriting with any non-nil updates.
6. If `Password` is being updated, hash the new password with Argon2ID before storing.
7. Call `repo.UpdateUser(ctx, currentUsername, &data)`.
8. Generate a fresh token (using the potentially updated username).
9. Return the updated `*User` with the token set.

### 4. `internal/adapters/in/webserver/handlers.go`

#### Why `NullableString` is needed for bio and image

Bio and image are nullable fields — the client can explicitly clear them by sending `null` or `""`. However, Go's `encoding/json` package cannot distinguish between a JSON field that is **absent** and one that is **explicitly `null`**: both result in a nil `*string` pointer. This makes it impossible to tell "don't touch bio" from "set bio to null" using a plain `*string`.

The fix is a custom `NullableString` type with a `Present` flag and a custom `UnmarshalJSON`:

```go
type NullableString struct {
    Value   *string
    Present bool
}

func (n *NullableString) UnmarshalJSON(data []byte) error {
    n.Present = true   // UnmarshalJSON is only called when the field is present in JSON
    if string(data) == "null" {
        n.Value = nil
        return nil
    }
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    n.Value = &s
    return nil
}
```

`UnmarshalJSON` is only invoked by `encoding/json` when the key appears in the JSON object, so `Present` stays `false` for absent fields. This gives three distinct states:

| JSON | `Present` | `Value` |
|------|-----------|---------|
| field absent | `false` | `nil` |
| `"bio": null` | `true` | `nil` |
| `"bio": ""` | `true` | `&""` |
| `"bio": "text"` | `true` | `&"text"` |

Both `null` and `""` are normalized to SQL `NULL` (an empty bio has no useful meaning). This is reflected in `domain.UpdateUser` where bio and image use `**string`: the outer pointer is nil when not provided, and non-nil when the field was present (with the inner pointer being nil for null or non-nil for a value).

**Add DTOs:**
```go
type UpdateUserInner struct {
    Email    *string        `json:"email"`
    Bio      NullableString `json:"bio"`
    Image    NullableString `json:"image"`
    Username *string        `json:"username"`
    Password *string        `json:"password"`
}

type UpdateUserRequest struct {
    User UpdateUserInner `json:"user"`
}
```

**Extend `userService` interface:**
```go
UpdateUser(ctx context.Context, token string, u *domain.UpdateUser) (*domain.User, error)
```

**Add `UpdateUser` handler:**
- Parse `Authorization: Token {jwt}` header — 401 if missing (same as `GetUser`).
- Decode JSON body into `UpdateUserRequest`.
- Build `domain.UpdateUser` manually: Email/Username/Password map directly as `*string`; Bio/Image are converted from `NullableString` to `**string` (nil if not present, `new(*string)` for null/empty, `&value` for a non-empty string).
- Call `h.service.UpdateUser`.
- Handle errors: `ValidationError` → 422, `CredentialsError` → 401, `DuplicateError` → 409, else 500.
- On success: 200 + `UserResponse`.

### 5. `internal/adapters/in/webserver/server.go`

**Extend `ServerHandlers` interface:**
```go
UpdateUser(http.ResponseWriter, *http.Request)
```

**Register route:**
```go
r.HandleFunc("/api/user", h.UpdateUser).Methods("PUT")
```

### 6. `arch.md`

- Add `PUT /api/user` to the routes table.
- Update the DB adapter section to mention `GetFullUserByUsername` and `UpdateUser`.
- Update Current State.

## Order of implementation

1. Add `UpdateUser` and `UpdateUserData` to `internal/domain/models.go`.
2. Add `GetFullUserByUsername` and `UpdateUser` to the Postgres adapter.
3. Extend `userRepo` interface and add `UpdateUser` to `UserController`.
4. Add DTOs, extend `userService` interface, and add `UpdateUser` handler.
5. Register route and extend `ServerHandlers` in `server.go`.
6. Run `make lint` and fix any errors.
7. Update `arch.md`.
