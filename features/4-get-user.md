# Get user

Get the details of the authenticated user.

## Implementation

This uses the endpoint GET /api/user, and the request must caontain an Authorization header whose value is "Token {token}", where {token} is the user.token returned from the user login endpoint.

If the Authorization header is missing, return status code 401 and response body
```json
{
  "errors": {
    "token": ["is missing"]
  }
}
```

If the token matches to a user, return status code 200 and the user data contained in the json generated from model webserver.UserResponse.

If the token does not match to any user, return status code 401 and response
```json
{
  "errors": {
    "credentials": ["invalid"]
  }
}
```
