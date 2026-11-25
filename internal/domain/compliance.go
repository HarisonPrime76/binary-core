package domain

import (
	"time"

	"github.com/google/uuid"
)

type KYCStatus string

const (
	KYCStatusPending  KYCStatus = "pending"
	KYCStatusApproved KYCStatus = "approved"
	KYCStatusRejected KYCStatus = "rejected"
)

type KYCDocument struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	DocumentType    string     `json:"document_type"`
	FilePath        string     `json:"file_path"`
	Status          KYCStatus  `json:"status"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	ReviewedBy      *uuid.UUID `json:"reviewed_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AMLAlertStatus string

const (
	AMLStatusOpen           AMLAlertStatus = "open"
	AMLStatusInvestigating  AMLAlertStatus = "investigating"
	AMLStatusDismissed      AMLAlertStatus = "dismissed"
	AMLStatusConfirmedFraud AMLAlertStatus = "confirmed_fraud"
)

type AMLAlert struct {
	ID            uuid.UUID      `json:"id"`
	UserID        uuid.UUID      `json:"user_id"`
	AccountID     uuid.UUID      `json:"account_id"`
	TransactionID *uuid.UUID     `json:"transaction_id,omitempty"`
	RuleTriggered string         `json:"rule_triggered"`
	Severity      string         `json:"severity"`
	Details       []byte         `json:"details"`
	Status        AMLAlertStatus `json:"status"`
	ReviewedBy    *uuid.UUID     `json:"reviewed_by,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
