package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/workers"
	"github.com/hibiken/asynq"
)


const workerConcurrency = 10

func main() {
	
	logging.InitLogger()

	if err := utils.LoadConfig(); err != nil {
		logging.Fatalf("Failed to load configuration: %v", err)
	}

	db.InitDB()
	defer db.CloseDB()

	
	utils.InitTokenCache()
	defer utils.CloseTokenCache()


	srv := asynq.NewServer(
		queue.RedisOpt(),
		asynq.Config{
			Concurrency: workerConcurrency,
		},
	)

	mux := workers.NewServeMux()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logging.Infof("Received shutdown signal: %s, stopping worker...", sig)
		srv.Shutdown()
	}()

	logging.Infof("Starting Asynq worker (concurrency=%d)", workerConcurrency)

	if err := srv.Run(mux); err != nil {
		logging.Fatalf("Worker server error: %v", err)
	}

	logging.Infof("Worker shut down successfully")
}