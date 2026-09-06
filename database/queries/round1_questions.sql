-- name: ListQuestionsByRound :many
SELECT
    id,
    description,
    title,
    q_type,
    input_format,
    COALESCE(buy_in::text, ''::text) AS buy_in,
    COALESCE(reward::text, ''::text) AS reward,
    points,
    round,
    constraints,
    output_format,
    sample_test_input,
    sample_test_output,
    explanation
FROM questions
WHERE round = $1
ORDER BY title ASC, id ASC;

-- name: GetQuestionByID :one
SELECT
    id,
    description,
    title,
    q_type,
    input_format,
    COALESCE(buy_in::text, ''::text) AS buy_in,
    COALESCE(reward::text, ''::text) AS reward,
    points,
    round,
    constraints,
    output_format,
    sample_test_input,
    sample_test_output,
    explanation
FROM questions
WHERE id = $1;

-- name: GetRoundOneVisualQuestion :one
SELECT id
FROM questions
WHERE id = $1
  AND round = 1
  AND LOWER(q_type) = 'visual';

-- name: ListVisualBlocksByQuestionID :many
SELECT id, question_id, content
FROM visual_blocks
WHERE question_id = $1
ORDER BY id ASC;
