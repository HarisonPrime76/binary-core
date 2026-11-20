-- name: CreateAccount :one
INSERT INTO accounts (
    id, user_id, iban, posted_balance, available_balance, currency, status
) VALUES (
    $1, $2, $3, 0.0000, 0.0000, $4, 'active'
) RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts
WHERE id = $1 LIMIT 1;

-- name: GetAccountByIBAN :one
SELECT * FROM accounts
WHERE iban = $1 LIMIT 1;

-- name: ListAccountsByUserID :many
SELECT * FROM accounts
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetAccountForUpdate :one
SELECT * FROM accounts
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: UpdateAccountBalances :one
UPDATE accounts
SET
    posted_balance = posted_balance + sqlc.arg(posted_delta),
    available_balance = available_balance + sqlc.arg(available_delta),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateAccountStatus :one
UPDATE accounts
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
