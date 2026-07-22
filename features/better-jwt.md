# Better JWT

The current JWT implementation uses the username attribute to build the claims. This data is not immutable. Use the user id instead.

## Implementation

The domain model for the User needs to contain the user id in order for this to work.

Once the JWT token generation is changed to use the user ID, some methods and queries that are currently using the username will need to updated.
