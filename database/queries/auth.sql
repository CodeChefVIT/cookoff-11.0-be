-- name: GetUserByGoogleID :one
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned FROM users WHERE google_id = $1;

-- name: GetUserByID :one
SELECT id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned FROM users WHERE id = $1;

-- name: CreateGoogleUser :one
INSERT INTO users (id, email, reg_no, role, google_id, name) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, reg_no, role, round_qualified, google_id, balance, score, name, is_banned;
