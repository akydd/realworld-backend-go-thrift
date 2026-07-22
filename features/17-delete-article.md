# Delete article

Allow the author of an article to delete it, given the slug.

## Implementation

Protected endpoint DELETE /api/articles/:slug

If no article with the slug exists, return status 404 and the usual error for when an article is not found.

If the calling user is not the author of the article, return 401 and the usual error for invalid credentials.

If the article exists, and the calling user is the author of the article, then remove the article from the database. Related data should also be deleted (his should already be handled by the db schema's cascade clauses in the table definitions). Then return status 204 and no other data.
