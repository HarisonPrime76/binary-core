package transaction

import (
	"encoding/json"
	"net/http"

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

// HTTPHandler contient le service injecté
type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(s *Service) *HTTPHandler {
	return &HTTPHandler{service: s}
}

// HandleTransfer gère la route POST /transfers
func (h *HTTPHandler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	var req HTTPRequest
	w.Header().Set("Content-Type", "application/json")

	// 1. Décodage du corps de la requête JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Format JSON invalide"}`))
		return
	}

	// 2. Appel du service de transaction (Ici naissent txID et err)
	txID, err := h.service.ExecuteTransfer(r.Context(), TransferParams{
		IdempotencyKey: req.IdempotencyKey,
		SourceAccount:  req.SourceAccount,
		DestAccount:    req.DestAccount,
		Amount:         req.Amount,
		Description:    req.Description,
	})

	// 3. Traitement de l'erreur retournée par le Ledger
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 4. Réponse en cas de succès (Utilisation de txID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "success",
		"transaction_id": txID,
		"message":        "Transfert effectué avec succès",
	})
}
