package utils

import (
	"context"
	"fmt"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

var TokenCache *redis.Client

// tokenTTL caps how long an unanswered Judge0 token lingers in Redis. Real
// callbacks arrive within seconds; this only reaps tokens whose callback
// never came.
const tokenTTL = 24 * time.Hour

func InitTokenCache() {
	addr := fmt.Sprintf("%s:%s", Config.RedisHost, Config.RedisPort)
	TokenCache = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: Config.RedisPassword,
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := TokenCache.Ping(ctx).Err(); err != nil {
		logging.Fatalf("Failed to connect token cache to Redis: %v", err)
	}
	logging.Infof("Token cache initialized successfully")
}
func CloseTokenCache() {
	if TokenCache == nil {
		return
	}

	if err := TokenCache.Close(); err != nil {
		logging.Errorf("Error closing token cache: %v", err)
		return
	}

	logging.Infof("Token cache connection closed")
}

func CacheToken(ctx context.Context, token string, submissionID string, testcaseID string) error {

	if TokenCache == nil {
		return fmt.Errorf("token cache is not initialized")
	}
	value := fmt.Sprintf("%s:%s", submissionID, testcaseID)
	pipe := TokenCache.TxPipeline()
	pipe.Set(ctx, tokenKey(token), value, tokenTTL)
	pipe.SAdd(ctx, submissionTokensKey(submissionID), token)
	pipe.Expire(ctx, submissionTokensKey(submissionID), tokenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to cache token %q: %w", token, err)
	}
	return nil
}

func CacheTokens(ctx context.Context, submissionID string, tokenToTestcase map[string]string) error {
	if TokenCache == nil {
		return fmt.Errorf("token cache is not initialized")
	}
	if len(tokenToTestcase) == 0 {
		return nil
	}

	pipe := TokenCache.TxPipeline()
	for token, testcaseID := range tokenToTestcase {
		value := fmt.Sprintf("%s:%s", submissionID, testcaseID)
		pipe.Set(ctx, tokenKey(token), value, tokenTTL)
		pipe.SAdd(ctx, submissionTokensKey(submissionID), token)
	}
	pipe.Expire(ctx, submissionTokensKey(submissionID), tokenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to cache tokens for submission %q: %w", submissionID, err)
	}
	return nil
}

func GetSubmissionIDByToken(ctx context.Context, token string) (submissionID, testcaseID string, err error) {
	val, err := TokenCache.Get(ctx, tokenKey(token)).Result()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve token %q: %w", token, err)
	}
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed cached value for token %q: %q", token, val)
	}
	return parts[0], parts[1], nil
}

// DeleteTokenAndCount removes a token AND reads how many tokens remain in
// the submission's outstanding-token set, as a single Redis MULTI/EXEC
// transaction (via TxPipeline). This is the fix for a real race condition:
// if "delete" and "count" were two separate Redis round trips (as they used
// to be), two workers finishing the last two testcases of the same
// submission at nearly the same instant could each delete their own token
// and THEN both read the set as empty -- both would believe they are "the
// last callback" and both would try to finalize the submission at once.
// Wrapping delete+count in one MULTI/EXEC block means no other client's
// command can be interleaved between them, so only the worker that
// genuinely empties the set will ever see remaining == 0.
func DeleteTokenAndCount(ctx context.Context, token, submissionID string) (int64, error) {
	pipe := TokenCache.TxPipeline()
	pipe.Del(ctx, tokenKey(token))
	pipe.SRem(ctx, submissionTokensKey(submissionID), token)
	countCmd := pipe.SCard(ctx, submissionTokensKey(submissionID))
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to delete token %q: %w", token, err)
	}
	return countCmd.Val(), nil
}

// RestoreToken re-adds a token that DeleteTokenAndCount already removed, for
// use when the Postgres side of a callback fails AFTER the token was
// removed from Redis. Without this compensating write, a failed DB commit
// would permanently shrink the fan-in counter by one and the affected
// submission could get stuck "pending" forever, waiting for a token that
// will never call back again.
func RestoreToken(ctx context.Context, token, submissionID, testcaseID string) error {
	return CacheToken(ctx, token, submissionID, testcaseID)
}

func tokenKey(token string) string {
	return "token:" + token
}
func submissionTokensKey(submissionID string) string {
	return "sub:" + submissionID + ":tokens"
}
