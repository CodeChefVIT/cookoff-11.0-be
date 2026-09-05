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


func DeleteToken(ctx context.Context, token, submissionID string) error {
	pipe := TokenCache.TxPipeline()
	pipe.Del(ctx, tokenKey(token))
	pipe.SRem(ctx, submissionTokensKey(submissionID), token)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete token %q: %w", token, err)
	}
	return nil
}


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
