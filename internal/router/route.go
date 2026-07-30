package router

import (
	"binary-core/internal/handlers"
	"binary-core/internal/middlewares/auth"
	"binary-core/internal/transaction"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRouter(pool *pgxpool.Pool, txService *transaction.Service) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	authHandler := handlers.NewAuthHandler(pool)
	txHandler := handlers.NewHTTPHandler(txService)

	// Route publique (Pas besoin de JWT)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "UP"}`))
	})

	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	// Routes protégées par JWT
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware) // <-- Applique le middleware uniquement à ce groupe

		// Cette route requiert désormais impérativement un JWT valide !
		r.Post("/transfers", txHandler.HandleTransfer)
	})

	return r
}
