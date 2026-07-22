# Create article comment

Allow users to comment on articles.

## Implementation

Use the protected endpoint POST /api/articles/:slug/comments.
This creates a comment for the article with the matching slug, authored by the authenticated user.

The request payload is
```json
{
  "comment": {
    "body": "His name was my name too."
  }
}
```

The body is required. If it's blank, return status 422 and the usual error for a field that can't be blank.

If no matching article is found, return status 404 and the usual error for an article that can't be found.

A successful request returns status 201 and
```json
{
  "comment": {
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
  }
}
```

The id is the primary key of the comment record in the database. The author section contains the profile of the comment's author.

