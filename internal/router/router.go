package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)


	//judge0 callback req
	e.PUT("/judge0callback", controllers.Judge0Callback)

	//for now all routes in same place, separate them later
	e.POST("/submit", controllers.SubmitCode)
	e.GET("/result/:submission_id", controllers.GetResult)
	//e.GET("/runcode", controllers.RunCode)
	//e.GET("/runcustom", controllers.RunCustom)
}
