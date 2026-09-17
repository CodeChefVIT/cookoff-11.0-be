package controllers

import (
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/labstack/echo/v4"
)

func Judge0Callback(c echo.Context) error {
	var payload dto.Judge0CallbackPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := queue.EnqueueJudge0Callback(payload); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusOK)
}
