package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	sqlc "github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
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

	pointsAwarded, err := c.submitVisualSolution(
		ctx.Request().Context(),
		userID,
		req,
	)

	if err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			return ctx.JSON(httpErr.Code, dto.NewErrorResponse(
				httpErr.Message.(string), nil,
			))
		}

		return ctx.JSON(http.StatusInternalServerError, dto.NewErrorResponse(
			"Internal server error", nil,
		))
	}

	return ctx.JSON(http.StatusOK, dto.NewSuccessResponse(
		"Visual solution submitted successfully", dto.SubmitVisualSolutionResponse{
			PointsAwarded: pointsAwarded,
		},
	))
}

func (c *VisualSubmissionController) submitVisualSolution(
	ctx context.Context,
	userID uuid.UUID,
	req dto.SubmitVisualSolutionRequest,
) (float64, error) {
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return 0, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	qtx := c.queries.WithTx(tx)

	_, err = qtx.GetRoundOneVisualQuestion(ctx, req.QuestionID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Question doesnt exist")
		}
		return 0, err
	}

	attempt, err := qtx.GetAttemptForUpdate(
		ctx,
		sqlc.GetAttemptForUpdateParams{
			UserID:     userID,
			QuestionID: req.QuestionID,
		},
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, echo.NewHTTPError(http.StatusForbidden, "Question not bought yet")
		}
		return 0, err
	}

	if attempt.Status == "available" {
		return 0, echo.NewHTTPError(http.StatusForbidden, "Question not bought yet")
	}

	availableBlocks, err := qtx.ListVisualBlocksByQuestionID(
		ctx,
		req.QuestionID,
	)

	if err != nil {
		return 0, err
	}

	validBlocksIDs := make(map[uuid.UUID]struct{})

	for _, block := range availableBlocks {
		validBlocksIDs[block.ID] = struct{}{}
	}

	for _, blockID := range req.Blocks {
		if _, exists := validBlocksIDs[blockID]; !exists {
			return 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid block ID")
		}
	}

	solutions, err := qtx.ListVisualSolutionsByQuestionID(
		ctx,
		req.QuestionID,
	)

	if err != nil {
		return 0, err
	}

	var (
		isCorrect      bool
		solutionPoints float64
		pointsAwarded  float64
	)

	for _, solution := range solutions {
		if uuidSlicesEqual(solution.Solution, req.Blocks) {
			isCorrect = true
			solutionPoints, err = utils.NumericToFloat64(solution.Points)
			if err != nil {
				return 0, err
			}
			break
		}
	}

	sourceCode, err := json.Marshal(req.Blocks)

	if err != nil {
		return 0, err
	}

	//updating the submisssion status
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
		return 0, err
	}

	if isCorrect && attempt.Status == "bought" {

		//updating the user score
		currentScoreNumeric, err := qtx.GetUserScoreForUpdate(ctx, userID)
		if err != nil {
			return 0, err
		}
		currentScore, err := utils.NumericToFloat64(currentScoreNumeric)
		if err != nil {
			return 0, err
		}

		newScore := currentScore + solutionPoints

		newScoreNumeric, err := utils.Float64ToNumeric(newScore)
		if err != nil {
			return 0, err
		}

		err = qtx.UpdateUserScore(ctx, sqlc.UpdateUserScoreParams{
			ID:    userID,
			Score: newScoreNumeric,
		})
		if err != nil {
			return 0, err
		}

		pointsAwarded = solutionPoints

		//updating the user balance
		currentUserBalanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, userID)
		if err != nil {
			return 0, err
		}
		questionRewardNumeric, err := qtx.GetQuestionReward(ctx, req.QuestionID)
		if err != nil {
			return 0, err
		}

		questionReward, err := utils.NumericToFloat64(questionRewardNumeric)
		if err != nil {
			return 0, err
		}
		currentBalance, err := utils.NumericToFloat64(currentUserBalanceNumeric)
		if err != nil {
			return 0, err
		}

		newBalance := currentBalance + questionReward

		newBalanceNumeric, err := utils.Float64ToNumeric(newBalance)
		if err != nil {
			return 0, err
		}

		err = qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
			ID:      userID,
			Balance: newBalanceNumeric,
		})
		if err != nil {
			return 0, err
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
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}

	return pointsAwarded, nil
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
