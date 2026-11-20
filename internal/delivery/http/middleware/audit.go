package middleware

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"binary-core/internal/domain"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// AutoAudit enregistre automatiquement toute mutation réussie effectuée dans le Back-Office.
func AutoAudit(auditSvc domain.AuditService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// On n'audite que les opérations de mutation (pas les simples lectures GET/HEAD)
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			rec := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			// Enregistrement dans la chaîne SHA-256 si l'action a abouti (200-299)
			if rec.statusCode >= 200 && rec.statusCode < 300 {
				claims, ok := GetAuthUser(r.Context())
				if !ok {
					return
				}

				clientIP := extractClientIP(r)
				details, _ := json.Marshal(map[string]any{
					"method":      r.Method,
					"path":        r.URL.Path,
					"status_code": rec.statusCode,
					"user_agent":  r.UserAgent(),
				})

				_, err := auditSvc.Record(r.Context(), domain.AuditEntryInput{
					UserID:       claims.UserID,
					Action:       r.Method + " " + r.URL.Path,
					TargetEntity: "http_endpoint",
					Details:      details,
					IPAddress:    clientIP,
				})
				if err != nil {
					logger.Error("échec critique d'enregistrement piste d'audit",
						slog.String("user_id", claims.UserID.String()),
						slog.String("path", r.URL.Path),
						slog.Any("error", err),
					)
				}
			}
		})
	}
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
