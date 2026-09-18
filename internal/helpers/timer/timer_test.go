package timer

import (
	"context"
	"testing"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setup(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	prev := utils.RedisClient
	utils.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { utils.RedisClient = prev })
	return mr
}

func TestStartThenRunning(t *testing.T) {
	setup(t)
	ctx := context.Background()
	if _, err := SetTime(ctx, 2, 600); err != nil {
		t.Fatal(err)
	}
	if res, _ := GetTime(ctx); res.IsRunning || res.EndTime != nil {
		t.Fatalf("set but not started should have no window: %+v", res)
	}
	round := int32(2)
	if _, err := StartRound(ctx, &round); err != nil {
		t.Fatal(err)
	}
	res, err := GetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsRunning || res.Round != 2 || res.TimeLeft <= 0 || res.EndTime == nil {
		t.Fatalf("expected running round 2: %+v", res)
	}
	if err := EnsureRoundRunning(ctx, 2); err != nil {
		t.Fatalf("round 2 should be running: %v", err)
	}
	if err := EnsureRoundRunning(ctx, 3); err != ErrRoundNotRunning {
		t.Fatalf("round 3 should not be running: %v", err)
	}
}

func TestExpiredRoundKeepsWindowWithoutWriting(t *testing.T) {
	mr := setup(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)
	_ = mr.Set(KeyRound, "2")
	_ = mr.Set(KeyDuration, "30")
	_ = mr.Set(KeyIsRunning, "true")
	_ = mr.Set(KeyStartTime, past.Add(-30*time.Second).Format(time.RFC3339))
	_ = mr.Set(KeyEndTime, past.Format(time.RFC3339))

	res, err := GetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsRunning || res.EndTime == nil || res.StartTime == nil {
		t.Fatalf("expired round should report ended with its window: %+v", res)
	}
	// Reads never write: the flag is still what the admin left.
	if v, _ := mr.Get(KeyIsRunning); v != "true" {
		t.Fatalf("GetTime wrote is_running=%q", v)
	}
}

// The old GetTime flipped is_running to false when it saw an expired end. A
// poll that read the old window just before an admin restarted the round
// could then stop the new round. Reads no longer write, so the restart holds.
func TestPollCannotStopFreshRound(t *testing.T) {
	mr := setup(t)
	ctx := context.Background()
	_ = mr.Set(KeyIsRunning, "true")
	_ = mr.Set(KeyEndTime, time.Now().UTC().Add(-time.Second).Format(time.RFC3339))

	round := int32(1)
	if _, err := StartRound(ctx, &round); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := GetTime(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if res, _ := GetTime(ctx); !res.IsRunning {
		t.Fatalf("round stopped by polling: %+v", res)
	}
}

func TestUpdateTimeMovesEnd(t *testing.T) {
	setup(t)
	ctx := context.Background()
	_, _ = SetTime(ctx, 1, 60)
	round := int32(1)
	before, _ := StartRound(ctx, &round)
	after, err := UpdateTime(ctx, 120, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := time.Parse(time.RFC3339, *before.EndTime)
	a, _ := time.Parse(time.RFC3339, *after.EndTime)
	if a.Sub(b) != 120*time.Second {
		t.Fatalf("end moved by %v", a.Sub(b))
	}
	// Shortening past now ends the round.
	if res, _ := UpdateTime(ctx, -3600, nil); res.IsRunning {
		t.Fatalf("shortened round still running: %+v", res)
	}
}

func TestResetClearsWindow(t *testing.T) {
	setup(t)
	ctx := context.Background()
	round := int32(1)
	_, _ = StartRound(ctx, &round)
	if _, err := ResetRound(ctx); err != nil {
		t.Fatal(err)
	}
	if res, _ := GetTime(ctx); res.IsRunning || res.EndTime != nil {
		t.Fatalf("reset should clear the window: %+v", res)
	}
}
