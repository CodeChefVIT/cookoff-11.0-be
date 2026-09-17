package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type tracerCtxKey struct{}

type tracerData struct {
	sql       string
	startTime time.Time
}

type sqlQueryTracer struct{}

func (t *sqlQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracerCtxKey{}, tracerData{
		sql:       data.SQL,
		startTime: time.Now(),
	})
}

func (t *sqlQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if td, ok := ctx.Value(tracerCtxKey{}).(tracerData); ok {
		duration := time.Since(td.startTime)
		if data.Err != nil {
			logging.Errorf("SQL Error [%s]: %v (latency: %v)", td.sql, data.Err, duration)
		} else {
			logging.Debugf("SQL [%s] (latency: %v)", td.sql, duration)
		}
	}
}

var (
	DBPool  *pgxpool.Pool
	Queries *sqlc.Queries
)

func InitDB() {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		utils.Config.PostgresUser,
		utils.Config.PostgresPassword,
		utils.Config.PostgresHost,
		utils.Config.PostgresPort,
		utils.Config.PostgresDB,
	)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logging.Fatalf("Unable to parse database DSN: %v", err)
	}

	if os.Getenv("DEBUG_SQL") == "true" || os.Getenv("ENV") == "development" {
		config.ConnConfig.Tracer = &sqlQueryTracer{}
		logging.Infof("Enabled SQL query tracer for pgx")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	DBPool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logging.Fatalf("Failed to create database connection pool: %v", err)
	}

	if err := DBPool.Ping(ctx); err != nil {
		logging.Fatalf("Failed to ping database: %v", err)
	}

	Queries = sqlc.New(DBPool)

	logging.Infof("Database connection pool initialized successfully")
}

func CloseDB() {
	if DBPool != nil {
		DBPool.Close()
		logging.Infof("Database connection pool closed")
	}
}
