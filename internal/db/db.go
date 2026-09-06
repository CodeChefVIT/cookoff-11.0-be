package db

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DBPool *pgxpool.Pool

type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Queries struct {
	db DBTX
}

func (q *Queries) WithTx(tx pgx.Tx) *Queries {
	return &Queries{
		db: tx,
	}
}

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

	logging.Infof("Database connection pool initialized successfully")
}

func CloseDB() {
	if DBPool != nil {
		DBPool.Close()
		logging.Infof("Database connection pool closed")
	}
}
