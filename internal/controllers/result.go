package controllers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/labstack/echo/v4"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
)

func GetResult(c echo.Context) error {

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()

	submissionID, err := uuid.Parse(c.Param("submission_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return c.JSON(http.StatusRequestTimeout, map[string]string{"error": "submission not processed yet"})

		case <-ticker.C:
			status, err := db.Queries.GetSubmissionStatusByID(ctx, submissionID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get submission status"})
			}

			statusStr := utils.Judge0InQueue.GetJudge0Status()
			if status != nil {
				statusStr = *status
			}

			if statusStr != utils.Judge0InQueue.GetJudge0Status() && statusStr != utils.Judge0Processing.GetJudge0Status() {
				result, err := getSubmissionResult(ctx, submissionID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
				return c.JSON(http.StatusOK, result)

			}
		}
	}
}

func getSubmissionResult(ctx context.Context, submissionID uuid.UUID) (dto.ResultResponse, error) {
	results, err := db.Queries.GetSubmissionResults(ctx, submissionID)
	if err != nil {
		return dto.ResultResponse{}, errors.New("failed to get submission result from database")
	}
	submission, err := db.Queries.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return dto.ResultResponse{}, errors.New("failed to get submission from database")
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
		ID:             submissionID.String(),
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
