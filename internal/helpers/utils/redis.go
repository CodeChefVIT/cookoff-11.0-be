package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func InitRedis() {
	addr := fmt.Sprintf("%s:%s", Config.RedisHost, Config.RedisPort)

	RedisClient = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     Config.RedisPassword,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		logging.Fatalf("Failed to connect to Redis: %v", err)
	}

	logging.Infof("Redis client initialized successfully")
}

func CloseRedis() {
	if RedisClient != nil {
		if err := RedisClient.Close(); err != nil {
			logging.Errorf("Error closing Redis client: %v", err)
		} else {
			logging.Infof("Redis connection closed")
		}
	}
}

// SetCache serializes data to JSON and stores it in Redis with the given expiration duration
func SetCache(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}
	return RedisClient.Set(ctx, key, jsonData, expiration).Err()
}

// GetCache retrieves a value from Redis and deserializes the JSON data into dest
func GetCache(ctx context.Context, key string, dest interface{}) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	val, err := RedisClient.Get(ctx, key).Result()
	if err != nil {
		return err // Returns redis.Nil if key does not exist
	}
	return json.Unmarshal([]byte(val), dest)
}

// DeleteCache deletes a key from Redis
func DeleteCache(ctx context.Context, key string) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return RedisClient.Del(ctx, key).Err()
}

func SubmissionResultKey(submissionID string) string {
	return "sub:status:" + submissionID
}

// CacheSubmissionResult stores the final result so GetResult never has to
// touch Postgres to serve it.
func CacheSubmissionResult(ctx context.Context, submissionID string, result interface{}) error {
	return SetCache(ctx, SubmissionResultKey(submissionID), result, 30*time.Minute)
}

// AuthUserTTL bounds how stale a cached ban/role/round can be if an
// invalidation is ever missed. Admin writes invalidate immediately.
const AuthUserTTL = 15 * time.Second

const authUserPrefix = "authuser:"

func AuthUserKey(userID string) string {
	return authUserPrefix + userID
}

// InvalidateAuthUser drops the cached authorization view of one user.
func InvalidateAuthUser(ctx context.Context, userID string) {
	if RedisClient == nil {
		return
	}
	if err := RedisClient.Del(ctx, AuthUserKey(userID)).Err(); err != nil {
		logging.Warnf("invalidate auth user %s: %v", userID, err)
	}
}

// InvalidateAllAuthUsers drops every cached authorization view, e.g. after
// promoting everyone to the next round.
func InvalidateAllAuthUsers(ctx context.Context) {
	DeleteByPattern(ctx, authUserPrefix+"*")
}
