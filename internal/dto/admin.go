package dto

type UserResponse struct {
	ID             string  `json:"id"`
	Email          string  `json:"email"`
	RegNo          string  `json:"reg_no"`
	Role           string  `json:"role"`
	RoundQualified int32   `json:"round_qualified"`
	Balance        float64 `json:"balance"`
	Score          float64 `json:"score"`
	Name           string  `json:"name"`
	IsBanned       bool    `json:"is_banned"`
}

type UpgradeUserRequest struct {
	Round          *int32  `json:"round"`
	RoundQualified *int32  `json:"round_qualified"`
	Role           *string `json:"role"`
}

type UserSubmissionResponse struct {
	ID              string  `json:"id"`
	QuestionID      string  `json:"question_id"`
	QuestionTitle   string  `json:"question_title"`
	QuestionRound   int32   `json:"question_round"`
	TestcasesPassed int32   `json:"testcases_passed"`
	TestcasesFailed int32   `json:"testcases_failed"`
	Runtime         float64 `json:"runtime"`
	Memory          float64 `json:"memory"`
	LanguageID      int32   `json:"language_id"`
	Status          string  `json:"status"`
	Description     string  `json:"description"`
	SourceCode      string  `json:"source_code"`
	SubmissionTime  string  `json:"submission_time"`
}

type LeaderboardEntry struct {
	Rank               int     `json:"rank"`
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	RegNo              string  `json:"reg_no"`
	Score              float64 `json:"score"`
	RoundQualified     int32   `json:"round_qualified"`
	TotalRuntime       float64 `json:"total_runtime"`
	LastSubmissionTime *string `json:"last_submission_time"`
	TotalSubmissions   int32   `json:"total_submissions"`
	SolvedCount        int32   `json:"solved_count"`
	IsBanned           bool    `json:"is_banned"`
}

type AnalyticsResponse struct {
	ActiveUsers            int32           `json:"active_users"`
	TotalUsers             int32           `json:"total_users"`
	BannedUsers            int32           `json:"banned_users"`
	TotalSubmissions       int32           `json:"total_submissions"`
	SuccessfulSubmissions  int32           `json:"successful_submissions"`
	FailedSubmissions      int32           `json:"failed_submissions"`
	OverallPassRate        float64         `json:"overall_pass_rate"`
	RecentSubmissionsCount int32           `json:"recent_submissions_count"`
	SubmissionRatePerMin   float64         `json:"submission_rate_per_min"`
	TotalTestcasesPassed   int64           `json:"total_testcases_passed"`
	TotalTestcasesFailed   int64           `json:"total_testcases_failed"`
	TestcasePassRate       float64         `json:"testcase_pass_rate"`
	LanguageDistribution   map[int32]int32 `json:"language_distribution"`
}

type SetTimeRequest struct {
	Round           *int32 `json:"round"`
	Duration        *int64 `json:"duration"`
	DurationMinutes *int64 `json:"duration_minutes"`
	DurationSeconds *int64 `json:"duration_seconds"`
}

type UpdateTimeRequest struct {
	Duration          *int64 `json:"duration"`
	AdditionalTime    *int64 `json:"additional_time"`
	AdditionalMinutes *int64 `json:"additional_minutes"`
	AdditionalSeconds *int64 `json:"additional_seconds"`
}

type TimerResponse struct {
	Round     int32   `json:"round"`
	IsRunning bool    `json:"is_running"`
	Duration  int64   `json:"duration"`
	StartTime *string `json:"start_time"`
	EndTime   *string `json:"end_time"`
	TimeLeft  int64   `json:"time_left"`
}
