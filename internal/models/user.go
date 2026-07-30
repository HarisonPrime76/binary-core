package models

import (
	"github.com/google/uuid"
)

// HTTPRequest définit la structure du JSON attendu depuis l'application mobile ou le web
type HTTPRequest struct {
	IdempotencyKey string    `json:"idempotency_key"`
	SourceAccount  uuid.UUID `json:"source_account"`
	DestAccount    uuid.UUID `json:"dest_account"`
	Amount         float64   `json:"amount"`
	Description    string    `json:"description"`
}

// Structures des requêtes JSON
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login request structure
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
