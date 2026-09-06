package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)

	questionController := controllers.NewQuestionController(db.New(db.DBPool))
	questionRoutes := e.Group("/question", middlewares.JWTAuth)
	// Add BanCheckUser to this group when the shared authorization middleware lands.
	questionRoutes.GET("/round", questionController.ListByRound)
	questionRoutes.GET("/:id", questionController.GetByID)
	questionRoutes.GET("/:id/blocks", questionController.ListBlocks)
}
