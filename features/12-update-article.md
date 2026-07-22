# Update article

Allow the author of an article to update the article.

## Implementation

Endpoint PUT /api/articles/:slug. This is a protected route.

The request payload looks like
```json
{
  "article": {
    "title": "Did you train your dragon?",
    "description": "This is an update.",
    "body": "This is a new body."
  }
}
```

Each field inside article are optional, but at least one field must be present. Otherwise return 422 and the usual error about missing fields.

An update to the title results in an update to the slug as well. If title is blank, return 422 and the usual error about blank fields.

The response of a succesful update is http status code 200, and the updated article, formattted similar to the data returned by the calls to create or get an article.

If no article matching the slug in the request is found, return status 404 and
```json
{
  "errors": {
    "article": ["not found"]
  }
}
```

If the update tries to set s title that is a duplicate, return status 409 and
```json
{
  "errors": {
    "title": ["already taken"]
  }
}
```

If the authenticated user is not the author of the article, return status 401 and
```json
{
  "errors": {
    "credentials": ["invalid"]
  }
}
```
