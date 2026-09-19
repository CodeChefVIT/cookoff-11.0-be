package middlewares

import (
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/labstack/echo/v4"
)

func Logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)
		if err != nil {
			// Render it here so the logged status is the real one; returning
			// nil below stops Echo from handling the same error twice.
			c.Error(err)
		}

		req := c.Request()
		res := c.Response()

		_ = logging.RouteLogger(c, logging.MiddlewareLogValues{
			Method:  req.Method,
			URI:     req.RequestURI,
			Status:  res.Status,
			Latency: time.Since(start),
			IP:      c.RealIP(),
			Error:   err,
		})

		return nil
	}
}
