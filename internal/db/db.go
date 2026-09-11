package db

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
