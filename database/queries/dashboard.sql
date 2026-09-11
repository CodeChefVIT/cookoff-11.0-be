-- name: ListDashboardQuestions :many
SELECT q.id, q.title, q.points, q.round,
       COALESCE(a.status, 'available') AS attempt_status
FROM questions q
JOIN users u ON u.id = $1
LEFT JOIN attempts a ON a.question_id = q.id AND a.user_id = u.id
WHERE q.round = u.round_qualified
ORDER BY q.round ASC, q.title ASC, q.id ASC;
