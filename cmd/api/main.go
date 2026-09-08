package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/router"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/labstack/echo/v4"
	emiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize logger
	logging.InitLogger()

	// Load configuration
	if err := utils.LoadConfig(); err != nil {
		logging.Fatalf("Failed to load configuration: %v", err)
	}
    
	// Initialize Asynq queue client
	queue.InitQueue()
	defer queue.CloseQueue()

	// Initialize DB pool
	db.InitDB()
	defer db.CloseDB()

	// Initialize Redis
	utils.InitRedis()
	defer utils.CloseRedis()

	// Initialize Echo instance
	e := echo.New()

	// Initialize Token Cache (Redis)
	utils.InitTokenCache()
	defer utils.CloseTokenCache()

	// Register request validator
	e.Validator = utils.NewValidator()

	// Setup middlewares
	e.Use(emiddleware.Recover())
	e.Use(middlewares.Logger)
	e.Use(emiddleware.Secure())
	e.Use(emiddleware.RateLimiter(emiddleware.NewRateLimiterMemoryStore(rate.Limit(20))))
	e.Use(emiddleware.BodyLimit("10M"))

	// Configure CORS
	if len(utils.CORSOrigins) > 0 {
		e.Use(emiddleware.CORSWithConfig(emiddleware.CORSConfig{
			AllowOrigins:     utils.CORSOrigins,
			AllowMethods:     []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete},
			AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
			AllowCredentials: true,
		}))
		logging.Infof("CORS enabled for origins: %v", utils.CORSOrigins)
	}

	// Register routes
	router.RegisterRoutes(e)

	// Start server in goroutine to allow graceful shutdown
	serverErrCh := make(chan error, 1)
	go func() {
		logging.Infof("Starting HTTP server on port %s", utils.Config.Port)

		srv := &http.Server{
			Addr:         ":" + utils.Config.Port,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		if err := e.StartServer(srv); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Graceful shutdown setup
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logging.Infof("Received shutdown signal: %s", sig)
	case err := <-serverErrCh:
		logging.Fatalf("HTTP server error: %v", err)
	}

	// Shutdown Echo context with configurable timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logging.Errorf("Failed to gracefully shutdown HTTP server: %v", err)
	} else {
		logging.Infof("HTTP server shut down successfully")
	}

	logging.Infof("Shutdown process completed")
}
