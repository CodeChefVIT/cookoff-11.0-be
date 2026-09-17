# syntax=docker/dockerfile:1.7
#
# Multi-stage, multi-target production Dockerfile.
# Builds all three binaries (api, worker, migrate) from one builder stage,
# then ships each as its own minimal, non-root runtime image:
#
#   docker build --target api      -t cookoff-api      .
#   docker build --target worker   -t cookoff-worker   .
#   docker build --target migrate  -t cookoff-migrate  .
#
# `docker build .` with no --target defaults to the last stage (api).

########################################
# Builder
########################################
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git build-base

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/api
RUN go build -trimpath -ldflags "-s -w" -o /out/worker ./cmd/worker
RUN go build -trimpath -ldflags "-s -w" -o /out/migrate ./cmd/migrate

########################################
# Shared minimal runtime base
########################################
FROM alpine:3.21 AS base

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app
USER app
ENV ENV=production

########################################
# migrate: one-shot goose migration runner
########################################
FROM base AS migrate

COPY --from=builder --chown=app:app /out/migrate ./migrate
COPY --from=builder --chown=app:app /src/database/schema ./database/schema

ENTRYPOINT ["./migrate"]
CMD ["up"]

########################################
# worker: asynq job consumer
########################################
FROM base AS worker

COPY --from=builder --chown=app:app /out/worker ./main

ENTRYPOINT ["./main"]

########################################
# api: HTTP server (default target)
########################################
FROM base AS api

USER root
RUN apk add --no-cache curl
USER app

COPY --from=builder --chown=app:app /out/api ./main
COPY --from=builder --chown=app:app /src/docs ./docs

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/health || exit 1

ENTRYPOINT ["./main"]
