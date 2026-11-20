package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	JWTSecret         []byte
	ApprovalThreshold decimal.Decimal
	ShutdownTimeout   time.Duration
}

// Load lit les variables d'environnement et valide les contraintes de sécurité au démarrage.
func Load() (*Config, error) {
	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if key, val, ok := strings.Cut(line, "="); ok && os.Getenv(strings.TrimSpace(key)) == "" {
				_ = os.Setenv(strings.TrimSpace(key), strings.Trim(strings.TrimSpace(val), `"'`))
			}
		}
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("variable DATABASE_URL obligatoire")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return nil, errors.New("variable JWT_SECRET obligatoire (32 caractères minimum)")
	}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}

	thresholdStr := os.Getenv("APPROVAL_THRESHOLD")
	if thresholdStr == "" {
		thresholdStr = "10000.0000" // Seuil Maker-Checker par défaut : 10 000
	}

	threshold, err := decimal.NewFromString(thresholdStr)
	if err != nil || threshold.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("APPROVAL_THRESHOLD invalide (%q): %w", thresholdStr, err)
	}

	return &Config{
		HTTPAddr:          httpAddr,
		DatabaseURL:       dbURL,
		JWTSecret:         []byte(jwtSecret),
		ApprovalThreshold: threshold,
		ShutdownTimeout:   15 * time.Second,
	}, nil
}
