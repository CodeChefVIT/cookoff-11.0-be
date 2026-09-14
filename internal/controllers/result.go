package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
)

func GetResult(c echo.Context) error {
	ctx := c.Request().Context()

	submissionID, err := uuid.Parse(c.Param("submission_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	submission, err := db.Queries.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("Submission not found", nil))
		}
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to get submission from database", nil))
	}

	userIDStr, ok := c.Get(middlewares.UserIDKey).(string)
	if !ok || userIDStr == "" {
		return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Unauthorized", nil))
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Invalid user id", nil))
	}

	if userID != submission.UserID {
		return c.JSON(http.StatusForbidden, dto.NewErrorResponse("Submission not owned by user", nil))
	}

	var result dto.ResultResponse
	err = utils.GetCache(ctx, utils.SubmissionResultKey(submissionID.String()), &result)
	if err == nil {
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission fetched successfully", result))
	}

	// Cache miss or error: check DB status
	if submission.Status != nil &&
		(*submission.Status == utils.Judge0InQueue.GetJudge0Status() ||
			*submission.Status == utils.Judge0Processing.GetJudge0Status()) {
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission is being processed", submission.Status))
	}

	res, resErr := getSubmissionResult(ctx, submission)
	if resErr != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch submission result", nil))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission fetched successfully", res))
}

func getSubmissionResult(ctx context.Context, submission sqlc.Submission) (dto.ResultResponse, error) {
	results, err := db.Queries.GetSubmissionResults(ctx, submission.ID)
	if err != nil {
		return dto.ResultResponse{}, errors.New("failed to get submission result from database")
	}

	testcases := make([]dto.TestcaseResult, len(results))

	for i, result := range results {
		runtime, _ := result.Runtime.Float64Value()
		memory, _ := result.Memory.Float64Value()

		resultID := ""
		if result.TestcaseID.Valid {
			resultID = result.TestcaseID.String()
		}

		resultDesc := ""
		if result.Description != nil {
			resultDesc = *result.Description
		}

		testcases[i] = dto.TestcaseResult{
			ID:          resultID,
			Runtime:     runtime.Float64,
			Memory:      memory.Float64,
			Status:      result.Status,
			Description: resultDesc,
		}
	}

	runtime, _ := submission.Runtime.Float64Value()
	memory, _ := submission.Memory.Float64Value()

	description := ""
	if submission.Description != nil {
		description = *submission.Description
	}

	passed := 0
	if submission.TestcasesPassed != nil {
		passed = int(*submission.TestcasesPassed)
	}

	failed := 0
	if submission.TestcasesFailed != nil {
		failed = int(*submission.TestcasesFailed)
	}

	submissionTimeStr := ""
	if submission.SubmissionTime.Valid {
		submissionTimeStr = submission.SubmissionTime.Time.String()
	}

	return dto.ResultResponse{
		ID:             submission.ID.String(),
		QuestionID:     submission.QuestionID.String(),
		Passed:         passed,
		Failed:         failed,
		Runtime:        runtime.Float64,
		Memory:         memory.Float64,
		SubmissionTime: submissionTimeStr,
		Description:    description,
		Testcases:      testcases,
	}, nil
}
