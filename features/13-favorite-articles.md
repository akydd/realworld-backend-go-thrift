# Favorite articles

Allow users to favorite and unfavorite articles.

## Implementation
There are two protected endpoints:
POST /api/articles/:slug/favorite - favorite an article.
DELETE /api/articles/:slug/favorite - unfavorite an article.

The endpoints allow an authenticated user to mark/unmark the article matching the slug as a favorite article for that user.

For both cases, the response of a succesful operation is a 200 status and the single article as returned by the GET /api/articles/:slug endpoint.

For all endpoints that return a single article, the 'article.favorited' field should return a value that reflects whether or not the article is a favorite of the authenticated caller. Also, the 'article.favoritesCount` should return the number of users that have favorited that article.

If no article matching the slug is found, return status 404 and
```json
{
  "errors": {
    "article": ["not found"]
  }
}
```
