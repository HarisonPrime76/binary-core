package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleClient     Role = "client"
	RoleAdmin      Role = "admin"
	RoleCompliance Role = "compliance"
	RoleSupport    Role = "support"
	RoleValidator  Role = "validator"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusBlocked   UserStatus = "blocked"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Jamais exposé en JSON
	Role         Role       `json:"role"`
	Status       UserStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type AuditLog struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Action       string    `json:"action"`
	TargetEntity string    `json:"target_entity"`
	TargetID     uuid.UUID `json:"target_id"`
	Details      []byte    `json:"details"` // JSON brut
	IPAddress    string    `json:"ip_address"`
	PreviousHash string    `json:"previous_hash"`
	Hash         string    `json:"hash"`
	CreatedAt    time.Time `json:"created_at"`
}
