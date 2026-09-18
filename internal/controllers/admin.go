package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/logging"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

// maxRound is the last contest round.
const maxRound int32 = 3

type AdminController struct {
	queries *sqlc.Queries
}

func NewAdminController(queries *sqlc.Queries) *AdminController {
	return &AdminController{queries: queries}
}

func AdminSession(_ *sqlc.Queries) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, ok := c.Get(middlewares.UserIDKey).(string)
		if !ok {
			return c.JSON(http.StatusUnauthorized, dto.NewErrorResponse("Unauthorized", nil))
		}
		return c.JSON(http.StatusOK, dto.NewSuccessResponse("Admin session validated", echo.Map{"user_id": userID, "role": c.Get(middlewares.RoleKey)}))
	}
}

func (ac *AdminController) GetAllUsers(c echo.Context) error {
	ctx := c.Request().Context()
	users, err := ac.queries.GetAllUsers(ctx)
	if err != nil {
		logging.Errorf("GetAllUsers failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch users", dto.CodeInternal))
	}

	res := make([]dto.UserResponse, len(users))
	for i, u := range users {
		balance, _ := utils.NumericToFloat64(u.Balance)
		score, _ := utils.NumericToFloat64(u.Score)
		res[i] = dto.UserResponse{
			ID:             u.ID.String(),
			Email:          u.Email,
			RegNo:          u.RegNo,
			Role:           u.Role,
			RoundQualified: u.RoundQualified,
			Balance:        balance,
			Score:          score,
			Name:           u.Name,
			IsBanned:       u.IsBanned,
		}
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Users fetched successfully", res))
}

func (ac *AdminController) BanUser(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid user ID", nil))
	}

	user, err := ac.queries.BanUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("User not found", nil))
		}
		logging.Errorf("BanUser failed for user %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to ban user", dto.CodeInternal))
	}

	utils.InvalidateAuthUser(ctx, id.String())
	logging.Infof("User %s banned by admin", id)
	balance, _ := utils.NumericToFloat64(user.Balance)
	score, _ := utils.NumericToFloat64(user.Score)
	res := dto.UserResponse{
		ID:             user.ID.String(),
		Email:          user.Email,
		RegNo:          user.RegNo,
		Role:           user.Role,
		RoundQualified: user.RoundQualified,
		Balance:        balance,
		Score:          score,
		Name:           user.Name,
		IsBanned:       user.IsBanned,
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("User banned successfully", res))
}

func (ac *AdminController) UnbanUser(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid user ID", nil))
	}

	user, err := ac.queries.UnbanUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("User not found", nil))
		}
		logging.Errorf("UnbanUser failed for user %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to unban user", dto.CodeInternal))
	}

	utils.InvalidateAuthUser(ctx, id.String())
	logging.Infof("User %s unbanned by admin", id)
	balance, _ := utils.NumericToFloat64(user.Balance)
	score, _ := utils.NumericToFloat64(user.Score)
	res := dto.UserResponse{
		ID:             user.ID.String(),
		Email:          user.Email,
		RegNo:          user.RegNo,
		Role:           user.Role,
		RoundQualified: user.RoundQualified,
		Balance:        balance,
		Score:          score,
		Name:           user.Name,
		IsBanned:       user.IsBanned,
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("User unbanned successfully", res))
}

func (ac *AdminController) UpgradeUser(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid user ID", nil))
	}

	existingUser, err := ac.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("User not found", nil))
		}
		logging.Errorf("UpgradeUser failed to fetch user %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to find user", dto.CodeInternal))
	}

	var req dto.UpgradeUserRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid request payload", dto.CodeValidation))
	}

	targetRound := req.RoundQualified
	if targetRound == nil {
		targetRound = req.Round
	}

	if targetRound != nil && (*targetRound < 1 || *targetRound > maxRound) {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("round_qualified must be between 1 and 3", dto.CodeValidation))
	}

	updatedUser := existingUser
	hasUpdate := false

	if req.Role != nil && *req.Role != "" {
		updatedUser, err = ac.queries.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
			ID:   id,
			Role: *req.Role,
		})
		if err != nil {
			logging.Errorf("UpgradeUser failed updating role for user %s: %v", id, err)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to update user role", dto.CodeInternal))
		}
		hasUpdate = true
	}

	if targetRound != nil {
		updatedUser, err = ac.queries.UpgradeUserRound(ctx, sqlc.UpgradeUserRoundParams{
			ID:             id,
			RoundQualified: *targetRound,
		})
		if err != nil {
			logging.Errorf("UpgradeUser failed upgrading round for user %s: %v", id, err)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to upgrade user round", dto.CodeInternal))
		}
		hasUpdate = true
	}

	if !hasUpdate {
		// The admin panel's "upgrade" button posts no body and relies on this
		// increment; it stops at the final round.
		if existingUser.RoundQualified >= maxRound {
			return c.JSON(http.StatusBadRequest, dto.NewCodedError("User is already in the final round", dto.CodeValidation))
		}
		updatedUser, err = ac.queries.IncrementUserRound(ctx, id)
		if err != nil {
			logging.Errorf("UpgradeUser failed incrementing round for user %s: %v", id, err)
			return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to increment user round", dto.CodeInternal))
		}
	}

	utils.InvalidateAuthUser(ctx, id.String())
	logging.Infof("User %s upgraded by admin", id)
	balance, _ := utils.NumericToFloat64(updatedUser.Balance)
	score, _ := utils.NumericToFloat64(updatedUser.Score)
	res := dto.UserResponse{
		ID:             updatedUser.ID.String(),
		Email:          updatedUser.Email,
		RegNo:          updatedUser.RegNo,
		Role:           updatedUser.Role,
		RoundQualified: updatedUser.RoundQualified,
		Balance:        balance,
		Score:          score,
		Name:           updatedUser.Name,
		IsBanned:       updatedUser.IsBanned,
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("User upgraded successfully", res))
}

func (ac *AdminController) UpgradeAllUsers(c echo.Context) error {
	ctx := c.Request().Context()
	var req dto.UpgradeAllUsersRequest
	if err := c.Bind(&req); err != nil && c.Request().ContentLength > 0 {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid request payload", dto.CodeValidation))
	}

	targetRound := int32(2)
	if req.TargetRound != nil && *req.TargetRound > 0 {
		targetRound = *req.TargetRound
	} else if req.Round != nil && *req.Round > 0 {
		targetRound = *req.Round
	}
	if targetRound > maxRound {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("round must be between 1 and 3", dto.CodeValidation))
	}

	rowsAffected, err := ac.queries.UpgradeAllUsersRound(ctx, targetRound)
	if err != nil {
		logging.Errorf("UpgradeAllUsers failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to upgrade all users", dto.CodeInternal))
	}

	utils.InvalidateAllAuthUsers(ctx)
	logging.Infof("Upgraded %d users to round %d by admin", rowsAffected, targetRound)
	return c.JSON(http.StatusOK, dto.NewSuccessResponse(
		"All non-banned users upgraded successfully",
		echo.Map{
			"round_qualified": targetRound,
			"users_upgraded":  rowsAffected,
		},
	))
}

func (ac *AdminController) GetUserSubmissions(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid user ID", nil))
	}

	if _, userErr := ac.queries.GetUserByID(ctx, id); userErr != nil {
		if errors.Is(userErr, pgx.ErrNoRows) {
			return c.JSON(http.StatusNotFound, dto.NewErrorResponse("User not found", nil))
		}
		logging.Errorf("GetUserSubmissions failed to fetch user %s: %v", id, userErr)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch user", dto.CodeInternal))
	}

	submissions, err := ac.queries.GetUserSubmissions(ctx, id)
	if err != nil {
		logging.Errorf("GetUserSubmissions failed for user %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch user submissions", dto.CodeInternal))
	}

	res := make([]dto.UserSubmissionResponse, len(submissions))
	for i, s := range submissions {
		var passed, failed int32
		if s.TestcasesPassed != nil {
			passed = *s.TestcasesPassed
		}
		if s.TestcasesFailed != nil {
			failed = *s.TestcasesFailed
		}

		runtime, _ := utils.NumericToFloat64(s.Runtime)
		memory, _ := utils.NumericToFloat64(s.Memory)

		status := ""
		if s.Status != nil {
			status = *s.Status
		}

		desc := ""
		if s.Description != nil {
			desc = *s.Description
		}

		submissionTimeStr := ""
		if s.SubmissionTime.Valid {
			submissionTimeStr = s.SubmissionTime.Time.Format(time.RFC3339)
		}

		res[i] = dto.UserSubmissionResponse{
			ID:              s.ID.String(),
			QuestionID:      s.QuestionID.String(),
			QuestionTitle:   s.QuestionTitle,
			QuestionRound:   s.QuestionRound,
			TestcasesPassed: passed,
			TestcasesFailed: failed,
			Runtime:         runtime,
			Memory:          memory,
			LanguageID:      s.LanguageID,
			Status:          status,
			Description:     desc,
			SourceCode:      s.SourceCode,
			SubmissionTime:  submissionTimeStr,
		}
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("User submissions fetched successfully", res))
}

// leaderboardTTL keeps an admin panel that polls the leaderboard from
// re-running the full aggregate over every submission each time.
const leaderboardTTL = 10 * time.Second

func (ac *AdminController) GetLeaderboard(c echo.Context) error {
	res, err := utils.Cached(c.Request().Context(), "cache:leaderboard", leaderboardTTL, ac.buildLeaderboard)
	if err != nil {
		logging.Errorf("GetLeaderboard failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch leaderboard data", dto.CodeInternal))
	}
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Leaderboard fetched successfully", res))
}

func (ac *AdminController) buildLeaderboard(ctx context.Context) ([]dto.LeaderboardEntry, error) {
	data, err := ac.queries.GetLeaderboardData(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]dto.LeaderboardEntry, len(data))
	for i, row := range data {
		score, _ := utils.NumericToFloat64(row.Score)
		totalRuntime, _ := utils.NumericToFloat64(row.TotalRuntime)

		var lastSubTime *string
		if row.LastSubmissionTime.Valid {
			formatted := row.LastSubmissionTime.Time.Format(time.RFC3339)
			lastSubTime = &formatted
		}

		res[i] = dto.LeaderboardEntry{
			Rank:               i + 1,
			ID:                 row.ID.String(),
			Name:               row.Name,
			Email:              row.Email,
			RegNo:              row.RegNo,
			Score:              score,
			RoundQualified:     row.RoundQualified,
			TotalRuntime:       totalRuntime,
			LastSubmissionTime: lastSubTime,
			TotalSubmissions:   row.TotalSubmissions,
			SolvedCount:        row.SolvedCount,
			IsBanned:           row.IsBanned,
		}
	}
	return res, nil
}

func (ac *AdminController) GetAnalytics(c echo.Context) error {
	ctx := c.Request().Context()

	activeUsers, err := ac.queries.GetActiveUsersCount(ctx)
	if err != nil {
		logging.Errorf("GetAnalytics failed to fetch active users count: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch active users count", dto.CodeInternal))
	}

	totalUsers, err := ac.queries.GetTotalUsersCount(ctx)
	if err != nil {
		logging.Errorf("GetAnalytics failed to fetch total users count: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch total users count", dto.CodeInternal))
	}

	bannedUsers, err := ac.queries.GetBannedUsersCount(ctx)
	if err != nil {
		logging.Errorf("GetAnalytics failed to fetch banned users count: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch banned users count", dto.CodeInternal))
	}

	subAnalytics, err := ac.queries.GetSubmissionsAnalytics(ctx)
	if err != nil {
		logging.Errorf("GetAnalytics failed to fetch submission analytics: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to fetch submission analytics", dto.CodeInternal))
	}

	tenMinutesAgo := time.Now().UTC().Add(-10 * time.Minute)
	recentSubmissions, err := ac.queries.GetRecentSubmissionsCount(ctx, pgtype.Timestamptz{
		Time:  tenMinutesAgo,
		Valid: true,
	})
	if err != nil {
		recentSubmissions = 0
	}

	langDistRows, err := ac.queries.GetLanguageDistribution(ctx)
	if err != nil {
		langDistRows = nil
	}

	langDist := make(map[int32]int32)
	for _, row := range langDistRows {
		langDist[row.LanguageID] = row.SubmissionCount
	}

	var overallPassRate float64
	if subAnalytics.TotalSubmissions > 0 {
		overallPassRate = (float64(subAnalytics.SuccessfulSubmissions) / float64(subAnalytics.TotalSubmissions)) * 100.0
	}

	ratePerMin := float64(recentSubmissions) / 10.0

	var tcPassRate float64
	totalTestcases := subAnalytics.TotalTestcasesPassed + subAnalytics.TotalTestcasesFailed
	if totalTestcases > 0 {
		tcPassRate = (float64(subAnalytics.TotalTestcasesPassed) / float64(totalTestcases)) * 100.0
	}

	res := dto.AnalyticsResponse{
		ActiveUsers:            activeUsers,
		TotalUsers:             totalUsers,
		BannedUsers:            bannedUsers,
		TotalSubmissions:       subAnalytics.TotalSubmissions,
		SuccessfulSubmissions:  subAnalytics.SuccessfulSubmissions,
		FailedSubmissions:      subAnalytics.FailedSubmissions,
		OverallPassRate:        overallPassRate,
		RecentSubmissionsCount: recentSubmissions,
		SubmissionRatePerMin:   ratePerMin,
		TotalTestcasesPassed:   subAnalytics.TotalTestcasesPassed,
		TotalTestcasesFailed:   subAnalytics.TotalTestcasesFailed,
		TestcasePassRate:       tcPassRate,
		LanguageDistribution:   langDist,
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Analytics fetched successfully", res))
}

func (ac *AdminController) SetTime(c echo.Context) error {
	var req dto.SetTimeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid request payload", dto.CodeValidation))
	}

	var durationSeconds int64 = 3600
	if req.DurationSeconds != nil && *req.DurationSeconds > 0 {
		durationSeconds = *req.DurationSeconds
	} else if req.DurationMinutes != nil && *req.DurationMinutes > 0 {
		durationSeconds = *req.DurationMinutes * 60
	} else if req.Duration != nil && *req.Duration > 0 {
		durationSeconds = *req.Duration
	}

	var round int32 = 1
	if req.Round != nil && *req.Round > 0 {
		round = *req.Round
	}

	res, err := timer.SetTime(c.Request().Context(), round, durationSeconds)
	if err != nil {
		logging.Errorf("SetTime failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to set round timer", dto.CodeInternal))
	}

	logging.Infof("Round timer set by admin: round=%d, duration=%ds", round, durationSeconds)
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round timer set successfully", res))
}

func (ac *AdminController) UpdateTime(c echo.Context) error {
	var req dto.UpdateTimeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewCodedError("Invalid request payload", dto.CodeValidation))
	}

	var additionalSeconds int64
	if req.AdditionalSeconds != nil {
		additionalSeconds = *req.AdditionalSeconds
	} else if req.AdditionalMinutes != nil {
		additionalSeconds = *req.AdditionalMinutes * 60
	} else if req.AdditionalTime != nil {
		additionalSeconds = *req.AdditionalTime
	}

	var newDurationSeconds *int64
	if req.Duration != nil && *req.Duration > 0 {
		sec := *req.Duration
		newDurationSeconds = &sec
	}

	res, err := timer.UpdateTime(c.Request().Context(), additionalSeconds, newDurationSeconds)
	if err != nil {
		logging.Errorf("UpdateTime failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to update round timer", dto.CodeInternal))
	}

	logging.Infof("Round timer updated by admin: additionalSeconds=%d", additionalSeconds)
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round timer updated successfully", res))
}

func (ac *AdminController) StartRound(c echo.Context) error {
	var roundPtr *int32
	if rStr := c.QueryParam("round"); rStr != "" {
		if r, err := strconv.ParseInt(rStr, 10, 32); err == nil && r > 0 {
			r32 := int32(r)
			roundPtr = &r32
		}
	}

	res, err := timer.StartRound(c.Request().Context(), roundPtr)
	if err != nil {
		logging.Errorf("StartRound failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to start round", dto.CodeInternal))
	}

	logging.Infof("Round started by admin")
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round started successfully", res))
}

func (ac *AdminController) ResetRound(c echo.Context) error {
	res, err := timer.ResetRound(c.Request().Context())
	if err != nil {
		logging.Errorf("ResetRound failed: %v", err)
		return c.JSON(http.StatusInternalServerError, dto.NewCodedError("Failed to reset round", dto.CodeInternal))
	}

	logging.Infof("Round reset by admin")
	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round reset successfully", res))
}
