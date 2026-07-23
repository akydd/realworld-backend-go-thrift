# RealWorld Backend — Go

![CI](https://github.com/akydd/realworld-backend-go/actions/workflows/docker-publish.yml/badge.svg)

A [RealWorld](https://github.com/gothinkster/realworld) spec-compliant backend API for a social blogging platform (think Medium.com). Users can register, publish articles, follow each other, comment, and favorite posts.

**Stack:** Go · gRPC · Thrift · PostgreSQL · Docker · AWS ECS Fargate · RDS · ALB · Terraform · GitHub Actions

## Key Design Decisions

**Hexagonal Architecture (Ports & Adapters)** — business logic in `internal/domain/` has zero framework dependencies. The HTTP layer and PostgreSQL adapter are fully interchangeable without touching domain code. This makes the codebase easy to test, extend, and reason about.

**Native gRPC alongside HTTP** — the server exposes both a Gorilla Mux HTTP API (spec-compliant with the RealWorld spec) and a native gRPC API, both backed by the same domain layer. See the [gRPC API](#grpc-api) section for the reasoning behind running them as separate servers rather than using grpc-gateway.

**Apache Thrift RPC (scoped demonstration)** — a third inbound adapter exposes a single Thrift endpoint (`registerUser`) plus a matching Python client. It is intentionally *not* a full API — the HTTP and gRPC adapters already cover every resource — but a focused, end-to-end slice implemented to demonstrate working knowledge of Thrift: IDL design with a declared exception, code generation, a deliberate transport/protocol choice, mTLS, and cross-language interop. See the [Thrift API](#thrift-api) section.

**AWS ECS Fargate over EC2** — no servers to manage or patch. Tasks run across two private subnets (one per AZ) behind an ALB for high availability and zero-downtime rolling deploys. Application Auto Scaling adjusts the task count between 2 and 4 based on CPU utilization, keeping costs low under normal load while handling traffic spikes automatically.

**Keyless CI/CD via OIDC** — GitHub Actions assumes an AWS IAM role via OpenID Connect rather than using static credentials. No long-lived AWS access keys exist anywhere in the pipeline.

**Separate task execution role and task role** — the execution role has the minimum permissions needed to start a container (pull from ECR, write logs, read secrets). The task role holds only the permissions the running application needs. Compromise of one does not imply compromise of the other.

**Secrets Manager over environment variables** — `DB_PASSWORD`, `JWT_SECRET`, and the three mTLS secrets (`GRPC_TLS_CA`, `GRPC_TLS_CERT`, `GRPC_TLS_KEY`) are stored in AWS Secrets Manager and injected at container startup. They are never committed to source control or stored in CI.

**Mutual TLS on the gRPC server** — the gRPC server requires mTLS: both the server and any connecting client must present a certificate signed by the shared CA. This provides transport encryption, server identity verification, and client authentication at the network layer — without application-layer credentials. Self-signed certificates for local development are committed to `certs/`; in production the PEM strings are stored in Secrets Manager.

**Observability via CloudWatch** — ECS CPU/memory, RDS CPU/connections, and ALB 5xx error rate are monitored with CloudWatch alarms. Breaches trigger SNS email notifications, enabling rapid response to availability and performance issues.

## Architecture

The project uses **Hexagonal Architecture** (Ports & Adapters):

- **Domain layer** (`internal/domain/`) — pure Go business logic with no framework dependencies. Each resource (user, profile, article, comment, tag) has its own controller and repository interface.
- **HTTP inbound adapter** (`internal/adapters/in/webserver/`) — Gorilla Mux HTTP server. Handlers decode requests, call domain services, and encode responses. Authentication is handled by JWT middleware.
- **gRPC inbound adapter** (`internal/adapters/in/grpc/`) — native gRPC server backed by proto-generated stubs. A unary interceptor and a separate stream interceptor each handle auth (mandatory, optional, or none) per method. Both servers share the same domain controller instances — no business logic duplication.
- **Outbound adapter** (`internal/adapters/out/db/`) — PostgreSQL persistence via `sqlx`. Goose migrations run automatically on startup.

See [arch.md](arch.md) for a full description of every layer, route, schema, and design decision.

## CI/CD

Every push to `main` runs the GitHub Actions pipeline. It can also be triggered manually via the **Run workflow** button in the Actions tab.

### Pipeline stages

1. **HTTP integration tests** — checks out the [gothinkster/realworld](https://github.com/gothinkster/realworld) spec repo, installs Hurl, and runs the full HTTP API test suite (`make int-tests`).
2. **gRPC integration tests** — runs the Go e2e test suite in `test/grpc/` against a live server and test database (`make int-tests-grpc`).
3. **Build and push** — builds the Docker image and pushes it to Amazon ECR, tagged with the branch name, semver (on tagged releases), and `latest` (on `main`).
4. **Deploy** — triggers a rolling deployment on ECS Fargate by forcing a new deployment of the `realworld-service`. Only runs on pushes to `main`, not on tag pushes. ECS pulls the new `latest` image, starts new tasks, waits for them to pass the ALB health check at `GET /api/healthcheck`, then drains the old tasks.

### Infrastructure

The app runs on AWS in `ca-west-1` using the following services:

- **ECS Fargate** — runs the containerised Go app across two private subnets (one per AZ) for high availability; Application Auto Scaling scales tasks between 2 and 4 based on CPU utilization
- **Application Load Balancer** — receives inbound HTTP traffic on port 80 and forwards to Fargate tasks on port 8090
- **RDS PostgreSQL 17** — database in private subnets, only reachable from ECS tasks
- **ECR** — stores Docker images pushed by the CI pipeline
- **Secrets Manager** — holds `DB_PASSWORD`, `JWT_SECRET`, and the three mTLS PEM secrets (`GRPC_TLS_CA`, `GRPC_TLS_CERT`, `GRPC_TLS_KEY`), injected into containers at startup
- **CloudWatch Logs** — container stdout/stderr streamed to `/ecs/realworld` (30 day retention)
- **CloudWatch Alarms + SNS** — email alerts for ECS CPU/memory, RDS CPU/connections, and ALB 5xx error rate

All infrastructure is defined in Terraform under `terraform/`.

### Required secrets

| Secret | Description |
|---|---|
| `AWS_ROLE_ARN` | ARN of the IAM role assumed via OIDC for ECR push and ECS deploy access |

## How it was developed

Features were written as plain-English specification files (e.g. `features/9-create-article.md`). Each feature was implemented with the assistance of **Claude Code**, an AI coding tool. The workflow for each feature was:

1. Write a feature specification describing the required behaviour.
2. Review and guide Claude Code's implementation plan in `features/plans/`.
3. Review the implementation across all required layers.
4. Verify `make lint` reported no issues and `make int-tests` passed all integration tests.
5. Review updates to `arch.md` to keep the architecture document current.

The infrastructure was designed and debugged collaboratively with Claude Code — including VPC layout, IAM policy scoping, ECS service configuration, and resolving deployment issues.

## gRPC API

The server exposes a native gRPC API alongside the existing HTTP API. The port is configurable via the `GRPC_PORT` environment variable (production default: **8099**, test environment: **8098**). Service definitions live in `api/proto/` and the generated Go stubs are committed to `api/proto/gen/pb/`. To regenerate after editing a `.proto` file:

```bash
make proto
```

**Why run HTTP and gRPC as separate servers rather than using grpc-gateway?**

[grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) can translate HTTP/JSON requests into gRPC calls, which sounds appealing — one set of handlers serving both protocols. In practice, making the HTTP path spec-compliant with the RealWorld API spec required too many compromises:

- **Error body shape** — the spec requires `{"errors": {"field": ["message"]}}`. grpc-gateway produces its own JSON error envelope; matching the spec would require a custom error handler rewriting every error response.
- **Status code mismatches** — the spec requires HTTP 422 for validation errors and 409 for duplicates. gRPC's `codes.InvalidArgument` maps to HTTP 400, not 422, with no standard override.
- **Null field semantics** — `PUT /api/user` distinguishes `bio: null` (clear the field) from `bio` absent (leave unchanged). proto3 cannot represent this distinction, so the grpc-gateway HTTP path would silently drop the "clear" behaviour.

Running both servers independently avoids all of this. The existing HTTP server is already fully spec-compliant and integration-tested; the gRPC server provides a typed interface for native gRPC clients. Both delegate to the same domain layer, so there is no business logic duplication.

**Authentication**

Authenticated RPCs expect an `authorization` metadata key with value `Token <jwt>`. Methods that require authentication (`GetUser`, `UpdateUser`, `FollowUser`, `UnfollowUser`, `CreateArticle`, `UpdateArticle`, `FavoriteArticle`, `UnfavoriteArticle`, `DeleteArticle`, `FeedArticles`, `CreateComment`, `DeleteComment`, `LiveArticleFeed`) return `UNAUTHENTICATED` if the token is absent or invalid. Methods with optional auth (`GetProfile`, `GetArticleBySlug`, `ListArticles`, `GetComments`, `LiveCommentFeed`) proceed unauthenticated if no token is supplied. `RegisterUser`, `LoginUser`, and `GetTags` require no token.

Unary and server-streaming RPCs are authenticated by separate interceptors. The unary `AuthInterceptor` handles all request/response RPCs. The `StreamAuthInterceptor` handles the two streaming RPCs (`LiveArticleFeed`, `LiveCommentFeed`) using the same three-level scheme — it wraps the `ServerStream` to propagate the enriched context (with `UserIDKey` set) to the handler.

**Structured errors**

Every error returned by the gRPC server carries a standard [`google.rpc.Status`](https://github.com/googleapis/googleapis/blob/master/google/rpc/status.proto) with one or more typed detail messages attached, so clients can inspect structured fields rather than parsing the string message:

| Domain error | gRPC code | Detail type | Key fields |
|---|---|---|---|
| Validation failure | `INVALID_ARGUMENT` | `google.rpc.BadRequest` | `field_violations[].field`, `field_violations[].description` |
| Duplicate field | `ALREADY_EXISTS` | `google.rpc.BadRequest` | `field_violations[].field`, `field_violations[].description` |
| Bad credentials | `UNAUTHENTICATED` | `google.rpc.ErrorInfo` | `reason: "INVALID_CREDENTIALS"`, `domain: "realworld"` |
| Profile not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "profile"` |
| Article not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "article"` |
| Comment not found | `NOT_FOUND` | `google.rpc.ResourceInfo` | `resource_type: "comment"` |
| Forbidden | `PERMISSION_DENIED` | `google.rpc.ErrorInfo` | `reason: "PERMISSION_DENIED"`, `domain: "realworld"` |

The mapping lives in `internal/adapters/in/grpc/errors.go`. All four handler files (`user.go`, `article.go`, `profile.go`, `comment.go`) call the single `domainErr` helper instead of building status errors inline.

**Proto3 limitations vs the HTTP API**

- **`UpdateUser` — bio and image use a `NullableString` wrapper.** `optional string` cannot represent the three states needed (absent = leave unchanged, null = clear, value = set). Both fields use `optional NullableString` instead: omit the field to leave it unchanged, send `bio: {}` to clear it to null, or send `bio: { value: "hello" }` to set a value.
- **`UpdateArticle` — tag list uses a `TagListValue` wrapper.** `repeated string` cannot distinguish absent from empty. The field uses `optional TagListValue` instead: omit to leave tags unchanged, send `tag_list: {}` to clear them, or send `tag_list: { tags: ["go"] }` to replace them.

## Thrift API

The server exposes a third inbound adapter using **Apache Thrift**, on the port set by `THRIFT_PORT` (production default: **8100**, test environment: **8101**). It shares the same domain layer as the HTTP and gRPC adapters, so there is no business logic duplication.

**Scope — a deliberate, focused demonstration.** Only a single endpoint (`registerUser`) and a matching Python client are implemented. This is intentional: the RealWorld API is already fully served by the HTTP adapter (spec-compliant) and the gRPC adapter, so re-implementing every resource in Thrift would add maintenance cost without demonstrating anything new. Instead, this adapter is a complete end-to-end vertical slice that exercises the parts of Thrift that matter — IDL design, a declared exception, code generation, transport/protocol selection, mTLS, and cross-language interop — to show working knowledge of the technology.

The service is defined in `api/thrift/user.thrift`; generated Go stubs are committed to `api/thrift/gen/thriftpb/`. To regenerate:

```bash
make thrift      # Go server stubs
make thrift-py   # Python client stubs → clients/python/
```

**Transport and protocol — binary over TCP.** The server uses Thrift's compact **binary protocol over a raw TCP socket** (buffered transport), which is fast and space-efficient for internal, service-to-service RPC. This is a deliberate choice with a trade-off: a raw TCP socket is **not reachable from a browser**, and Thrift's JavaScript client speaks `TJSONProtocol` over HTTP rather than the RealWorld JSON contract. Browser and REST consumers are therefore served by the HTTP adapter; Thrift is positioned as the internal RPC layer where its efficiency and typed contract are the advantage. The client must match the server's transport/protocol pairing exactly.

**Transport security.** The Thrift server requires the same mutual TLS as the gRPC server, sharing a single `tls.Config` and the `certs/` chain (see [Generating dev TLS certificates](#generating-dev-tls-certificates)).

**Structured errors.** The IDL declares one exception — `exception ValidationError { map<string, list<string>> errors }`. The adapter's `domainErr` helper (`internal/adapters/in/thrift/errors.go`) folds both `*domain.ValidationError` and `*domain.DuplicateError` into it, keyed by field name (matching the RealWorld 422 shape); any other error is returned as a generic `TApplicationException`. Unlike the gRPC adapter — which maps each domain error to a distinct status code — Thrift can only surface *declared* exceptions structurally, so this folding is deliberate; distinguishing further error types would mean adding exceptions to the IDL.

**Polyglot client.** A Python client lives in `clients/python/` — generated stubs plus a hand-written `client.py` that registers a user over the same binary/buffered/mTLS stack. This is the cross-language interop demonstration that is Thrift's core strength: one IDL, a Go server, and a Python client. (Note that Thrift keeps the IDL method name verbatim per language, so the Python call is `client.registerUser(...)` versus Go's `RegisterUser`.)

## Running the app

**Prerequisites:** Docker, Go 1.21+

```bash
make start
```

This will:
1. Start the production PostgreSQL database via Docker Compose
2. Wait until the database is ready
3. Build the server binary
4. Start the server on port **8090**

Stop the server with Ctrl+C. To also stop the database:

```bash
docker compose down
```

## Running the integration tests

**Prerequisites:** Docker, Go 1.21+, [Hurl](https://hurl.dev), and the [realworld](https://github.com/gothinkster/realworld) repo checked out as a sibling directory (`../realworld`).

```bash
make int-tests
```

This will:
1. Start a dedicated test database on port 8096
2. Build and start the server against the test environment (port 8097)
3. Run the full RealWorld Hurl API test suite
4. Tear down the server and test database

## Running the gRPC integration tests

**Prerequisites:** Docker, Go 1.21+, and the dev TLS certificates (see below).

```bash
make int-tests-grpc
```

This will:
1. Start a dedicated test database on port 8096
2. Build and start the server against the test environment (ports 8097/8098)
3. Run the full gRPC test suite in `test/grpc/`
4. Truncate the test database and tear it down

The suite covers all gRPC endpoints across ten test files:

| File | What it tests |
|---|---|
| `auth_test.go` | Register, login, get user, update bio/image/username/email, nullable field semantics |
| `articles_test.go` | Create, list (all/by-author/by-tag), get, update, tag list patch, delete |
| `comments_test.go` | Create, list (authed/unauthed), delete, selective deletion |
| `profiles_test.go` | Get profile (authed/unauthed), follow, unfollow, persist check |
| `tags_test.go` | Create article with tags, verify tags appear in `GetTags` |
| `feed_test.go` | Empty feed, follow author, feed count/author, pagination |
| `favorites_test.go` | Favorite, get as favoriter/non-favoriter, list by favorited, unfavorite |
| `pagination_test.go` | Limit/offset combinations, empty page total count, most-recent-first order |
| `errors_test.go` | Missing fields, duplicates, wrong password, `NotFound`, `PermissionDenied`, `Unauthenticated` |
| `streaming_test.go` | `LiveArticleFeed` (auth required, filters to followed authors); `LiveCommentFeed` authenticated (`following: true` for followed authors) and unauthenticated (`following: false`), plus per-slug isolation |

### Generating dev TLS certificates

The gRPC and Thrift servers require mTLS. Self-signed certificates for local development are committed to `certs/` (public certs only — private keys are in `.gitignore`). To regenerate the full chain (CA, server, client), run the script from the repo root:

```bash
./certs/gen-certs.sh
```

It writes `ca.crt`, `server.crt`, and `client.crt` (plus their `.key` files) into `certs/` and verifies the chain. After regenerating, **restart any running server** so it reloads the new certs.

**Note on X.509 extensions.** An earlier version of these certs was generated without `keyUsage` / `extendedKeyUsage` extensions (and a CA with no `keyUsage` at all). Go's TLS verifier and `openssl s_client` accept such minimal certs, so the gRPC server worked fine — but **strict RFC 5280 verifiers reject them**. The Thrift Python client (Python 3.14 + OpenSSL 3.6) failed its TLS handshake with `certificate verify failed: CA cert does not include key usage extension`, which surfaced misleadingly as a `bad record MAC` error in the Go server logs. The fix — baked into `gen-certs.sh` — is to issue standards-compliant certs:

| Cert | Required extensions |
|---|---|
| CA | `basicConstraints=CA:TRUE`, `keyUsage=keyCertSign,cRLSign` |
| Server | `keyUsage=digitalSignature,keyEncipherment`, `extendedKeyUsage=serverAuth`, `subjectAltName=DNS:localhost,IP:127.0.0.1` |
| Client | `keyUsage=digitalSignature`, `extendedKeyUsage=clientAuth` |

These certs are for local development only. They are self-signed and trusted only within this dev environment. In production, TLS is handled at the AWS infrastructure layer using certificates managed outside the codebase.

### Why Go instead of shell scripts or Bruno

The gRPC test suite started as shell scripts using `grpcurl` piped into `jq`. This turned out to be the wrong tool for the job for several reasons:

**Proto3 zero-value omission.** gRPC JSON encoding omits fields set to their zero value — `false` booleans and `0` integers simply don't appear in the JSON output. Every boolean or counter assertion required a `// false` or `// 0` jq fallback to avoid silent false-positives, and any assertion that didn't have one would incorrectly pass.

**Shell fragility.** Two separate bugs surfaced within the first test run:
- `UID` is a read-only variable in bash and zsh; using it as a test identifier caused every test to fail immediately with `UID: readonly variable`.
- `${2:-{}}` (a common pattern for defaulting a missing argument to `{}`) silently appends a spurious `}` to the argument when it is set, because `}` closes the parameter expansion before the literal `}` is consumed. This made every JSON request body malformed and every `grpcurl` call fail silently.

**Bruno** requires the desktop application for the full authoring experience and has limited CI integration. Its collection format is designed around HTTP; gRPC support is present but less complete.

**Go** solves all of this cleanly. The generated proto client stubs use native Go types, so zero values (`false`, `0`) are just zero values — no JSON serialization workarounds. `t.Fatalf`, `t.Cleanup`, and build tags (`//go:build integration`) are standard. The compiler catches type mismatches against the proto contract before the test even runs. The tests live in the same repository and run with `go test`, requiring no external binaries beyond the running server.

## Running the linter

```bash
make lint
```
