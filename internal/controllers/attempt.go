package controllers

import (
	"errors"
	"net/http"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/services"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type AttemptController struct {
	service *services.AttemptService
}

func NewAttemptController(service *services.AttemptService) *AttemptController {
	return &AttemptController{
		service: service,
	}
}

func (c *AttemptController) CreateAttempt(ctx echo.Context) error {
	questionID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid question ID", nil))
	}

	userID, isauthenticated := ctx.Get("userID").(uuid.UUID)
	if !isauthenticated {
		return ctx.JSON(http.StatusUnauthorized, dto.NewErrorResponse(
			"Unauthorized", nil))
	}

	err = c.service.CreateAttempt(
		ctx.Request().Context(),
		userID,
		questionID,
	)

	if err != nil {
		switch {
		case errors.Is(err, services.ErrAttemptAlreadyExists):
			return ctx.JSON(http.StatusConflict, dto.NewErrorResponse(
				"Attempt already exists", nil))
		case errors.Is(err, services.ErrInsufficientBalance):
			return ctx.JSON(http.StatusPaymentRequired, dto.NewErrorResponse(
				"Insufficient balance", nil))
		default:
			return ctx.JSON(http.StatusInternalServerError, dto.NewErrorResponse(
				"Internal server error", nil))

		}
	}

	return ctx.JSON(http.StatusOK, dto.NewSuccessResponse(
		"Attempt created successfully", nil))
}
