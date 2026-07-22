# Delete comment

Allow the author of a comment to delete that comment.

## Implementation

Protected endpoint DELETE /api/articles/:slug/comments/:id

If no article matching the slug is found, return status 404 and the usual error for when an article is not found.

If no comment matching the id is found, or if the comment with id exists but does not belong to the article matching the slug, return status 404 and
```json
{
  "error": {
    "comment": ["not found"]
  }
}
```

If the comment exists and belongs to the article matching the slug, but the authenticated user is not the author of the comment, return status 401 and the usual error for invalid credentials.

If the comment with the id exists, and it belongs to the article with slug, and if the authenticated user is the author of the comment, then delete the comment record from the database.

A succesful delete operation returns status 204 and no other data.

Let me know if there are any use cases that I missed above.
