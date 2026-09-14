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

-- name: GetSubmissionForUpdate :one
SELECT * FROM submissions
WHERE id = $1
FOR UPDATE;


-- name: GetSubmissionStatusByID :one
SELECT status FROM submissions
WHERE id = $1;

-- name: GetBestScoreForQuestion :one
SELECT (
	COALESCE(
		MAX(
			(
				COALESCE(testcases_passed, 0)::numeric /
				NULLIF(
					(COALESCE(testcases_passed, 0) + COALESCE(testcases_failed, 0))::numeric,
					0::numeric
				)
			) * questions.points::numeric
		),
		0::numeric
	)
)::numeric AS best_score
FROM submissions
JOIN questions ON submissions.question_id = questions.id
WHERE submissions.user_id = $1
  AND submissions.question_id = $2
  AND submissions.id != $3;