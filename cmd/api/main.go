// Command api adalah entrypoint HTTP server payment-service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pravasta/payment-service/internal/adapter/gateway/doku"
	httpadapter "github.com/Pravasta/payment-service/internal/adapter/http"
	"github.com/Pravasta/payment-service/internal/adapter/repository"
	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
	"github.com/Pravasta/payment-service/internal/infrastructure/database"
	"github.com/Pravasta/payment-service/internal/infrastructure/logger"
	"github.com/Pravasta/payment-service/internal/infrastructure/metrics"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log := logger.New(cfg.App.Env)
	// Jadikan default agar helper delivery (writeError) memakai logger yang sama.
	slog.SetDefault(log)

	db, err := database.New(cfg.Database)
	if err != nil {
		log.Error("database init failed", "err", err)
		os.Exit(1)
	}

	// Parse master key (AES-GCM untuk enkripsi secret at-rest).
	// Boleh kosong di development; wajib terisi di production (sudah divalidasi config.Load).
	var masterKey []byte
	if cfg.Security.MasterKey != "" {
		masterKey, err = crypto.KeyFromHex(cfg.Security.MasterKey)
		if err != nil {
			log.Error("master key tidak valid", "err", err)
			os.Exit(1)
		}
	}

	// Wiring Clean Architecture: adapter -> usecase -> delivery.
	paymentRepo := repository.NewPaymentRepository(db)
	credRepo := repository.NewCredentialRepository(db)
	idemRepo := repository.NewIdempotencyRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
	refundRepo := repository.NewRefundRepository(db)
	metric := metrics.New()
	dokuGW := doku.New(doku.Config{
		BaseURL:   cfg.DOKU.BaseURL,
		ClientID:  cfg.DOKU.ClientID,
		SecretKey: cfg.DOKU.SecretKey,
	}, doku.WithObserver(metric))
	paymentSvc := usecase.NewService(paymentRepo, refundRepo, dokuGW)

	// Readiness: /readyz sehat hanya bila DB bisa di-ping.
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("get sql.DB failed", "err", err)
		os.Exit(1)
	}
	ready := func(ctx context.Context) error { return sqlDB.PingContext(ctx) }

	router := httpadapter.NewRouter(paymentSvc, ready, credRepo, masterKey, idemRepo, outboxRepo, log, metric)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Info("http server listening", "addr", srv.Addr, "env", cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
	log.Info("bye")
}
