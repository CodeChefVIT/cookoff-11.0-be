-- name: GetAllUsers :many
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned
FROM users
ORDER BY name ASC, id ASC;

-- name: BanUser :one
UPDATE users
SET is_banned = true
WHERE id = $1
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: UnbanUser :one
UPDATE users
SET is_banned = false
WHERE id = $1
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: UpgradeUserRound :one
UPDATE users
SET round_qualified = $2
WHERE id = $1
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: IncrementUserRound :one
UPDATE users
SET round_qualified = round_qualified + 1
WHERE id = $1
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: UpdateUserRole :one
UPDATE users
SET role = $2
WHERE id = $1
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: GetUserSubmissions :many
SELECT s.id, s.question_id, s.testcases_passed, s.testcases_failed, s.runtime, s.submission_time, s.source_code, s.language_id, s.description, s.memory, s.user_id, s.status,
       q.title AS question_title, q.round AS question_round
FROM submissions s
JOIN questions q ON q.id = s.question_id
WHERE s.user_id = $1
ORDER BY s.submission_time DESC, s.id DESC;

-- name: GetLeaderboardData :many
SELECT 
    u.id, 
    u.name, 
    u.email, 
    u.reg_no, 
    u.score,
    u.round_qualified,
    u.is_banned,
    COALESCE(SUM(s.runtime), 0)::DECIMAL AS total_runtime,
    MAX(s.submission_time)::TIMESTAMPTZ AS last_submission_time,
    COUNT(s.id)::INT AS total_submissions,
    COUNT(DISTINCT CASE WHEN LOWER(s.status) = 'success' THEN s.question_id END)::INT AS solved_count
FROM users u
LEFT JOIN submissions s ON s.user_id = u.id AND LOWER(s.status) = 'success'
GROUP BY u.id, u.name, u.email, u.reg_no, u.score, u.round_qualified, u.is_banned
ORDER BY u.score DESC, total_runtime ASC, last_submission_time ASC NULLS LAST, u.name ASC;

-- name: GetActiveUsersCount :one
SELECT COUNT(DISTINCT user_id)::INT
FROM submissions;

-- name: GetTotalUsersCount :one
SELECT COUNT(*)::INT
FROM users;

-- name: GetBannedUsersCount :one
SELECT COUNT(*)::INT
FROM users
WHERE is_banned = true;

-- name: GetSubmissionsAnalytics :one
SELECT 
    COUNT(*)::INT AS total_submissions,
    COUNT(CASE WHEN LOWER(status) = 'success' THEN 1 END)::INT AS successful_submissions,
    COUNT(CASE WHEN LOWER(status) != 'success' OR status IS NULL THEN 1 END)::INT AS failed_submissions,
    COALESCE(SUM(testcases_passed), 0)::BIGINT AS total_testcases_passed,
    COALESCE(SUM(testcases_failed), 0)::BIGINT AS total_testcases_failed
FROM submissions;

-- name: GetLanguageDistribution :many
SELECT language_id, COUNT(*)::INT AS submission_count
FROM submissions
GROUP BY language_id
ORDER BY submission_count DESC;

-- name: GetRecentSubmissionsCount :one
SELECT COUNT(*)::INT
FROM submissions
WHERE submission_time >= $1;
