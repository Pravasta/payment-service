package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
	"github.com/Pravasta/payment-service/internal/outbox"
)

// OutboxRepository mengimplementasikan outbox.Store dengan GORM.
type OutboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// pastikan memenuhi kontrak port.
var _ outbox.Store = (*OutboxRepository)(nil)

// FetchDue mengambil pesan pending yang sudah jatuh tempo (next_retry_at <= now),
// urut paling lama dulu.
func (r *OutboxRepository) FetchDue(ctx context.Context, limit int) ([]outbox.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	var ms []model.NotificationOutbox
	err := r.db.WithContext(ctx).
		Where("status = ? AND next_retry_at <= ?", "pending", time.Now().UTC()).
		Order("next_retry_at asc").
		Limit(limit).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	out := make([]outbox.Message, len(ms))
	for i := range ms {
		out[i] = outbox.Message{
			ID:            ms[i].ID,
			AppID:         ms[i].AppID,
			TransactionID: ms[i].TransactionID,
			EventID:       ms[i].EventID,
			EventType:     ms[i].EventType,
			Payload:       map[string]any(ms[i].Payload),
			Attempts:      ms[i].Attempts,
		}
	}
	return out, nil
}

// ActiveEndpoint mengambil webhook_endpoint aktif milik merchant.
func (r *OutboxRepository) ActiveEndpoint(ctx context.Context, merchantID uuid.UUID) (outbox.Endpoint, error) {
	var m model.WebhookEndpoint
	err := r.db.WithContext(ctx).
		Where("merchant_id = ? AND status = ?", merchantID, "active").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return outbox.Endpoint{}, outbox.ErrNoEndpoint
	}
	if err != nil {
		return outbox.Endpoint{}, err
	}
	return outbox.Endpoint{URL: m.URL, SigningSecretEnc: m.SigningSecretEnc}, nil
}

func (r *OutboxRepository) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":       "delivered",
			"delivered_at": now,
			"last_error":   "",
		}).Error
}

func (r *OutboxRepository) MarkRetry(ctx context.Context, id uuid.UUID, attempts int, nextRetryAt time.Time, lastErr string) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        "pending",
			"attempts":      attempts,
			"next_retry_at": nextRetryAt.UTC(),
			"last_error":    truncate(lastErr, 1000),
		}).Error
}

func (r *OutboxRepository) MarkDead(ctx context.Context, id uuid.UUID, attempts int, lastErr string) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     "dead",
			"attempts":   attempts,
			"last_error": truncate(lastErr, 1000),
		}).Error
}

// Replay mengembalikan pesan dead ke pending agar dikirim ulang (admin DLQ).
func (r *OutboxRepository) Replay(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ? AND status = ?", id, "dead").
		Updates(map[string]any{
			"status":        "pending",
			"attempts":      0,
			"next_retry_at": time.Now().UTC(),
			"last_error":    "",
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return outbox.ErrNotReplayable
	}
	return nil
}

// Counts mengembalikan jumlah pesan outbox per status untuk gauge backlog
// observability (pending = antri/akan retry, dead = dead-letter). Dipanggil
// periodik oleh worker; bukan bagian dari port outbox.Store.
func (r *OutboxRepository) Counts(ctx context.Context) (pending, dead int64, err error) {
	type row struct {
		Status string
		N      int64
	}
	var rows []row
	err = r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Select("status, count(*) as n").
		Where("status IN ?", []string{"pending", "dead"}).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return 0, 0, err
	}
	for _, rw := range rows {
		switch rw.Status {
		case "pending":
			pending = rw.N
		case "dead":
			dead = rw.N
		}
	}
	return pending, dead, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
