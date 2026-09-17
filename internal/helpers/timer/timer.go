package timer

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/redis/go-redis/v9"
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

// In-memory fallback if Redis is not initialized
var (
	memMu        sync.RWMutex
	memRound     int32 = DefaultRound
	memDuration  int64 = DefaultDuration
	memStartTime string
	memEndTime   string
	memIsRunning bool
)

func SetTime(ctx context.Context, round int32, durationSeconds int64) (dto.TimerResponse, error) {
	if round <= 0 {
		round = GetCurrentRound(ctx)
	}
	if durationSeconds <= 0 {
		durationSeconds = GetCurrentDuration(ctx)
	}

	if utils.RedisClient == nil {
		memMu.Lock()
		defer memMu.Unlock()
		memRound = round
		memDuration = durationSeconds
		memIsRunning = false
		memStartTime = ""
		memEndTime = ""
		return dto.TimerResponse{
			Round:     round,
			IsRunning: false,
			Duration:  durationSeconds,
			TimeLeft:  durationSeconds,
		}, nil
	}

	pipe := utils.RedisClient.Pipeline()
	pipe.Set(ctx, KeyRound, round, 0)
	pipe.Set(ctx, KeyDuration, durationSeconds, 0)
	pipe.Set(ctx, KeyIsRunning, "false", 0)
	pipe.Del(ctx, KeyStartTime)
	pipe.Del(ctx, KeyEndTime)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}

	return dto.TimerResponse{
		Round:     round,
		IsRunning: false,
		Duration:  durationSeconds,
		TimeLeft:  durationSeconds,
	}, nil
}

func UpdateTime(ctx context.Context, additionalSeconds int64, newDurationSeconds *int64) (dto.TimerResponse, error) {
	currentStatus, err := GetTime(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}

	duration := currentStatus.Duration
	if newDurationSeconds != nil && *newDurationSeconds > 0 {
		duration = *newDurationSeconds
	} else if additionalSeconds != 0 {
		duration += additionalSeconds
		if duration < 0 {
			duration = 0
		}
	}

	isRunning := currentStatus.IsRunning
	var startTimeStr, endTimeStr *string
	var timeLeft int64 = 0

	if isRunning && currentStatus.EndTime != nil {
		endT, parseErr := time.Parse(time.RFC3339, *currentStatus.EndTime)
		if parseErr == nil {
			var newEndT time.Time
			if additionalSeconds != 0 {
				newEndT = endT.Add(time.Duration(additionalSeconds) * time.Second)
			} else if newDurationSeconds != nil && currentStatus.StartTime != nil {
				startT, sErr := time.Parse(time.RFC3339, *currentStatus.StartTime)
				if sErr == nil {
					newEndT = startT.Add(time.Duration(duration) * time.Second)
				} else {
					newEndT = endT
				}
			} else {
				newEndT = endT
			}

			now := time.Now().UTC()
			if now.After(newEndT) {
				isRunning = false
				timeLeft = 0
			} else {
				timeLeft = int64(newEndT.Sub(now).Seconds())
			}

			formatted := newEndT.Format(time.RFC3339)
			endTimeStr = &formatted
			startTimeStr = currentStatus.StartTime
		}
	}

	if utils.RedisClient == nil {
		memMu.Lock()
		defer memMu.Unlock()
		memDuration = duration
		memIsRunning = isRunning
		if startTimeStr != nil {
			memStartTime = *startTimeStr
		} else {
			memStartTime = ""
		}
		if endTimeStr != nil {
			memEndTime = *endTimeStr
		} else {
			memEndTime = ""
		}
		return dto.TimerResponse{
			Round:     currentStatus.Round,
			IsRunning: isRunning,
			Duration:  duration,
			StartTime: startTimeStr,
			EndTime:   endTimeStr,
			TimeLeft:  timeLeft,
		}, nil
	}

	pipe := utils.RedisClient.Pipeline()
	pipe.Set(ctx, KeyDuration, duration, 0)
	if isRunning {
		pipe.Set(ctx, KeyIsRunning, "true", 0)
		if endTimeStr != nil {
			pipe.Set(ctx, KeyEndTime, *endTimeStr, 0)
		}
	} else {
		pipe.Set(ctx, KeyIsRunning, "false", 0)
		if currentStatus.IsRunning {
			pipe.Del(ctx, KeyStartTime)
			pipe.Del(ctx, KeyEndTime)
		}
	}
	_, execErr := pipe.Exec(ctx)
	if execErr != nil {
		return dto.TimerResponse{}, execErr
	}

	return dto.TimerResponse{
		Round:     currentStatus.Round,
		IsRunning: isRunning,
		Duration:  duration,
		StartTime: startTimeStr,
		EndTime:   endTimeStr,
		TimeLeft:  timeLeft,
	}, nil
}

func StartRound(ctx context.Context, round *int32) (dto.TimerResponse, error) {
	currentRound := GetCurrentRound(ctx)
	if round != nil && *round > 0 {
		currentRound = *round
	}

	duration := GetCurrentDuration(ctx)
	now := time.Now().UTC()
	endTime := now.Add(time.Duration(duration) * time.Second)

	startTimeStr := now.Format(time.RFC3339)
	endTimeStr := endTime.Format(time.RFC3339)

	if utils.RedisClient == nil {
		memMu.Lock()
		defer memMu.Unlock()
		memRound = currentRound
		memDuration = duration
		memIsRunning = true
		memStartTime = startTimeStr
		memEndTime = endTimeStr
		return dto.TimerResponse{
			Round:     currentRound,
			IsRunning: true,
			Duration:  duration,
			StartTime: &startTimeStr,
			EndTime:   &endTimeStr,
			TimeLeft:  duration,
		}, nil
	}

	pipe := utils.RedisClient.Pipeline()
	pipe.Set(ctx, KeyRound, currentRound, 0)
	pipe.Set(ctx, KeyDuration, duration, 0)
	pipe.Set(ctx, KeyIsRunning, "true", 0)
	pipe.Set(ctx, KeyStartTime, startTimeStr, 0)
	pipe.Set(ctx, KeyEndTime, endTimeStr, 0)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}

	return dto.TimerResponse{
		Round:     currentRound,
		IsRunning: true,
		Duration:  duration,
		StartTime: &startTimeStr,
		EndTime:   &endTimeStr,
		TimeLeft:  duration,
	}, nil
}

func ResetRound(ctx context.Context) (dto.TimerResponse, error) {
	currentRound := GetCurrentRound(ctx)
	duration := GetCurrentDuration(ctx)

	if utils.RedisClient == nil {
		memMu.Lock()
		defer memMu.Unlock()
		memIsRunning = false
		memStartTime = ""
		memEndTime = ""
		return dto.TimerResponse{
			Round:     currentRound,
			IsRunning: false,
			Duration:  duration,
			TimeLeft:  0,
		}, nil
	}

	pipe := utils.RedisClient.Pipeline()
	pipe.Set(ctx, KeyIsRunning, "false", 0)
	pipe.Del(ctx, KeyStartTime)
	pipe.Del(ctx, KeyEndTime)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return dto.TimerResponse{}, err
	}

	return dto.TimerResponse{
		Round:     currentRound,
		IsRunning: false,
		Duration:  duration,
		TimeLeft:  0,
	}, nil
}

func GetTime(ctx context.Context) (dto.TimerResponse, error) {
	if utils.RedisClient == nil {
		memMu.RLock()
		round := memRound
		duration := memDuration
		isRunning := memIsRunning
		startStr := memStartTime
		endStr := memEndTime
		memMu.RUnlock()

		if !isRunning {
			return dto.TimerResponse{
				Round:     round,
				IsRunning: false,
				Duration:  duration,
				TimeLeft:  0,
			}, nil
		}

		now := time.Now().UTC()
		var endT time.Time
		var parseErr error

		if endStr != "" {
			endT, parseErr = time.Parse(time.RFC3339, endStr)
		} else {
			parseErr = errors.New("missing end time")
		}

		if parseErr != nil {
			// If EndTime is missing or invalid, try calculating from StartTime + duration
			if startStr != "" {
				if startT, sErr := time.Parse(time.RFC3339, startStr); sErr == nil {
					endT = startT.Add(time.Duration(duration) * time.Second)
					parseErr = nil
					endStr = endT.Format(time.RFC3339)
				}
			}
		}

		if parseErr != nil || endT.IsZero() {
			memMu.Lock()
			memIsRunning = false
			memStartTime = ""
			memEndTime = ""
			memMu.Unlock()
			return dto.TimerResponse{
				Round:     round,
				IsRunning: false,
				Duration:  duration,
				TimeLeft:  0,
			}, nil
		}

		if now.After(endT) {
			memMu.Lock()
			memIsRunning = false
			memMu.Unlock()
			return dto.TimerResponse{
				Round:     round,
				IsRunning: false,
				Duration:  duration,
				StartTime: &startStr,
				EndTime:   &endStr,
				TimeLeft:  0,
			}, nil
		}

		remaining := int64(endT.Sub(now).Seconds())
		return dto.TimerResponse{
			Round:     round,
			IsRunning: true,
			Duration:  duration,
			StartTime: &startStr,
			EndTime:   &endStr,
			TimeLeft:  remaining,
		}, nil
	}

	round := GetCurrentRound(ctx)
	duration := GetCurrentDuration(ctx)

	isRunningVal, err := utils.RedisClient.Get(ctx, KeyIsRunning).Result()
	if err != nil && err != redis.Nil {
		return dto.TimerResponse{}, err
	}
	isRunning := isRunningVal == "true"

	if !isRunning {
		return dto.TimerResponse{
			Round:     round,
			IsRunning: false,
			Duration:  duration,
			TimeLeft:  0,
		}, nil
	}

	startStr, _ := utils.RedisClient.Get(ctx, KeyStartTime).Result()
	endStr, err := utils.RedisClient.Get(ctx, KeyEndTime).Result()

	now := time.Now().UTC()
	var endT time.Time
	var parseErr error

	if err == nil && endStr != "" {
		endT, parseErr = time.Parse(time.RFC3339, endStr)
	} else {
		parseErr = errors.New("missing end time")
	}

	if parseErr != nil {
		// If EndTime is missing or invalid, try calculating from StartTime + duration
		if startStr != "" {
			if startT, sErr := time.Parse(time.RFC3339, startStr); sErr == nil {
				endT = startT.Add(time.Duration(duration) * time.Second)
				parseErr = nil
				endStr = endT.Format(time.RFC3339)
				_ = utils.RedisClient.Set(ctx, KeyEndTime, endStr, 0).Err()
			}
		}
	}

	if parseErr != nil || endT.IsZero() {
		pipe := utils.RedisClient.Pipeline()
		pipe.Set(ctx, KeyIsRunning, "false", 0)
		pipe.Del(ctx, KeyStartTime)
		pipe.Del(ctx, KeyEndTime)
		_, _ = pipe.Exec(ctx)

		return dto.TimerResponse{
			Round:     round,
			IsRunning: false,
			Duration:  duration,
			TimeLeft:  0,
		}, nil
	}

	if now.After(endT) {
		_ = utils.RedisClient.Set(ctx, KeyIsRunning, "false", 0).Err()
		return dto.TimerResponse{
			Round:     round,
			IsRunning: false,
			Duration:  duration,
			StartTime: &startStr,
			EndTime:   &endStr,
			TimeLeft:  0,
		}, nil
	}

	remaining := int64(endT.Sub(now).Seconds())
	return dto.TimerResponse{
		Round:     round,
		IsRunning: true,
		Duration:  duration,
		StartTime: &startStr,
		EndTime:   &endStr,
		TimeLeft:  remaining,
	}, nil
}

func GetCurrentRound(ctx context.Context) int32 {
	if utils.RedisClient == nil {
		memMu.RLock()
		defer memMu.RUnlock()
		return memRound
	}
	val, err := utils.RedisClient.Get(ctx, KeyRound).Result()
	if err != nil {
		return DefaultRound
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed <= 0 {
		return DefaultRound
	}
	return int32(parsed)
}

func GetCurrentDuration(ctx context.Context) int64 {
	if utils.RedisClient == nil {
		memMu.RLock()
		defer memMu.RUnlock()
		return memDuration
	}
	val, err := utils.RedisClient.Get(ctx, KeyDuration).Result()
	if err != nil {
		return DefaultDuration
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil || parsed <= 0 {
		return DefaultDuration
	}
	return parsed
}
