package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"binary-core/internal/domain"
)

type contextKey string

const (
	userContextKey        contextKey = "binary_auth_user"
	idempotencyContextKey contextKey = "binary_idempotency_key"
)

type AuthClaims struct {
	UserID uuid.UUID   `json:"sub"`
	Role   domain.Role `json:"role"`
	Exp    int64       `json:"exp"`
}

// GetAuthUser extrait l'utilisateur authentifié depuis le contexte de la requête.
func GetAuthUser(ctx context.Context) (AuthClaims, bool) {
	claims, ok := ctx.Value(userContextKey).(AuthClaims)
	return claims, ok
}

// RequireAuth valide le JWT Bearer Token signé en HS256.
func RequireAuth(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"jeton d'authentification manquant ou invalide"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := verifyHS256JWT(tokenStr, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"session expirée ou jeton invalide"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func verifyHS256JWT(token string, secret []byte) (AuthClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AuthClaims{}, errors.New("format jwt invalide")
	}

	// Vérification de la signature cryptographique en temps constant
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(actualSig, expectedSig) {
		return AuthClaims{}, errors.New("signature jwt invalide")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthClaims{}, errors.New("payload jwt illisible")
	}

	var claims AuthClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return AuthClaims{}, err
	}

	if time.Now().Unix() >= claims.Exp {
		return AuthClaims{}, errors.New("jeton expiré")
	}

	return claims, nil
}
