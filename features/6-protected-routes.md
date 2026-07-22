# Protected Routes

Consolidate all routes that require the Authorization header into a single group of routes that use middleware to implement authentication.

## Implementation

There are two prpotected routes currently that contain the same logic for handling the jwt token contained within the Authorization header.
* GET /api/user
* PUT /api/user

More protected routes are expected.

Instead of duplicated the logic for each protected route seprately, use middleware to provide the common logic, such that it can be reused for future protected routes.
