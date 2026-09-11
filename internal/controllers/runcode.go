package controllers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"
)

func RunCode(c echo.Context) error {

	var req dto.SubmissionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	submissionID := uuid.New()
	logging.Infof("Created submission ID: %v", submissionID)

	ctx := c.Request().Context()

	//fetch testcases from db
	testcases, err := db.Queries.GetPublicTestCasesByQuestion(ctx, questionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := make([]dto.Judge0CallbackPayload, len(testcases))

	client := &http.Client{}

	for i, testcase := range testcases {

		payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcase)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failure at Judge0"})
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error reading response body"})
		}

		if err = json.Unmarshal(body, &result[i]); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to unmarshal response"})
		}

		if result[i].StdOut != nil {
			decoded, _ := base64.StdEncoding.DecodeString(*result[i].StdOut)
			*result[i].StdOut = string(decoded)
		}
		if result[i].StdErr != nil {
			decoded, _ := base64.StdEncoding.DecodeString(*result[i].StdErr)
			*result[i].StdErr = string(decoded)
		}
		if result[i].Message != nil {
			decoded, _ := base64.StdEncoding.DecodeString(*result[i].Message)
			*result[i].Message = string(decoded)
		}
	}

	return c.JSON(http.StatusOK, result)
}

func RunCustom(c echo.Context) error {

	var req dto.CustomSubmissionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	//dummy testcase
	testcase := sqlc.Testcase{
		//ID             uuid.UUID
		//ExpectedOutput string
		//Memory         pgtype.Numeric
		Input: req.Stdin,
		//Hidden         bool
		//Runtime        pgtype.Numeric
		//QuestionID     uuid.UUID
	}

	payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcase)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	client := &http.Client{}

	resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failure at Judge0"})
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error reading response body"})
	}

	var result dto.Judge0CallbackPayload

	json.Unmarshal(body, &result)

	decoded, _ := base64.StdEncoding.DecodeString(*result.StdOut)
	*result.StdOut = string(decoded)
	decoded, _ = base64.StdEncoding.DecodeString(*result.StdErr)
	*result.StdErr = string(decoded)
	decoded, _ = base64.StdEncoding.DecodeString(*result.Message)
	*result.Message = string(decoded)

	return c.JSON(http.StatusOK, result)
}
