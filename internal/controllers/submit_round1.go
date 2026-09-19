package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

const VisualLanguageID int32 = 0

type VisualSubmissionController struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewVisualSubmissionController(
	db *pgxpool.Pool,
	queries *sqlc.Queries,
) *VisualSubmissionController {
	return &VisualSubmissionController{
		db:      db,
		queries: queries,
	}
}

func (c *VisualSubmissionController) SubmitVisualSolution(ctx echo.Context) error {
	var req dto.SubmitVisualSolutionRequest

	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid request body", nil,
		))
	}

	if req.QuestionID == uuid.Nil {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Invalid question ID", nil,
		))
	}

	if len(req.Blocks) == 0 {
		return ctx.JSON(http.StatusBadRequest, dto.NewErrorResponse(
			"Blocks cannot be empty", nil,
		))
	}

	user, ok := middlewares.CurrentUser(ctx)
	if !ok {
		return ctx.JSON(http.StatusUnauthorized, dto.NewCodedError("Unauthorized", dto.CodeUnauthorized))
	}
	userID := user.ID

	// Visual questions only exist in round 1 (GetRoundOneVisualQuestion).
	if !ensureRoundRunning(ctx, 1) {
		return nil
	}
	if user.RoundQualified != 1 {
		return ctx.JSON(http.StatusForbidden, dto.NewCodedError("User not qualified for this round", dto.CodeNotQualified))
	}

	outcome, err := c.submitVisualSolution(
		ctx.Request().Context(),
		userID,
		req,
	)

	if err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			return ctx.JSON(httpErr.Code, dto.NewCodedError(fmt.Sprint(httpErr.Message), dto.CodeForStatus(httpErr.Code)))
		}

		logging.Errorf("visual submit: %v", err)
		return ctx.JSON(http.StatusInternalServerError, dto.NewCodedError("Internal server error", dto.CodeInternal))
	}

	status := "wrong answer"
	if outcome.Correct {
		status = "success"
	}

	return ctx.JSON(http.StatusOK, dto.NewSuccessResponse(
		"Visual solution submitted successfully", dto.SubmitVisualSolutionResponse{
			Status:          status,
			PointsAwarded:   outcome.PointsAwarded,
			Correct:         outcome.Correct,
			AlreadyAnswered: outcome.AlreadyAnswered,
		},
	))
}

// visualOutcome separates "the chain was right" from "the chain was paid for".
// A resubmission on a settled attempt is still correct but awards nothing.
type visualOutcome struct {
	PointsAwarded   float64
	Correct         bool
	AlreadyAnswered bool
}

func (c *VisualSubmissionController) submitVisualSolution(
	ctx context.Context,
	userID uuid.UUID,
	req dto.SubmitVisualSolutionRequest,
) (visualOutcome, error) {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return visualOutcome{}, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	qtx := c.queries.WithTx(tx)

	_, err = qtx.GetRoundOneVisualQuestion(ctx, req.QuestionID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return visualOutcome{}, echo.NewHTTPError(http.StatusNotFound, "Question doesn't exist")
		}
		return visualOutcome{}, err
	}

	attempt, err := qtx.GetAttemptForUpdate(
		ctx,
		sqlc.GetAttemptForUpdateParams{
			UserID:     userID,
			QuestionID: req.QuestionID,
		},
	)

	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return visualOutcome{}, err
		}
		// Round 1 has no buy-in, so the first submission opens the attempt.
		// ON CONFLICT DO NOTHING plus the locking re-read means two
		// concurrent first submissions share one row instead of one of them
		// failing on the unique constraint.
		if err = qtx.EnsureAttempt(ctx, sqlc.EnsureAttemptParams{
			ID:         uuid.New(),
			UserID:     userID,
			QuestionID: req.QuestionID,
		}); err != nil {
			return visualOutcome{}, err
		}
		attempt, err = qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
			UserID:     userID,
			QuestionID: req.QuestionID,
		})
		if err != nil {
			return visualOutcome{}, err
		}
	} else if attempt.Status == "available" {
		attempt, err = qtx.UpdateAttemptToBought(ctx, sqlc.UpdateAttemptToBoughtParams{
			UserID:     userID,
			QuestionID: req.QuestionID,
		})
		if err != nil {
			return visualOutcome{}, err
		}
	}

	availableBlocks, err := qtx.ListVisualBlocksByQuestionID(
		ctx,
		req.QuestionID,
	)

	if err != nil {
		return visualOutcome{}, err
	}

	validBlocksIDs := make(map[uuid.UUID]struct{})

	for _, block := range availableBlocks {
		validBlocksIDs[block.ID] = struct{}{}
	}

	for _, blockID := range req.Blocks {
		if _, exists := validBlocksIDs[blockID]; !exists {
			return visualOutcome{}, echo.NewHTTPError(http.StatusBadRequest, "Invalid block ID")
		}
	}

	solutions, err := qtx.ListVisualSolutionsByQuestionID(
		ctx,
		req.QuestionID,
	)

	if err != nil {
		return visualOutcome{}, err
	}

	var (
		isCorrect      bool
		solutionPoints float64
		pointsAwarded  float64
	)

	for _, solution := range solutions {
		if uuidSlicesEqual(solution.Solution, req.Blocks) {
			isCorrect = true
			solutionPoints, _ = utils.NumericToFloat64(solution.Points)
			break
		}
	}

	sourceCode, err := json.Marshal(req.Blocks)

	if err != nil {
		return visualOutcome{}, err
	}

	//updating the submission status
	submissionStatus := "wrong answer"
	if isCorrect {
		submissionStatus = "success"
	}

	_, err = qtx.CreateVisualSubmission(
		ctx,
		sqlc.CreateVisualSubmissionParams{
			ID:         uuid.New(),
			UserID:     userID,
			QuestionID: req.QuestionID,
			SourceCode: string(sourceCode),
			LanguageID: VisualLanguageID,
			Status:     &submissionStatus,
		},
	)

	if err != nil {
		return visualOutcome{}, err
	}

	if isCorrect && attempt.Status == "bought" {

		//updating the user score
		currentScoreNumeric, err := qtx.GetUserScoreForUpdate(ctx, userID)
		if err != nil {
			return visualOutcome{}, err
		}
		currentScore, err := utils.NumericToFloat64(currentScoreNumeric)
		if err != nil {
			return visualOutcome{}, err
		}

		newScore := currentScore + solutionPoints

		newScoreNumeric, err := utils.Float64ToNumeric(newScore)
		if err != nil {
			return visualOutcome{}, err
		}

		err = qtx.UpdateUserScore(ctx, sqlc.UpdateUserScoreParams{
			ID:    userID,
			Score: newScoreNumeric,
		})
		if err != nil {
			return visualOutcome{}, err
		}

		pointsAwarded = solutionPoints

		//updating the user balance
		currentUserBalanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, userID)
		if err != nil {
			return visualOutcome{}, err
		}
		questionRewardNumeric, err := qtx.GetQuestionReward(ctx, req.QuestionID)
		if err != nil {
			return visualOutcome{}, err
		}

		questionReward, err := utils.NumericToFloat64(questionRewardNumeric)
		if err != nil {
			return visualOutcome{}, err
		}
		currentBalance, err := utils.NumericToFloat64(currentUserBalanceNumeric)
		if err != nil {
			return visualOutcome{}, err
		}

		newBalance := currentBalance + questionReward

		newBalanceNumeric, err := utils.Float64ToNumeric(newBalance)
		if err != nil {
			return visualOutcome{}, err
		}

		err = qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
			ID:      userID,
			Balance: newBalanceNumeric,
		})
		if err != nil {
			return visualOutcome{}, err
		}

		err = qtx.UpdateAttemptStatus(
			ctx,
			sqlc.UpdateAttemptStatusParams{
				UserID:     userID,
				QuestionID: req.QuestionID,
				Status:     "answered",
				AnsweredAt: pgtype.Timestamptz{
					Time:  time.Now(),
					Valid: true,
				},
			},
		)
		if err != nil {
			return visualOutcome{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return visualOutcome{}, err
	}

	return visualOutcome{
		PointsAwarded: pointsAwarded,
		Correct:       isCorrect,
		// The payout branch above runs only while the attempt is still
		// "bought", so anything else means it was settled by an earlier
		// submission.
		AlreadyAnswered: attempt.Status != "bought",
	}, nil
}

func uuidSlicesEqual(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
