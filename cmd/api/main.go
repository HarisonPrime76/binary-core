package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"binary-core/internal/config"
	deliveryhttp "binary-core/internal/delivery/http"
	"binary-core/internal/delivery/http/handler"
	"binary-core/internal/repository/postgres"
	"binary-core/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 1. Initialisation du logger structuré JSON officiel (log/slog)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("arrêt critique de l'application", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// 2. Chargement et validation de la configuration
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Contexte racine annulé automatiquement sur SIGINT ou SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 3. Initialisation du pool de connexions PostgreSQL (pgxpool)
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	poolCfg.MaxConns = 25
	poolCfg.MinConns = 5
	poolCfg.MaxConnLifetime = 30 * time.Minute

	dbPool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		return err
	}
	logger.Info("connexion PostgreSQL établie avec succès")

	// 4. Assemblage des dépendances (Injection manuelle : Repository -> Service -> Handler)
	queries := postgres.New(dbPool)
	txManager := postgres.NewTxManager(dbPool)

	authSvc := service.NewAuthService(txManager, queries, cfg.JWTSecret)
	accountSvc := service.NewAccountService(txManager, queries)
	auditSvc := service.NewAuditService(txManager, queries)
	transferSvc := service.NewTransferService(txManager, queries, cfg.ApprovalThreshold)

	authHandler := handler.NewAuthHandler(authSvc)
	accountHandler := handler.NewAccountHandler(accountSvc)
	transferHandler := handler.NewTransferHandler(transferSvc)

	router := deliveryhttp.NewRouter(deliveryhttp.RouterConfig{
		JWTSecret:       cfg.JWTSecret,
		Logger:          logger,
		AuditService:    auditSvc,
		AuthHandler:     authHandler,
		AccountHandler:  accountHandler,
		TransferHandler: transferHandler,
	})

	// 5. Configuration du serveur HTTP avec timeouts stricts contre les attaques Slowloris
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Démarrage du serveur dans une goroutine dédiée
	errCh := make(chan error, 1)
	go func() {
		logger.Info("démarrage du serveur Core Banking Binary", slog.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 6. Attente d'un signal d'arrêt ou d'une erreur serveur
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("signal d'arrêt reçu, fermeture gracieuse en cours...")
	}

	// Arrêt gracieux : laisse le temps aux transactions ACID actives de se terminer
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	logger.Info("serveur arrêté proprement")
	return nil
}
