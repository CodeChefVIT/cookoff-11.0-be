package utils

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
)

// SubmissionDoneChannelPrefix is the Redis pub/sub channel the worker
// publishes on once a submission's verdict is committed and cached.
const SubmissionDoneChannelPrefix = "sub:done:"

// PublishSubmissionDone wakes every GET /result request waiting on this
// submission, on whichever API instance it is held.
func PublishSubmissionDone(ctx context.Context, submissionID string) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return RedisClient.Publish(ctx, SubmissionDoneChannelPrefix+submissionID, "1").Err()
}

// One pattern subscription per API process fans out to in-memory waiters. A
// Subscribe per request would hold a dedicated Redis connection for every
// participant waiting on a verdict.
var resultWaiters = struct {
	sync.Mutex
	byID map[string]map[chan struct{}]struct{}
}{byID: make(map[string]map[chan struct{}]struct{})}

// StartResultNotifier subscribes to verdict notifications until ctx is done.
// Without Redis it only logs: GetResult still has its fallback DB check.
func StartResultNotifier(ctx context.Context) {
	if RedisClient == nil {
		logging.Warnf("result notifier not started: redis client is not initialized")
		return
	}

	pubsub := RedisClient.PSubscribe(ctx, SubmissionDoneChannelPrefix+"*")
	messages := pubsub.Channel()

	go func() {
		defer func() { _ = pubsub.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-messages:
				if !ok {
					return
				}
				notifyResultWaiters(strings.TrimPrefix(msg.Channel, SubmissionDoneChannelPrefix))
			}
		}
	}()

	logging.Infof("Result notifier subscribed to %s*", SubmissionDoneChannelPrefix)
}

// WaitForResult registers interest in a submission's verdict. The returned
// channel receives once when the worker publishes it; cancel must be called
// when the caller stops waiting.
func WaitForResult(submissionID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	resultWaiters.Lock()
	if resultWaiters.byID[submissionID] == nil {
		resultWaiters.byID[submissionID] = make(map[chan struct{}]struct{})
	}
	resultWaiters.byID[submissionID][ch] = struct{}{}
	resultWaiters.Unlock()

	cancel := func() {
		resultWaiters.Lock()
		defer resultWaiters.Unlock()
		waiters := resultWaiters.byID[submissionID]
		delete(waiters, ch)
		if len(waiters) == 0 {
			delete(resultWaiters.byID, submissionID)
		}
	}
	return ch, cancel
}

func notifyResultWaiters(submissionID string) {
	resultWaiters.Lock()
	defer resultWaiters.Unlock()
	for ch := range resultWaiters.byID[submissionID] {
		// Buffered and non-blocking: a slow handler never stalls the fan-out.
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
