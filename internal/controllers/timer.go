package controllers

import (
	"errors"
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/labstack/echo/v4"
)

func GetTime(c echo.Context) error {
	res, err := timer.GetTime(c.Request().Context())
	if err != nil {
		logging.Errorf("GetTime failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch contest timer", err.Error()))
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Contest timer fetched successfully", res))
}

// ensureRoundRunning writes the error response and returns false when round isn't the running round.
// 423 keeps it distinct from the 402/403/409 statuses the portal already maps to buy-in states.
func ensureRoundRunning(c echo.Context, round int32) bool {
	err := timer.EnsureRoundRunning(c.Request().Context(), round)
	if err == nil {
		return true
	}
	if errors.Is(err, timer.ErrRoundNotRunning) {
		_ = c.JSON(http.StatusLocked, dto.NewErrorResponse("Round is not running", nil))
		return false
	}
	logging.Errorf("round running check failed: %v", err)
	_ = c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to check round timer", nil))
	return false
}
