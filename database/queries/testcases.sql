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
SELECT * FROM testcases
WHERE question_id = $1;

-- name: GetTestCaseByID :one
SELECT * FROM testcases
WHERE id = $1;

-- name: GetPublicTestCasesByQuestion :many
SELECT * FROM testcases
WHERE question_id = $1 AND hidden = false
ORDER BY id ASC;

-- name: UpdateTestCase :one
UPDATE testcases
SET expected_output = $2, memory = $3, input = $4, hidden = $5,
    runtime = $6, question_id = $7
WHERE id = $1
RETURNING *;

-- name: DeleteTestCase :one
DELETE FROM testcases WHERE id = $1 RETURNING id;