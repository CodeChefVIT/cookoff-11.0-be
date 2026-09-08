package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)

	// judge0 callback req
	e.PUT("/judge0callback", controllers.Judge0Callback)

	// submit & result routes
	e.POST("/submit", controllers.SubmitCode)
	e.GET("/result/:submission_id", controllers.GetResult)

	RegisterAttemptRoutes(e)
	RegisterVisualSubmissionRoutes(e)

	questionController := controllers.NewQuestionController(sqlc.New(db.DBPool))
	questionRoutes := e.Group("/question", middlewares.JWTAuth)
	// Add BanCheckUser to this group when the shared authorization middleware lands.
	questionRoutes.GET("/round", questionController.ListByRound)
	questionRoutes.GET("/:id", questionController.GetByID)
	questionRoutes.GET("/:id/blocks", questionController.ListBlocks)
}
