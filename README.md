# Cookoff 11.0 Backend

A production-level, highly scalable Go backend built using the **Echo v4** HTTP framework. It utilizes connection pooled **PostgreSQL** via `pgx/v5` and compiled type-safe database queries via **SQLC**, backed by **Redis** and structured logging via **Zap**.

## Tech Stack

- **HTTP Framework**: [Echo v4](https://echo.labstack.com/)
- **Database Client**: [pgx/v5](https://github.com/jackc/pgx) with connection pooling (`pgxpool`)
- **Query Generator**: [SQLC](https://sqlc.dev/) for compilation-time type-safe database methods
- **Database Migrations**: [Goose](https://github.com/pressly/goose)
- **Configuration Parsing**: [caarlos0/env/v11](https://github.com/caarlos0/env) with `.env` loading using `godotenv`
- **Structured Logging**: [Uber Zap](https://github.com/uber-go/zap)
- **Caching**: [go-redis/v9](https://github.com/redis/go-redis)
- **Validation**: [go-playground/validator/v10](https://github.com/go-playground/validator)
- **Hot-Reloading**: [Air](https://github.com/air-verse/air)
- **API UI Documentation**: [Scalar API Reference](https://github.com/MarceloPetrucio/go-scalar-api-reference)

---

## Directory Structure

```
├── cmd/
│   ├── api/                    # HTTP server entrypoint & graceful shutdown
│   ├── worker/                 # Asynq job consumer entrypoint
│   └── migrate/                # Goose migration CLI entrypoint
├── internal/
│   ├── controllers/           # HTTP handlers
│   ├── db/                    # SQLC generated code
│   ├── dto/                   # Request/Response structures & validation
│   ├── logging/               # Structured logging using Zap
│   ├── middlewares/           # Custom middlewares (JWT, Custom logger)
│   ├── queue/                 # Asynq client (producer side)
│   ├── workers/                # Asynq task handlers (consumer side)
│   └── helpers/utils/         # DB Pool, Redis, config, validator
├── database/
│   ├── queries/               # SQLC raw queries
│   └── schema/                # Goose SQL migrations
├── docs/                      # Documentation mirror explaining files
└── tests/                     # HTTP and unit test cases
```

---

## Getting Started

### Local Setup

1. Make sure you have **PostgreSQL** and **Redis** running locally.
2. Create your `.env` file from the example:
   ```bash
   cp .env.example .env
   ```
3. Initialize the database schema using Goose:
   ```bash
   make migrate-up
   ```
4. Run the application with hot-reloading:
   ```bash
   make dev
   ```
5. In a separate shell, run the worker:
   ```bash
   make worker
   ```

### Docker Setup

There are two Docker Compose stacks, both defined by the same `docker-compose.yml`
and a `docker-compose.dev.yml` overlay:

**Production-mode stack** (`Dockerfile`, compiled binaries, one-shot migration job):
```bash
make docker-up      # build + start postgres, redis, migrate, api, worker
make docker-logs    # tail logs
make docker-down    # stop
```
The `migrate` service runs `goose up` once and exits; `api` and `worker` wait for
it to complete successfully before starting.

**Hot-reload dev stack** (`Dockerfile.dev`, air, bind-mounted source):
```bash
make dev-up         # build + start the same stack with live reload
make dev-logs
make dev-down
```

---

## Interactive Documentation

Once the backend is running, navigate to:
```
http://localhost:8080/docs
```
to view the interactive API reference served via Scalar.

---

## Makefile Automation

- `make build`: Compile all binaries (`api`, `worker`, `migrate`) into `bin/`.
- `make run`: Run the API server locally.
- `make worker`: Run the worker locally.
- `make dev`: Run the API with hot-reloading (`air`).
- `make test`: Run automated tests.
- `make lint`: Run code formatting and static analysis checkers (`golangci-lint`).
- `make vulncheck`: Scan dependencies for known security vulnerabilities (`govulncheck`).
- `make sqlc`: Compile SQL queries into Go code using SQLC.
- `make migrate-up` / `make migrate-down` / `make migrate-status`: Manage Goose database migrations.
- `make docker-up` / `make docker-down` / `make docker-logs`: Manage the production-mode Docker stack (postgres, redis, migrate, api, worker).
- `make docker-migrate`: Run the one-shot migration job against the Docker stack.
- `make dev-up` / `make dev-down` / `make dev-logs`: Manage the hot-reload Docker dev stack.
- `make clean`: Remove build artifacts (`bin/`, `tmp/`).
- `make help`: List all available targets.

---

## 🔄 Workflow & Code Quality

### Git Hooks (Commit Message Validation)
To enforce professional commit names (following the Conventional Commits spec), run the following command to bind the git hooks path:
```bash
git config core.hooksPath .githooks
```
*(On Linux/macOS, ensure the hook is executable: `chmod +x .githooks/commit-msg`)*

### CI/CD Pipeline
Every Pull Request and commit pushed to `main`/`master`/`dev` triggers a GitHub Actions pipeline validating:
- Proper Go code formatting (`gofmt`).
- Static analysis checks (`go vet`).
- Code styling & security linters (`golangci-lint`).
- Automated tests suite completion (`go test`).
- Go application compilation health (`go build`).
