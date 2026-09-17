package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/labstack/echo/v4"
)

func GetTime(c echo.Context) error {
	res, err := timer.GetTime(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch contest timer", err.Error()))
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Contest timer fetched successfully", res))
}
