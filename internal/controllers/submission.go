package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

func SubmitCode(c echo.Context) error {
	var req dto.SubmissionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid request body", dto.CodeValidation))
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError(validationMessage(err), dto.CodeValidation))
	}
	if !utils.IsSupportedLanguage(req.LanguageID) {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Unsupported language", dto.CodeValidation))
	}

	user, ok := middlewares.CurrentUser(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, dto.NewCodedError("Unauthorized", dto.CodeUnauthorized))
	}

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid question ID", dto.CodeValidation))
	}

	ctx := c.Request().Context()

	question, err := db.Queries.GetQuestionByID(ctx, questionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewCodedError("Question not found", dto.CodeNotFound))
		}
		logging.Errorf("submit: get question %s: %v", questionID, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch question", dto.CodeInternal))
	}

	if question.Round == 1 || question.QType == "visual" {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Code submission is only available for Round 2 and Round 3 questions", dto.CodeValidation))
	}

	if !ensureRoundRunning(c, question.Round) {
		return nil
	}

	if user.RoundQualified != question.Round {
		return c.JSON(http.StatusForbidden, dto.NewCodedError("User not qualified for this round", dto.CodeNotQualified))
	}

	if question.Round == 2 {
		attempt, attemptErr := db.Queries.GetAttempt(ctx, sqlc.GetAttemptParams{UserID: user.ID, QuestionID: questionID})
		if attemptErr != nil && !errors.Is(attemptErr, pgx.ErrNoRows) {
			logging.Errorf("submit: get attempt: %v", attemptErr)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to check purchase", dto.CodeInternal))
		}
		if attemptErr != nil || (attempt.Status != "bought" && attempt.Status != "answered") {
			return c.JSON(http.StatusForbidden, dto.NewCodedError("Question not purchased — buy this question before submitting", dto.CodeNotPurchased))
		}
	}

	testcases, err := db.Queries.GetAllTestCasesByQuestion(ctx, questionID)
	if err != nil {
		logging.Errorf("submit: get testcases for %s: %v", questionID, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch testcases", dto.CodeInternal))
	}
	if len(testcases) == 0 {
		logging.Errorf("submit: question %s has no testcases", questionID)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("This question has no testcases yet", dto.CodeInternal))
	}

	payload, err := submission.CreateBatchSubmissionPayload(req.SourceCode, req.LanguageID, testcases)
	if err != nil {
		logging.Errorf("submit: build payload: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to prepare submission", dto.CodeInternal))
	}

	// The row goes in before Judge0 hears about it, so a callback can never
	// arrive for a submission that does not exist yet.
	submissionID := uuid.New()
	statusInQueue := utils.Judge0InQueue.GetJudge0Status()
	if err = db.Queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:     user.ID,
		ID:         submissionID,
		QuestionID: questionID,
		SourceCode: req.SourceCode,
		LanguageID: int32(req.LanguageID), // #nosec G115
		Status:     &statusInQueue,
	}); err != nil {
		logging.Errorf("submit: create submission: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to create submission", dto.CodeInternal))
	}

	tokenToTestcase, err := sendBatch(payload, testcases)
	if err == nil {
		err = utils.CacheTokens(ctx, submissionID.String(), tokenToTestcase)
	}
	if err != nil {
		logging.Errorf("submit %s: %v", submissionID, err)
		// Nobody has seen this id yet; drop the row rather than leave it pending.
		if delErr := db.Queries.DeleteSubmission(context.WithoutCancel(ctx), submissionID); delErr != nil {
			logging.Errorf("submit %s: delete after failure: %v", submissionID, delErr)
		}
		return c.JSON(http.StatusBadGateway, dto.NewCodedError("The judge could not accept your submission, try again", dto.CodeJudgeFailed))
	}

	logging.Infof("Created submission ID: %v", submissionID)
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission created successfully", echo.Map{
		"submission_id": submissionID,
	}))
}

// sendBatch posts the batch to Judge0 and maps each returned token to the
// testcase it runs, in the order they were sent.
func sendBatch(payload []byte, testcases []sqlc.Testcase) (map[string]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := submission.SendBatchSubmissionPayload(client, payload)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, errors.New("judge0 batch rejected with status " + resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokens []struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(body, &tokens); err != nil {
		return nil, err
	}
	if len(tokens) != len(testcases) {
		return nil, errors.New("judge0 returned a different number of tokens than testcases submitted")
	}

	tokenToTestcase := make(map[string]string, len(tokens))
	for i, t := range tokens {
		if t.Token == "" {
			return nil, errors.New("judge0 returned an empty token")
		}
		tokenToTestcase[t.Token] = testcases[i].ID.String()
	}
	return tokenToTestcase, nil
}
