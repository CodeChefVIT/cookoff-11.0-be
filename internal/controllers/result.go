package controllers

import(
	"errors"
	"context"
	"time"
	"net/http"

	"github.com/google/uuid"

	"github.com/labstack/echo/v4"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
)





type testcaseResult struct{
	ID string `json:"id"`
	Runtime float64 `json:"runtime"`
	Memory float64 `json:"memory"`
	Status string `json:"status"`
	Description string `json:"description"`
}

type resultResp struct{
	ID string `json:"id"`
	QuestionID string `json:"question_id"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Runtime float64 `json:"runtime"`
	Memory float64 `json:"memory"`
	SubmissionTime string `json:"submission_time"`
	Description string `json:"description"`
	Testcases []testcaseResult `json:"testcases"`
}



func GetResult(c echo.Context) error {
	
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()

	submissionID, err := uuid.Parse(c.Param("submission_id"))
	if err!=nil{
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for{
		select{
		case <-ctx.Done():
			return c.JSON(http.StatusRequestTimeout, map[string]string{"error": "submission not processed yet"})

		case <-ticker.C:
			done, err := checkSubmissionStatus(ctx, submissionID)
			if err!=nil{
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get submission status"})
			}
			if done{
				result, err := getSubmissionResult(ctx, submissionID)
				if err!=nil{
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
				return c.JSON(http.StatusOK, result)

			}
		}
	}
	
	return nil
}


func checkSubmissionStatus(ctx context.Context, submissionID uuid.UUID) (bool, error){
	status, err := db.Queries.GetSubmissionStatusByID(ctx, submissionID)
	if err!=nil{
		return false, err
	}
	if status==nil{
		return false, nil
	}

	//refactor later
	const SUBMISSION_DONE_STATUS = "DONE"

	return *status==SUBMISSION_DONE_STATUS, nil
}



func getSubmissionResult(ctx context.Context, submissionID uuid.UUID) (resultResp, error){
	results, err := db.Queries.GetSubmissionResultsBySubmissionID(ctx, submissionID)
	if err!=nil{
		return resultResp{}, errors.New("failed to get submission result from database")
	}
	submission, err := db.Queries.GetSubmissionByID(ctx, submissionID)
	if err!=nil{
		return resultResp{}, errors.New("failed to get submission from database")
	}

	testcases:=make([]testcaseResult, len(results))

	for i, result := range results{
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


		testcases[i]=testcaseResult{
			ID: resultID,
			Runtime: runtime.Float64,
			Memory: memory.Float64,
			Status: result.Status,
			Description: resultDesc,
		}
	}

	runtime, _ := submission.Runtime.Float64Value()
	memory, _ := submission.Memory.Float64Value()


	description := ""
	if submission.Description != nil {
		description = *submission.Description
	}

	return resultResp{
		ID: submissionID.String(),
		QuestionID: submission.QuestionID.String(),
		Passed: int(*submission.TestcasesPassed),
		Failed: int(*submission.TestcasesFailed),
		Runtime: runtime.Float64,
		Memory: memory.Float64,
		SubmissionTime: submission.SubmissionTime.Time.String(),
		Description: description,
		Testcases: testcases,
	}, nil
	
}