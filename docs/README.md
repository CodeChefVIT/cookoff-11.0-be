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
1. Build and run via Docker Compose:
   ```bash
   docker compose up --build
   ```

## Interactive API Docs
Once running, visit `http://localhost:8080/docs` to view the interactive Scalar API documentation UI.
