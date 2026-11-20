package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"binary-core/internal/domain"
)

func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func Error(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "erreur interne du serveur"

	switch {
	case errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameAccountTransfer),
		errors.Is(err, domain.ErrCurrencyMismatch),
		errors.Is(err, domain.ErrInvalidTxState),
		errors.Is(err, domain.ErrUnsupportedCurrency):
		status = http.StatusBadRequest
		message = err.Error()

	case errors.Is(err, domain.ErrUnauthorizedAction),
		errors.Is(err, domain.ErrSelfApprovalDenied):
		status = http.StatusForbidden
		message = err.Error()

	case errors.Is(err, domain.ErrAccountNotFound):
		status = http.StatusNotFound
		message = err.Error()

	case errors.Is(err, domain.ErrInsufficientFunds),
		errors.Is(err, domain.ErrAccountFrozen),
		errors.Is(err, domain.ErrAccountClosed),
		errors.Is(err, domain.ErrNonZeroAccountBalance):
		status = http.StatusUnprocessableEntity
		message = err.Error()
	}

	JSON(w, status, map[string]string{"error": message})
}
