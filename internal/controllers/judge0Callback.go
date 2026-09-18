package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/labstack/echo/v4"
)

func Judge0Callback(c echo.Context) error {
	// This route cannot sit behind the JWT middleware — Judge0 calls it, not a
	// signed-in user — so the shared secret from the callback URL is the only
	// thing separating a real verdict from a forged one. No secret configured
	// means no check, which keeps existing deployments working.
	if !utils.CallbackTokenValid(c.QueryParam(utils.CallbackTokenParam)) {
		return c.JSON(http.StatusUnauthorized, dto.NewCodedError("invalid callback token", dto.CodeUnauthorized))
	}

	var payload dto.Judge0CallbackPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("invalid callback payload", dto.CodeValidation))
	}

	if err := queue.EnqueueJudge0Callback(payload); err != nil {
		logging.Errorf("enqueue judge0 callback: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("failed to queue callback", dto.CodeInternal))
	}

	return c.NoContent(http.StatusOK)
}
