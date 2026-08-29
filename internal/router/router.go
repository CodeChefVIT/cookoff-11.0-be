package router

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/controllers"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(
	e *echo.Echo) {
	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)

	RegisterAttemptRoutes(e)

}
