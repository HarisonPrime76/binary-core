-- name: GetLastAuditLogForUpdate :one
SELECT * FROM audit_logs
ORDER BY created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    id, user_id, action, target_entity, target_id,
    details, ip_address, previous_hash, hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: ListAuditLogsChronological :many
SELECT * FROM audit_logs
ORDER BY created_at ASC, id ASC;
