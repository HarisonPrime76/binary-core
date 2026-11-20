package middleware

import (
	"net/http"
	"slices"

	"binary-core/internal/domain"
)

// RequireRoles vérifie que l'utilisateur authentifié possède l'un des rôles autorisés.
func RequireRoles(allowedRoles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetAuthUser(r.Context())
			if !ok {
				http.Error(w, `{"error":"utilisateur non authentifié"}`, http.StatusUnauthorized)
				return
			}

			if !slices.Contains(allowedRoles, claims.Role) {
				http.Error(w, `{"error":"privilèges insuffisants pour cette opération"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
