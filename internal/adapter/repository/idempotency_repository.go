package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	appmw "github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
)

// IdempotencyRepository mengimplementasikan middleware.IdempotencyStore dengan GORM.
type IdempotencyRepository struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

// pastikan implementasi memenuhi kontrak port.
var _ appmw.IdempotencyStore = (*IdempotencyRepository)(nil)

// Begin mencoba INSERT record in-progress. Bila (app_id, key) sudah ada,
// pelanggaran UNIQUE terdeteksi (gorm.ErrDuplicatedKey) dan record lama dimuat.
// Pola ini race-safe: hanya satu INSERT konkuren yang menang.
func (r *IdempotencyRepository) Begin(ctx context.Context, appID uuid.UUID, key, requestHash string) (*appmw.IdempotencyRecord, bool, error) {
	m := &model.IdempotencyKey{
		ID:          uuid.New(),
		AppID:       appID,
		Key:         key,
		RequestHash: requestHash,
		StatusCode:  0, // in-progress
	}
	err := r.db.WithContext(ctx).Create(m).Error
	if err == nil {
		return toIdemRecord(m), true, nil
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil, false, err
	}

	// Key sudah ada — muat record lama untuk diputuskan oleh middleware.
	var existing model.IdempotencyKey
	if e := r.db.WithContext(ctx).
		Where("app_id = ? AND idempotency_key = ?", appID, key).
		First(&existing).Error; e != nil {
		return nil, false, e
	}
	return toIdemRecord(&existing), false, nil
}

// Complete menyimpan response final ke record.
func (r *IdempotencyRepository) Complete(ctx context.Context, id uuid.UUID, statusCode int, responseBody []byte) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.IdempotencyKey{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status_code":   statusCode,
			"response_body": responseBody,
			"completed_at":  now,
		}).Error
}

// Discard menghapus record in-progress (mis. handler mengembalikan 5xx).
func (r *IdempotencyRepository) Discard(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&model.IdempotencyKey{}, "id = ?", id).Error
}

func toIdemRecord(m *model.IdempotencyKey) *appmw.IdempotencyRecord {
	return &appmw.IdempotencyRecord{
		ID:           m.ID,
		RequestHash:  m.RequestHash,
		StatusCode:   m.StatusCode,
		ResponseBody: m.ResponseBody,
	}
}
