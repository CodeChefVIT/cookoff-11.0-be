# internal/middlewares/ Directory Documentation

Middlewares intercept incoming HTTP requests before reaching controllers to perform actions like logging, recovery, authentication, and rate limiting.

## Built-In Middlewares
1. **Zap Logger Middleware** (`logger.go`): Intercepts the request and logs HTTP method, URI, response status, latency, real client IP, and any encountered errors using the production-ready Zap logger.
2. **JWT Authentication Middleware** (`jwt.go`): Validates incoming `Authorization: Bearer <token>` headers. Parses token claims, validates signatures and expiration times, and saves claims into Echo context (`c.Set("user", claims)`).
3. **CORS Middleware**: Mounted globally in `main.go`. Configured using CORS origins from `.env` configuration.
4. **Recovery Middleware**: Standard Echo recovery middleware protecting the server from unexpected runtime panics by logging stack traces and returning 500 Internals.
