-- name: CreateSubmissionResult :one
INSERT INTO submission_results (
    id, testcase_id, submission_id, runtime, memory, points_awarded, status, description
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetSubmissionResults :many
SELECT * FROM submission_results
WHERE submission_id = $1;

-- name: UpdateSubmissionStatus :exec
UPDATE submissions
SET testcases_passed = $2,
    testcases_failed = $3,
    runtime = $4,
    memory = $5,
    status = $6,
    description = $7
WHERE id = $1;