# cmd/ Directory Documentation

This directory contains the entry points for the Go application binaries.

## Files
- `cmd/api/main.go`: Main execution entrypoint for the HTTP API server.

## Lifecycle Details
1. **Logging Initialization**: Initialized first using Zap.
2. **Configuration Loading**: Parses variables using `caarlos0/env`. If variables are invalid, execution fails immediately.
3. **Database Client Pool**: Connection pool configured with limits.
4. **Redis Connection**: Establish client wrapper with instant ping validation.
5. **Validator Registering**: Plugs custom playground-validator into Echo.
6. **Graceful Shutdown**:
   - Captures OS signals `SIGINT` and `SIGTERM`.
   - Stops the Echo server with a 10-second timeout context allowing running requests to finish.
   - Safely closes Postgres connection pool.
   - Safely closes Redis connection pool.
