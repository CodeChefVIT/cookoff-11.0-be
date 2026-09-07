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

	tx, err := db.DBPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := db.Queries.WithTx(tx)

	// Step 2: record this single testcase's result.
	if _, err := qtx.CreateSubmissionResult(ctx, sqlc.CreateSubmissionResultParams{
		ID:            uuid.New(),
		TestcaseID:    testcaseID,
		SubmissionID:  submissionID,
		Runtime:       runtimeNumeric,
		Memory:        memoryNumeric,
		PointsAwarded: pointsAwarded,
		Status:        status,
		Description:   &description,
	}); err != nil {
		return fmt.Errorf("create submission result: %w", err)
	}

	// Step 3: this token is now resolved -- remove it from the outstanding
	// set for this submission (the fan-out/fan-in counter from HLD 1.5).
	//
	// DeleteTokenAndCount does the removal AND the remaining-count check as
	// one atomic Redis operation. That matters: if these were two separate
	// calls, two workers finishing the last two testcases of the same
	// submission at nearly the same moment could each remove their own
	// token and then BOTH see the set as empty, so both would try to
	// finalize the same submission at once. Doing it atomically guarantees
	// only one worker ever observes remaining == 0.
	remaining, err := utils.DeleteTokenAndCount(ctx, payload.Token, submissionIDStr)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}

	if remaining > 0 {
		// Other testcases for this submission are still pending -- nothing
		// more to do until the last one calls back.
		if err := tx.Commit(ctx); err != nil {
			// The token was already removed from Redis above, but the DB
			// write that was supposed to go with it just failed to commit.
			// Put the token back so the fan-in counter isn't left one short
			// -- otherwise this submission would wait forever for a token
			// that is never coming back.
			restoreTokenOrLog(ctx, submissionID, payload.Token, submissionIDStr, testcaseIDStr, "commit", err)
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	}

	// Step 4: this was the last outstanding testcase. Aggregate every
	// submission_results row into the parent submissions row, and (if every
	// testcase passed) award balance/score once, mirroring the Round-1
	// visual-submission flow in controllers/submit_round1.go.
	if err := finalizeSubmission(ctx, qtx, submissionID); err != nil {
		restoreTokenOrLog(ctx, submissionID, payload.Token, submissionIDStr, testcaseIDStr, "finalize", err)
		return fmt.Errorf("finalize submission: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		restoreTokenOrLog(ctx, submissionID, payload.Token, submissionIDStr, testcaseIDStr, "commit", err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// restoreTokenOrLog is the best-effort compensation for the one remaining
// dual-write gap: Redis and Postgres can't be rolled back together, so if
// the DB side fails AFTER the token was already deleted from Redis, we try
// to put it back. If even that fails, there's nothing left to do
// automatically -- log loudly so it can be fixed by hand (re-add the token
// for this submission, or ask the judge to re-run it).
func restoreTokenOrLog(ctx context.Context, submissionID uuid.UUID, token, submissionIDStr, testcaseIDStr, stage string, causeErr error) {
	if restoreErr := utils.RestoreToken(ctx, token, submissionIDStr, testcaseIDStr); restoreErr != nil {
		logging.Errorf(
			"submission %s: %s failed AND restoring token %q failed -- manual fix needed (%s error: %v, restore error: %v)",
			submissionID, stage, token, stage, causeErr, restoreErr,
		)
	}
}

// finalizeSubmission runs once per submission, exactly when the last
// testcase's callback arrives (tracked via the Redis token set). It must run
// inside the same transaction as the triggering CreateSubmissionResult call
// so a crash between "write result" and "aggregate" can't leave the
// submission stuck half-updated.
func finalizeSubmission(ctx context.Context, qtx *sqlc.Queries, submissionID uuid.UUID) error {
	results, err := qtx.GetSubmissionResults(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("get submission results: %w", err)
	}
	if len(results) == 0 {
		// This should be impossible by the time the fan-in counter hits
		// zero. If it ever happens, something upstream is broken (e.g. a
		// read-committed visibility gap between two racing workers) -- fail
		// loudly instead of silently rewarding a submission with nothing
		// actually graded.
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
				// First failing testcase's status becomes the submission's
				// overall status (e.g. "wrong answer", "Time Limit Exceeded").
				overallStatus = r.Status
			}
		}
		if rt, err := utils.NumericToFloat64(r.Runtime); err == nil {
			if rt > maxRuntime {
				maxRuntime = rt // worst-case runtime across testcases
			}
		} else {
			logging.Warnf("submission %s: could not read runtime for result %s: %v", submissionID, r.ID, err)
		}
		if mem, err := utils.NumericToFloat64(r.Memory); err == nil {
			if mem > maxMemory {
				maxMemory = mem // peak memory across testcases (same aggregation as runtime, not a sum)
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

	if err := qtx.UpdateSubmissionStatus(ctx, sqlc.UpdateSubmissionStatusParams{
		ID:              submissionID,
		TestcasesPassed: &passed,
		TestcasesFailed: &failed,
		Runtime:         runtimeNumeric,
		Memory:          memoryNumeric,
		Status:          &overallStatus,
		Description:     nil,
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
			// No attempt row for this user/question -- nothing to award.
			// Logged (not silent) because this is expected to be common
			// right now if user_id isn't wired into SubmitCode yet.
			logging.Warnf("submission %s: no attempt row for user=%s question=%s -- skipping reward",
				submissionID, submission.UserID, submission.QuestionID)
			return nil
		}
		return fmt.Errorf("get attempt: %w", err)
	}
	if attempt.Status == "answered" {
		// Already rewarded once; a re-judged callback can't double-pay.
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
		reward = 0 // reward is nullable on some questions; treat unset as 0
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