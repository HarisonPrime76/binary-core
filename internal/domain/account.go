package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AccountStatus string

const (
	AccountStatusActive AccountStatus = "active"
	AccountStatusFrozen AccountStatus = "frozen"
	AccountStatusClosed AccountStatus = "closed"
)

type Account struct {
	ID               uuid.UUID       `json:"id"`
	UserID           uuid.UUID       `json:"user_id"`
	IBAN             string          `json:"iban"`
	PostedBalance    decimal.Decimal `json:"posted_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	Currency         string          `json:"currency"`
	Status           AccountStatus   `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// CanDebit vérifie si le compte est actif et dispose des fonds suffisants.
func (a *Account) CanDebit(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if a.Status == AccountStatusFrozen {
		return ErrAccountFrozen
	}
	if a.Status == AccountStatusClosed {
		return ErrAccountClosed
	}
	if a.AvailableBalance.LessThan(amount) {
		return ErrInsufficientFunds
	}
	return nil
}

// CanCredit vérifie si le compte peut recevoir des fonds.
func (a *Account) CanCredit() error {
	if a.Status == AccountStatusClosed {
		return ErrAccountClosed
	}
	return nil
}
