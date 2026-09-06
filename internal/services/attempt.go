package services

import (
	"context"
	"errors"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrAttemptAlreadyExists = errors.New("attempt already exists")
	ErrInsufficientBalance  = errors.New("insufficient balance")
)

type AttemptService struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewAttemptService(db *pgxpool.Pool, queries *sqlc.Queries) *AttemptService {
	return &AttemptService{
		db:      db,
		queries: queries,
	}
}

func (s *AttemptService) CreateAttempt(
	ctx context.Context,
	userID uuid.UUID,
	questionID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	_, err = qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
		UserID:     userID,
		QuestionID: questionID,
	})
	if err == nil {
		return ErrAttemptAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	balanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, userID)
	if err != nil {
		return err
	}

	buyInNumeric, err := qtx.GetQuestionBuyIn(ctx, questionID)
	if err != nil {
		return err
	}

	balance, err := utils.NumericToFloat64(balanceNumeric)
	if err != nil {
		return err
	}

	buyIn, err := utils.NumericToFloat64(buyInNumeric)
	if err != nil {
		return err
	}

	if balance < buyIn {
		return ErrInsufficientBalance
	}

	balance -= buyIn

	newBalanceNumeric, err := utils.Float64ToNumeric(balance)
	if err != nil {
		return err
	}

	err = qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
		ID:      userID,
		Balance: newBalanceNumeric,
	})
	if err != nil {
		return err
	}

	_, err = qtx.CreateAttempt(ctx, sqlc.CreateAttemptParams{
		ID:          uuid.New(),
		UserID:      userID,
		QuestionID:  questionID,
		Status:      "bought",
		IsBuyInPaid: true,
	})

	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
