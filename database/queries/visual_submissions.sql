-- name: ListVisualSolutionsByQuestionID :many
SELECT id,question_id,solution,points
FROM visual_solutions
WHERE question_id = $1;

-- name: UpdateUserScore :exec
UPDATE users
SET score = $2
WHERE id = $1;

-- name: UpdateAttemptStatus :exec
UPDATE attempts
SET
    status = $3,
    answered_at = $4
WHERE user_id = $1
  AND question_id = $2;

-- name: CreateVisualSubmission :one
INSERT INTO submissions(
    id,
    question_id,
    source_code,
    language_id,
    user_id,
    status
)
VALUES(
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetUserScoreForUpdate :one
SELECT score
FROM users
WHERE id = $1
FOR UPDATE;

-- name: GetQuestionReward :one 
SELECT reward
FROM questions
WHERE id = $1;
