package controllers

import (
	"context"
	"errors"
	"net/http"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type AttemptController struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewAttemptController(
	db *pgxpool.Pool,
	queries *sqlc.Queries,
) *AttemptController {
	return &AttemptController{
		db:      db,
		queries: queries,
	}
}

func (c *AttemptController) CreateAttempt(ctx echo.Context) error {
	questionID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid question ID", nil,
		))
	}

	userIDValue, ok := ctx.Get("user_id").(string)
	if !ok {
		return ctx.JSON(http.StatusUnauthorized, dto.NewErrorResponse(
			"Unauthorized", nil,
		))
	}

	userID, err := uuid.Parse(userIDValue)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid user ID", nil,
		))
	}

	attemptResp, err := c.createAttempt(
		ctx.Request().Context(),
		userID,
		questionID,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrAttemptAlreadyExists):
			return ctx.JSON(http.StatusConflict, dto.NewErrorResponse(
				"Attempt already exists", nil,
			))

		case errors.Is(err, ErrInsufficientBalance):
			return ctx.JSON(http.StatusPaymentRequired, dto.NewErrorResponse(
				"Insufficient balance", nil,
			))

		default:
			return ctx.JSON(http.StatusInternalServerError, dto.NewErrorResponse(
				"Internal server error", nil,
			))
		}
	}

	return ctx.JSON(http.StatusOK, dto.NewSuccessResponse(
		"Attempt created successfully", attemptResp,
	))
}

var (
	ErrAttemptAlreadyExists = errors.New("attempt already exists")
	ErrInsufficientBalance  = errors.New("insufficient balance")
)

func (c *AttemptController) createAttempt(
	ctx context.Context,
	userID uuid.UUID,
	questionID uuid.UUID,
) (*dto.AttemptResponse, error) {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	qtx := c.queries.WithTx(tx)

	_, err = qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
		UserID:     userID,
		QuestionID: questionID,
	})
	if err == nil {
		return nil, ErrAttemptAlreadyExists
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	balanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, userID)
	if err != nil {
		return nil, err
	}

	buyInNumeric, err := qtx.GetQuestionBuyIn(ctx, questionID)
	if err != nil {
		return nil, err
	}

	balance, err := utils.NumericToFloat64(balanceNumeric)
	if err != nil {
		return nil, err
	}

	buyIn, err := utils.NumericToFloat64(buyInNumeric)
	if err != nil {
		return nil, err
	}

	if balance < buyIn {
		return nil, ErrInsufficientBalance
	}

	balance -= buyIn

	newBalanceNumeric, err := utils.Float64ToNumeric(balance)
	if err != nil {
		return nil, err
	}

	err = qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
		ID:      userID,
		Balance: newBalanceNumeric,
	})
	if err != nil {
		return nil, err
	}

	attempt, err := qtx.CreateAttempt(ctx, sqlc.CreateAttemptParams{
		ID:          uuid.New(),
		UserID:      userID,
		QuestionID:  questionID,
		Status:      "bought",
		IsBuyInPaid: true,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &dto.AttemptResponse{
		ID:          attempt.ID,
		QuestionID:  attempt.QuestionID,
		UserID:      attempt.UserID,
		Status:      attempt.Status,
		NewBalance:  balance,
		IsBuyInPaid: attempt.IsBuyInPaid,
	}, nil
}
