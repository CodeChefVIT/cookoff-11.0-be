<div align="center">
  <a href="https://www.codechefvit.com" target="_blank">
    <img src="https://i.ibb.co/4J9LXxS/cclogo.png" width="160" title="CodeChef-VIT" alt="CodeChef-VIT">
  </a>

  <h1>CookOff 11.0 Backend</h1>

  <p>
    A production-level, highly scalable Go backend built using the <b>Echo v4</b> HTTP framework. It utilizes connection pooled <b>PostgreSQL</b> via <code>pgx/v5</code> and compiled type-safe database queries via <b>SQLC</b>, backed by <b>Redis</b> and structured logging via <b>Zap</b>.
  </p>

  <p>
    <a href="https://github.com/CodeChefVIT/cookoff-11.0-be">
      <img src="https://img.shields.io/badge/STATUS-LIVE-green?style=for-the-badge" alt="Live">
    </a>
    <a href="https://github.com/CodeChefVIT/cookoff-11.0-be/pulls">
      <img src="https://img.shields.io/badge/PRs-WELCOME-blue?style=for-the-badge" alt="PRs Welcome">
    </a>
  </p>
</div>

---


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

### Deploying

- **CORS and cookies:** `FRONTEND_URL` and `ADMIN_URL` must be the exact origins of the portal and admin panel (scheme, host, no path); only those origins can call the API. Keep both on a `codechefvit.com` subdomain so the session cookies are same-site; `COOKIE_DOMAIN` can stay unset.
- **Reverse proxy:** `GET /result/:id` holds the request for up to 90s while a verdict is judged, and `/runcode` / `/runcustom` for up to 45s. Nginx's default `proxy_read_timeout` (60s) cuts `/result` short, so set it to at least 100s and turn buffering off for it:

  ```nginx
  location /result/ {
      proxy_pass http://127.0.0.1:8080;
      proxy_read_timeout 100s;
      proxy_buffering off;
  }
  ```

  Forward the client address (`proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`); behind Cloudflare the API uses `CF-Connecting-IP`.
- **Tuning:** `RATE_LIMIT_MAX`/`RATE_LIMIT_WINDOW` (per player, all routes), `JUDGE0_WAIT_SLOTS` (concurrent synchronous Judge0 calls; size it to the Judge0 worker count), `WORKER_COUNT` and `POSTGRES_MAX_CONNS`. See `.env.example`.

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

---

## 🔗 Related Projects

- **Participant Portal:** [`cookoff-portal-11.0`](https://github.com/CodeChefVIT/cookoff-portal-11.0)
- **Admin Portal:** [`cookoff-admin-11.0`](https://github.com/CodeChefVIT/cookoff-admin-11.0)

These services work together to provide the complete Cookoff 11.0 contest infrastructure.

---

## 🚀 Contributors

<table align="center">
<tr align="center">

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/YOGESH-08" width="150" height="150" alt="YOGESH-08">
</p>
<p align="center">
<a href="https://github.com/YOGESH-08" target="_blank">Yogesh Kumar</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/mharshil1234" width="150" height="150" alt="Harshil Maheshwari">
</p>
<p align="center">
<a href="https://github.com/mharshil1234" target="_blank">Harshil Maheshwari</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/lakshraja" width="150" height="150" alt="lakshraja">
</p>
<p align="center">
<a href="https://github.com/lakshraja" target="_blank">Laksh Raja</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/namitg105" width="150" height="150" alt="namitg105">
</p>
<p align="center">
<a href="https://github.com/namitg105" target="_blank">Namit Gupta</a>
</p>
</td>

</tr>

<tr align="center">

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/Xen-org" width="150" height="150" alt="Xen-org">
</p>
<p align="center">
<a href="https://github.com/Xen-org" target="_blank">Tejas Maurya</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/atharvaSharma17" width="150" height="150" alt="atharvaSharma17">
</p>
<p align="center">
<a href="https://github.com/atharvaSharma17" target="_blank">Atharva Sharma</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/VPK570" width="150" height="150" alt="VPK570">
</p>
<p align="center">
<a href="https://github.com/VPK570" target="_blank">VP Krishna</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/Radical11" width="150" height="150" alt="Radical11">
</p>
<p align="center">
<a href="https://github.com/Radical11" target="_blank">Vihaan Jain</a>
</p>
</td>

</tr>

<tr align="center">

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/upayanmazumder" width="150" height="150" alt="Upayan Mazumder">
</p>
<p align="center">
<a href="https://github.com/upayanmazumder" target="_blank">Upayan Mazumder</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/NeharikaChinnappa" width="150" height="150" alt="NeharikaChinnappa">
</p>
<p align="center">
<a href="https://github.com/NeharikaChinnappa" target="_blank">Neharika Chinnappa</a>
</p>
</td>

<td>
<p align="center">
<img src="https://avatars.githubusercontent.com/Rithish-2914" width="150" height="150" alt="Rithish-2914">
</p>
<p align="center">
<a href="https://github.com/Rithish-2914" target="_blank">Rithish</a>
</p>
</td>

</tr>
</table>
---

## 📝 License

Distributed under the MIT License. See `LICENSE` for more information.

---

<p align="center">
  Made with ❤️ by <a href="https://www.codechefvit.com" target="_blank">CodeChef-VIT</a>
</p>
