# Get article comments

Allow users to get all the comments for an article.

## Implementation

Endpoint GET /api/articles/:slug/comments. Authentication is optional.

This endpoint returns an array of comments for the article matching the slug.

If no article matching the slug is found, return status 404 and the usual error when an article cannot be found.

When there are comments found for the article, return an array of comments formatted in json like:
```json
{
  "comments": [{
    "id": 1,
    "createdAt": "2016-02-18T03:22:56.637Z",
    "updatedAt": "2016-02-18T03:22:56.637Z",
    "body": "It takes a Jacobian",
    "author": {
      "username": "jake",
      "bio": "I work at statefarm",
      "image": "https://i.stack.imgur.com/xHWG8.jpg",
      "following": false
    }
  }]
}
```

Each elememt of the 'comments' array is a comment object as returned when creating a comment.

When the call is made by an authenticated user, the 'author.following' field of each comment must reflect the following status of the caller and the author of the comment.

