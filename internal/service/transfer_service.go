package service

import (
	"context"
	"errors"
	"fmt"

	"binary-core/internal/domain"
	"binary-core/internal/repository/postgres"

	"github.com/shopspring/decimal"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

type transferService struct {
	txManager         postgres.TxManager
	queries           postgres.Querier
	approvalThreshold decimal.Decimal
}

func NewTransferService(
	txManager postgres.TxManager,
	queries postgres.Querier,
	approvalThreshold decimal.Decimal,
) domain.TransferService {
	return &transferService{
		txManager:         txManager,
		queries:           queries,
		approvalThreshold: approvalThreshold,
	}
}

func (s *transferService) ExecuteInternalTransfer(ctx context.Context, req domain.TransferRequest) (*domain.Transaction, error) {
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}
	if req.SourceAccountID == req.DestinationAccountID {
		return nil, domain.ErrSameAccountTransfer
	}

	// 1. Vérification d'idempotence rapide avant d'ouvrir une transaction
	existingTx, err := s.queries.GetTransactionByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		return mapTxToDomain(existingTx), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}

	var resultTx *domain.Transaction

	// 2. Exécution dans une transaction PostgreSQL ACID
	err = s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		// Verrouillage ordonné des comptes pour éviter tout deadlock
		srcAcc, dstAcc, err := lockAccountsOrdered(ctx, q, req.SourceAccountID, req.DestinationAccountID)
		if err != nil {
			return err
		}

		// Vérifications métier via le domaine
		domainSrc := mapAccountToDomain(srcAcc)
		if err := domainSrc.CanDebit(req.Amount); err != nil {
			return err
		}

		domainDst := mapAccountToDomain(dstAcc)
		if err := domainDst.CanCredit(); err != nil {
			return err
		}

		if srcAcc.Currency != req.Currency || dstAcc.Currency != req.Currency {
			return domain.ErrCurrencyMismatch
		}

		// Détermination du statut (Exécution immédiate ou Double Validation Maker-Checker)
		needsApproval := req.Amount.GreaterThanOrEqual(s.approvalThreshold)
		txStatus := domain.TxStatusCompleted
		if needsApproval {
			txStatus = domain.TxStatusAwaitingApproval
		}

		txID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate tx uuid: %w", err)
		}

		createdTx, err := q.CreateTransaction(ctx, postgres.CreateTransactionParams{
			ID:                   txID,
			IdempotencyKey:       req.IdempotencyKey,
			SourceAccountID:      &req.SourceAccountID,
			DestinationAccountID: &req.DestinationAccountID,
			Amount:               req.Amount,
			Currency:             req.Currency,
			Status:               string(txStatus),
			Reference:            req.Reference,
			CreatedBy:            req.InitiatedBy,
		})
		if err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		if needsApproval {
			// Réservation des fonds uniquement sur le solde disponible
			_, err = q.UpdateAccountBalances(ctx, postgres.UpdateAccountBalancesParams{
				ID:             srcAcc.ID,
				PostedDelta:    decimal.Zero,
				AvailableDelta: req.Amount.Neg(),
			})
			if err != nil {
				return fmt.Errorf("reserve available balance: %w", err)
			}
		} else {
			// Règlement immédiat : mise à jour des deux soldes et écriture en partie double
			if err := settleDoubleEntry(ctx, q, createdTx, srcAcc.ID, dstAcc.ID, req.Amount); err != nil {
				return err
			}
		}

		resultTx = mapTxToDomain(createdTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return resultTx, nil
}

func (s *transferService) ApproveTransfer(ctx context.Context, txID uuid.UUID, approverID uuid.UUID) (*domain.Transaction, error) {
	var resultTx *domain.Transaction

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		txRow, err := q.GetTransactionForUpdate(ctx, txID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidTxState
			}
			return fmt.Errorf("lock transaction: %w", err)
		}

		domainTx := mapTxToDomain(txRow)
		if err := domainTx.CanApprove(approverID); err != nil {
			return err
		}

		if txRow.SourceAccountID == nil || txRow.DestinationAccountID == nil {
			return domain.ErrAccountNotFound
		}

		srcAcc, dstAcc, err := lockAccountsOrdered(ctx, q, *txRow.SourceAccountID, *txRow.DestinationAccountID)
		if err != nil {
			return err
		}

		// Le available_balance du compte source a déjà été réservé lors de l'initiation.
		// On débite maintenant son posted_balance, et on crédite les deux soldes du bénéficiaire.
		updatedSrc, err := q.UpdateAccountBalances(ctx, postgres.UpdateAccountBalancesParams{
			ID:             srcAcc.ID,
			PostedDelta:    txRow.Amount.Neg(),
			AvailableDelta: decimal.Zero,
		})
		if err != nil {
			return fmt.Errorf("debit source posted balance: %w", err)
		}

		updatedDst, err := q.UpdateAccountBalances(ctx, postgres.UpdateAccountBalancesParams{
			ID:             dstAcc.ID,
			PostedDelta:    txRow.Amount,
			AvailableDelta: txRow.Amount,
		})
		if err != nil {
			return fmt.Errorf("credit destination balances: %w", err)
		}

		if err := insertLedgerPair(ctx, q, txRow.ID, updatedSrc, updatedDst, txRow.Amount); err != nil {
			return err
		}

		approvedTx, err := q.UpdateTransactionStatus(ctx, postgres.UpdateTransactionStatusParams{
			ID:         txRow.ID,
			Status:     string(domain.TxStatusCompleted),
			ApprovedBy: &approverID,
		})
		if err != nil {
			return fmt.Errorf("update transaction status: %w", err)
		}

		resultTx = mapTxToDomain(approvedTx)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return resultTx, nil
}

// lockAccountsOrdered verrouille toujours les comptes par ordre croissant d'UUID pour éviter les deadlocks.
func lockAccountsOrdered(ctx context.Context, q postgres.Querier, srcID, dstID uuid.UUID) (postgres.Account, postgres.Account, error) {
	firstID, secondID := srcID, dstID
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}

	firstAcc, err := q.GetAccountForUpdate(ctx, firstID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return postgres.Account{}, postgres.Account{}, domain.ErrAccountNotFound
		}
		return postgres.Account{}, postgres.Account{}, err
	}

	secondAcc, err := q.GetAccountForUpdate(ctx, secondID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return postgres.Account{}, postgres.Account{}, domain.ErrAccountNotFound
		}
		return postgres.Account{}, postgres.Account{}, err
	}

	if firstID == srcID {
		return firstAcc, secondAcc, nil
	}
	return secondAcc, firstAcc, nil
}

func settleDoubleEntry(ctx context.Context, q postgres.Querier, tx postgres.Transaction, srcID, dstID uuid.UUID, amount decimal.Decimal) error {
	updatedSrc, err := q.UpdateAccountBalances(ctx, postgres.UpdateAccountBalancesParams{
		ID:             srcID,
		PostedDelta:    amount.Neg(),
		AvailableDelta: amount.Neg(),
	})
	if err != nil {
		return fmt.Errorf("debit source account: %w", err)
	}

	updatedDst, err := q.UpdateAccountBalances(ctx, postgres.UpdateAccountBalancesParams{
		ID:             dstID,
		PostedDelta:    amount,
		AvailableDelta: amount,
	})
	if err != nil {
		return fmt.Errorf("credit destination account: %w", err)
	}

	return insertLedgerPair(ctx, q, tx.ID, updatedSrc, updatedDst, amount)
}

func insertLedgerPair(ctx context.Context, q postgres.Querier, txID uuid.UUID, src, dst postgres.Account, amount decimal.Decimal) error {
	debitID, _ := uuid.NewV7()
	if _, err := q.CreateLedgerEntry(ctx, postgres.CreateLedgerEntryParams{
		ID:            debitID,
		TransactionID: txID,
		AccountID:     src.ID,
		Direction:     string(domain.DirectionDebit),
		Amount:        amount,
		BalanceAfter:  src.PostedBalance,
	}); err != nil {
		return fmt.Errorf("insert debit ledger entry: %w", err)
	}

	creditID, _ := uuid.NewV7()
	if _, err := q.CreateLedgerEntry(ctx, postgres.CreateLedgerEntryParams{
		ID:            creditID,
		TransactionID: txID,
		AccountID:     dst.ID,
		Direction:     string(domain.DirectionCredit),
		Amount:        amount,
		BalanceAfter:  dst.PostedBalance,
	}); err != nil {
		return fmt.Errorf("insert credit ledger entry: %w", err)
	}

	return nil
}

func mapAccountToDomain(a postgres.Account) *domain.Account {
	return &domain.Account{
		ID:               a.ID,
		UserID:           a.UserID,
		IBAN:             a.Iban,
		PostedBalance:    a.PostedBalance,
		AvailableBalance: a.AvailableBalance,
		Currency:         a.Currency,
		Status:           domain.AccountStatus(a.Status),
		CreatedAt:        a.CreatedAt.Time,
		UpdatedAt:        a.UpdatedAt.Time,
	}
}

func mapTxToDomain(t postgres.Transaction) *domain.Transaction {
	return &domain.Transaction{
		ID:                   t.ID,
		IdempotencyKey:       t.IdempotencyKey,
		SourceAccountID:      t.SourceAccountID,
		DestinationAccountID: t.DestinationAccountID,
		Amount:               t.Amount,
		Currency:             t.Currency,
		Status:               domain.TransactionStatus(t.Status),
		Reference:            t.Reference,
		CreatedBy:            t.CreatedBy,
		ApprovedBy:           t.ApprovedBy,
		CreatedAt:            t.CreatedAt.Time,
		UpdatedAt:            t.UpdatedAt.Time,
	}
}
