package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"binary-core/internal/domain"
	"binary-core/internal/repository/postgres"

	"github.com/jackc/pgx/v5"
)

// GenesisHash représente l'empreinte initiale de 64 zéros pour le tout premier log du système.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

type auditService struct {
	txManager postgres.TxManager
	queries   postgres.Querier
}

func NewAuditService(txManager postgres.TxManager, queries postgres.Querier) domain.AuditService {
	return &auditService{
		txManager: txManager,
		queries:   queries,
	}
}

// Record enregistre une action sensible et la lie cryptographiquement au log précédent.
func (s *auditService) Record(ctx context.Context, input domain.AuditEntryInput) (*domain.AuditLog, error) {
	ipAddr, err := netip.ParseAddr(input.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid ip address %q: %w", input.IPAddress, err)
	}

	details := input.Details
	if len(details) == 0 {
		details = []byte("{}")
	}

	var createdLog *domain.AuditLog

	err = s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		// 1. Récupérer et verrouiller le dernier log pour garantir une chaîne linéaire stricte
		prevHash := GenesisHash
		lastLog, err := q.GetLastAuditLogForUpdate(ctx)
		if err == nil {
			prevHash = lastLog.Hash
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("fetch last audit log: %w", err)
		}

		// 2. Générer un UUIDv7 triable chronologiquement
		logID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate audit uuid: %w", err)
		}

		// 3. Calculer l'empreinte SHA-256 déterministe
		currentHash := computeAuditHash(
			prevHash,
			logID,
			input.UserID,
			input.Action,
			input.TargetEntity,
			input.TargetID,
			details,
			ipAddr.String(),
		)

		// 4. Insérer dans la table append-only audit_logs
		row, err := q.CreateAuditLog(ctx, postgres.CreateAuditLogParams{
			ID:           logID,
			UserID:       input.UserID,
			Action:       input.Action,
			TargetEntity: input.TargetEntity,
			TargetID:     input.TargetID,
			Details:      details,
			IpAddress:    ipAddr,
			PreviousHash: prevHash,
			Hash:         currentHash,
		})
		if err != nil {
			return fmt.Errorf("insert audit log: %w", err)
		}

		createdLog = mapAuditLogToDomain(row)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdLog, nil
}

// VerifyChainIntegrity recalcule toute la chaîne SHA-256 pour détecter toute falsification en base.
func (s *auditService) VerifyChainIntegrity(ctx context.Context) (bool, error) {
	logs, err := s.queries.ListAuditLogsChronological(ctx)
	if err != nil {
		return false, fmt.Errorf("list audit logs: %w", err)
	}

	expectedPrevHash := GenesisHash

	for _, entry := range logs {
		if entry.PreviousHash != expectedPrevHash {
			return false, fmt.Errorf("rupture de chaîne détectée sur le log %s", entry.ID)
		}

		recomputed := computeAuditHash(
			entry.PreviousHash,
			entry.ID,
			entry.UserID,
			entry.Action,
			entry.TargetEntity,
			entry.TargetID,
			entry.Details,
			entry.IpAddress.String(),
		)

		if recomputed != entry.Hash {
			return false, fmt.Errorf("altération des données détectée sur le log %s", entry.ID)
		}

		expectedPrevHash = entry.Hash
	}

	return true, nil
}

// computeAuditHash produit une empreinte SHA-256 canonique sur 64 caractères hexadécimaux.
func computeAuditHash(
	prevHash string,
	id uuid.UUID,
	userID uuid.UUID,
	action string,
	targetEntity string,
	targetID *uuid.UUID,
	details []byte,
	ip string,
) string {
	targetIDStr := "null"
	if targetID != nil {
		targetIDStr = targetID.String()
	}

	// Format canonique avec séparateurs stricts pour éviter les collisions de concaténation
	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s|%s",
		prevHash,
		id.String(),
		userID.String(),
		action,
		targetEntity,
		targetIDStr,
		string(details),
		ip,
	)

	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func mapAuditLogToDomain(row postgres.AuditLog) *domain.AuditLog {
	var targetID uuid.UUID
	if row.TargetID != nil {
		targetID = *row.TargetID
	}

	return &domain.AuditLog{
		ID:           row.ID,
		UserID:       row.UserID,
		Action:       row.Action,
		TargetEntity: row.TargetEntity,
		TargetID:     targetID,
		Details:      row.Details,
		IPAddress:    row.IpAddress.String(),
		PreviousHash: row.PreviousHash,
		Hash:         row.Hash,
		CreatedAt:    row.CreatedAt.Time.UTC().Truncate(time.Microsecond),
	}
}
