// Package workers holds Asynq task handlers -- the code that runs inside the
// separate cmd/worker process and actually does the DB writes for a Judge0
// callback. See docs/LLD.MD 2.6 "Sequence -- Judge0 Callback -> Result
// Persistence" for the exact flow this file implements.
package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

//bro too many useless comments

// NewServeMux wires up every registered task type this worker process can
// handle. cmd/worker/main.go passes this mux into asynq.Server.Run. If you
// add another background job later (e.g. leaderboard recompute), register
// its handler here too.
func NewServeMux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeJudge0Callback, HandleJudge0CallbackTask)
	return mux
}

// HandleJudge0CallbackTask is the consumer side of what
// controllers.Judge0Callback enqueues. Asynq calls this once per queued
// task, retrying automatically (with backoff) if it returns a non-nil error.
func HandleJudge0CallbackTask(ctx context.Context, t *asynq.Task) error {
	var payload dto.Judge0CallbackPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		// Malformed payload will never succeed on retry, so tell Asynq not
		// to bother retrying it.
		return fmt.Errorf("%w: unmarshal judge0 callback payload: %v", asynq.SkipRetry, err)
	}

	// Step 1: resolve the Judge0 token back to which submission/testcase it
	// belongs to. This mapping was written by SubmitCode (utils.CacheToken)
	// when the batch was first sent to Judge0.
	submissionIDStr, testcaseIDStr, err := utils.GetSubmissionIDByToken(ctx, payload.Token)
	if err != nil {
		return fmt.Errorf("resolve token %q: %w", payload.Token, err)
	}
	submissionID, err := uuid.Parse(submissionIDStr)
	if err != nil {
		return fmt.Errorf("%w: invalid submission id %q cached for token: %v", asynq.SkipRetry, submissionIDStr, err)
	}

	var testcaseID pgtype.UUID
	if parsed, parseErr := uuid.Parse(testcaseIDStr); parseErr == nil {
		testcaseID = pgtype.UUID{Bytes: parsed, Valid: true}
	} else {
		// Don't silently drop this -- a malformed cached testcase id means
		// something upstream (CacheToken's caller) wrote a bad value.
		logging.Warnf("submission %s: cached testcase id %q is not a valid UUID, storing result with no testcase link: %v",
			submissionID, testcaseIDStr, parseErr)
	}

	status := utils.GetJudge0StatusFromID(payload.Status.ID)

	// Judge0 sends "time" as a string like "0.045" (seconds).
	runtimeSeconds, err := strconv.ParseFloat(payload.Time, 64)
	if err != nil {
		logging.Warnf("submission %s: could not parse judge0 time %q, defaulting runtime to 0: %v",
			submissionID, payload.Time, err)
		runtimeSeconds = 0
	}
	runtimeNumeric, err := utils.Float64ToNumeric(runtimeSeconds)
	if err != nil {
		return fmt.Errorf("convert runtime: %w", err)
	}
	memoryNumeric, err := utils.Float64ToNumeric(float64(payload.Memory))
	if err != nil {
		return fmt.Errorf("convert memory: %w", err)
	}

	// Placeholder scoring: 1 point for a passing testcase, 0 otherwise.
	// The schema has no per-testcase weighting yet (testcases table has no
	// points column) -- revisit this if/when per-testcase weights are added.
	var pointsAwarded int32
	if status == utils.Judge0Accepted.GetJudge0Status() {
		pointsAwarded = 1
	}

	description := payload.Status.Description
	if payload.Message != nil && *payload.Message != "" {
		description = description + ": " + *payload.Message
	}

	// Generate deterministic ID for result to ensure idempotency on retries
	resultID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(payload.Token))

	tx, err := db.DBPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	qtx := db.Queries.WithTx(tx)

	// Step 2: record per-testcase result in Postgres.
	if _, createErr := qtx.CreateSubmissionResult(ctx, sqlc.CreateSubmissionResultParams{
		ID:            resultID,
		TestcaseID:    testcaseID,
		SubmissionID:  submissionID,
		Runtime:       runtimeNumeric,
		Memory:        memoryNumeric,
		PointsAwarded: pointsAwarded,
		Status:        status,
		Description:   &description,
	}); createErr != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("create submission result: %w", createErr)
	}

	// Commit DB write before removing token from Redis for read visibility
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return fmt.Errorf("commit result tx: %w", commitErr)
	}

	// Step 3: remove token from outstanding set
	remaining, err := utils.DeleteTokenAndCount(ctx, payload.Token, submissionIDStr)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}

	if remaining > 0 {
		// Other testcases for this submission are still pending.
		return nil
	}

	// Step 4: this was the last outstanding testcase. Aggregate every
	// submission_results row into the parent submissions row in a dedicated final transaction.
	finalTx, err := db.DBPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin final tx: %w", err)
	}
	defer func() { _ = finalTx.Rollback(ctx) }()
	finalQtx := db.Queries.WithTx(finalTx)

	if err := finalizeSubmission(ctx, finalQtx, submissionID); err != nil {
		return fmt.Errorf("finalize submission: %w", err)
	}

	if err := finalTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit final tx: %w", err)
	}
	return nil
}

// finalizeSubmission runs once per submission, exactly when the last
// testcase's callback arrives.
func finalizeSubmission(ctx context.Context, qtx *sqlc.Queries, submissionID uuid.UUID) error {
	submission, err := qtx.GetSubmissionForUpdate(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission for update: %w", err)
	}

	results, err := qtx.GetSubmissionResults(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission results: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no submission results found for submission %s", submissionID)
	}

	testcases, err := qtx.GetAllTestCasesByQuestion(ctx, submission.QuestionID)
	if err != nil {
		return fmt.Errorf("get testcases for question: %w", err)
	}
	if len(results) != len(testcases) {
		return fmt.Errorf("incomplete results for submission %s: got %d, expected %d",
			submissionID, len(results), len(testcases))
	}

	var passed, failed int32
	var maxRuntime, maxMemory float64
	overallStatus := utils.Judge0Accepted.GetJudge0Status()
	for _, r := range results {
		if r.Status == utils.Judge0Accepted.GetJudge0Status() {
			passed++
		} else {
			failed++
			if overallStatus == utils.Judge0Accepted.GetJudge0Status() {
				overallStatus = r.Status
			}
		}
		if rt, rtErr := utils.NumericToFloat64(r.Runtime); rtErr == nil {
			if rt > maxRuntime {
				maxRuntime = rt
			}
		} else {
			logging.Warnf("submission %s: could not read runtime for result %s: %v", submissionID, r.ID, rtErr)
		}
		if mem, memErr := utils.NumericToFloat64(r.Memory); memErr == nil {
			if mem > maxMemory {
				maxMemory = mem
			}
		} else {
			logging.Warnf("submission %s: could not read memory for result %s: %v", submissionID, r.ID, memErr)
		}
	}

	runtimeNumeric, err := utils.Float64ToNumeric(maxRuntime)
	if err != nil {
		return fmt.Errorf("convert aggregate runtime: %w", err)
	}
	memoryNumeric, err := utils.Float64ToNumeric(maxMemory)
	if err != nil {
		return fmt.Errorf("convert aggregate memory: %w", err)
	}

	var overallDesc string
	if failed > 0 {
		overallDesc = fmt.Sprintf("%d/%d testcases passed (%s)", passed, passed+failed, overallStatus)
	} else {
		overallDesc = fmt.Sprintf("All %d testcases passed", passed)
	}

	if updateErr := qtx.UpdateSubmissionStatus(ctx, sqlc.UpdateSubmissionStatusParams{
		ID:              submissionID,
		TestcasesPassed: &passed,
		TestcasesFailed: &failed,
		Runtime:         runtimeNumeric,
		Memory:          memoryNumeric,
		Status:          &overallStatus,
		Description:     &overallDesc,
	}); updateErr != nil {
		return fmt.Errorf("update submission status: %w", updateErr)
	}

	if failed > 0 {
		return nil // not a full solve -- no reward/score to hand out
	}

	attempt, err := qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
		UserID:     submission.UserID,
		QuestionID: submission.QuestionID,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get attempt: %w", err)
		}

		// No attempt row yet -- create one instead of dropping the reward.
		logging.Warnf("submission %s: no attempt row for user=%s question=%s -- auto-creating one",
			submissionID, submission.UserID, submission.QuestionID)

		if ensureErr := qtx.EnsureAttempt(ctx, sqlc.EnsureAttemptParams{
			ID:         uuid.New(),
			UserID:     submission.UserID,
			QuestionID: submission.QuestionID,
		}); ensureErr != nil {
			return fmt.Errorf("ensure attempt: %w", ensureErr)
		}

		// Re-fetch + lock: either the row we just made, or one a concurrent
		// finalize beat us to.
		attempt, err = qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
			UserID:     submission.UserID,
			QuestionID: submission.QuestionID,
		})
		if err != nil {
			return fmt.Errorf("get attempt after ensure: %w", err)
		}
	}
	if attempt.Status == "answered" {
		logging.Infof("submission %s: attempt already answered, skipping duplicate reward", submissionID)
		return nil
	}

	question, err := qtx.GetQuestionByID(ctx, submission.QuestionID)
	if err != nil {
		return fmt.Errorf("get question: %w", err)
	}
	rewardNumeric, err := qtx.GetQuestionReward(ctx, submission.QuestionID)
	if err != nil {
		return fmt.Errorf("get question reward: %w", err)
	}
	reward, err := utils.NumericToFloat64(rewardNumeric)
	if err != nil {
		reward = 0
	}

	// Lock score before balance to prevent deadlocks
	scoreNumeric, err := qtx.GetUserScoreForUpdate(ctx, submission.UserID)
	if err != nil {
		return fmt.Errorf("get user score: %w", err)
	}
	score, err := utils.NumericToFloat64(scoreNumeric)
	if err != nil {
		return fmt.Errorf("convert score: %w", err)
	}
	newScoreNumeric, err := utils.Float64ToNumeric(score + float64(question.Points))
	if err != nil {
		return fmt.Errorf("convert new score: %w", err)
	}
	if updateScoreErr := qtx.UpdateUserScore(ctx, sqlc.UpdateUserScoreParams{
		ID:    submission.UserID,
		Score: newScoreNumeric,
	}); updateScoreErr != nil {
		return fmt.Errorf("update user score: %w", updateScoreErr)
	}

	balanceNumeric, err := qtx.GetUserBalanceForUpdate(ctx, submission.UserID)
	if err != nil {
		return fmt.Errorf("get user balance: %w", err)
	}
	balance, err := utils.NumericToFloat64(balanceNumeric)
	if err != nil {
		return fmt.Errorf("convert balance: %w", err)
	}
	newBalanceNumeric, err := utils.Float64ToNumeric(balance + reward)
	if err != nil {
		return fmt.Errorf("convert new balance: %w", err)
	}
	if err := qtx.UpdateUserBalance(ctx, sqlc.UpdateUserBalanceParams{
		ID:      submission.UserID,
		Balance: newBalanceNumeric,
	}); err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}

	if err := qtx.UpdateAttemptStatus(ctx, sqlc.UpdateAttemptStatusParams{
		UserID:     submission.UserID,
		QuestionID: submission.QuestionID,
		Status:     "answered",
		AnsweredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("update attempt status: %w", err)
	}

	logging.Infof("submission %s finalized: user=%s question=%s reward=%.2f points=%d",
		submissionID, submission.UserID, submission.QuestionID, reward, question.Points)
	return nil
}
