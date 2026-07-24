# internal/controllers/ Directory Documentation

Controllers (Handlers) receive incoming Echo requests, invoke database queries, validate payloads, and return standardized JSON responses.

## Controllers Included
- `docs.go`: Integrates Scalar API documentation and serves it dynamically on `/docs`.
- `health.go`: Runs connection status pings against Postgres and Redis in parallel. Returns 200 if OK, 503 if any service is down.

## Design Rules
1. **Binding & Validation**: Always check `c.Bind` and `c.Validate` before doing any processing.
2. **Success Responses**: Wrap returning payloads in `dto.NewSuccessResponse(message, data)`.
3. **Error Responses**: Return errors using `dto.NewErrorResponse(message, errors)` with appropriate HTTP status codes.
4. **Context**: Pass `c.Request().Context()` into DB operations to handle request cancellation propagation.
