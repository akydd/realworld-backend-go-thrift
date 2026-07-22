package domain

import "context"

type profileRepo interface {
	GetProfileByUsername(ctx context.Context, profileUsername string, viewerID int) (*Profile, error)
	FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
	UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error)
}

// ProfileController implements the profile and follow/unfollow use-cases of the domain.
type ProfileController struct {
	repo profileRepo
}

// NewProfileController creates a ProfileController backed by the given repository.
func NewProfileController(r profileRepo) *ProfileController {
	return &ProfileController{repo: r}
}

// GetProfile retrieves the public profile of the user with the given username,
// enriched with the viewing user's following status.
func (c *ProfileController) GetProfile(ctx context.Context, profileUsername string, viewerID int) (*Profile, error) {
	return c.repo.GetProfileByUsername(ctx, profileUsername, viewerID)
}

// FollowUser creates a follow relationship between the follower and the user identified by followeeUsername.
func (c *ProfileController) FollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error) {
	return c.repo.FollowUser(ctx, followerID, followeeUsername)
}

// UnfollowUser removes the follow relationship between the follower and the user identified by followeeUsername.
func (c *ProfileController) UnfollowUser(ctx context.Context, followerID int, followeeUsername string) (*Profile, error) {
	return c.repo.UnfollowUser(ctx, followerID, followeeUsername)
}
