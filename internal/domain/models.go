package domain

import "time"

// RegisterUser holds the data required to register a new user account.
type RegisterUser struct {
	Username string
	Email    string
	Password string
}

// LoginUser holds the credentials used to authenticate an existing user.
type LoginUser struct {
	Email    string
	Password string
}

// User represents an authenticated user returned by the domain layer.
type User struct {
	ID       int
	Email    string
	Token    string
	Username string
	Bio      *string
	Image    *string
}

// UpdateUser carries optional fields for updating a user profile.
// A nil field means the caller did not supply that attribute.
type UpdateUser struct {
	Email    *string
	Bio      **string // nil = not provided; non-nil: *Bio==nil means set to null, *Bio!=nil means set to value
	Image    **string // nil = not provided; non-nil: *Image==nil means set to null, *Image!=nil means set to value
	Username *string
	Password *string
}

// Profile represents a public user profile, optionally enriched with
// following status relative to the viewing user.
type Profile struct {
	Username  string
	Bio       *string
	Image     *string
	Following bool
}

// UpdateUserData is the fully resolved set of values written to the repository
// when updating a user; all fields are required and non-nil.
type UpdateUserData struct {
	Email    string
	Username string
	Password string
	Bio      *string
	Image    *string
}

// Article represents a published article including its metadata and author profile.
type Article struct {
	Slug           string
	Title          string
	Description    string
	Body           string
	TagList        []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Favorited      bool
	FavoritesCount int
	Author         Profile
}

// CreateArticle holds the data required to publish a new article.
type CreateArticle struct {
	Title       string
	Description string
	Body        string
	TagList     []string
}

// UpdateArticle carries optional fields for modifying an existing article.
// A nil field means the caller did not supply that attribute.
type UpdateArticle struct {
	Title       *string
	Description *string
	Body        *string
	TagList     *[]string // nil = not provided (preserve); non-nil = new list (may be empty)
}

// ArticleList is a paginated collection of articles together with the total
// unpaginated count.
type ArticleList struct {
	Articles   []*Article
	TotalCount int
}

// Comment represents a comment posted on an article.
type Comment struct {
	ID        int
	CreatedAt time.Time
	UpdatedAt time.Time
	Body      string
	Author    Profile
}

// CreateComment holds the data required to post a new comment on an article.
type CreateComment struct {
	Body string
}

// ListArticlesFilter specifies the optional filtering and pagination parameters
// for the global article listing endpoint.
type ListArticlesFilter struct {
	Tag       *string
	Author    *string
	Favorited *string
	Limit     int
	Offset    int
}

// ArticleFeedFilter specifies the pagination parameters for the personalised
// article feed endpoint.
type ArticleFeedFilter struct {
	Limit  int
	Offset int
}
