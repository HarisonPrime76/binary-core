package handler

import (
	"binary-core/internal/delivery/http/middleware"
	"binary-core/internal/delivery/http/response"
	"binary-core/internal/domain"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/go-chi/chi/v5"
)

type TransferHandler struct {
	transferSvc domain.TransferService
}

func NewTransferHandler(transferSvc domain.TransferService) *TransferHandler {
	return &TransferHandler{transferSvc: transferSvc}
}

type createTransferPayload struct {
	SourceAccountID      uuid.UUID `json:"source_account_id"`
	DestinationAccountID uuid.UUID `json:"destination_account_id"`
	Amount               string    `json:"amount"` // Reçu en string pour préserver la précision exacte
	Currency             string    `json:"currency"`
	Reference            string    `json:"reference"`
}

// CreateTransfer gère POST /api/v1/transfers
func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	idempotencyKey, ok := middleware.GetIdempotencyKey(r.Context())
	if !ok {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "clé d'idempotence manquante"})
		return
	}

	var payload createTransferPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "corps JSON invalide"})
		return
	}

	amount, err := decimal.NewFromString(payload.Amount)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "format de montant invalide"})
		return
	}

	tx, err := h.transferSvc.ExecuteInternalTransfer(r.Context(), domain.TransferRequest{
		IdempotencyKey:       idempotencyKey,
		SourceAccountID:      payload.SourceAccountID,
		DestinationAccountID: payload.DestinationAccountID,
		Amount:               amount,
		Currency:             payload.Currency,
		Reference:            payload.Reference,
		InitiatedBy:          claims.UserID,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	status := http.StatusCreated
	if tx.Status == domain.TxStatusAwaitingApproval {
		status = http.StatusAccepted // 202 : Fonds réservés, en attente du Validateur Financier
	}

	response.JSON(w, status, tx)
}

// ApproveTransfer gère POST /api/v1/backoffice/approvals/{txID}/approve (Maker-Checker)
func (h *TransferHandler) ApproveTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	rawTxID := chi.URLParam(r, "txID")
	txID, err := uuid.Parse(rawTxID)
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de transaction invalide"})
		return
	}

	tx, err := h.transferSvc.ApproveTransfer(r.Context(), txID, claims.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, tx)
}
