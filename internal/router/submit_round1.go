package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	db "github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/labstack/echo/v4"
)

func RegisterVisualSubmissionRoutes(e *echo.Echo, middleware ...echo.MiddlewareFunc) {
	visualSubmissionController := controllers.NewVisualSubmissionController(
		db.DBPool,
		sqlc.New(db.DBPool),
	)
	e.POST("/submit/visual", visualSubmissionController.SubmitVisualSolution, middleware...)

}
