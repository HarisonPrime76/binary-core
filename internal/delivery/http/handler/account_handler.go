package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"binary-core/internal/delivery/http/middleware"
	"binary-core/internal/delivery/http/response"
	"binary-core/internal/domain"

	"github.com/go-chi/chi/v5"
)

type AccountHandler struct {
	accountSvc domain.AccountService
}

func NewAccountHandler(accountSvc domain.AccountService) *AccountHandler {
	return &AccountHandler{accountSvc: accountSvc}
}

type openAccountPayload struct {
	Currency string `json:"currency"`
}

// ============================================================================
// ENDPOINTS PORTAIL CLIENT
// ============================================================================

// OpenAccount gère POST /api/v1/accounts
func (h *AccountHandler) OpenAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	var payload openAccountPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "corps JSON invalide"})
		return
	}

	acc, err := h.accountSvc.OpenAccount(r.Context(), claims.UserID, payload.Currency)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, acc)
}

// ListMyAccounts gère GET /api/v1/accounts
func (h *AccountHandler) ListMyAccounts(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	accounts, err := h.accountSvc.ListCustomerAccounts(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, accounts)
}

// GetMyAccount gère GET /api/v1/accounts/{accountID}
func (h *AccountHandler) GetMyAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthUser(r.Context())
	if !ok {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "non authentifié"})
		return
	}

	accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de compte invalide"})
		return
	}

	acc, err := h.accountSvc.GetAccount(r.Context(), accountID)
	if err != nil {
		response.Error(w, err)
		return
	}

	// Isolation stricte : un client ne peut consulter que ses propres comptes
	if claims.Role == domain.RoleClient && acc.UserID != claims.UserID {
		response.JSON(w, http.StatusForbidden, map[string]string{"error": "accès refusé à ce compte"})
		return
	}

	response.JSON(w, http.StatusOK, acc)
}

// ============================================================================
// ENDPOINTS BACK-OFFICE OPÉRATIONNEL (Tracés automatiquement par AutoAudit)
// ============================================================================

// FreezeAccount gère POST /api/v1/backoffice/accounts/{accountID}/freeze
func (h *AccountHandler) FreezeAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de compte invalide"})
		return
	}

	acc, err := h.accountSvc.FreezeAccount(r.Context(), accountID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, acc)
}

// UnfreezeAccount gère POST /api/v1/backoffice/accounts/{accountID}/unfreeze
func (h *AccountHandler) UnfreezeAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de compte invalide"})
		return
	}

	acc, err := h.accountSvc.UnfreezeAccount(r.Context(), accountID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, acc)
}

// CloseAccount gère POST /api/v1/backoffice/accounts/{accountID}/close
func (h *AccountHandler) CloseAccount(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(chi.URLParam(r, "accountID"))
	if err != nil {
		response.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifiant de compte invalide"})
		return
	}

	acc, err := h.accountSvc.CloseAccount(r.Context(), accountID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.JSON(w, http.StatusOK, acc)
}
