package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"game-wallet-ledger/internal/broker/rabbitmq"
	"game-wallet-ledger/internal/config"
	handler "game-wallet-ledger/internal/handler/http"
	"game-wallet-ledger/internal/repository/postgres"
	redisRepo "game-wallet-ledger/internal/repository/redis"
	"game-wallet-ledger/internal/service"
	"game-wallet-ledger/pkg/logger"
)

func main() {
	cfg := config.Load()
	log := logger.Init(cfg.Environment)

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Info("initializing game-wallet-ledger service",
		"environment", cfg.Environment,
		"port", cfg.ServerPort,
	)

	// Quick check if external infrastructure is reachable (3 second timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	isStandalone := os.Getenv("STANDALONE") == "true"
	var db *postgres.DB
	var rClient *redisRepo.Client
	var broker *rabbitmq.Broker
	var idempRepo redisRepo.IdempotencyStore
	var walletService service.WalletService

	if !isStandalone {
		var err error
		db, err = postgres.NewPool(ctx, cfg.Postgres)
		if err == nil {
			rClient, err = redisRepo.NewClient(ctx, cfg.Redis)
			if err == nil {
				broker, err = rabbitmq.NewBroker(ctx, cfg.RabbitMQ)
			}
		}

		if err != nil {
			log.Warn("external infrastructure (Postgres/Redis/RabbitMQ) not reachable; falling back to Standalone Mode for local testing",
				"notice", err.Error(),
			)
			isStandalone = true
		}
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	if isStandalone {
		log.Info("RUNNING IN STANDALONE MODE (Thread-safe In-Memory Ledger, Idempotency & Simulated RabbitMQ Worker)")
		walletService = service.NewStandaloneWalletService()
		idempRepo = redisRepo.NewInMemoryIdempotencyRepository()
	} else {
		log.Info("RUNNING IN PRODUCTION MODE (Connected to PostgreSQL, Redis, and RabbitMQ)")
		_ = db.RunMigrations(ctx, "migrations")

		walletRepo := postgres.NewWalletRepository(db)
		txRepo := postgres.NewTransactionRepository(db)
		ledgerRepo := postgres.NewLedgerRepository(db)
		eventRepo := postgres.NewEventRepository(db)
		idempRepo = redisRepo.NewIdempotencyRepository(rClient)

		publisher := rabbitmq.NewPublisher(broker)
		walletService = service.NewWalletService(db, walletRepo, txRepo, ledgerRepo, publisher)

		worker := rabbitmq.NewWorker(broker, eventRepo)
		go func() {
			if err := worker.Start(workerCtx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("background event worker error", "error", err.Error())
			}
		}()

		defer func() {
			if broker != nil {
				_ = broker.Close()
			}
			if rClient != nil {
				_ = rClient.Close()
			}
			if db != nil {
				db.Close()
			}
		}()
	}

	// HTTP Router & Handlers
	engine := gin.New()
	httpHandler := handler.NewHandler(walletService, db, rClient, broker)
	handler.RegisterRoutes(engine, httpHandler, idempRepo, cfg.Redis.IdempotencyTTL)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run HTTP server in background
	go func() {
		log.Info("HTTP server listening", "addr", fmt.Sprintf("http://localhost:%d", cfg.ServerPort))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server failed", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Info("shutdown signal received, stopping gracefully...", "signal", sig.String())
	workerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown forced", "error", err.Error())
	} else {
		log.Info("HTTP server cleanly stopped")
	}
}
