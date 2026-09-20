-- name: CreateSubmissionResult :one
INSERT INTO submission_results (
    id, testcase_id, submission_id, runtime, memory, points_awarded, status, description, stdout
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetSubmissionResults :many
SELECT submission_results.*, COALESCE(testcases.hidden, true) AS hidden
FROM submission_results
LEFT JOIN testcases ON testcases.id = submission_results.testcase_id
WHERE submission_results.submission_id = $1;

-- name: UpdateSubmissionStatus :exec
UPDATE submissions
SET testcases_passed = $2,
    testcases_failed = $3,
    runtime = $4,
    memory = $5,
    status = $6,
    description = $7
WHERE id = $1;
