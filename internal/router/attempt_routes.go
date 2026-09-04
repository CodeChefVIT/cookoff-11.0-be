package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	db "github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/services"
	"github.com/labstack/echo/v4"
)

func RegisterAttemptRoutes(e *echo.Echo) {
	queries := sqlc.New(db.DBPool)

	attemptService := services.NewAttemptService(
		db.DBPool,
		queries,
	)

	attemptController := controllers.NewAttemptController(attemptService)

	e.POST("/attempts/:id", attemptController.CreateAttempt)
}
