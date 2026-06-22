package model

import (
	"time"

	"github.com/google/uuid"
)

// Merchant adalah aplikasi pemanggil (detailed-design §2.1).
type Merchant struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code      string    `gorm:"not null;uniqueIndex"` // mis. "invoice-saas"
	Name      string    `gorm:"not null"`
	Status    string    `gorm:"not null;default:'active'"` // active | suspended
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Merchant) TableName() string { return "merchant" }

// APICredential: auth app -> PS (detailed-design §2.2).
type APICredential struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	MerchantID       uuid.UUID `gorm:"type:uuid;not null;index"`
	KeyID            string    `gorm:"not null;uniqueIndex"`      // public, mis. "pk_live_xxx"
	SecretHash       string    `gorm:"not null"`                  // argon2id(secret)
	SigningSecretEnc []byte    `gorm:"type:bytea"`                // HMAC request signing (terenkripsi)
	Scopes           []string  `gorm:"serializer:json"`           // ["payments:write", ...]
	Status           string    `gorm:"not null;default:'active'"` // active | revoked
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (APICredential) TableName() string { return "api_credential" }

// WebhookEndpoint: callback PS -> app (detailed-design §2.3).
type WebhookEndpoint struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	MerchantID       uuid.UUID `gorm:"type:uuid;not null;index"`
	URL              string    `gorm:"not null"`
	SigningSecretEnc []byte    `gorm:"type:bytea"`                // PS menandatangani callback
	Status           string    `gorm:"not null;default:'active'"` // active | disabled
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (WebhookEndpoint) TableName() string { return "webhook_endpoint" }
