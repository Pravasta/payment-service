// Command seed membuat data dev minimal agar API bisa langsung dicoba (mis. via
// Postman): satu merchant + satu API credential aktif dengan scope penuh.
//
// Idempoten: dijalankan ulang akan menyetel ulang credential ke nilai di bawah.
// HMAC request-signing DIMATIKAN (SigningSecretEnc nil) supaya cukup pakai header
// Authorization saja. Khusus development — JANGAN dipakai di produksi.
//
//	go run ./cmd/seed
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm/clause"

	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
	"github.com/Pravasta/payment-service/internal/infrastructure/config"
	"github.com/Pravasta/payment-service/internal/infrastructure/database"
)

// Kredensial dev yang dibuat. Ubah lewat env bila perlu.
const (
	defaultMerchantCode = "demo"
	defaultMerchantName = "Demo Merchant"
	defaultKeyID        = "pk_demo_001"
	defaultSecret       = "sk_demo_secret_change_me"
)

// parameter argon2id (selaras dengan verifier di middleware auth).
const (
	argonMemory  = 64 * 1024
	argonTime    = 1
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
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

	merchantCode := getenv("SEED_MERCHANT_CODE", defaultMerchantCode)
	keyID := getenv("SEED_KEY_ID", defaultKeyID)
	secret := getenv("SEED_SECRET", defaultSecret)

	now := time.Now().UTC()

	// 1. Merchant (upsert by code).
	merchant := model.Merchant{
		ID:        uuid.New(),
		Code:      merchantCode,
		Name:      defaultMerchantName,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "status", "updated_at"}),
	}).Create(&merchant).Error; err != nil {
		fmt.Fprintln(os.Stderr, "seed merchant error:", err)
		os.Exit(1)
	}
	// Pastikan punya ID merchant yang benar (saat conflict, ID di-merchant tetap
	// yang baru kita generate hanya bila insert; ambil ulang untuk amannya).
	if err := db.Where("code = ?", merchantCode).First(&merchant).Error; err != nil {
		fmt.Fprintln(os.Stderr, "seed lookup merchant error:", err)
		os.Exit(1)
	}

	// 2. API credential (upsert by key_id). HMAC off → SigningSecretEnc nil.
	hash, err := hashArgon2id(secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed hash error:", err)
		os.Exit(1)
	}
	cred := model.APICredential{
		ID:         uuid.New(),
		MerchantID: merchant.ID,
		KeyID:      keyID,
		SecretHash: hash,
		Scopes:     []string{"payments:read", "payments:write", "outbox:admin"},
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"merchant_id", "secret_hash", "scopes", "status", "updated_at"}),
	}).Create(&cred).Error; err != nil {
		fmt.Fprintln(os.Stderr, "seed credential error:", err)
		os.Exit(1)
	}

	fmt.Println("seed: ok")
	fmt.Println("  merchant_id :", merchant.ID)
	fmt.Println("  key_id      :", keyID)
	fmt.Println("  secret      :", secret)
	fmt.Println("  scopes      : payments:read payments:write outbox:admin")
	fmt.Println()
	fmt.Printf("  Authorization: Bearer %s:%s\n", keyID, secret)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// hashArgon2id menghasilkan hash PHC argon2id yang kompatibel dengan verifier
// di internal/adapter/http/middleware/auth.go.
func hashArgon2id(secret string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}
