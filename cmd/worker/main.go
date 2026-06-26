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

	"github.com/Pravasta/payment-service/internal/adapter/repository"
	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
	"github.com/Pravasta/payment-service/internal/infrastructure/database"
	"github.com/Pravasta/payment-service/internal/infrastructure/logger"
	"github.com/Pravasta/payment-service/internal/outbox"
)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Info("worker started", "env", cfg.App.Env)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n, err := dispatcher.RunOnce(ctx)
			if err != nil {
				log.Error("outbox dispatch error", "err", err)
			} else if n > 0 {
				log.Debug("outbox batch diproses", "count", n)
			}
			// TODO: reconciler (detailed-design §6.3) — issue 0010.
		case <-stop:
			log.Info("worker shutting down")
			return
		case <-ctx.Done():
			return
		}
	}
}
