package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/labstack/echo/v4"
)

func Judge0Callback(c echo.Context) error {
	// This route cannot sit behind the JWT middleware — Judge0 calls it, not a
	// signed-in user — so the shared secret from the callback URL is the only
	// thing separating a real verdict from a forged one. No secret configured
	// means no check, which keeps existing deployments working.
	if !utils.CallbackTokenValid(c.QueryParam(utils.CallbackTokenParam)) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid callback token"})
	}

	var payload dto.Judge0CallbackPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := queue.EnqueueJudge0Callback(payload); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusOK)
}
