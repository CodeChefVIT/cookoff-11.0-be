package controllers

import (
	"context"
	"errors"
	"net/http"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
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

	claims, ok := ctx.Get("user").(*middlewares.JWTClaims)
	if !ok {
		return ctx.JSON(http.StatusUnauthorized, dto.NewErrorResponse(
			"Unauthorized", nil,
		))
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid user ID", nil,
		))
	}

	attempt, err := c.createAttempt(
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
		"Attempt created successfully", attempt,
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
) (sqlc.Attempt, error) {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	defer tx.Rollback(ctx)

	qtx := c.queries.WithTx(tx)

	_, err = qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
		UserID:     userID,
		QuestionID: questionID,
	})
	if err == nil {
		return sqlc.Attempt{}, ErrAttemptAlreadyExists
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Attempt{}, err
	}

	balanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, userID)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	buyInNumeric, err := qtx.GetQuestionBuyIn(ctx, questionID)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	balance, err := utils.NumericToFloat64(balanceNumeric)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	buyIn, err := utils.NumericToFloat64(buyInNumeric)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	if balance < buyIn {
		return sqlc.Attempt{}, ErrInsufficientBalance
	}

	balance -= buyIn

	newBalanceNumeric, err := utils.Float64ToNumeric(balance)
	if err != nil {
		return sqlc.Attempt{}, err
	}

	err = qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
		ID:      userID,
		Balance: newBalanceNumeric,
	})
	if err != nil {
		return sqlc.Attempt{}, err
	}

	attempt, err := qtx.CreateAttempt(ctx, sqlc.CreateAttemptParams{
		ID:          uuid.New(),
		UserID:      userID,
		QuestionID:  questionID,
		Status:      "bought",
		IsBuyInPaid: true,
	})
	if err != nil {
		return sqlc.Attempt{}, err
	}

	return attempt, err
}
