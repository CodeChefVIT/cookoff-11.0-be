package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/labstack/echo/v4"
)

func AdminSession(_ *sqlc.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, ok := c.Get(middlewares.UserIDKey).(string)
		if !ok {
			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
		return c.JSON(http.StatusOK, echo.Map{"status": "ok", "user_id": userID, "role": c.Get(middlewares.RoleKey)})
	}
}
