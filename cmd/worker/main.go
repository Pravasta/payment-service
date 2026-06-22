// Command worker menjalankan background job: outbox dispatcher (callback ke app)
// + reconciler (polling GetStatus). Saat ini scaffold — loop kosong.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	log := logger.New(cfg.App.Env)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.Info("worker started (scaffold)")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// TODO: outbox dispatcher (detailed-design §6.2) + reconciler (§6.3).
			log.Debug("worker tick — TODO outbox & reconciler")
		case <-stop:
			log.Info("worker shutting down")
			return
		case <-ctx.Done():
			return
		}
	}
}
