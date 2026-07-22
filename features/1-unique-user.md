# Unique user

## Overview
This feature prevents duplicate users from registering. A duplicate user is a user who has the same username or email as an existing user.

## Implementation
Do not include any new libraries or external dependencies without my approval.
Before changing any code, prepare a plan for my review, output to file `features/1-unique-user-plan.md`.

### Migrations
Create a new db migrationn, which adds unique constraints to the email and username fields of the user table.

### Domain layer
Add a new string to the domain errors:
`DuplicateErrMsg = "has already been taken"`
Create a new custom error type `DuplicateError` that contains two fields, `Field` and `Error`, both strings. THe `Error` field should always be equal to `DuplicateErrMsg`.

### Postgres Adapter
Update `InsertUser` to detect errors caused by trying to insert duplicate username and email values. In each case, return a new DuplicateError whose Field value equals the name of the column whose value was duplicated, and whose Error contains onlt the `DuplicateErrMsg` string.

### HTTP adapter
The RegisterUser handler should detect when a DuplicateError is returned from the call to service.RegisterUser, When that error is returned, the http response code should be 409.
When the duplicate column was the email, he reponse body should be:
```json
{
  "errors": {
    "email": [
      "has already been taken"
    ]
  }
}
```
When the duplicate column was the username, the response body should be:
```json
{
  "errors": {
    "username": [
      "has already been taken"
    ]
  }
}
```
You should be able to use the existing createErrResponse to produce the above 2 responses.
