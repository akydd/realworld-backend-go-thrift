# User Login

Provide an endpoint to handle user logins.

## Implementation

Logins are handled by the route POST /api/users/login.
The payload looks like
```json
{
  "user": {
    "email": "user@meail"
    "password": "password"
  }
}
```


The email and password fields are required.

A successful request returns an http status code of 200, together with the user data contained in the json produced by the model webserver.UserResponse.

If the email is missing, return code 422 and
```json
{
  "errors": {
    "email": ["can't be blank"]
  }
}
```

If the password is missing, return code 422 and
```json
{
  "errors": {
    "password": ["can't be blank"]
  }
}
```

If the password is incorrrect, return code 401 and
```json
{
  "errors": {
    "credentials": ["invalid"]
  }
}
```
