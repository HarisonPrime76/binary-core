package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransactionStatus string

const (
	TxStatusPending          TransactionStatus = "pending"
	TxStatusAwaitingApproval TransactionStatus = "awaiting_approval"
	TxStatusCompleted        TransactionStatus = "completed"
	TxStatusFailed           TransactionStatus = "failed"
	TxStatusCancelled        TransactionStatus = "cancelled"
)

type LedgerDirection string

const (
	DirectionDebit  LedgerDirection = "DEBIT"
	DirectionCredit LedgerDirection = "CREDIT"
)

type Transaction struct {
	ID                   uuid.UUID         `json:"id"`
	IdempotencyKey       uuid.UUID         `json:"idempotency_key"`
	SourceAccountID      *uuid.UUID        `json:"source_account_id,omitempty"`
	DestinationAccountID *uuid.UUID        `json:"destination_account_id,omitempty"`
	Amount               decimal.Decimal   `json:"amount"`
	Currency             string            `json:"currency"`
	Status               TransactionStatus `json:"status"`
	Reference            string            `json:"reference"`
	CreatedBy            uuid.UUID         `json:"created_by"`
	ApprovedBy           *uuid.UUID        `json:"approved_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type LedgerEntry struct {
	ID            uuid.UUID       `json:"id"`
	TransactionID uuid.UUID       `json:"transaction_id"`
	AccountID     uuid.UUID       `json:"account_id"`
	Direction     LedgerDirection `json:"direction"`
	Amount        decimal.Decimal `json:"amount"`
	BalanceAfter  decimal.Decimal `json:"balance_after"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CanApprove applique le principe bancaire des 4 yeux (Maker-Checker).
func (t *Transaction) CanApprove(approverID uuid.UUID) error {
	if t.Status != TxStatusAwaitingApproval {
		return ErrInvalidTxState
	}
	if t.CreatedBy == approverID {
		return ErrSelfApprovalDenied
	}
	return nil
}
