
-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions
WHERE idempotency_key = $1 LIMIT 1;

-- name: GetTransactionForUpdate :one
SELECT * FROM transactions
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: CreateTransaction :one
INSERT INTO transactions (
    id, idempotency_key, source_account_id, destination_account_id,
    amount, currency, status, reference, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: UpdateTransactionStatus :one
UPDATE transactions
SET
    status = $2,
    approved_by = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateLedgerEntry :one
INSERT INTO ledger_entries (
    id, transaction_id, account_id, direction, amount, balance_after
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;
