-- name: CreateTestCase :one
INSERT INTO testcases (
    id,
    expected_output,
    memory,
    input,
    hidden,
    runtime,
    question_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetAllTestCasesByQuestion :many
SELECT
    id,
    memory,
    expected_output,
    input,
    hidden,
    runtime,
    question_id
FROM testcases
WHERE question_id = $1;