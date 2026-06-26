package model

import (
	"time"

	"github.com/google/uuid"
)

// IdempotencyKey menyimpan hasil request tulis pertama agar retry dengan
// Idempotency-Key sama mengembalikan response yang sama (detailed-design §5).
//
// UNIQUE(app_id, idempotency_key) menjamin race-safety: dua request konkuren
// dengan key sama hanya satu yang berhasil INSERT, sisanya mendapat replay
// atau 409 (lihat middleware idempotency).
type IdempotencyKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	AppID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_idem_app_key,priority:1"`
	Key         string    `gorm:"column:idempotency_key;not null;uniqueIndex:uq_idem_app_key,priority:2"`
	RequestHash string    `gorm:"not null"` // sha256(method+uri+body) untuk deteksi payload conflict

	StatusCode   int    `gorm:"not null;default:0"` // 0 = masih diproses; >=100 = response final tersimpan
	ResponseBody []byte `gorm:"type:bytea"`

	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (IdempotencyKey) TableName() string { return "idempotency_key" }
