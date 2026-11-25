package handler

import (
	"encoding/json"
	"net/http"

	"binary-core/internal/delivery/http/middleware"
	"binary-core/internal/delivery/http/response"
	"binary-core/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ComplianceHandler struct {
	complianceSvc domain.ComplianceService
}

func NewComplianceHandler(complianceSvc domain.ComplianceService) *ComplianceHandler {
	return &ComplianceHandler{complianceSvc: complianceSvc}
}

type submitKYCPayload struct {
	DocumentType string `json:"document_type"` // passport, id_card, proof_of_address
	FilePath     string `json:"file_path"`
}

type reviewKYCPayload struct {
	Approved        bool   `json:"approved"`
	RejectionReason string `json:"rejection_reason"`
}

type resolveAMLPayload struct {
	Decision string `json:"decision"` // dismissed ou confirmed_fraud
}

// ============================================================================
// ENDPOINT PORTAIL CLIENT
// ============================================================================

// SubmitKYC gère POST /api/v1/kyc
func (h *ComplianceHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	var payload submitKYCPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "corps JSON invalide"})
		return
	}

	doc, err := h.complianceSvc.SubmitKYCDocument(r.Context(), claims.UserID, payload.DocumentType, payload.FilePath)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	response.JSON(w, http.StatusCreated, doc)
}

// ============================================================================
// ENDPOINTS BACK-OFFICE (Chargé de Conformité & Admin — Tracés par AutoAudit)
// ============================================================================

// ListPendingKYC gère GET /api/v1/backoffice/kyc/pending
func (h *ComplianceHandler) ListPendingKYC(w http.ResponseWriter, r *http.Request) {
	docs, err := h.complianceSvc.ListPendingKYC(r.Context())
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, docs)
}

// ReviewKYC gère POST /api/v1/backoffice/kyc/{docID}/review
func (h *ComplianceHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	docID, err := uuid.Parse(chi.URLParam(r, "docID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de document invalide"})
		return
	}

	var payload reviewKYCPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "corps JSON invalide"})
		return
	}

	doc, err := h.complianceSvc.ReviewKYCDocument(r.Context(), docID, claims.UserID, payload.Approved, payload.RejectionReason)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	response.JSON(w, http.StatusOK, doc)
}

// ListOpenAlerts gère GET /api/v1/backoffice/aml/alerts
func (h *ComplianceHandler) ListOpenAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.complianceSvc.ListOpenAMLAlerts(r.Context())
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, alerts)
}

// ResolveAlert gère POST /api/v1/backoffice/aml/alerts/{alertID}/resolve
func (h *ComplianceHandler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	alertID, err := uuid.Parse(chi.URLParam(r, "alertID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant d'alerte invalide"})
		return
	}

	var payload resolveAMLPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "corps JSON invalide"})
		return
	}

	alert, err := h.complianceSvc.ResolveAMLAlert(r.Context(), alertID, claims.UserID, domain.AMLAlertStatus(payload.Decision))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	response.JSON(w, http.StatusOK, alert)
}
