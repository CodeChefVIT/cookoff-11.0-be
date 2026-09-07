// Package queue owns the Asynq *client* side of the submission pipeline --
// the part that lives inside the API process (cmd/api) and pushes jobs onto
// Redis. The consumer side (the actual job handler) lives in internal/workers
// and runs inside the separate cmd/worker binary. See docs/HLD.md 1.3-1.5.
package queue

import (
	"encoding/json"
	"fmt"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/utils"
	"github.com/hibiken/asynq"
)

// TypeJudge0Callback is the Asynq task name for "a Judge0 callback needs
// processing". internal/workers.NewServeMux registers its handler against
// this exact string, so it must match on both sides.
const TypeJudge0Callback = "submission:process"

// Client is the process-wide Asynq client. It is a separate Redis connection
// from utils.RedisClient / utils.TokenCache -- Asynq speaks its own protocol
// (streams + sorted sets) on top of Redis, so it needs its own client type
// (asynq.Client), even though it points at the same Redis server.
var Client *asynq.Client

// InitQueue creates the Asynq client. Call once from cmd/api/main.go after
// utils.LoadConfig(), and `defer queue.CloseQueue()` right after.
func InitQueue() {
	Client = asynq.NewClient(RedisOpt())
	logging.Infof("Asynq queue client initialized successfully")
}

// CloseQueue releases the client's Redis connections on shutdown.
func CloseQueue() {
	if Client == nil {
		return
	}
	if err := Client.Close(); err != nil {
		logging.Errorf("Error closing asynq client: %v", err)
		return
	}
	logging.Infof("Asynq queue client closed")
}

// RedisOpt is shared by both the API's client (InitQueue) and the worker's
// server (cmd/worker/main.go) so they always point at the same Redis
// instance/credentials that utils.Config already parsed from the env.
func RedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%s", utils.Config.RedisHost, utils.Config.RedisPort),
		Password: utils.Config.RedisPassword,
		DB:       0,
	}
}

// EnqueueJudge0Callback is called by the /judge0callback controller. It just
// re-marshals the payload Judge0 sent us and drops it on the queue -- no DB
// or Redis token-cache work happens here, that's the worker's job. This is
// what makes the HTTP handler fast: Judge0 gets its 200 OK immediately.
func EnqueueJudge0Callback(payload dto.Judge0CallbackPayload) error {
	if Client == nil {
		return fmt.Errorf("queue client is not initialized")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal judge0 callback payload: %w", err)
	}

	task := asynq.NewTask(TypeJudge0Callback, data)

	info, err := Client.Enqueue(task)
	if err != nil {
		return fmt.Errorf("failed to enqueue judge0 callback task: %w", err)
	}

	logging.Infof("Enqueued task id=%s queue=%s type=%s", info.ID, info.Queue, info.Type)
	return nil
}
