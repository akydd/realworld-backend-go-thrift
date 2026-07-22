# Get article

Endpoint to return a single article.

## Implementation

Endpoint GET /api/articles/:slug. Authentication is optional.

When the article is found with a matching slug, return http status 200 and the body:
```json
{
  "article": {
    "slug": "how-to-train-your-dragon",
    "title": "How to train your dragon",
    "description": "Ever wonder how?",
    "body": "It takes a Jacobian",
    "tagList": ["dragons", "training"],
    "createdAt": "2016-02-18T03:22:56.637Z",
    "updatedAt": "2016-02-18T03:48:35.824Z",
    "favorited": false,
    "favoritesCount": 0,
    "author": {
      "username": "jake",
      "bio": "I work at statefarm",
      "image": "https://i.stack.imgur.com/xHWG8.jpg",
      "following": false
    }
  }
}
```

Note that favorited and favoritesCount should be false and 0, respectively, until those features are implemented.

When the call is made by an authenticated user, the value of 'author.following' should reflect the following state of the caller and the article author.

When no article is found, return status 404 and
```json
{
  "errors": {
    "article": ["not found"]
  }
}
```
