package http

import (
	"log/slog"
	"net/http"

	"binary-core/internal/delivery/http/handler"
	"binary-core/internal/delivery/http/middleware"
	"binary-core/internal/delivery/http/response"
	"binary-core/internal/domain"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"
)

type RouterConfig struct {
	JWTSecret         []byte
	Logger            *slog.Logger
	AuditService      domain.AuditService
	AuthHandler       *handler.AuthHandler
	AccountHandler    *handler.AccountHandler
	TransferHandler   *handler.TransferHandler
	ComplianceHandler *handler.ComplianceHandler
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1:5173")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, Idempotency-Key")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// 1. Routes Publiques d'Authentification
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", cfg.AuthHandler.Register)
			r.Post("/login", cfg.AuthHandler.Login)
			r.Post("/refresh", cfg.AuthHandler.Refresh)
			r.Post("/logout", cfg.AuthHandler.Logout)
		})

		// 2. Routes Protégées par JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(cfg.JWTSecret))

			// Espace Client (Comptes, Virements & Dépôt KYC)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles(domain.RoleClient, domain.RoleAdmin))

				r.Post("/accounts", cfg.AccountHandler.OpenAccount)
				r.Get("/accounts", cfg.AccountHandler.ListMyAccounts)
				r.Get("/accounts/{accountID}", cfg.AccountHandler.GetMyAccount)

				r.Post("/kyc", cfg.ComplianceHandler.SubmitKYC)

				r.With(middleware.RequireIdempotencyKey).Post("/transfers", cfg.TransferHandler.CreateTransfer)
			})

			// Espace Back-Office Opérationnel (Piste d'Audit SHA-256 automatique)
			r.Route("/backoffice", func(r chi.Router) {
				r.Use(middleware.RequireRoles(
					domain.RoleAdmin,
					domain.RoleCompliance,
					domain.RoleSupport,
					domain.RoleValidator,
				))
				r.Use(middleware.AutoAudit(cfg.AuditService, cfg.Logger))

				// Conformité KYC/AML & Gel réglementaire (Chargé de Conformité + Admin)
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRoles(domain.RoleCompliance, domain.RoleAdmin))

					// Gestion des comptes
					r.Post("/accounts/{accountID}/freeze", cfg.AccountHandler.FreezeAccount)
					r.Post("/accounts/{accountID}/unfreeze", cfg.AccountHandler.UnfreezeAccount)
					r.Post("/accounts/{accountID}/close", cfg.AccountHandler.CloseAccount)

					// Revue des dossiers KYC
					r.Get("/kyc/pending", cfg.ComplianceHandler.ListPendingKYC)
					r.Post("/kyc/{docID}/review", cfg.ComplianceHandler.ReviewKYC)

					// Traitement des alertes anti-blanchiment (AML)
					r.Get("/aml/alerts", cfg.ComplianceHandler.ListOpenAlerts)
					r.Post("/aml/alerts/{alertID}/resolve", cfg.ComplianceHandler.ResolveAlert)
				})

				// Double validation financière Maker-Checker (Validateur + Admin)
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRoles(domain.RoleValidator, domain.RoleAdmin))
					r.With(middleware.RequireIdempotencyKey).Post("/approvals/{txID}/approve", cfg.TransferHandler.ApproveTransfer)
				})

				// Intégrité de la chaîne d'audit (Admin + Conformité)
				r.Group(func(r chi.Router) {
					r.Use(middleware.RequireRoles(domain.RoleAdmin, domain.RoleCompliance))
					r.Get("/audit/verify", func(w http.ResponseWriter, r *http.Request) {
						valid, err := cfg.AuditService.VerifyChainIntegrity(r.Context())
						if err != nil {
							response.JSON(w, http.StatusConflict, map[string]any{
								"valid": false,
								"error": err.Error(),
							})
							return
						}
						response.JSON(w, http.StatusOK, map[string]any{"valid": valid})
					})
				})
			})
		})
	})

	return r
}
