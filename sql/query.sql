-- name: GetAccountForUpdate :one
-- Verrouille la ligne du compte pour empêcher les modifications concurrentes (Anti-Race Condition)
SELECT * FROM accounts
WHERE id = $1 FOR UPDATE;

-- name: CreateTransaction :one
INSERT INTO transactions (idempotency_key, status, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateTransactionStatus :exec
UPDATE transactions
SET status = $2
WHERE id = $1;

-- name: CreatePosting :exec
INSERT INTO postings (transaction_id, account_id, amount)
VALUES ($1, $2, $3);

-- name: GetBalance :one
-- Calcule le solde exact en temps réel basé sur le grand livre
SELECT COALESCE(SUM(amount), 0)::numeric AS balance
FROM postings
WHERE account_id = $1;