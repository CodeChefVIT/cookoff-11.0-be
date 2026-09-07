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
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/queue"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// judge0StatusMap mirrors docs/LLD.MD 2.6's status-code table: Judge0's
// numeric status.id -> the human string we store in submission_results.status.
var judge0StatusMap = map[int]string{
	1:  "In Queue",
	2:  "Processing",
	3:  "success",
	4:  "wrong answer",
	5:  "Time Limit Exceeded",
	6:  "Compilation error",
	7:  "Runtime error (SIGSEGV)",
	8:  "Runtime error (SIGXFSZ)",
	9:  "Runtime error (SIGFPE)",
	10: "Runtime error (SIGABRT)",
	11: "Runtime error (NZEC)",
	12: "Runtime error (Other)",
	13: "Internal Error",
	14: "Exec Format Error",
}

func mapJudge0Status(statusID int) string {
	if s, ok := judge0StatusMap[statusID]; ok {
		return s
	}
	return "Unknown"
}

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
	if parsed, err := uuid.Parse(testcaseIDStr); err == nil {
		testcaseID = pgtype.UUID{Bytes: parsed, Valid: true}
	} else {
		// Don't silently drop this -- a malformed cached testcase id means
		// something upstream (CacheToken's caller) wrote a bad value.
		logging.Warnf("submission %s: cached testcase id %q is not a valid UUID, storing result with no testcase link: %v",
			submissionID, testcaseIDStr, err)
	}

	status := mapJudge0Status(payload.Status.ID)

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
	if status == "success" {
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
	if _, err := qtx.CreateSubmissionResult(ctx, sqlc.CreateSubmissionResultParams{
		ID:            resultID,
		TestcaseID:    testcaseID,
		SubmissionID:  submissionID,
		Runtime:       runtimeNumeric,
		Memory:        memoryNumeric,
		PointsAwarded: pointsAwarded,
		Status:        status,
		Description:   &description,
	}); err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("create submission result: %w", err)
	}

	// Commit DB write before removing token from Redis for read visibility
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit result tx: %w", err)
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
	defer finalTx.Rollback(ctx)
	finalQtx := db.Queries.WithTx(finalTx)

	if err := finalizeSubmission(ctx, finalQtx, submissionID); err != nil {
		return fmt.Errorf("finalize submission: %w", err)
	}

	if err := finalTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit final tx: %w", err)
	}
	return nil
}

// restoreTokenOrLog is the best-effort compensation for dual-write gaps.
func restoreTokenOrLog(ctx context.Context, submissionID uuid.UUID, token, submissionIDStr, testcaseIDStr, stage string, causeErr error) {
	if restoreErr := utils.RestoreToken(ctx, token, submissionIDStr, testcaseIDStr); restoreErr != nil {
		logging.Errorf(
			"submission %s: %s failed AND restoring token %q failed -- manual fix needed (%s error: %v, restore error: %v)",
			submissionID, stage, token, stage, causeErr, restoreErr,
		)
	}
}

// finalizeSubmission runs once per submission, exactly when the last
// testcase's callback arrives.
func finalizeSubmission(ctx context.Context, qtx *sqlc.Queries, submissionID uuid.UUID) error {
	results, err := qtx.GetSubmissionResults(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission results: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("no submission results found for submission %s", submissionID)
	}

	var passed, failed int32
	var maxRuntime, maxMemory float64
	overallStatus := "success"
	for _, r := range results {
		if r.Status == "success" {
			passed++
		} else {
			failed++
			if overallStatus == "success" {
				overallStatus = r.Status
			}
		}
		if rt, err := utils.NumericToFloat64(r.Runtime); err == nil {
			if rt > maxRuntime {
				maxRuntime = rt
			}
		} else {
			logging.Warnf("submission %s: could not read runtime for result %s: %v", submissionID, r.ID, err)
		}
		if mem, err := utils.NumericToFloat64(r.Memory); err == nil {
			if mem > maxMemory {
				maxMemory = mem
			}
		} else {
			logging.Warnf("submission %s: could not read memory for result %s: %v", submissionID, r.ID, err)
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

	if err := qtx.UpdateSubmissionStatus(ctx, sqlc.UpdateSubmissionStatusParams{
		ID:              submissionID,
		TestcasesPassed: &passed,
		TestcasesFailed: &failed,
		Runtime:         runtimeNumeric,
		Memory:          memoryNumeric,
		Status:          &overallStatus,
		Description:     &overallDesc,
	}); err != nil {
		return fmt.Errorf("update submission status: %w", err)
	}

	if failed > 0 {
		return nil // not a full solve -- no reward/score to hand out
	}

	submission, err := qtx.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission: %w", err)
	}

	attempt, err := qtx.GetAttemptForUpdate(ctx, sqlc.GetAttemptForUpdateParams{
		UserID:     submission.UserID,
		QuestionID: submission.QuestionID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logging.Warnf("submission %s: no attempt row for user=%s question=%s -- skipping reward",
				submissionID, submission.UserID, submission.QuestionID)
			return nil
		}
		return fmt.Errorf("get attempt: %w", err)
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
	if err := qtx.UpdateUserScore(ctx, sqlc.UpdateUserScoreParams{
		ID:    submission.UserID,
		Score: newScoreNumeric,
	}); err != nil {
		return fmt.Errorf("update user score: %w", err)
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
