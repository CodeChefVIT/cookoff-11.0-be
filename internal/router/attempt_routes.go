package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	db "github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/labstack/echo/v4"
)

func RegisterAttemptRoutes(e *echo.Echo) {
	queries := sqlc.New(db.DBPool)

	attemptController := controllers.NewAttemptController(
		db.DBPool,
		queries,
	)
	e.POST("/attempts/:id", attemptController.CreateAttempt)
}
