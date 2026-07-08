package transaction

import (
	"context"
	"errors"
	"fmt"

	"binary-core/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype" // On importe pgtype pour les conversions express
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type TransferParams struct {
	IdempotencyKey string
	SourceAccount  uuid.UUID
	DestAccount    uuid.UUID
	Amount         float64
	Description    string
}

func (s *Service) ExecuteTransfer(ctx context.Context, p TransferParams) (uuid.UUID, error) {
	if p.Amount <= 0 {
		return uuid.Nil, errors.New("montant invalide")
	}
	if p.SourceAccount == p.DestAccount {
		return uuid.Nil, errors.New("comptes identiques")
	}

	// Initialisation express des types pgtype en une seule ligne
	var descText pgtype.Text
	_ = descText.Scan(p.Description)

	var amountNum, negAmountNum pgtype.Numeric
	_ = amountNum.Scan(fmt.Sprintf("%.4f", p.Amount))
	_ = negAmountNum.Scan(fmt.Sprintf("%.4f", -p.Amount))

	// Isolation maximale pour le Ledger
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)
	// 1. Idempotence
	txRecord, err := qtx.CreateTransaction(ctx, db.CreateTransactionParams{
		IdempotencyKey: p.IdempotencyKey,
		Status:         "pending",
		Description:    descText,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("concurrence/idempotence détectée: %w", err)
	}

	// 2. Lock déterministe anti-deadlock
	first, second := p.SourceAccount, p.DestAccount
	if p.SourceAccount.String() > p.DestAccount.String() {
		first, second = p.DestAccount, p.SourceAccount
	}
	if _, err = qtx.GetAccountForUpdate(ctx, first); err == nil {
		_, _ = qtx.GetAccountForUpdate(ctx, second)
	}

	// 3. Validation du solde basé sur le Grand Livre
	balanceNumeric, err := qtx.GetBalance(ctx, p.SourceAccount)
	var currentBalance float64
	_ = balanceNumeric.Scan(&currentBalance)

	if err != nil || currentBalance < p.Amount {
		_ = qtx.UpdateTransactionStatus(ctx, db.UpdateTransactionStatusParams{ID: txRecord.ID, Status: "failed"})
		_ = tx.Commit(ctx)
		return uuid.Nil, errors.New("solde insuffisant")
	}

	// 4. Écritures comptables en partie double (Double-entry)
	if err = qtx.CreatePosting(ctx, db.CreatePostingParams{TransactionID: txRecord.ID, AccountID: p.SourceAccount, Amount: negAmountNum}); err != nil {
		return uuid.Nil, err
	}
	if err = qtx.CreatePosting(ctx, db.CreatePostingParams{TransactionID: txRecord.ID, AccountID: p.DestAccount, Amount: amountNum}); err != nil {
		return uuid.Nil, err
	}

	// 5. Clôture atomique de la transaction
	_ = qtx.UpdateTransactionStatus(ctx, db.UpdateTransactionStatusParams{ID: txRecord.ID, Status: "completed"})

	return txRecord.ID, tx.Commit(ctx)
}
