package auth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Définition d'un type de clé privé pour le contexte afin d'éviter les collisions de clés
type contextKey string

const UserIDKey contextKey = "user_id"

// Claims représente la structure des données embarquées dans notre JWT
type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// GetUserIDFromContext extrait de manière sûre l'UUID de l'utilisateur authentifié
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	val := ctx.Value(UserIDKey)
	if val == nil {
		return uuid.Nil, errors.New("aucun utilisateur trouvé dans le contexte")
	}
	userIDStr, ok := val.(string)
	if !ok {
		return uuid.Nil, errors.New("type d'ID utilisateur invalide")
	}
	return uuid.Parse(userIDStr)
}

// JWTMiddleware valide le jeton d'accès présent dans le header Authorization
func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error": "Jeton d'authentification manquant"}`, http.StatusUnauthorized)
			return
		}

		// Extraction du format "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"error": "Format d'autorisation invalide (Bearer requis)"}`, http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		jwtSecret := []byte(os.Getenv("JWT_SECRET"))

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			// Validation de l'algorithme de signature (HMAC)
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("méthode de signature inattendue")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error": "Jeton JWT invalide ou expiré"}`, http.StatusUnauthorized)
			return
		}

		// On injecte le user_id validé dans le contexte de la requête HTTP
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
