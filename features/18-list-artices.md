# List articles

Returns most recent articles globally by default, provide tag, author or favorited query parameter to filter results.

## Implementation

Endpoint is GET /api/articles.

It supports the following query parameters:
* tag - return articles with the matching tag. String matching is case insensitive. Ex: ?tag=reactjs
* author - return articles authored by author, case insensitive. Ex: ?author=joe
* favorited - return articles favorited by the user with matching username, case insensitive. Ex: ?favorited=joe
* limit - limit the number of returned results. When not present, the default is 20. Ex: ?limit=8
* offset - offset/skip the number of articles. Default is 0. Ex: ?offset=8

Multiple query parameters can be applied at once. The endpoint can also be called without any query parameters.

Authentication is optional.

When authenticates is provided, each article's 'author.following' should reflect the following status of the authenticated user and the article's author, and each article's 'favorited' field must reflect the favorited status of the article for the authenticated user.

Articles are returned with the most recent ones first.

A succesful request returns status 200, the list of articles contained in the 'articles' field, and the number of returned articles in the 'articleCount' field. Note that the body field of the articles is not returned.

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
