package controllers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

// do logging
func SubmitCode(c echo.Context) error {
	var req dto.SubmissionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	//get user id
	userIDStr, ok := c.Get(middlewares.UserIDKey).(string)
	if !ok || userIDStr == "" {
		return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Unauthorized", nil))
	}

	// userID is set by middlewares.VerifyJWTMiddleware (applied to this route
	// in router.go), which validates the JWT cookie and calls
	// c.Set(middlewares.UserIDKey, claims.UserID) before this handler runs.
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Invalid user id", nil))
	}

	questionID, err := uuid.Parse(req.QuestionID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse(err.Error(), nil))
	}

	ctx := c.Request().Context()

	user, err := db.Queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("User not found", nil))
		}
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch user", nil))
	}

	question, err := db.Queries.GetQuestionByID(ctx, questionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("Question not found", nil))
		}
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch question", nil))
	}

	if question.Round == 1 || question.QType == "visual" {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Code submission is only available for Round 2 and Round 3 questions", nil))
	}

	if user.RoundQualified < question.Round {
		return c.JSON(http.StatusForbidden, dto.NewErrorResponse("User not qualified for this round", nil))
	}
	if timer.GetCurrentRound(ctx) != question.Round {
		return c.JSON(http.StatusForbidden, dto.NewErrorResponse("Round not active", nil))
	}
	

	// The buy-in/reward economy applies to every round's code questions, not
	// just the visual one — mirrors the same check submit_round1.go already
	// performs for visual submissions. Without this, a user can skip
	// POST /attempts/:id entirely and still collect the reward on a correct
	// submission, since EnsureAttempt would otherwise silently backfill an
	// attempt row at result-finalize time.
	attempt, err := db.Queries.GetAttempt(ctx, sqlc.GetAttemptParams{
		UserID:     userID,
		QuestionID: questionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusForbidden, dto.NewErrorResponse("Question not purchased — buy this question before submitting", nil))
		}
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}
	if !attemptAllowsSubmission(attempt.Status) {
		return c.JSON(http.StatusForbidden, dto.NewErrorResponse("Question not purchased — buy this question before submitting", nil))
	}

	submissionID := uuid.New()
	logging.Infof("Created submission ID: %v", submissionID)

	//fetch testcases from db
	testcases, err := db.Queries.GetAllTestCasesByQuestion(ctx, questionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}

	//zero testcase validation
	if len(testcases) == 0 {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("No testcases found for the question", nil))
	}

	//make payload
	payload, err := submission.CreateBatchSubmissionPayload(req.SourceCode, req.LanguageID, testcases)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse(err.Error(), nil))
	}

	client := &http.Client{Timeout: 10 * time.Second}

	//send the payload
	resp, err := submission.SendBatchSubmissionPayload(client, payload)
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

	type Token struct {
		Token string `json:"token"`
	}

	var tokens []Token
	err = json.Unmarshal(body, &tokens)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to decode body", nil))
	}

	if len(tokens) != len(testcases) {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Judge0 returned a different number of tokens than testcases submitted", nil))
	}

	tokenToTestcase := make(map[string]string, len(tokens))
	for i, t := range tokens {
		if t.Token == "" {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Judge0 returned an empty token for one or more testcases", nil))
		}
		tokenToTestcase[t.Token] = testcases[i].ID.String()
	}

	if err = utils.CacheTokens(ctx, submissionID.String(), tokenToTestcase); err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to cache submission tokens", nil))
	}

	statusInQueue := utils.Judge0InQueue.GetJudge0Status()
	err = db.Queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:     userID,
		ID:         submissionID,
		QuestionID: questionID,
		SourceCode: req.SourceCode,
		LanguageID: int32(req.LanguageID), // #nosec G115
		Status:     &statusInQueue,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to create submission in database", nil))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Submission created successfully", echo.Map{
		"submission_id": submissionID,
	}))
}

// attemptAllowsSubmission reports whether an attempt's status permits a code
// submission to be judged. Mirrors the check submit_round1.go already
// performs for visual submissions — "available" (or a missing row) must
// never reach Judge0, since EnsureAttempt would otherwise silently backfill
// an unpaid attempt at result-finalize time and still pay out the reward.
func attemptAllowsSubmission(status string) bool {
	return status == "bought" || status == "answered"
}
