package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"binary-core/internal/domain"
	"binary-core/internal/pkg/iban"
	"binary-core/internal/repository/postgres"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jackc/pgx/v5"
)

var supportedCurrencies = []string{"EUR", "USD", "GBP"}

type accountService struct {
	txManager postgres.TxManager
	queries   postgres.Querier
}

func NewAccountService(txManager postgres.TxManager, queries postgres.Querier) domain.AccountService {
	return &accountService{
		txManager: txManager,
		queries:   queries,
	}
}

func (s *accountService) OpenAccount(ctx context.Context, userID uuid.UUID, currency string) (*domain.Account, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if !slices.Contains(supportedCurrencies, currency) {
		return nil, domain.ErrUnsupportedCurrency
	}

	accountID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("génération uuid compte: %w", err)
	}

	// Jusqu'à 5 tentatives en cas de collision d'IBAN aléatoire
	for range 5 {
		generatedIBAN, err := iban.Generate()
		if err != nil {
			return nil, err
		}

		row, err := s.queries.CreateAccount(ctx, postgres.CreateAccountParams{
			ID:       accountID,
			UserID:   userID,
			Iban:     generatedIBAN,
			Currency: currency,
		})
		if err == nil {
			return mapAccountToDomain(row), nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue // Collision IBAN, on retente avec un nouveau tirage
		}

		return nil, fmt.Errorf("ouverture compte: %w", err)
	}

	return nil, errors.New("échec de génération d'un IBAN unique après plusieurs tentatives")
}

func (s *accountService) GetAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error) {
	row, err := s.queries.GetAccountByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}
	return mapAccountToDomain(row), nil
}

func (s *accountService) ListCustomerAccounts(ctx context.Context, userID uuid.UUID) ([]*domain.Account, error) {
	rows, err := s.queries.ListAccountsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	accounts := make([]*domain.Account, 0, len(rows))
	for _, r := range rows {
		accounts = append(accounts, mapAccountToDomain(r))
	}
	return accounts, nil
}

func (s *accountService) FreezeAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error) {
	return s.changeStatus(ctx, accountID, domain.AccountStatusFrozen)
}

func (s *accountService) UnfreezeAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error) {
	return s.changeStatus(ctx, accountID, domain.AccountStatusActive)
}

func (s *accountService) CloseAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error) {
	var closed *domain.Account

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		acc, err := q.GetAccountForUpdate(ctx, accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrAccountNotFound
			}
			return err
		}

		if acc.Status == string(domain.AccountStatusClosed) {
			return domain.ErrAccountClosed
		}

		// Vérification stricte : ni solde comptable ni fonds réservés en attente
		if !acc.PostedBalance.IsZero() || !acc.AvailableBalance.IsZero() {
			return domain.ErrNonZeroAccountBalance
		}

		updated, err := q.UpdateAccountStatus(ctx, postgres.UpdateAccountStatusParams{
			ID:     accountID,
			Status: string(domain.AccountStatusClosed),
		})
		if err != nil {
			return err
		}

		closed = mapAccountToDomain(updated)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return closed, nil
}

func (s *accountService) changeStatus(ctx context.Context, accountID uuid.UUID, targetStatus domain.AccountStatus) (*domain.Account, error) {
	var result *domain.Account

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		acc, err := q.GetAccountForUpdate(ctx, accountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrAccountNotFound
			}
			return err
		}

		if acc.Status == string(domain.AccountStatusClosed) {
			return domain.ErrAccountClosed
		}

		updated, err := q.UpdateAccountStatus(ctx, postgres.UpdateAccountStatusParams{
			ID:     accountID,
			Status: string(targetStatus),
		})
		if err != nil {
			return err
		}

		result = mapAccountToDomain(updated)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
