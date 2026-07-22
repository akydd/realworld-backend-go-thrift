# Create article

Alloww users to create articles.

## Implementation

This uses a protected endpoint, POST /api/articles.

The request payload is
```json
{
  "article": {
    "title": "How to train your dragon",
    "description": "Ever wonder how?",
    "body": "You have to believe",
    "tagList": ["reactjs", "angularjs", "dragons"]
  }
}
```

The fields title, description, and body are required. tagList is optional. You can ignore the tagList values for now. Any missing required field should result in a 422 status code and an error like
```json
{
  "errors": {
    "title": ["can't be blank"]
  }
}
```

A successful response returns status code 201 and
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

The author section caontains the profile of the article's author, who was also the authenticated user who made the POST request to create the article.

For now, tagList will always be empty array, favorited will always be false, and favoritesCount will always be 0.

The slug and title of an article are unique. Enforce this in the db. Notice that the slug is the title converted to kebab case.

An attempt to create an article whose title is a duplicate will result in an 409 error with body
```json
{
  "errors": {
    "title": ["has already been taken"]
  }
}
```
