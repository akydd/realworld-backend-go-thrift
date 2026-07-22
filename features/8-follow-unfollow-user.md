# Follow user

Allow authenticated users to follow and unfollow other users.

## Implementation

There are two new protected endpoints:
POST /api/profiles/:username/follow - follow a user
DELETE /api/profiles/:username/follow - unfollow a user

The logic is that the authenticated user should follow/unfollow the user with username.
