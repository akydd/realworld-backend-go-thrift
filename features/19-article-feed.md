# Article Feed

Returns a list of articles authored by followed users, ordered by most recent first.

## Implementation

Protected endpoint GET /api/articles/feed.

Accepts two query parameters:
* limit - limit the number of returned results. When not present, the default is 20. Ex: ?limit=8
* offset - offset/skip the number of articles. Default is 0. Ex: ?offset=8

Multiple query parameters can be applied at once. The endpoint can also be called without any query parameters.

A succesful request returns status 200, the list of articles contained in the 'articles' field, and the number of returned articles in the 'articleCount' field. Note that the body field of the articles is not returned. The list of arricles contais only articles authored by users that the authenticated user is following.

```json
{
  "articles":[{
    "slug": "how-to-train-your-dragon",
    "title": "How to train your dragon",
    "description": "Ever wonder how?",
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
  }, {
    "slug": "how-to-train-your-dragon-2",
    "title": "How to train your dragon 2",
    "description": "So toothless",
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
  }],
  "articlesCount": 2
}
```

There are some similaritied betweeh the sql in @internal/adapters/out/db/postgres.go methods GetArticleBySlug, ListArticles, and the sql that is needed for this new feature. Ultrathink if the sql can be made common and shared betweem two or three of the separate methods.
