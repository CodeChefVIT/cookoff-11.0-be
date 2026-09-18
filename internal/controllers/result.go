package controllers

import (
	"context"
	"errors"
	"net/http"
	"time"

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

	// Long-poll until the verdict is terminal. The portal issues exactly one
	// GET per submission with a 130s client timeout and has no polling loop, so
	// returning a non-terminal placeholder leaves it with nothing to render —
	// and because the placeholder was a bare status *string* where the success
	// payload is an object, it could not even be parsed. Holding the request is
	// also the only option that survives the per-IP rate limiter
	// (cmd/api/main.go) when a whole hall shares one NAT address; a
	// client-side poll would multiply request volume by the number of players.
	deadline := time.After(resultLongPollTimeout)
	ticker := time.NewTicker(resultPollInterval)
	defer ticker.Stop()

	for {
		var result dto.ResultResponse
		if cacheErr := utils.GetCache(ctx, utils.SubmissionResultKey(submissionID.String()), &result); cacheErr == nil {
			return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission fetched successfully", result))
		}

		if !isPendingStatus(submission.Status) {
			res, resErr := getSubmissionResult(ctx, submission)
			if resErr != nil {
				return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch submission result", nil))
			}
			return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission fetched successfully", res))
		}

		select {
		case <-ctx.Done():
			// Client hung up (navigated away); stop holding the connection.
			return nil
		case <-deadline:
			// The portal maps 408 to its "Check again" affordance.
			return c.JSON(http.StatusRequestTimeout, dto.NewErrorResponse("Submission is still being judged", nil))
		case <-ticker.C:
		}

		submission, err = db.Queries.GetSubmissionByID(ctx, submissionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(http.StatusNotFound, dto.NewErrorResponse("Submission not found", nil))
			}
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to get submission from database", nil))
		}
	}
}

const (
	// Comfortably inside the portal's 130s axios timeout (api/submissions.ts).
	resultLongPollTimeout = 120 * time.Second
	resultPollInterval    = 500 * time.Millisecond
)

// isPendingStatus reports whether the judge is still working on a submission.
// A nil status means the row was written before Judge0 answered, which is
// pending too — never a terminal verdict.
func isPendingStatus(status *string) bool {
	if status == nil {
		return true
	}
	return *status == utils.Judge0InQueue.GetJudge0Status() ||
		*status == utils.Judge0Processing.GetJudge0Status()
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
