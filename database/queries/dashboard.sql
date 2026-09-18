-- name: GetDashboardRoundStats :many
SELECT 
    q.round,
    COUNT(CASE WHEN a.status = 'answered' THEN 1 END)::int AS questions_completed,
    COUNT(CASE WHEN a.status = 'bought' OR (q.round = 1 AND (a.status IS NULL OR a.status = 'available')) THEN 1 END)::int AS questions_incomplete,
    COALESCE(SUM(CASE WHEN a.status = 'answered' THEN q.points ELSE 0 END), 0)::int AS round_score
FROM questions q
JOIN users u ON u.id = $1
LEFT JOIN attempts a ON a.question_id = q.id AND a.user_id = u.id
GROUP BY q.round
ORDER BY q.round ASC;

-- name: GetDashboardQuestions :many
SELECT 
    q.id,
    q.title,
    q.points,
    q.round,
    COALESCE(a.status, 'available')::text AS attempt_status
FROM questions q
JOIN users u ON u.id = $1
LEFT JOIN attempts a ON a.question_id = q.id AND a.user_id = u.id
WHERE q.round = u.round_qualified
ORDER BY q.title ASC, q.id ASC;
