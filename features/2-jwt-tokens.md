# JWT tokens

## Overview
This feature replaces the user token placeholder with an actual JWT token.

## Implementation

### Domain layer
The UserController's RegisterUser function should replace the fake user token with an actual JWT token befor returning the user to the caller.
