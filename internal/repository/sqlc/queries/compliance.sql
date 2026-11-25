-- name: CreateKYCDocument :one
INSERT INTO kyc_documents (
    id, user_id, document_type, file_path, status
) VALUES (
    $1, $2, $3, $4, 'pending'
) RETURNING *;

-- name: GetKYCDocumentForUpdate :one
SELECT * FROM kyc_documents
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: UpdateKYCDocumentStatus :one
UPDATE kyc_documents
SET
    status = $2,
    rejection_reason = $3,
    reviewed_by = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListPendingKYCDocuments :many
SELECT * FROM kyc_documents
WHERE status = 'pending'
ORDER BY created_at ASC;

-- name: GetRecentOutgoingActivity :one
SELECT
    COUNT(*)::bigint AS tx_count,
    COALESCE(SUM(amount), 0)::numeric(19, 4) AS total_amount
FROM transactions
WHERE source_account_id = $1
  AND created_at >= $2
  AND status IN ('completed', 'awaiting_approval', 'pending');

-- name: CreateAMLAlert :one
INSERT INTO aml_alerts (
    id, user_id, account_id, transaction_id, rule_triggered, severity, details, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, 'open'
) RETURNING *;

-- name: GetAMLAlertForUpdate :one
SELECT * FROM aml_alerts
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: UpdateAMLAlertStatus :one
UPDATE aml_alerts
SET
    status = $2,
    reviewed_by = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListOpenAMLAlerts :many
SELECT * FROM aml_alerts
WHERE status IN ('open', 'investigating')
ORDER BY created_at DESC;
