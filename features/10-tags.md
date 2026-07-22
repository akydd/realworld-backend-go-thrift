# Tags

Allow users to add tags to articles that they have authored.

## Implementation

### Creating tags

Tags aren't created directly. They are created during article creation from the array of strings contained in article.tagList of the payload to create an article.

If a tag already exists in the database, do not create a duplicate one.

Also deduplicate the tagList input before persisting the data.

### Get tags

A new endpoint GET /api/tags returns a list of tags:
```json
{
  "tags": [
    "reactjs",
    "angularjs"
  ]
}
```
