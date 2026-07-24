# internal/db/ Directory Documentation

This folder contains generated database models and queries created by SQLC, mirroring the database structure.

## Database Migrations
Migrations are written in `database/schema` in SQL format, executed via Goose.
- Running migrations up: `make migrate-up`
- Rolling back: `make migrate-down`

## SQLC Code Generation
1. Schema files in `database/schema/` define the tables.
2. Query files in `database/queries/` define raw SQL queries with SQLC parameter annotations.
3. Run `make sqlc` or `sqlc generate` to compile them into typesafe Go code inside `internal/db`.

## Transaction Guidelines
For queries needing database transactions:
```go
tx, err := utils.DBPool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

qtx := db.New(utils.DBPool).WithTx(tx)
// Perform operations using qtx...

err = tx.Commit(ctx)
```

---

## ⚠️ Production Migration Warning
In production environments (e.g., Kubernetes, AWS ECS, or multiple server replicas):
1. **Do not** trigger migrations directly inside the application binary startup entrypoint. Running multiple instances of the server concurrently can lead to migrations executing simultaneously, causing transaction deadlocks or schema corruption.
2. **Do** execute Goose migrations as a single-replica job, init-container, or a dedicated deployment pipeline step *before* releasing the new application pods.
