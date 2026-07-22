package domain

import "context"

type tagRepo interface {
	GetAllTags(ctx context.Context) ([]string, error)
}

// TagController implements the tag listing use-case of the domain.
type TagController struct {
	repo tagRepo
}

// NewTagController creates a TagController backed by the given repository.
func NewTagController(r tagRepo) *TagController {
	return &TagController{repo: r}
}

// GetTags returns all tags that have been used on published articles.
func (c *TagController) GetTags(ctx context.Context) ([]string, error) {
	return c.repo.GetAllTags(ctx)
}
