-- name: GetUserByGoogleID :one
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned FROM users WHERE google_id = $1;

-- name: GetUserByID :one
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned FROM users WHERE email = $1;

-- name: LinkGoogleID :one
UPDATE users SET google_id = $2 WHERE id = $1 AND google_id IS NULL
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;

-- name: CreateUserFromGoogle :one
INSERT INTO users (id, email, reg_no, role, name, google_id, balance)
VALUES ($1, $2, $3, 'user', $4, $5, $6)
ON CONFLICT DO NOTHING
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;
