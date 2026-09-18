package utils

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func withRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	prev := RedisClient
	RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { RedisClient = prev })
	return mr
}

func TestCachedCollapsesConcurrentMisses(t *testing.T) {
	withRedis(t)
	var loads atomic.Int32
	release := make(chan struct{})
	load := func(context.Context) ([]string, error) {
		loads.Add(1)
		<-release
		return []string{"q1", "q2"}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Cached(context.Background(), "cache:content:round:2", time.Minute, load)
			if err != nil || len(got) != 2 {
				t.Errorf("got %v, %v", got, err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if n := loads.Load(); n != 1 {
		t.Fatalf("50 concurrent misses ran %d loads", n)
	}
	// Served from Redis afterwards.
	if _, err := Cached(context.Background(), "cache:content:round:2", time.Minute, load); err != nil {
		t.Fatal(err)
	}
	if n := loads.Load(); n != 1 {
		t.Fatalf("cached read reloaded (%d loads)", n)
	}
}

func TestCachedDoesNotCacheErrors(t *testing.T) {
	withRedis(t)
	calls := 0
	load := func(context.Context) (int, error) {
		calls++
		if calls == 1 {
			return 0, errors.New("db down")
		}
		return 7, nil
	}
	if _, err := Cached(context.Background(), "k", time.Minute, load); err == nil {
		t.Fatal("want error")
	}
	if v, err := Cached(context.Background(), "k", time.Minute, load); err != nil || v != 7 {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestInvalidateContentCache(t *testing.T) {
	mr := withRedis(t)
	_ = mr.Set("cache:content:round:1", "[]")
	_ = mr.Set("cache:content:question:x", "{}")
	_ = mr.Set("authuser:1", "{}")
	InvalidateContentCache(context.Background())
	if mr.Exists("cache:content:round:1") || mr.Exists("cache:content:question:x") {
		t.Fatal("content keys survived")
	}
	if !mr.Exists("authuser:1") {
		t.Fatal("unrelated key removed")
	}
}
