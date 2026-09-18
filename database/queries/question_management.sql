-- name: CreateQuestion :one
INSERT INTO questions (
    id, description, title, q_type, input_format, buy_in, reward, points,
    round, constraints, output_format, sample_test_input, sample_test_output,
    explanation, bounty_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING *;

-- name: UpdateQuestion :one
UPDATE questions
SET description = $2, title = $3, q_type = $4, input_format = $5,
    buy_in = $6, reward = $7, points = $8, round = $9, constraints = $10,
    output_format = $11, sample_test_input = $12, sample_test_output = $13,
    explanation = $14, bounty_active = $15
WHERE id = $1
RETURNING *;

-- name: DeleteQuestion :one
DELETE FROM questions WHERE id = $1 RETURNING id;

-- name: ListAllQuestions :many
SELECT id, description, title, q_type, input_format,
       COALESCE(buy_in::text, ''::text) AS buy_in,
       COALESCE(reward::text, ''::text) AS reward, points, round,
       constraints, output_format, sample_test_input, sample_test_output,
       explanation, bounty_active
FROM questions
ORDER BY title ASC, id ASC;

-- name: SetQuestionBountyActive :one
UPDATE questions SET bounty_active = $2 WHERE id = $1 RETURNING *;

-- name: GetQuestionForUser :one
SELECT q.id, q.description, q.title, q.q_type, q.input_format,
       COALESCE(q.buy_in::text, ''::text) AS buy_in,
       COALESCE(q.reward::text, ''::text) AS reward, q.points, q.round,
       q.constraints, q.output_format, q.sample_test_input, q.sample_test_output,
       q.explanation, q.bounty_active
FROM questions q
JOIN users u ON u.id = $2
WHERE q.id = $1 AND q.round = u.round_qualified;

-- name: ListQuestionsForUser :many
SELECT q.id, q.description, q.title, q.q_type, q.input_format,
       COALESCE(q.buy_in::text, ''::text) AS buy_in,
       COALESCE(q.reward::text, ''::text) AS reward, q.points, q.round,
       q.constraints, q.output_format, q.sample_test_input, q.sample_test_output,
       q.explanation, q.bounty_active
FROM questions q
JOIN users u ON u.id = $1
WHERE q.round = u.round_qualified
ORDER BY q.title ASC, q.id ASC;
