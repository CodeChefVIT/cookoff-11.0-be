// internal/utils/tokenCache.go
package utils

import (
	"context"
	"fmt"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/redis/go-redis/v9"
	"strings"
	"time"
)

// TokenCache is a dedicated Redis client for the Judge0 token <-> submission
// mapping, kept separate from the general-purpose RedisClient (see redis.go)
// per LLD §2.1.
var TokenCache *redis.Client

// the type. redis.Client is a struct (a bundle of fields) defined inside the go-redis library that represents "a connection to a Redis server." The * in front means this variable doesn't hold the actual Client struct — it holds a pointer, i.e. the address of one.
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

// CacheToken stores token -> "submissionID:testcaseID" and adds the token to
// the submission's outstanding-token set. Called by SubmitCode per LLD §2.7.
func CacheToken(ctx context.Context, token string, submissionID string, testcaseID string) error {

	if TokenCache == nil {
		return fmt.Errorf("token cache is not initialized")
	}
	value := fmt.Sprintf("%s:%s", submissionID, testcaseID)
	if err := TokenCache.Set(ctx, tokenKey(token), value, 0).Err(); err != nil {
		return fmt.Errorf("failed to cache token %q: %w", token, err)
	}
	if err := TokenCache.SAdd(ctx, submissionTokensKey(submissionID), token).Err(); err != nil {
		return fmt.Errorf("failed to add token %q to submission set: %w", token, err)
	}
	return nil
}

// GetSubmissionIDByToken resolves a Judge0 token back into (submissionID, testcaseID).
// Called by the worker on every callback per LLD §2.6.
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

// DeleteToken removes a resolved token from both the direct mapping and the
// submission's outstanding-token set, atomically.
func DeleteToken(ctx context.Context, token, submissionID string) error {
	pipe := TokenCache.TxPipeline()
	pipe.Del(ctx, tokenKey(token))
	pipe.SRem(ctx, submissionTokensKey(submissionID), token)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete token %q: %w", token, err)
	}
	return nil
}

// GetTokenCount returns how many Judge0 tokens are still outstanding for a
// submission. When this hits 0, the worker finalizes the submission.
func GetTokenCount(ctx context.Context, submissionID string) (int64, error) {
	count, err := TokenCache.SCard(ctx, submissionTokensKey(submissionID)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens for submission %q: %w", submissionID, err)
	}
	return count, nil
}
func tokenKey(token string) string {
	return "token:" + token
}
func submissionTokensKey(submissionID string) string {
	return "sub:" + submissionID + ":tokens"
}
