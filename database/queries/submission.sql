-- name: CreateSubmission :exec
INSERT INTO submissions (
	id,
	question_id,
	testcases_passed,
	testcases_failed,
	runtime,
	submission_time,
	source_code,
	language_id,
	description,
	memory,
	user_id,
	status
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
);