# # Integration tests

This feature adds the the ability to run the integration tests easily, with a clean database state for each run.

## Implementation
Refer to the root file arch.md for details on the projects architecture.

### Existing integration tests
These should be disregarded and removed from the project structure.

### Entry point
Refactor the entry point so that the application can start up with either the env variables in .env, or in .env_test. The default option should be .env.

### Docker
Split the existing docker compose file into two separate files, one for starting the production database, and the other for starting the test database.

### Test runner
The integration test runner should be a make target `make int-tests`  that does the following:
1. Start the test db docker instance.
2. Build the application: go build ./cmd/server
3. Start the application using the env vars in .env_test
2. Run the integration test script: HOST=http://localhost:8097 ../realworld/specs/api/run-api-tests-hurl.sh
3. After the previous step completes, delete all data from the test db instance and stop the test db docker instance.
