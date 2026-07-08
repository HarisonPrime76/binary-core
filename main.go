package main

import (
	"binary-core/internal/transaction"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Config regroupe les variables d'environnement indispensables
type Config struct {
	Port        string
	DatabaseURL string
}

func main() {
	// 1. Charger la configuration depuis le fichier .env
	cfg := loadConfig()

	// Contexte temporaire pour l'initialisation de la base de données (timeout de 10s)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Initialiser le pool de connexions PostgreSQL
	pool := initDatabase(ctx, cfg.DatabaseURL)
	defer pool.Close()

	// 3. ASSEMBLAGE : Injecter le pool de données dans le Service Métier (Ledger)
	txService := transaction.NewService(pool)

	// EXECUTION DU TEST DE VIREMENT LOCAL
	runTransferTest(txService)

	// 4. Configuration du routeur avec le service injecté
	router := setupRouter(txService)

	// 5. Lancement final du serveur HTTP de Binary
	startServer(cfg.Port, router)
}

// runTransferTest simule un virement en appelant directement la logique métier
func runTransferTest(txService *transaction.Service) {
	fmt.Println("\n Démarrage du test de virement...")

	// Utilisation des IDs de test insérés dans PostgreSQL
	sourceID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	destID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	// Génération d'une clé d'idempotence unique à chaque lancement pour éviter les blocages
	uniqueIdempotencyKey := fmt.Sprintf("test_transfer_%d", time.Now().Unix())

	ctx := context.Background()
	txID, err := txService.ExecuteTransfer(ctx, transaction.TransferParams{
		IdempotencyKey: uniqueIdempotencyKey,
		SourceAccount:  sourceID,
		DestAccount:    destID,
		Amount:         150.50, // On tente un virement de 150.50 €
		Description:    "Virement de test technique",
	})

	if err != nil {
		log.Printf(" Échec du virement de test : %v\n\n", err)
		return
	}

	fmt.Printf("Virement réussi ! Transaction ID généré : %s\n\n", txID)
}

// loadConfig extrait les variables d'environnement
func loadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Aucun fichier .env trouvé, utilisation des variables système")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("La variable d'environnement DATABASE_URL est manquante")
	}
	// --- AJOUTE CETTE LIGNE ICI POUR DEBUGGER ---
	fmt.Printf(">> DEBUG: DATABASE_URL lue = %s\n", dbURL)
	// ---------------------------------------------

	return Config{
		Port:        port,
		DatabaseURL: dbURL,
	}
}

// initDatabase connecte et vérifie l'accès à PostgreSQL
func initDatabase(ctx context.Context, dbURL string) *pgxpool.Pool {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Impossible de créer le pool de connexions: %v", err)
	}

	// Ping physique pour valider que la DB locale est active
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("La base de données ne répond pas au Ping: %v", err)
	}

	fmt.Println("Connexion PostgreSQL (pgxpool) établie avec succès.")
	return pool
}

// setupRouter assemble la couche HTTP avec la couche Métier
func setupRouter(txService *transaction.Service) *chi.Mux {
	r := chi.NewRouter()

	// Middlewares de base (Sécurité et logs)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Route de test d'état du système
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "UP", "platform": "Binary E-Money"}`))
	})

	// ASSEMBLAGE : On injecte le service financier dans le Handler HTTP
	txHandler := transaction.NewHTTPHandler(txService)

	// Définition de la route d'API officielle pour les virements
	r.Post("/transfers", txHandler.HandleTransfer)

	return r
}

// startServer démarre l'écoute réseau
func startServer(port string, router *chi.Mux) {
	fmt.Printf("Plateforme Binary démarrée localement sur le port :%s\n", port)
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erreur lors du fonctionnement du serveur: %v", err)
	}
}
