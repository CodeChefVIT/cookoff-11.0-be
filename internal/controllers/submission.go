package controllers

import (
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"

	"encoding/json"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

// do logging
func SubmitCode(c echo.Context) error {
	var req dto.SubmissionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	//get user id here
	//userID :=

	//auth stuff
	//here

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	submissionID := uuid.New()
	logging.Infof("Created submission ID: %v", submissionID)

	ctx := c.Request().Context()

	//fetch testcases from db
	testcases, err := db.Queries.GetAllTestCasesByQuestion(ctx, questionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	//make payload
	payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcases)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	client := &http.Client{}

	//send the payload
	resp, err := submission.SendSubmissionPayload(client, payload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failure at Judge0"})
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error reading response body"})
	}

	type Token struct {
		Token string `json:"token"`
	}

	var tokens []Token
	err = json.Unmarshal(body, &tokens)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode body"})
	}

	/*
		for i, t:= range tokens{
			//add the token to the queue and all.....
		}*/

	err = db.Queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		ID:         submissionID,
		QuestionID: questionID,
		SourceCode: req.SourceCode,
		LanguageID: int32(req.LanguageID), // #nosec G115
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create submission in database"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"submission_id": submissionID,
	})
}
