// Command worker menjalankan background job: outbox dispatcher (callback ke app)
// + reconciler (polling GetStatus, issue 0010).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pravasta/payment-service/internal/adapter/gateway/doku"
	"github.com/Pravasta/payment-service/internal/adapter/repository"
	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
	"github.com/Pravasta/payment-service/internal/infrastructure/database"
	"github.com/Pravasta/payment-service/internal/infrastructure/logger"
	"github.com/Pravasta/payment-service/internal/outbox"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// reconcileBatch membatasi jumlah transaksi pending per siklus reconciler.
const reconcileBatch = 100

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	log := logger.New(cfg.App.Env)

	db, err := database.New(cfg.Database)
	if err != nil {
		log.Error("database init failed", "err", err)
		os.Exit(1)
	}

	var masterKey []byte
	if cfg.Security.MasterKey != "" {
		masterKey, err = crypto.KeyFromHex(cfg.Security.MasterKey)
		if err != nil {
			log.Error("master key tidak valid", "err", err)
			os.Exit(1)
		}
	}

	outboxRepo := repository.NewOutboxRepository(db)
	dispatcher := outbox.NewDispatcher(outboxRepo, masterKey, log, outbox.DefaultConfig())

	paymentRepo := repository.NewPaymentRepository(db)
	refundRepo := repository.NewRefundRepository(db)
	dokuGW := doku.New(doku.Config{
		BaseURL:   cfg.DOKU.BaseURL,
		ClientID:  cfg.DOKU.ClientID,
		SecretKey: cfg.DOKU.SecretKey,
	})
	paymentSvc := usecase.NewService(paymentRepo, refundRepo, dokuGW)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Info("worker started", "env", cfg.App.Env)
	// Outbox dispatch sering; reconciler lebih jarang (polling gateway).
	outboxTicker := time.NewTicker(5 * time.Second)
	defer outboxTicker.Stop()
	reconcileTicker := time.NewTicker(60 * time.Second)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-outboxTicker.C:
			n, err := dispatcher.RunOnce(ctx)
			if err != nil {
				log.Error("outbox dispatch error", "err", err)
			} else if n > 0 {
				log.Debug("outbox batch diproses", "count", n)
			}
		case <-reconcileTicker.C:
			changed, err := paymentSvc.ReconcilePending(ctx, reconcileBatch)
			if err != nil {
				log.Error("reconcile error", "err", err)
			} else if changed > 0 {
				log.Info("reconciler memperbarui transaksi", "changed", changed)
			}
		case <-stop:
			log.Info("worker shutting down")
			return
		case <-ctx.Done():
			return
		}
	}
}
