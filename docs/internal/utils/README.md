# internal/utils/ Directory Documentation

This directory contains utility modules providing clients, configuration, and validation configuration.

## Files
- `config.go`: Parses environment variables using `caarlos0/env` and `.env` parsing.
- `db.go`: Configures connection pooling using `pgxpool.Pool` for high concurrency PostgreSQL.
- `redis.go`: Creates and exports the Redis Client instance (`RedisClient`). Includes production-grade helpers:
  - `SetCache(ctx, key, value, expiration)`: Marshals any Go object into JSON and stores it in Redis.
  - `GetCache(ctx, key, &dest)`: Retrieves a key from Redis and unmarshals the JSON back into the destination pointer.
  - `DeleteCache(ctx, key)`: Invalidates/deletes a key from the cache.
- `validator.go`: Implements custom validation adapter for Echo using the playground validator engine.
