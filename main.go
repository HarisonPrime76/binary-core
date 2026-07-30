package main

import (
	"binary-core/internal/db"
	"binary-core/internal/router"
	"binary-core/internal/transaction"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	//"github.com/go-chi/chi/v5/middleware"
	//"github.com/google/uuid"
)

func main() {
	// 1. Charger la configuration depuis le fichier .env
	cfg := db.LoadConfig()

	// Contexte temporaire pour l'initialisation de la base de données (timeout de 10s)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. Initialiser le pool de connexions PostgreSQL
	pool := db.InitDatabase(ctx, cfg.DatabaseURL)
	defer pool.Close()

	// 3. ASSEMBLAGE : Injecter le pool de données dans le Service Métier (Ledger)
	txService := transaction.NewService(pool)

	// EXECUTION DU TEST DE VIREMENT LOCAL
	//runTransferTest(txService)

	// 4. Configuration du routeur avec le service injecté
	r := router.SetupRouter(pool, txService)

	// 5. Lancement final du serveur HTTP de Binary
	startServer(cfg.Port, r)
}

// runTransferTest simule un virement en appelant directement la logique métier
// func runTransferTest(txService *transaction.Service) {
// 	fmt.Println("\n Démarrage du test de virement...")

// 	// Utilisation des IDs de test insérés dans PostgreSQL
// 	sourceID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
// 	destID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

// 	// Génération d'une clé d'idempotence unique à chaque lancement pour éviter les blocages
// 	uniqueIdempotencyKey := fmt.Sprintf("test_transfer_%d", time.Now().Unix())

// 	ctx := context.Background()
// 	txID, err := txService.ExecuteTransfer(ctx, transaction.TransferParams{
// 		IdempotencyKey: uniqueIdempotencyKey,
// 		SourceAccount:  sourceID,
// 		DestAccount:    destID,
// 		Amount:         150.50, // On tente un virement de 150.50 €
// 		Description:    "Virement de test technique",
// 	})

// 	if err != nil {
// 		log.Printf(" Échec du virement de test : %v\n\n", err)
// 		return
// 	}

// 	fmt.Printf("Virement réussi ! Transaction ID généré : %s\n\n", txID)
// }

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
