// Command migrate menjalankan GORM AutoMigrate untuk membuat/menyesuaikan skema.
// Untuk produksi, pertimbangkan migrasi SQL berversi (mis. golang-migrate).
package main

import (
	"fmt"
	"os"

	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database error:", err)
		os.Exit(1)
	}

	if err := db.AutoMigrate(
		&model.Merchant{},
		&model.APICredential{},
		&model.WebhookEndpoint{},
		&model.GatewayAccount{},
		&model.Transaction{},
		&model.TransactionEvent{},
		&model.Refund{},
		&model.WebhookInbox{},
		&model.NotificationOutbox{},
	); err != nil {
		fmt.Fprintln(os.Stderr, "migrate error:", err)
		os.Exit(1)
	}

	// Partial unique index untuk dedup webhook: gateway_event_id unik hanya saat
	// terisi (detailed-design §2.6). Kolom bertipe string (default ''), jadi
	// padanan "WHERE NOT NULL" adalah "<> ''". Tidak bisa via tag GORM.
	if err := db.Exec(
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_txn_event_gateway_event_id ` +
			`ON transaction_event (gateway_event_id) WHERE gateway_event_id <> ''`,
	).Error; err != nil {
		fmt.Fprintln(os.Stderr, "migrate error (partial index):", err)
		os.Exit(1)
	}

	fmt.Println("migrate: ok")
}
