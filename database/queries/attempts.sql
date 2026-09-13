-- name: GetAttemptForUpdate :one
SELECT *
FROM attempts
WHERE user_id = $1
  AND question_id = $2
FOR UPDATE;


-- name: GetUserBalanceForUpdate :one
SELECT balance
FROM users
WHERE id = $1
FOR UPDATE;


-- name: GetQuestionBuyIn :one
SELECT buy_in
FROM questions
WHERE id = $1;


-- name: UpdateUserBalance :exec
UPDATE users
SET balance = $1
WHERE id = $2;


-- name: CreateAttempt :one
INSERT INTO attempts(
    id,
    user_id,
    question_id,
    status,
    is_buy_in_paid
)
VALUES(
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: EnsureAttempt :exec
INSERT INTO attempts (id, user_id, question_id, status, is_buy_in_paid)
VALUES ($1, $2, $3, 'bought', false)
ON CONFLICT ON CONSTRAINT uq_attempts_user_question DO NOTHING;

-- name: UpdateAttemptToBought :one
UPDATE attempts
SET
    status = 'bought',
    is_buy_in_paid = true
WHERE user_id = $1
  AND question_id = $2
RETURNING *;