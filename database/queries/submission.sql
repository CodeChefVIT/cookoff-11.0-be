-- name: CreateSubmission :exec
INSERT INTO submissions (
	id,
	question_id,
	runtime,
	source_code,
	language_id,
	description,
	memory,
	user_id,
	status
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: GetSubmissionByID :one
SELECT * FROM submissions
WHERE id = $1;


-- name: GetSubmissionStatusByID :one
SELECT status FROM submissions
WHERE id = $1;