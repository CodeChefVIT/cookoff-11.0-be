package utils

import (
	"context"
	"encoding/json"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"golang.org/x/sync/singleflight"
)

// ContentCachePrefix namespaces cached contest content (questions, public
// testcases, blocks). Admin edits drop the whole namespace.
const ContentCachePrefix = "cache:content:"

var cacheGroup singleflight.Group

// Cached returns the JSON value stored under key, or loads, stores and
// returns it. Concurrent misses in this process share one load, so a whole
// hall opening a round at once costs one query, not hundreds. Errors from
// load are returned and never cached.
func Cached[T any](ctx context.Context, key string, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	var zero T
	if RedisClient != nil {
		if raw, err := RedisClient.Get(ctx, key).Bytes(); err == nil {
			var v T
			if json.Unmarshal(raw, &v) == nil {
				return v, nil
			}
		}
	}

	v, err, _ := cacheGroup.Do(key, func() (interface{}, error) {
		loaded, loadErr := load(context.WithoutCancel(ctx))
		if loadErr != nil {
			return nil, loadErr
		}
		if RedisClient != nil {
			if raw, marshalErr := json.Marshal(loaded); marshalErr == nil {
				if setErr := RedisClient.Set(ctx, key, raw, ttl).Err(); setErr != nil {
					logging.Warnf("cache set %s: %v", key, setErr)
				}
			}
		}
		return loaded, nil
	})
	if err != nil {
		return zero, err
	}
	return v.(T), nil
}

// DeleteByPattern removes every key matching pattern.
func DeleteByPattern(ctx context.Context, pattern string) {
	if RedisClient == nil {
		return
	}
	iter := RedisClient.Scan(ctx, 0, pattern, 500).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) == 500 {
			RedisClient.Del(ctx, keys...)
			keys = keys[:0]
		}
	}
	if len(keys) > 0 {
		RedisClient.Del(ctx, keys...)
	}
	if err := iter.Err(); err != nil {
		logging.Warnf("delete %s: %v", pattern, err)
	}
}

// InvalidateContentCache drops cached questions, testcases and blocks.
func InvalidateContentCache(ctx context.Context) {
	DeleteByPattern(ctx, ContentCachePrefix+"*")
}
