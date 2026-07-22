# Get profile

An endpoint to return a user's profile, given that user's username.

## Implementation
This is a route where the presence of the Authorization header, and the jwt token inside it, is optional. We might need a new middleware and route subgroup for this route, and for future routes where authorization is also optional.

The route for this feature is GET /api/profile/:username.

The domain logic should look up the user by username. The method that performs this should be located in a new ProfileController. Although the method will be aware of the presence or absence of an authenticated user via the username held in the context, the method will not do anything with this information for now.

When no user is found, return status 404 and response
```json
{
  "errors": {
    "profile": ["not found"]
  }
}
```

When a user is found, return status 200 and body
```json
{
  "profile": {
    "username": "jake",
    "bio": "I work at statefarm",
    "image": "https://api.realworld.io/images/smiley-cyrus.jpg",
    "following": false
  }
}
```

Note that the following field is always false for now.
