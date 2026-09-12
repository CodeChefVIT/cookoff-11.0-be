package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/labstack/echo/v4"
)

func AdminSession(_ *sqlc.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, ok := c.Get(middlewares.UserIDKey).(string)
		if !ok {
			return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Unauthorized", nil))
		}
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Admin session validated", echo.Map{"user_id": userID, "role": c.Get(middlewares.RoleKey)}))
	}
}
