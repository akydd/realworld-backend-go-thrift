# Plan: Integration Tests

## Overview

Four changes are required:
1. Delete the existing `tests/` directory entirely.
2. Refactor the server entry point to accept a configurable env file via a CLI flag.
3. Split `compose.yaml` into separate prod and test files.
4. Add a `Makefile` with an `int-tests` target that orchestrates the full test run and cleanup.

---

## 1. Delete `tests/` directory

Remove the entire `tests/` directory, including `tests/integration/api_test.go`. The Go integration tests are superseded by the hurl-based test runner and are no longer maintained.

Since `stretchr/testify` is only used by this test file, run `go mod tidy` afterwards to remove it from `go.mod` and `go.sum`.

---

## 2. Entry Point — `cmd/server/server.go`

Add a `-env` CLI flag using the standard `flag` package. Default is `.env`.

```go
import "flag"

envFile := flag.String("env", ".env", "path to env file")
flag.Parse()
err := godotenv.Load(*envFile)
```

This allows the test runner to start the server with `.env_test`:
```
./server -env .env_test
```

Also fix `Start()` in `internal/adapters/in/webserver/server.go`, which silently discards the error from `http.ListenAndServe`, causing the process to exit with no output on failure:

```go
// before
func (s *Server) Start() {
    http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.router)
}

// after
func (s *Server) Start() {
    log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", s.port), s.router))
}
```

---

## 3. Docker Compose — split into two files

### `compose.yaml` (production DB only)

Remove `test_db` and the `app-test-data` volume. Keep only:

```yaml
services:
  db:
    image: postgres
    restart: always
    shm_size: 128mb
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_USER: admin
      POSTGRES_DB: app
    volumes:
      - app-data:/var/lib/postgres
    ports:
      - 8095:5432

volumes:
  app-data:
```

### New `compose.test.yaml` (test DB only)

```yaml
services:
  test_db:
    image: postgres
    restart: always
    shm_size: 128mb
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_USER: admin
      POSTGRES_DB: test-app
    volumes:
      - app-test-data:/var/lib/postgres
    ports:
      - 8096:5432

volumes:
  app-test-data:
```

---

## 4. Makefile — new file at project root

### `int-tests` target

The target follows the steps from the feature spec:

```makefile
.PHONY: int-tests

int-tests:
	docker compose -f compose.test.yaml up -d
	until docker compose -f compose.test.yaml exec -T test_db pg_isready -U admin -d test-app; do sleep 1; done
	go build ./cmd/server
	./server -env .env_test & echo $$! > server.pid
	sleep 2
	HOST=http://localhost:8097 ../realworld/specs/api/run-api-tests-hurl.sh; \
	RESULT=$$?; \
	kill $$(cat server.pid) 2>/dev/null || true; \
	rm -f server.pid; \
	docker compose -f compose.test.yaml exec -T test_db psql -U admin -d test-app -c "TRUNCATE TABLE users;"; \
	docker compose -f compose.test.yaml down; \
	exit $$RESULT
```

**Notes on each step:**
- `docker compose -f compose.test.yaml up -d` — starts only the test DB container.
- `until pg_isready ...` — polls once per second until PostgreSQL is accepting connections. `pg_isready` is bundled in the official `postgres` image; this replaces a fixed `sleep` and unblocks as soon as the DB is genuinely ready.
- `go build ./cmd/server` — produces `./server` binary in the project root (Go uses the directory name as the output binary name).
- `./server -env .env_test &` — starts the server in the background with the test env; the PID is saved to `server.pid` for clean shutdown.
- `sleep 2` — gives the server time to start and run migrations. There is no health endpoint yet; this can be improved in the future.
- The hurl script runs against `http://localhost:8097` (the `SERVER_PORT` in `.env_test`). The exit code is captured so cleanup always runs regardless of test outcome.
- `kill $(cat server.pid)` — stops the background server.
- `TRUNCATE TABLE users` — clears test data while preserving the schema (migrations do not re-run on next start since Goose tracks applied versions).
- `docker compose -f compose.test.yaml down` — stops the test DB container (volume is preserved for the next run).
- `exit $RESULT` — propagates the hurl script exit code so `make` reports failure correctly.

**External dependency:** The hurl test script is expected at `../realworld/specs/api/run-api-tests-hurl.sh` relative to the project root. This path must exist before running `make int-tests`.

---

## Summary of Files Changed

| File | Change |
|------|--------|
| `tests/` | **Delete** entire directory; run `go mod tidy` to remove `stretchr/testify` |
| `cmd/server/server.go` | Add `-env` CLI flag (default `.env`) using `flag` package |
| `internal/adapters/in/webserver/server.go` | Fix `Start()` to log fatal on `http.ListenAndServe` error |
| `compose.yaml` | Remove `test_db` service and `app-test-data` volume |
| `compose.test.yaml` | **New** — test DB service only |
| `Makefile` | **New** — `int-tests` target |

No new external dependencies are required.
