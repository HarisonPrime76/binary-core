package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TransferRequest struct {
	IdempotencyKey       uuid.UUID
	SourceAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	Amount               decimal.Decimal
	Currency             string
	Reference            string
	InitiatedBy          uuid.UUID
}

type TransferService interface {
	ExecuteInternalTransfer(ctx context.Context, req TransferRequest) (*Transaction, error)
	ApproveTransfer(ctx context.Context, txID uuid.UUID, approverID uuid.UUID) (*Transaction, error)
}

type AuditEntryInput struct {
	UserID       uuid.UUID
	Action       string
	TargetEntity string
	TargetID     *uuid.UUID
	Details      []byte // JSON sérialisé
	IPAddress    string
}

type AuditService interface {
	Record(ctx context.Context, input AuditEntryInput) (*AuditLog, error)
	VerifyChainIntegrity(ctx context.Context) (bool, error)
}

// user port
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AuthService interface {
	RegisterClient(ctx context.Context, email, password string) (*User, error)
	Login(ctx context.Context, email, password, userAgent, ipAddress string) (*TokenPair, error)
	RefreshSession(ctx context.Context, rawRefreshToken, userAgent, ipAddress string) (*TokenPair, error)
	Logout(ctx context.Context, rawRefreshToken string) error
}

type AccountService interface {
	OpenAccount(ctx context.Context, userID uuid.UUID, currency string) (*Account, error)
	GetAccount(ctx context.Context, accountID uuid.UUID) (*Account, error)
	ListCustomerAccounts(ctx context.Context, userID uuid.UUID) ([]*Account, error)
	FreezeAccount(ctx context.Context, accountID uuid.UUID) (*Account, error)
	UnfreezeAccount(ctx context.Context, accountID uuid.UUID) (*Account, error)
	CloseAccount(ctx context.Context, accountID uuid.UUID) (*Account, error)
}

type ComplianceService interface {
	SubmitKYCDocument(ctx context.Context, userID uuid.UUID, docType, filePath string) (*KYCDocument, error)
	ReviewKYCDocument(ctx context.Context, docID, reviewerID uuid.UUID, approved bool, rejectionReason string) (*KYCDocument, error)
	ListPendingKYC(ctx context.Context) ([]*KYCDocument, error)

	EvaluateTransactionAML(ctx context.Context, tx *Transaction) ([]*AMLAlert, error)
	ListOpenAMLAlerts(ctx context.Context) ([]*AMLAlert, error)
	ResolveAMLAlert(ctx context.Context, alertID, reviewerID uuid.UUID, decision AMLAlertStatus) (*AMLAlert, error)
}
