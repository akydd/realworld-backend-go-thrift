# Plan: Better JWT (Feature better-jwt)

## Overview

The JWT `sub` claim currently stores the username, which is mutable. Changing it to use the user ID (an immutable integer) means a renamed user's token remains valid. This requires propagating the user ID through the domain model, repository interfaces, DB queries, and HTTP middleware.

---

## 1. Domain Model (`internal/domain/models.go`)

Add `ID int` to the `User` struct.

---

## 2. Domain — User (`internal/domain/user.go`)

### 2a. Update `userRepo` interface

- Replace `GetUserByUsername(ctx, username string) (*User, error)` with `GetUserByID(ctx, id int) (*User, error)`.
- Replace `GetFullUserByUsername(ctx, username string) (*User, string, error)` with `GetFullUserByID(ctx, id int) (*User, string, error)`.
- Change `UpdateUser(ctx, currentUsername string, u *UpdateUserData)` → `UpdateUser(ctx, userID int, u *UpdateUserData)`.

### 2b. Update `generateToken`

Change signature from `generateToken(username string, secret string)` to `generateToken(id int, secret string)`. Store `strconv.Itoa(id)` in the `sub` claim.

### 2c. Update `GetUser`

Call `repo.GetUserByID(ctx, userID)` (receiving `userID int`) instead of `GetUserByUsername`.

### 2d. Update `UpdateUser`

Call `repo.GetFullUserByID(ctx, userID)` and `repo.UpdateUser(ctx, userID, &data)`.

### 2e. All `generateToken` call sites

Pass `user.ID` instead of `user.Username` in `RegisterUser`, `LoginUser`, `GetUser`, and `UpdateUser`.

---

## 3. Domain — Profile (`internal/domain/profile.go`)

### 3a. Update `profileRepo` interface

- Remove the `viewerUsername string` parameter from `GetProfileByUsername` — this branch does not include the `follows` table, so `following` is always `false` and no viewer context is needed.

### 3b. Update `ProfileController.GetProfile`

- Remove the `viewerID int` parameter; pass only `profileUsername` to `repo.GetProfileByUsername`.

> **Note:** `FollowUser` and `UnfollowUser` are not part of this branch. The `follows` table migration (`003_follows.sql`) is not included here; it lives on the `8-follow-user` branch. When the two branches are eventually merged, the viewer ID parameter will be reintroduced and used in the LEFT JOIN query.

---

## 4. DB Adapter (`internal/adapters/out/db/postgres.go`)

### 4a. Update `user` struct

Add `ID int \`db:"id"\`` field.

### 4b. Update `convertUser`

Copy `u.ID` into the returned `domain.User`.

### 4c. Add `id` to all SELECT / RETURNING clauses

Update `InsertUser`, `GetUserByEmail`, and `UpdateUser` queries to include `id` in `RETURNING` / `SELECT`.

### 4d. Add `GetUserByID` and `GetFullUserByID`

```sql
-- GetUserByID
SELECT id, username, email, bio, image FROM users WHERE id = $1

-- GetFullUserByID
SELECT id, username, email, bio, image, password FROM users WHERE id = $1
```

Both return `*domain.CredentialsError` when no row is found.

### 4e. Remove `GetUserByUsername` and `GetFullUserByUsername`

These are no longer in the `userRepo` interface; remove the implementations.

### 4f. Update `UpdateUser`

Change `WHERE username = $6` → `WHERE id = $6` (passing `userID int`).

### 4g. Update `GetProfileByUsername`

Remove the `viewerID` parameter — this branch does not include the `follows` table. The query returns `following: false` unconditionally:

```sql
SELECT id, username, bio, image FROM users WHERE username = $1
```

---

## 5. HTTP Middleware (`internal/adapters/in/webserver/middleware.go`)

- Rename `usernameKey` → `userIDKey` (context key, type `contextKey`).
- After validating the JWT, parse `claims.Subject` as an integer with `strconv.Atoi`. If parsing fails, treat as invalid token.
- Store the resulting `int` user ID in context under `userIDKey`.

Both `authMiddleware` and `optionalAuthMiddleware` need updating. For `optionalAuthMiddleware`, store user ID only when parsing succeeds; otherwise leave context unchanged (zero value 0 means unauthenticated).

---

## 6. HTTP Handlers (`internal/adapters/in/webserver/handlers.go`)

### 6a. Update `userService` interface

- `GetUser(ctx, username string)` → `GetUser(ctx, userID int)`
- `UpdateUser(ctx, username string, u *domain.UpdateUser)` → `UpdateUser(ctx, userID int, u *domain.UpdateUser)`

### 6b. Update `profileService` interface

- Remove viewer parameter: `GetProfile(ctx, profileUsername string)` — no viewer ID needed on this branch.

### 6c. Update handlers

- Replace all `r.Context().Value(usernameKey).(string)` reads with `r.Context().Value(userIDKey).(int)` (protected handlers).
- `GetProfile` handler no longer reads viewer ID from context.

---

## 7. Update `arch.md`

- Note that `User` model now includes `ID`.
- Update JWT description: `sub` claim now holds the user ID (integer as string).
- Update middleware description: extracts user ID (int) from `sub` and stores it in context.
- Update DB adapter descriptions for changed/added/removed methods.
