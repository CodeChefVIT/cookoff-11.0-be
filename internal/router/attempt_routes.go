package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/services"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/labstack/echo/v4"
)

func RegisterAttemptRoutes(
	e *echo.Echo) {
	queries := db.New(utils.DBPool)
	attemptService := services.NewAttemptService(
		utils.DBPool,
		queries)
	attemptController := controllers.NewAttemptController(attemptService)
	e.POST("/attempts/:id", attemptController.CreateAttempt)

}
