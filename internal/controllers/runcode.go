package controllers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/sync/semaphore"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/submission"
)

const (
	// judge0WaitTimeout bounds one synchronous (wait=true) Judge0 call.
	judge0WaitTimeout = 30 * time.Second
	// runWriteDeadline replaces the server's 15s WriteTimeout for run
	// requests, which would otherwise cut the connection while Judge0 is
	// still executing under contest load.
	runWriteDeadline = judge0WaitTimeout + 15*time.Second
	// judgeBusyRetryAfter is what a client is told to wait when every
	// Judge0 slot is taken.
	judgeBusyRetryAfter = "3"
)

var (
	// judge0Slots caps concurrent synchronous Judge0 calls from this API
	// process. One Run costs one slot per public testcase; when a whole hall
	// clicks Run together the excess is turned away with 503 instead of
	// piling onto Judge0 and holding goroutines.
	judge0Slots     *semaphore.Weighted
	judge0SlotsOnce sync.Once
)

func waitSlots() *semaphore.Weighted {
	judge0SlotsOnce.Do(func() {
		n := utils.Config.Judge0WaitSlots
		if n <= 0 {
			n = 64
		}
		judge0Slots = semaphore.NewWeighted(int64(n))
	})
	return judge0Slots
}

func judgeBusy(c echo.Context) error {
	c.Response().Header().Set("Retry-After", judgeBusyRetryAfter)
	return c.JSON(http.StatusServiceUnavailable, dto.NewCodedError("The judge is busy, try again in a moment", dto.CodeJudgeBusy))
}

func RunCode(c echo.Context) error {
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

	// Only a question in the player's own round, while that round runs.
	question, err := db.Queries.GetQuestionForUser(ctx, sqlc.GetQuestionForUserParams{ID: questionID, ID_2: user.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewCodedError("Question not found", dto.CodeNotFound))
		}
		logging.Errorf("runcode: get question %s: %v", questionID, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch question", dto.CodeInternal))
	}
	if question.Round == 1 || question.QType == "visual" {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Run is only available for Round 2 and Round 3 questions", dto.CodeValidation))
	}
	if !ensureRoundRunning(c, question.Round) {
		return nil
	}

	testcases, err := db.Queries.GetPublicTestCasesByQuestion(ctx, questionID)
	if err != nil {
		logging.Errorf("runcode: get public testcases for %s: %v", questionID, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch testcases", dto.CodeInternal))
	}
	if len(testcases) == 0 {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("No public testcases found for this question", dto.CodeValidation))
	}

	slots := waitSlots()
	if !slots.TryAcquire(int64(len(testcases))) {
		return judgeBusy(c)
	}
	defer slots.Release(int64(len(testcases)))

	_ = http.NewResponseController(c.Response()).SetWriteDeadline(time.Now().Add(runWriteDeadline))

	result := make([]dto.Judge0CallbackPayload, len(testcases))
	errs := make([]error, len(testcases))
	client := &http.Client{Timeout: judge0WaitTimeout}

	var wg sync.WaitGroup
	for i, testcase := range testcases {
		wg.Add(1)
		go func(index int, tc sqlc.Testcase) {
			defer wg.Done()
			result[index], errs[index] = runOnJudge0(client, req.SourceCode, req.LanguageID, tc)
		}(i, testcase)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		logging.Errorf("runcode %s: %v", questionID, err)
		return c.JSON(http.StatusBadGateway, dto.NewCodedError("The judge could not run your code, try again", dto.CodeJudgeFailed))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Code successfully run", result))
}

func RunCustom(c echo.Context) error {
	var req dto.CustomSubmissionRequest
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

	// Custom input is only for players in a running code round.
	status, err := timer.GetTime(c.Request().Context())
	if err != nil {
		logging.Errorf("runcustom: get time: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to check round timer", dto.CodeInternal))
	}
	if !status.IsRunning || status.Round == 1 || status.Round != user.RoundQualified {
		return c.JSON(http.StatusLocked, dto.NewCodedError("Round is not running", dto.CodeRoundNotRunning))
	}

	runtimeNum, err := utils.Float64ToNumeric(1.0)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to initialize execution timeout", dto.CodeInternal))
	}
	testcase := sqlc.Testcase{Input: req.Stdin, Runtime: runtimeNum}

	slots := waitSlots()
	if !slots.TryAcquire(1) {
		return judgeBusy(c)
	}
	defer slots.Release(1)

	_ = http.NewResponseController(c.Response()).SetWriteDeadline(time.Now().Add(runWriteDeadline))

	result, err := runOnJudge0(&http.Client{Timeout: judge0WaitTimeout}, req.SourceCode, req.LanguageID, testcase)
	if err != nil {
		logging.Errorf("runcustom: %v", err)
		return c.JSON(http.StatusBadGateway, dto.NewCodedError("The judge could not run your code, try again", dto.CodeJudgeFailed))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Custom testcase successfully run", result))
}

// runOnJudge0 runs one testcase synchronously and returns Judge0's answer with
// the base64 output fields decoded.
func runOnJudge0(client *http.Client, sourceCode string, languageID int, tc sqlc.Testcase) (dto.Judge0CallbackPayload, error) {
	var out dto.Judge0CallbackPayload

	payload, err := submission.CreateSubmissionPayload(sourceCode, languageID, tc)
	if err != nil {
		return out, fmt.Errorf("create submission payload: %w", err)
	}

	resp, err := submission.SendSubmissionPayloadWithWait(client, payload)
	if err != nil {
		return out, fmt.Errorf("send submission payload: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return out, fmt.Errorf("judge0 answered %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("read judge0 response: %w", err)
	}
	if err = json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode judge0 response: %w", err)
	}

	decodeBase64Field(out.StdOut)
	decodeBase64Field(out.StdErr)
	decodeBase64Field(out.Message)
	return out, nil
}

// decodeBase64Field decodes a base64 Judge0 field in place, leaving it as is
// when it is not valid base64.
func decodeBase64Field(field *string) {
	if field == nil {
		return
	}
	if decoded, err := base64.StdEncoding.DecodeString(*field); err == nil {
		*field = string(decoded)
	}
}
