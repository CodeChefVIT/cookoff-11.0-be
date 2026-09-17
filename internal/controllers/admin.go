package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/timer"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch users", err.Error()))
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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to ban user", err.Error()))
	}

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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to unban user", err.Error()))
	}

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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to find user", err.Error()))
	}

	var req dto.UpgradeUserRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid request payload", bindErr.Error()))
	}

	updatedUser := existingUser
	hasUpdate := false

	if req.Role != nil && *req.Role != "" {
		updatedUser, err = ac.queries.UpdateUserRole(ctx, sqlc.UpdateUserRoleParams{
			ID:   id,
			Role: *req.Role,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to update user role", err.Error()))
		}
		hasUpdate = true
	}

	targetRound := req.RoundQualified
	if targetRound == nil {
		targetRound = req.Round
	}

	if targetRound != nil && *targetRound > 0 {
		updatedUser, err = ac.queries.UpgradeUserRound(ctx, sqlc.UpgradeUserRoundParams{
			ID:             id,
			RoundQualified: *targetRound,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to upgrade user round", err.Error()))
		}
		hasUpdate = true
	}

	if !hasUpdate {
		updatedUser, err = ac.queries.IncrementUserRound(ctx, id)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to increment user round", err.Error()))
		}
	}

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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch user", userErr.Error()))
	}

	submissions, err := ac.queries.GetUserSubmissions(ctx, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch user submissions", err.Error()))
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

func (ac *AdminController) GetLeaderboard(c echo.Context) error {
	ctx := c.Request().Context()
	data, err := ac.queries.GetLeaderboardData(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch leaderboard data", err.Error()))
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

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Leaderboard fetched successfully", res))
}

func (ac *AdminController) GetAnalytics(c echo.Context) error {
	ctx := c.Request().Context()

	activeUsers, err := ac.queries.GetActiveUsersCount(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch active users count", err.Error()))
	}

	totalUsers, err := ac.queries.GetTotalUsersCount(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch total users count", err.Error()))
	}

	bannedUsers, err := ac.queries.GetBannedUsersCount(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch banned users count", err.Error()))
	}

	subAnalytics, err := ac.queries.GetSubmissionsAnalytics(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to fetch submission analytics", err.Error()))
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
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid request payload", err.Error()))
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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to set round timer", err.Error()))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round timer set successfully", res))
}

func (ac *AdminController) UpdateTime(c echo.Context) error {
	var req dto.UpdateTimeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, dto.NewErrorResponse("Invalid request payload", err.Error()))
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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to update round timer", err.Error()))
	}

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
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to start round", err.Error()))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round started successfully", res))
}

func (ac *AdminController) ResetRound(c echo.Context) error {
	res, err := timer.ResetRound(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("Failed to reset round", err.Error()))
	}

	return c.JSON(http.StatusOK, dto.NewSuccessResponse("Round reset successfully", res))
}
