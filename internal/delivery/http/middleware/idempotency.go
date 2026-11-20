package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// GetIdempotencyKey récupère l'UUID d'idempotence validé depuis le contexte.
func GetIdempotencyKey(ctx context.Context) (uuid.UUID, bool) {
	key, ok := ctx.Value(idempotencyContextKey).(uuid.UUID)
	return key, ok
}

// RequireIdempotencyKey exige un en-tête HTTP Idempotency-Key au format UUID valide.
func RequireIdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawKey := r.Header.Get("Idempotency-Key")
		if rawKey == "" {
			http.Error(w, `{"error":"en-tête Idempotency-Key obligatoire"}`, http.StatusBadRequest)
			return
		}

		parsedKey, err := uuid.Parse(rawKey)
		if err != nil {
			http.Error(w, `{"error":"Idempotency-Key doit être un UUID valide"}`, http.StatusBadRequest)
			return
		}

		ctx := context.WithValue(r.Context(), idempotencyContextKey, parsedKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
