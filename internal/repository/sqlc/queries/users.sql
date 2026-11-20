-- name: CreateUser :one
INSERT INTO users (
    id, email, password_hash, role, status
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: CreateSession :one
INSERT INTO sessions (
    id, user_id, refresh_token_hash, user_agent, ip_address, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetSessionByTokenHashForUpdate :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1 LIMIT 1
FOR UPDATE;

-- name: RevokeSession :exec
UPDATE sessions
SET is_revoked = TRUE
WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE sessions
SET is_revoked = TRUE
WHERE user_id = $1 AND is_revoked = FALSE;
