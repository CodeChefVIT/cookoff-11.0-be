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
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	submissionID := uuid.New()
	logging.Infof("Created submission ID: %v", submissionID)

	ctx := c.Request().Context()

	// fetch testcases from db
	testcases, err := db.Queries.GetPublicTestCasesByQuestion(ctx, questionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}

	result := make([]dto.Judge0CallbackPayload, len(testcases))
	client := &http.Client{}

	for i, testcase := range testcases {
		payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcase)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
		}

		resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
		}

		if resp.StatusCode != http.StatusCreated {
			_ = resp.Body.Close()
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failure at Judge0", nil))
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Error reading response body", nil))
		}

		if err = json.Unmarshal(body, &result[i]); err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to unmarshal response", nil))
		}

		if result[i].StdOut != nil {
			if decoded, err := base64.StdEncoding.DecodeString(*result[i].StdOut); err == nil {
				str := string(decoded)
				result[i].StdOut = &str
			}
		}
		if result[i].StdErr != nil {
			if decoded, err := base64.StdEncoding.DecodeString(*result[i].StdErr); err == nil {
				str := string(decoded)
				result[i].StdErr = &str
			}
		}
		if result[i].Message != nil {
			if decoded, err := base64.StdEncoding.DecodeString(*result[i].Message); err == nil {
				str := string(decoded)
				result[i].Message = &str
			}
		}
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Code successfully run", result))
}

func RunCustom(c echo.Context) error {
	var req dto.CustomSubmissionRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	// dummy testcase
	testcase := sqlc.Testcase{
		Input: req.Stdin,
	}

	payload, err := submission.CreateSubmissionPayload(req.SourceCode, req.LanguageID, testcase)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}

	client := &http.Client{}

	resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failure at Judge0", nil))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Error reading response body", nil))
	}

	var result dto.Judge0CallbackPayload
	if err := json.Unmarshal(body, &result); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to unmarshal response", nil))
	}

	if result.StdOut != nil {
		if decoded, err := base64.StdEncoding.DecodeString(*result.StdOut); err == nil {
			str := string(decoded)
			result.StdOut = &str
		}
	}
	if result.StdErr != nil {
		if decoded, err := base64.StdEncoding.DecodeString(*result.StdErr); err == nil {
			str := string(decoded)
			result.StdErr = &str
		}
	}
	if result.Message != nil {
		if decoded, err := base64.StdEncoding.DecodeString(*result.Message); err == nil {
			str := string(decoded)
			result.Message = &str
		}
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Custom testcase successfully run", result))
}
