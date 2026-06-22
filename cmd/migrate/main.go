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
		&model.Transaction{},
		&model.TransactionEvent{},
		&model.Refund{},
		// TODO: merchant, api_credential, webhook_endpoint, gateway_account,
		//       webhook_inbox, notification_outbox (detailed-design §2).
	); err != nil {
		fmt.Fprintln(os.Stderr, "migrate error:", err)
		os.Exit(1)
	}
	fmt.Println("migrate: ok")
}
