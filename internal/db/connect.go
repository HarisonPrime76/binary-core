package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Config 	regroupe les variables d'environnement indispensables
type Config struct {
	Port        string
	DatabaseURL string
}

// loadConfig extrait les variables d'environnement
func LoadConfig() Config {
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
func InitDatabase(ctx context.Context, dbURL string) *pgxpool.Pool {
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
