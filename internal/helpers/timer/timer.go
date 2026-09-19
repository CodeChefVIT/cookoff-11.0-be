// Package timer owns the contest clock. It lives in Redis so every API
// replica agrees on it. Reads never write: whether a round is running is
// derived from its stored window, so a player's poll can never race an
// admin action and stop a round that was just started.
package timer

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
)

const (
	KeyRound     = "contest:round"
	KeyDuration  = "contest:timer:duration"
	KeyStartTime = "contest:timer:start_time"
	KeyEndTime   = "contest:timer:end_time"
	KeyIsRunning = "contest:timer:is_running"

	DefaultRound    int32 = 1
	DefaultDuration int64 = 3600 // 1 hour in seconds
)

var errNoRedis = errors.New("redis client is not initialized")

// ErrRoundNotRunning means the contest timer is stopped or running a different round.
var ErrRoundNotRunning = errors.New("round is not running")

// state is the raw clock as stored in Redis.
type state struct {
	round    int32
	duration int64
	started  bool // is_running flag: set by StartRound, cleared by a stop
	start    string
	end      string
}

func load(ctx context.Context) (state, error) {
	if utils.RedisClient == nil {
		return state{}, errNoRedis
	}
	vals, err := utils.RedisClient.MGet(ctx, KeyRound, KeyDuration, KeyIsRunning, KeyStartTime, KeyEndTime).Result()
	if err != nil {
		return state{}, err
	}
	str := func(v interface{}) string {
		s, _ := v.(string)
		return s
	}
	s := state{round: DefaultRound, duration: DefaultDuration}
	if r, perr := strconv.ParseInt(str(vals[0]), 10, 32); perr == nil && r > 0 {
		s.round = int32(r)
	}
	if d, perr := strconv.ParseInt(str(vals[1]), 10, 64); perr == nil && d > 0 {
		s.duration = d
	}
	s.started = str(vals[2]) == "true"
	s.start = str(vals[3])
	s.end = str(vals[4])
	return s, nil
}

// window returns the round's end, deriving it from start+duration when the
// end key is missing.
func (s state) window() (start, end time.Time, ok bool) {
	start, startErr := time.Parse(time.RFC3339, s.start)
	end, endErr := time.Parse(time.RFC3339, s.end)
	if endErr != nil && startErr == nil {
		end, endErr = start.Add(time.Duration(s.duration)*time.Second), nil
	}
	return start, end, endErr == nil
}

func (s state) response(now time.Time) dto.TimerResponse {
	res := dto.TimerResponse{Round: s.round, Duration: s.duration}
	start, end, ok := s.window()
	if !ok {
		// Never started (or the window was cleared by a reset).
		return res
	}
	endStr := end.Format(time.RFC3339)
	res.EndTime = &endStr
	if !start.IsZero() {
		startStr := start.Format(time.RFC3339)
		res.StartTime = &startStr
	}
	if s.started && now.Before(end) {
		res.IsRunning = true
		res.TimeLeft = int64(end.Sub(now).Seconds())
	}
	// A stopped or expired round keeps its window so "ended" stays
	// distinguishable from "not started".
	return res
}

func GetTime(ctx context.Context) (dto.TimerResponse, error) {
	s, err := load(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}
	return s.response(time.Now().UTC()), nil
}

// SetTime selects the round and its duration, leaving the clock stopped with
// no window ("not started").
func SetTime(ctx context.Context, round int32, durationSeconds int64) (dto.TimerResponse, error) {
	s, err := load(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}
	if round <= 0 {
		round = s.round
	}
	if durationSeconds <= 0 {
		durationSeconds = s.duration
	}

	pipe := utils.RedisClient.TxPipeline()
	pipe.Set(ctx, KeyRound, round, 0)
	pipe.Set(ctx, KeyDuration, durationSeconds, 0)
	pipe.Set(ctx, KeyIsRunning, "false", 0)
	pipe.Del(ctx, KeyStartTime, KeyEndTime)
	if _, err = pipe.Exec(ctx); err != nil {
		return dto.TimerResponse{}, err
	}

	return dto.TimerResponse{
		Round:     round,
		IsRunning: false,
		Duration:  durationSeconds,
		TimeLeft:  durationSeconds,
	}, nil
}

// UpdateTime adds (or, when negative, removes) time on the current round.
// While the round is running its end moves with it.
func UpdateTime(ctx context.Context, additionalSeconds int64) (dto.TimerResponse, error) {
	s, err := load(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}
	now := time.Now().UTC()
	current := s.response(now)

	duration := s.duration + additionalSeconds
	if duration < 0 {
		duration = 0
	}

	pipe := utils.RedisClient.TxPipeline()
	pipe.Set(ctx, KeyDuration, duration, 0)
	if current.IsRunning {
		_, end, _ := s.window()
		newEnd := end.Add(time.Duration(additionalSeconds) * time.Second)
		pipe.Set(ctx, KeyEndTime, newEnd.Format(time.RFC3339), 0)
		s.end = newEnd.Format(time.RFC3339)
	}
	if _, err = pipe.Exec(ctx); err != nil {
		return dto.TimerResponse{}, err
	}

	s.duration = duration
	return s.response(now), nil
}

// StartRound starts round (or the configured one) now for the configured duration.
func StartRound(ctx context.Context, round *int32) (dto.TimerResponse, error) {
	s, err := load(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}
	if round != nil && *round > 0 {
		s.round = *round
	}

	now := time.Now().UTC()
	s.started = true
	s.start = now.Format(time.RFC3339)
	s.end = now.Add(time.Duration(s.duration) * time.Second).Format(time.RFC3339)

	pipe := utils.RedisClient.TxPipeline()
	pipe.Set(ctx, KeyRound, s.round, 0)
	pipe.Set(ctx, KeyDuration, s.duration, 0)
	pipe.Set(ctx, KeyStartTime, s.start, 0)
	pipe.Set(ctx, KeyEndTime, s.end, 0)
	pipe.Set(ctx, KeyIsRunning, "true", 0)
	if _, err = pipe.Exec(ctx); err != nil {
		return dto.TimerResponse{}, err
	}
	return s.response(now), nil
}

// ResetRound stops the clock and clears the window ("not started").
func ResetRound(ctx context.Context) (dto.TimerResponse, error) {
	s, err := load(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}

	pipe := utils.RedisClient.TxPipeline()
	pipe.Set(ctx, KeyIsRunning, "false", 0)
	pipe.Del(ctx, KeyStartTime, KeyEndTime)
	if _, err = pipe.Exec(ctx); err != nil {
		return dto.TimerResponse{}, err
	}

	return dto.TimerResponse{Round: s.round, Duration: s.duration}, nil
}

// EnsureRoundRunning returns ErrRoundNotRunning unless the contest timer is currently running round.
func EnsureRoundRunning(ctx context.Context, round int32) error {
	status, err := GetTime(ctx)
	if err != nil {
		return err
	}
	if !status.IsRunning || status.Round != round {
		return ErrRoundNotRunning
	}
	return nil
}
