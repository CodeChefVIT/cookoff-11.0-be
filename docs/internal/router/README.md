# internal/router/ Directory Documentation

The router module is responsible for setting up the Echo instance, mapping URL endpoints to their corresponding handlers (controllers), and attaching relevant middleware to groups/routes.

## Routing Structure
All routes are registered inside a single configuration function:

```go
package router

import (
	"github.com/CodeChefVIT/go-backend-template/internal/controllers"
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo) {
	// Standard operational routes
	e.GET("/health", controllers.HealthCheck)
	e.GET("/docs", controllers.ServeDocs)
	
	// API V1 Group
	v1 := e.Group("/api/v1")
	{
		// Group routes and apply middlewares
		v1.GET("/users", controllers.GetUsers)
	}
}
```

## Design Rules
1. **Clean Mapping**: Router should only mapping path/method to controllers. Keep logic within the handlers and services.
2. **Grouping & Middleware**: Group endpoints logically (e.g. `/api/v1`) and apply middleware (like authentication, rate limiting, logging) to the group level where possible.
