# Documentation

This directory contains system architecture specifications, OpenAPI schemas, HLD/LLD documents, and API documentation for the project.

## Running the Server

### Local Development
1. Start PostgreSQL and Redis instances.
2. Edit `.env` values (copied from `.env.example`).
3. Run the development server with hot-reloading:
   ```bash
   make dev
   ```
4. Run standard Go server:
   ```bash
   make run
   ```

### Docker Development
Production-mode stack (postgres, redis, migrate, api, worker):
```bash
make docker-up
```
Hot-reload dev stack (bind-mounted source, air):
```bash
make dev-up
```

## Interactive API Docs
Once running, visit `http://localhost:8080/docs` to view the interactive Scalar API documentation UI.
