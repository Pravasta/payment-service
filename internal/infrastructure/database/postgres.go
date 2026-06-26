// Package database menyediakan koneksi PostgreSQL via GORM.
package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Pravasta/payment-service/internal/infrastructure/config"
)

// New membuka koneksi GORM ke PostgreSQL dan mengatur connection pool.
func New(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		// TranslateError memetakan error driver ke error GORM portabel
		// (mis. gorm.ErrDuplicatedKey untuk pelanggaran UNIQUE) — dipakai
		// idempotency repository untuk mendeteksi key duplikat.
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database: get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return db, nil
}
