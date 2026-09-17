-include .env
export

.PHONY: all help setup \
	build build-api build-worker build-migrate \
	run worker dev test fmt lint vulncheck \
	sqlc migrate-status migrate-up migrate-down \
	docker-build docker-up docker-down docker-down-v docker-logs docker-restart docker-migrate \
	dev-up dev-down dev-logs dev-restart \
	clean

all: build

help:
	@echo "Cookoff 11.0 Backend -- available targets:"
	@echo ""
	@echo "  setup            Configure git hooks and install dev tools"
	@echo "  build            Build all binaries (api, worker, migrate) into bin/"
	@echo "  run              Run the API server locally (no hot reload)"
	@echo "  worker           Run the worker locally (no hot reload)"
	@echo "  dev              Run the API with hot reload via air"
	@echo "  test             Run go test ./..."
	@echo "  fmt              Run go fmt ./..."
	@echo "  lint             Run golangci-lint"
	@echo "  vulncheck        Run govulncheck"
	@echo "  sqlc             Regenerate sqlc code"
	@echo "  migrate-up       Apply pending goose migrations"
	@echo "  migrate-down     Roll back the last goose migration"
	@echo "  migrate-status   Show goose migration status"
	@echo "  docker-up        Build and start the full prod-mode stack"
	@echo "  docker-down      Stop the prod-mode stack"
	@echo "  docker-down-v    Stop the prod-mode stack and remove volumes"
	@echo "  docker-logs      Tail logs for the prod-mode stack"
	@echo "  docker-migrate   Run the migrate service once against the stack"
	@echo "  dev-up           Build and start the hot-reload dev stack"
	@echo "  dev-down         Stop the hot-reload dev stack"
	@echo "  dev-logs         Tail logs for the hot-reload dev stack"
	@echo "  clean            Remove build artifacts (bin/, tmp/)"

setup:
	@echo "Configuring git hooks..."
	@git config core.hooksPath .githooks
	@echo "Installing development tools..."
	@go install github.com/air-verse/air@v1.63.0
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Done. Setup completed successfully."

# --- Build ---

build: build-api build-worker build-migrate

build-api:
	@echo "Building api binary..."
	@go build -o bin/api ./cmd/api

build-worker:
	@echo "Building worker binary..."
	@go build -o bin/worker ./cmd/worker

build-migrate:
	@echo "Building migrate binary..."
	@go build -o bin/migrate ./cmd/migrate

# --- Run locally (no containers) ---

run:
	@echo "Running local API server..."
	@go run ./cmd/api

worker:
	@echo "Running local worker..."
	@go run ./cmd/worker

dev:
	@echo "Running hot-reload dev server with air..."
	@air -c .air.toml

# --- Quality ---

test:
	@echo "Running tests..."
	@go test -v ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...

lint:
	@echo "Running golangci-lint..."
	@golangci-lint run --config=.golangci.yml

vulncheck:
	@echo "Running govulncheck..."
	@govulncheck ./...

# --- Database ---

sqlc:
	@echo "Generating sqlc code..."
	@sqlc generate

migrate-status:
	@go run ./cmd/migrate status

migrate-up:
	@go run ./cmd/migrate up

migrate-down:
	@go run ./cmd/migrate down

# --- Docker: production-mode stack (postgres, redis, migrate, api, worker) ---

docker-build:
	@echo "Building docker images..."
	@docker compose build

docker-up:
	@echo "Starting docker services..."
	@docker compose up --build -d

docker-down:
	@echo "Stopping docker services..."
	@docker compose down

docker-down-v:
	@echo "Stopping docker services and removing volumes..."
	@docker compose down -v

docker-logs:
	@docker compose logs -f

docker-restart: docker-down docker-up

docker-migrate:
	@echo "Running migrations against the docker stack..."
	@docker compose run --rm migrate up

# --- Docker: hot-reload dev stack ---

dev-up:
	@echo "Starting hot-reload dev stack..."
	@docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build -d

dev-down:
	@echo "Stopping hot-reload dev stack..."
	@docker compose -f docker-compose.yml -f docker-compose.dev.yml down

dev-logs:
	@docker compose -f docker-compose.yml -f docker-compose.dev.yml logs -f

dev-restart: dev-down dev-up

# --- Cleanup ---

clean:
	@echo "Removing build artifacts..."
	@rm -rf bin/ tmp/
