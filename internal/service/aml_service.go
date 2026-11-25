package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgtype"

	"binary-core/internal/domain"
	"binary-core/internal/repository/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var validKYCDocTypes = []string{"passport", "id_card", "proof_of_address"}

type amlService struct {
	txManager          postgres.TxManager
	queries            postgres.Querier
	reportingThreshold decimal.Decimal // ex: 10000.0000
}

func NewComplianceService(
	txManager postgres.TxManager,
	queries postgres.Querier,
	reportingThreshold decimal.Decimal,
) domain.ComplianceService {
	return &amlService{
		txManager:          txManager,
		queries:            queries,
		reportingThreshold: reportingThreshold,
	}
}

func (s *amlService) SubmitKYCDocument(ctx context.Context, userID uuid.UUID, docType, filePath string) (*domain.KYCDocument, error) {
	if !slices.Contains(validKYCDocTypes, docType) {
		return nil, errors.New("type de document KYC invalide (passport, id_card, proof_of_address)")
	}
	if filePath == "" {
		return nil, errors.New("chemin du fichier justificatif obligatoire")
	}

	docID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateKYCDocument(ctx, postgres.CreateKYCDocumentParams{
		ID:           docID,
		UserID:       userID,
		DocumentType: docType,
		FilePath:     filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("soumission document KYC: %w", err)
	}

	return mapKYCToDomain(row), nil
}

func (s *amlService) ReviewKYCDocument(ctx context.Context, docID, reviewerID uuid.UUID, approved bool, rejectionReason string) (*domain.KYCDocument, error) {
	if !approved && rejectionReason == "" {
		return nil, errors.New("un motif de rejet est obligatoire")
	}

	var updatedDoc *domain.KYCDocument

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		doc, err := q.GetKYCDocumentForUpdate(ctx, docID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("document KYC introuvable")
			}
			return err
		}

		if doc.Status != string(domain.KYCStatusPending) {
			return errors.New("ce dossier KYC a déjà été traité")
		}

		newStatus := domain.KYCStatusRejected
		var reasonText pgtype.Text
		if approved {
			newStatus = domain.KYCStatusApproved
		} else {
			reasonText = pgtype.Text{String: rejectionReason, Valid: true}
		}

		row, err := q.UpdateKYCDocumentStatus(ctx, postgres.UpdateKYCDocumentStatusParams{
			ID:              docID,
			Status:          string(newStatus),
			RejectionReason: reasonText,
			ReviewedBy:      &reviewerID,
		})
		if err != nil {
			return err
		}

		updatedDoc = mapKYCToDomain(row)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return updatedDoc, nil
}

func (s *amlService) ListPendingKYC(ctx context.Context) ([]*domain.KYCDocument, error) {
	rows, err := s.queries.ListPendingKYCDocuments(ctx)
	if err != nil {
		return nil, err
	}

	docs := make([]*domain.KYCDocument, 0, len(rows))
	for _, r := range rows {
		docs = append(docs, mapKYCToDomain(r))
	}
	return docs, nil
}

// EvaluateTransactionAML analyse une transaction pour détecter le fractionnement (structuring) et la vélocité anormale.
func (s *amlService) EvaluateTransactionAML(ctx context.Context, tx *domain.Transaction) ([]*domain.AMLAlert, error) {
	if tx.SourceAccountID == nil {
		return nil, nil
	}

	since := time.Now().UTC().Add(-24 * time.Hour)
	activity, err := s.queries.GetRecentOutgoingActivity(ctx, postgres.GetRecentOutgoingActivityParams{
		SourceAccountID: tx.SourceAccountID,
		CreatedAt:       pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("analyse activité 24h AML: %w", err)
	}

	var alerts []*domain.AMLAlert

	// Règle 1 : Détection de fractionnement (Structuring / Smurfing)
	// La transaction unitaire est sous le seuil de déclaration, mais le cumul 24h dépasse le seuil sur >= 2 opérations
	if tx.Amount.LessThan(s.reportingThreshold) &&
		activity.TxCount >= 2 &&
		activity.TotalAmount.GreaterThanOrEqual(s.reportingThreshold) {

		details, _ := json.Marshal(map[string]any{
			"rule":                 "fractionnement_24h",
			"tx_count_24h":         activity.TxCount,
			"cumulative_total_24h": activity.TotalAmount.String(),
			"threshold":            s.reportingThreshold.String(),
			"current_tx_amount":    tx.Amount.String(),
		})

		alert, err := s.createAlert(ctx, tx.CreatedBy, *tx.SourceAccountID, &tx.ID, "STRUCTURING_24H", "high", details)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}

	// Règle 2 : Vélocité élevée (plus de 5 virements sortants sur 24h)
	if activity.TxCount > 5 {
		details, _ := json.Marshal(map[string]any{
			"rule":         "velocite_anormale_24h",
			"tx_count_24h": activity.TxCount,
		})

		alert, err := s.createAlert(ctx, tx.CreatedBy, *tx.SourceAccountID, &tx.ID, "RAPID_VELOCITY_24H", "medium", details)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func (s *amlService) ListOpenAMLAlerts(ctx context.Context) ([]*domain.AMLAlert, error) {
	rows, err := s.queries.ListOpenAMLAlerts(ctx)
	if err != nil {
		return nil, err
	}

	alerts := make([]*domain.AMLAlert, 0, len(rows))
	for _, r := range rows {
		alerts = append(alerts, mapAMLAlertToDomain(r))
	}
	return alerts, nil
}

// ResolveAMLAlert permet au Chargé de Conformité de classer l'alerte et gèle le compte si fraude confirmée.
func (s *amlService) ResolveAMLAlert(ctx context.Context, alertID, reviewerID uuid.UUID, decision domain.AMLAlertStatus) (*domain.AMLAlert, error) {
	if decision != domain.AMLStatusDismissed && decision != domain.AMLStatusConfirmedFraud {
		return nil, errors.New("décision AML invalide (dismissed ou confirmed_fraud attendu)")
	}

	var resolved *domain.AMLAlert

	err := s.txManager.ExecTx(ctx, func(q postgres.Querier) error {
		alert, err := q.GetAMLAlertForUpdate(ctx, alertID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("alerte AML introuvable")
			}
			return err
		}

		updated, err := q.UpdateAMLAlertStatus(ctx, postgres.UpdateAMLAlertStatusParams{
			ID:         alertID,
			Status:     string(decision),
			ReviewedBy: &reviewerID,
		})
		if err != nil {
			return err
		}

		// Mesure conservatoire immédiate en cas de fraude confirmée
		if decision == domain.AMLStatusConfirmedFraud {
			if _, err := q.UpdateAccountStatus(ctx, postgres.UpdateAccountStatusParams{
				ID:     alert.AccountID,
				Status: string(domain.AccountStatusFrozen),
			}); err != nil {
				return fmt.Errorf("gel automatique du compte suite fraude AML: %w", err)
			}
		}

		resolved = mapAMLAlertToDomain(updated)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func (s *amlService) createAlert(
	ctx context.Context,
	userID, accountID uuid.UUID,
	txID *uuid.UUID,
	rule, severity string,
	details []byte,
) (*domain.AMLAlert, error) {
	alertID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	row, err := s.queries.CreateAMLAlert(ctx, postgres.CreateAMLAlertParams{
		ID:            alertID,
		UserID:        userID,
		AccountID:     accountID,
		TransactionID: txID,
		RuleTriggered: rule,
		Severity:      severity,
		Details:       details,
	})
	if err != nil {
		return nil, fmt.Errorf("création alerte AML: %w", err)
	}

	return mapAMLAlertToDomain(row), nil
}

func mapKYCToDomain(row postgres.KycDocument) *domain.KYCDocument {
	var reason *string
	if row.RejectionReason.Valid {
		reason = &row.RejectionReason.String
	}
	return &domain.KYCDocument{
		ID:              row.ID,
		UserID:          row.UserID,
		DocumentType:    row.DocumentType,
		FilePath:        row.FilePath,
		Status:          domain.KYCStatus(row.Status),
		RejectionReason: reason,
		ReviewedBy:      row.ReviewedBy,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

func mapAMLAlertToDomain(row postgres.AmlAlert) *domain.AMLAlert {
	return &domain.AMLAlert{
		ID:            row.ID,
		UserID:        row.UserID,
		AccountID:     row.AccountID,
		TransactionID: row.TransactionID,
		RuleTriggered: row.RuleTriggered,
		Severity:      row.Severity,
		Details:       row.Details,
		Status:        domain.AMLAlertStatus(row.Status),
		ReviewedBy:    row.ReviewedBy,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}
