package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeChefVIT/cookoff-11.0-be/internal/db/sqlc"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/dto"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/helpers/utils"
	"github.com/CodeChefVIT/cookoff-11.0-be/internal/middlewares"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
)

type mockAdminQueries struct {
	users          map[uuid.UUID]sqlc.User
	submissions    map[uuid.UUID][]sqlc.GetUserSubmissionsRow
	leaderboard    []sqlc.GetLeaderboardDataRow
	analytics      sqlc.GetSubmissionsAnalyticsRow
	langDist       []sqlc.GetLanguageDistributionRow
	recentSubCount int32
}

func newMockAdminQueries() *mockAdminQueries {
	return &mockAdminQueries{
		users:       make(map[uuid.UUID]sqlc.User),
		submissions: make(map[uuid.UUID][]sqlc.GetUserSubmissionsRow),
	}
}

func (m *mockAdminQueries) GetAllUsers(ctx context.Context) ([]sqlc.User, error) {
	list := make([]sqlc.User, 0, len(m.users))
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, nil
}

func (m *mockAdminQueries) GetUserByID(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	u, ok := m.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockAdminQueries) BanUser(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	u, ok := m.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.IsBanned = true
	m.users[id] = u
	return u, nil
}

func (m *mockAdminQueries) UnbanUser(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	u, ok := m.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.IsBanned = false
	m.users[id] = u
	return u, nil
}

func (m *mockAdminQueries) UpgradeUserRound(ctx context.Context, arg sqlc.UpgradeUserRoundParams) (sqlc.User, error) {
	u, ok := m.users[arg.ID]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.RoundQualified = arg.RoundQualified
	m.users[arg.ID] = u
	return u, nil
}

func (m *mockAdminQueries) IncrementUserRound(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	u, ok := m.users[id]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.RoundQualified++
	m.users[id] = u
	return u, nil
}

func (m *mockAdminQueries) UpdateUserRole(ctx context.Context, arg sqlc.UpdateUserRoleParams) (sqlc.User, error) {
	u, ok := m.users[arg.ID]
	if !ok {
		return sqlc.User{}, pgx.ErrNoRows
	}
	u.Role = arg.Role
	m.users[arg.ID] = u
	return u, nil
}

func (m *mockAdminQueries) GetUserSubmissions(ctx context.Context, userID uuid.UUID) ([]sqlc.GetUserSubmissionsRow, error) {
	return m.submissions[userID], nil
}

func (m *mockAdminQueries) GetLeaderboardData(ctx context.Context) ([]sqlc.GetLeaderboardDataRow, error) {
	return m.leaderboard, nil
}

func (m *mockAdminQueries) GetActiveUsersCount(ctx context.Context) (int32, error) {
	return int32(len(m.submissions)), nil
}

func (m *mockAdminQueries) GetTotalUsersCount(ctx context.Context) (int32, error) {
	return int32(len(m.users)), nil
}

func (m *mockAdminQueries) GetBannedUsersCount(ctx context.Context) (int32, error) {
	var count int32
	for _, u := range m.users {
		if u.IsBanned {
			count++
		}
	}
	return count, nil
}

func (m *mockAdminQueries) GetSubmissionsAnalytics(ctx context.Context) (sqlc.GetSubmissionsAnalyticsRow, error) {
	return m.analytics, nil
}

func (m *mockAdminQueries) GetLanguageDistribution(ctx context.Context) ([]sqlc.GetLanguageDistributionRow, error) {
	return m.langDist, nil
}

func (m *mockAdminQueries) GetRecentSubmissionsCount(ctx context.Context, submissionTime pgtype.Timestamptz) (int32, error) {
	return m.recentSubCount, nil
}

func TestGetAllUsers(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	u1ID := uuid.New()
	u2ID := uuid.New()
	mockQ.users[u1ID] = sqlc.User{
		ID:             u1ID,
		Name:           "Alice",
		Email:          "alice@example.com",
		RegNo:          "21BCE0001",
		Role:           "participant",
		RoundQualified: 1,
	}
	mockQ.users[u2ID] = sqlc.User{
		ID:             u2ID,
		Name:           "Bob",
		Email:          "bob@example.com",
		RegNo:          "21BCE0002",
		Role:           "participant",
		RoundQualified: 0,
	}

	ac := NewAdminController(mockQ)

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := ac.GetAllUsers(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp dto.SuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success: true")
	}
}

func TestBanAndUnbanUser(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	uID := uuid.New()
	mockQ.users[uID] = sqlc.User{
		ID:       uID,
		Name:     "Charlie",
		Email:    "charlie@example.com",
		IsBanned: false,
	}

	ac := NewAdminController(mockQ)

	// Ban user
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+uID.String()+"/ban", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uID.String())

	if err := ac.BanUser(c); err != nil {
		t.Fatalf("unexpected error on ban: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on ban, got %d", rec.Code)
	}
	if !mockQ.users[uID].IsBanned {
		t.Fatalf("user should be banned in DB")
	}

	// Unban user
	reqUnban := httptest.NewRequest(http.MethodPost, "/admin/users/"+uID.String()+"/unban", nil)
	recUnban := httptest.NewRecorder()
	cUnban := e.NewContext(reqUnban, recUnban)
	cUnban.SetParamNames("id")
	cUnban.SetParamValues(uID.String())

	if err := ac.UnbanUser(cUnban); err != nil {
		t.Fatalf("unexpected error on unban: %v", err)
	}
	if recUnban.Code != http.StatusOK {
		t.Fatalf("expected status 200 on unban, got %d", recUnban.Code)
	}
	if mockQ.users[uID].IsBanned {
		t.Fatalf("user should be unbanned in DB")
	}

	// Non-existent user
	reqNonExist := httptest.NewRequest(http.MethodPost, "/admin/users/"+uuid.New().String()+"/ban", nil)
	recNonExist := httptest.NewRecorder()
	cNonExist := e.NewContext(reqNonExist, recNonExist)
	cNonExist.SetParamNames("id")
	cNonExist.SetParamValues(uuid.New().String())
	_ = ac.BanUser(cNonExist)
	if recNonExist.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent user ban, got %d", recNonExist.Code)
	}

	// Invalid UUID
	reqInvalid := httptest.NewRequest(http.MethodPost, "/admin/users/invalid-uuid/ban", nil)
	recInvalid := httptest.NewRecorder()
	cInvalid := e.NewContext(reqInvalid, recInvalid)
	cInvalid.SetParamNames("id")
	cInvalid.SetParamValues("invalid-uuid")
	_ = ac.BanUser(cInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d", recInvalid.Code)
	}
}

func TestUpgradeUser(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	uID := uuid.New()
	mockQ.users[uID] = sqlc.User{
		ID:             uID,
		Name:           "Dave",
		Role:           "participant",
		RoundQualified: 1,
	}

	ac := NewAdminController(mockQ)

	// Increment round automatically when no payload is sent
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+uID.String()+"/upgrade", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uID.String())

	if err := ac.UpgradeUser(c); err != nil {
		t.Fatalf("unexpected error on upgrade: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mockQ.users[uID].RoundQualified != 2 {
		t.Fatalf("expected round_qualified to increment to 2, got %d", mockQ.users[uID].RoundQualified)
	}

	// Upgrade with target round payload
	payload := `{"round": 4}`
	req2 := httptest.NewRequest(http.MethodPost, "/admin/users/"+uID.String()+"/upgrade", bytes.NewReader([]byte(payload)))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("id")
	c2.SetParamValues(uID.String())

	if err := ac.UpgradeUser(c2); err != nil {
		t.Fatalf("unexpected error on upgrade with round: %v", err)
	}
	if mockQ.users[uID].RoundQualified != 4 {
		t.Fatalf("expected round_qualified to be 4, got %d", mockQ.users[uID].RoundQualified)
	}
}

func TestGetUserSubmissions(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	uID := uuid.New()
	qID := uuid.New()
	mockQ.users[uID] = sqlc.User{ID: uID, Name: "Eve"}

	passed := int32(5)
	failed := int32(0)
	status := "Success"
	desc := "All passed"
	runtimeNum, _ := utils.Float64ToNumeric(0.045)
	memoryNum, _ := utils.Float64ToNumeric(1200)

	mockQ.submissions[uID] = []sqlc.GetUserSubmissionsRow{
		{
			ID:              uuid.New(),
			QuestionID:      qID,
			QuestionTitle:   "Reverse A String",
			QuestionRound:   1,
			TestcasesPassed: &passed,
			TestcasesFailed: &failed,
			Runtime:         runtimeNum,
			Memory:          memoryNum,
			LanguageID:      63,
			Status:          &status,
			Description:     &desc,
			SourceCode:      "print('hello')",
			SubmissionTime:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}

	ac := NewAdminController(mockQ)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+uID.String()+"/submissions", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uID.String())

	if err := ac.GetUserSubmissions(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp dto.SuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success: true")
	}
}

func TestGetLeaderboard(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	u1ID := uuid.New()
	u2ID := uuid.New()
	score1, _ := utils.Float64ToNumeric(100)
	score2, _ := utils.Float64ToNumeric(80)
	rt1, _ := utils.Float64ToNumeric(0.12)
	rt2, _ := utils.Float64ToNumeric(0.18)

	mockQ.leaderboard = []sqlc.GetLeaderboardDataRow{
		{
			ID:                 u1ID,
			Name:               "Rank 1 User",
			Email:              "r1@test.com",
			Score:              score1,
			TotalRuntime:       rt1,
			TotalSubmissions:   2,
			SolvedCount:        2,
			LastSubmissionTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		{
			ID:                 u2ID,
			Name:               "Rank 2 User",
			Email:              "r2@test.com",
			Score:              score2,
			TotalRuntime:       rt2,
			TotalSubmissions:   3,
			SolvedCount:        1,
			LastSubmissionTime: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
	}

	ac := NewAdminController(mockQ)

	req := httptest.NewRequest(http.MethodGet, "/admin/leaderboard", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := ac.GetLeaderboard(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    []dto.LeaderboardEntry `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 leaderboard entries, got %d", len(resp.Data))
	}
	if resp.Data[0].Rank != 1 || resp.Data[1].Rank != 2 {
		t.Fatalf("expected ranks 1 and 2, got %d and %d", resp.Data[0].Rank, resp.Data[1].Rank)
	}
}

func TestGetAnalytics(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()

	uID := uuid.New()
	mockQ.users[uID] = sqlc.User{ID: uID}
	mockQ.submissions[uID] = []sqlc.GetUserSubmissionsRow{{ID: uuid.New()}}

	mockQ.analytics = sqlc.GetSubmissionsAnalyticsRow{
		TotalSubmissions:      10,
		SuccessfulSubmissions: 8,
		FailedSubmissions:     2,
		TotalTestcasesPassed:  80,
		TotalTestcasesFailed:  20,
	}
	mockQ.langDist = []sqlc.GetLanguageDistributionRow{
		{LanguageID: 63, SubmissionCount: 7},
		{LanguageID: 71, SubmissionCount: 3},
	}
	mockQ.recentSubCount = 4

	ac := NewAdminController(mockQ)

	req := httptest.NewRequest(http.MethodGet, "/admin/analytics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := ac.GetAnalytics(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Success bool                  `json:"success"`
		Data    dto.AnalyticsResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.TotalSubmissions != 10 {
		t.Fatalf("expected total submissions 10, got %d", resp.Data.TotalSubmissions)
	}
	if resp.Data.OverallPassRate != 80.0 {
		t.Fatalf("expected pass rate 80%%, got %f", resp.Data.OverallPassRate)
	}
	if resp.Data.LanguageDistribution[63] != 7 {
		t.Fatalf("expected language 63 count 7, got %d", resp.Data.LanguageDistribution[63])
	}
}

func TestRoundTimerControls(t *testing.T) {
	e := echo.New()
	mockQ := newMockAdminQueries()
	ac := NewAdminController(mockQ)

	// SetTime
	setTimePayload := `{"round": 2, "duration": 45}` // 45 minutes
	reqSet := httptest.NewRequest(http.MethodPost, "/admin/setTime", bytes.NewReader([]byte(setTimePayload)))
	reqSet.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recSet := httptest.NewRecorder()
	cSet := e.NewContext(reqSet, recSet)

	if err := ac.SetTime(cSet); err != nil {
		t.Fatalf("unexpected error on SetTime: %v", err)
	}
	if recSet.Code != http.StatusOK {
		t.Fatalf("expected status 200 on SetTime, got %d", recSet.Code)
	}

	var timerResp struct {
		Success bool              `json:"success"`
		Data    dto.TimerResponse `json:"data"`
	}
	_ = json.Unmarshal(recSet.Body.Bytes(), &timerResp)
	if timerResp.Data.Round != 2 || timerResp.Data.Duration != 45*60 {
		t.Fatalf("expected round 2 duration 2700, got round %d duration %d", timerResp.Data.Round, timerResp.Data.Duration)
	}
	if timerResp.Data.IsRunning {
		t.Fatalf("timer should not be running after SetTime")
	}

	// UpdateTime
	updatePayload := `{"additional_time": 10}` // +10 minutes
	reqUp := httptest.NewRequest(http.MethodPost, "/admin/updateTime", bytes.NewReader([]byte(updatePayload)))
	reqUp.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recUp := httptest.NewRecorder()
	cUp := e.NewContext(reqUp, recUp)

	if err := ac.UpdateTime(cUp); err != nil {
		t.Fatalf("unexpected error on UpdateTime: %v", err)
	}
	_ = json.Unmarshal(recUp.Body.Bytes(), &timerResp)
	if timerResp.Data.Duration != 55*60 {
		t.Fatalf("expected duration 3300 after extension, got %d", timerResp.Data.Duration)
	}

	// StartRound
	reqStart := httptest.NewRequest(http.MethodGet, "/admin/startRound?round=2", nil)
	recStart := httptest.NewRecorder()
	cStart := e.NewContext(reqStart, recStart)

	if err := ac.StartRound(cStart); err != nil {
		t.Fatalf("unexpected error on StartRound: %v", err)
	}
	_ = json.Unmarshal(recStart.Body.Bytes(), &timerResp)
	if !timerResp.Data.IsRunning {
		t.Fatalf("expected timer to be running after StartRound")
	}
	if timerResp.Data.StartTime == nil || timerResp.Data.EndTime == nil {
		t.Fatalf("expected StartTime and EndTime to be populated")
	}

	// Participant GetTime
	reqGet := httptest.NewRequest(http.MethodGet, "/getTime", nil)
	recGet := httptest.NewRecorder()
	cGet := e.NewContext(reqGet, recGet)

	if err := GetTime(cGet); err != nil {
		t.Fatalf("unexpected error on GetTime: %v", err)
	}
	var getResp struct {
		Success bool              `json:"success"`
		Data    dto.TimerResponse `json:"data"`
	}
	_ = json.Unmarshal(recGet.Body.Bytes(), &getResp)
	if !getResp.Data.IsRunning || getResp.Data.TimeLeft <= 0 {
		t.Fatalf("expected running timer with positive time left")
	}

	// ResetRound
	reqReset := httptest.NewRequest(http.MethodGet, "/admin/resetRound", nil)
	recReset := httptest.NewRecorder()
	cReset := e.NewContext(reqReset, recReset)

	if err := ac.ResetRound(cReset); err != nil {
		t.Fatalf("unexpected error on ResetRound: %v", err)
	}
	_ = json.Unmarshal(recReset.Body.Bytes(), &timerResp)
	if timerResp.Data.IsRunning {
		t.Fatalf("expected timer to be stopped after ResetRound")
	}
}

func TestAdminSecurityMiddleware(t *testing.T) {
	e := echo.New()

	handler := middlewares.AdminOnly(func(c echo.Context) error {
		return c.String(http.StatusOK, "admin granted")
	})

	// Non-admin user
	reqUser := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	recUser := httptest.NewRecorder()
	cUser := e.NewContext(reqUser, recUser)
	cUser.Set(middlewares.RoleKey, "participant")

	err := handler(cUser)
	if err == nil {
		t.Fatalf("expected error for non-admin user")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-admin, got %v", err)
	}

	// Admin user
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	recAdmin := httptest.NewRecorder()
	cAdmin := e.NewContext(reqAdmin, recAdmin)
	cAdmin.Set(middlewares.RoleKey, "admin")

	err = handler(cAdmin)
	if err != nil {
		t.Fatalf("unexpected error for admin user: %v", err)
	}
	if recAdmin.Code != http.StatusOK {
		t.Fatalf("expected status 200 for admin user, got %d", recAdmin.Code)
	}
}
