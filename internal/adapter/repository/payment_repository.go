// Package repository mengimplementasi port domain.Repository memakai GORM.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// PaymentRepository implementasi GORM dari domain.Repository.
type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// pastikan memenuhi kontrak port.
var _ domain.Repository = (*PaymentRepository)(nil)

func (r *PaymentRepository) Create(ctx context.Context, txn *domain.Transaction) error {
	m := toModel(txn)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
		txn.ID = m.ID
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *PaymentRepository) Update(ctx context.Context, txn *domain.Transaction) error {
	return r.db.WithContext(ctx).Save(toModel(txn)).Error
}

func (r *PaymentRepository) GetByID(ctx context.Context, appID, id uuid.UUID) (*domain.Transaction, error) {
	return r.first(ctx, "app_id = ? AND id = ?", appID, id)
}

func (r *PaymentRepository) GetByExternalReference(ctx context.Context, appID uuid.UUID, ref string) (*domain.Transaction, error) {
	return r.first(ctx, "app_id = ? AND external_reference = ?", appID, ref)
}

func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*domain.Transaction, error) {
	return r.first(ctx, "app_id = ? AND idempotency_key = ?", appID, key)
}

func (r *PaymentRepository) GetByGatewayTxnID(ctx context.Context, gateway, gatewayTxnID string) (*domain.Transaction, error) {
	return r.first(ctx, "gateway = ? AND gateway_txn_id = ?", gateway, gatewayTxnID)
}

func (r *PaymentRepository) GetByGatewayRequestID(ctx context.Context, gateway, requestID string) (*domain.Transaction, error) {
	return r.first(ctx, "gateway = ? AND gateway_request_id = ?", gateway, requestID)
}

func (r *PaymentRepository) ListTransactions(ctx context.Context, f domain.ListFilter) ([]*domain.Transaction, error) {
	q := r.db.WithContext(ctx).Model(&model.Transaction{}).Where("app_id = ?", f.AppID)
	if f.Status != "" {
		q = q.Where("status = ?", string(f.Status))
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	// Keyset pagination: ambil yang "lebih lama" dari cursor (urutan created_at DESC, id DESC).
	if f.CursorCreated != nil && f.CursorID != nil {
		q = q.Where("(created_at, id) < (?, ?)", *f.CursorCreated, *f.CursorID)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}

	var ms []model.Transaction
	if err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.Transaction, len(ms))
	for i := range ms {
		out[i] = toDomain(&ms[i])
	}
	return out, nil
}

func (r *PaymentRepository) ListPendingForReconcile(ctx context.Context, olderThan time.Time, limit int) ([]*domain.Transaction, error) {
	if limit <= 0 {
		limit = 100
	}
	var ms []model.Transaction
	err := r.db.WithContext(ctx).
		Where("status = ? AND created_at <= ?", string(domain.StatusPending), olderThan).
		Order("created_at asc").
		Limit(limit).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Transaction, len(ms))
	for i := range ms {
		out[i] = toDomain(&ms[i])
	}
	return out, nil
}

func (r *PaymentRepository) AppendEvent(ctx context.Context, e *domain.TransactionEvent) error {
	m := eventToModel(e)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
		e.ID = m.ID
	}
	return r.db.WithContext(ctx).Create(m).Error
}

// --- webhook (detailed-design §6.1) ---

func (r *PaymentRepository) SaveWebhookInbox(ctx context.Context, rec *domain.WebhookInboxRecord) error {
	m := inboxToModel(rec)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
		rec.ID = m.ID
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *PaymentRepository) UpdateWebhookInbox(ctx context.Context, rec *domain.WebhookInboxRecord) error {
	return r.db.WithContext(ctx).Save(inboxToModel(rec)).Error
}

func (r *PaymentRepository) EventExists(ctx context.Context, gatewayEventID string) (bool, error) {
	if gatewayEventID == "" {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.TransactionEvent{}).
		Where("gateway_event_id = ?", gatewayEventID).
		Count(&count).Error
	return count > 0, err
}

// ApplyWebhook menulis update txn + event + outbox dalam SATU transaksi DB
// (atomic, detailed-design §6.1 langkah 5).
func (r *PaymentRepository) ApplyWebhook(ctx context.Context, txn *domain.Transaction, event *domain.TransactionEvent, outbox *domain.OutboxMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(toModel(txn)).Error; err != nil {
			return err
		}
		em := eventToModel(event)
		if em.ID == uuid.Nil {
			em.ID = uuid.New()
			event.ID = em.ID
		}
		if err := tx.Create(em).Error; err != nil {
			return err
		}
		if outbox != nil {
			om := outboxToModel(outbox)
			if om.ID == uuid.Nil {
				om.ID = uuid.New()
				outbox.ID = om.ID
			}
			if err := tx.Create(om).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func inboxToModel(rec *domain.WebhookInboxRecord) *model.WebhookInbox {
	return &model.WebhookInbox{
		ID:            rec.ID,
		Gateway:       rec.Gateway,
		Signature:     rec.Signature,
		Headers:       model.JSONMap(rec.Headers),
		RawBody:       rec.RawBody,
		Verified:      rec.Verified,
		Processed:     rec.Processed,
		TransactionID: rec.TransactionID,
		ReceivedAt:    rec.ReceivedAt,
	}
}

func outboxToModel(o *domain.OutboxMessage) *model.NotificationOutbox {
	return &model.NotificationOutbox{
		ID:            o.ID,
		AppID:         o.AppID,
		TransactionID: o.TransactionID,
		EventID:       o.EventID,
		EventType:     o.EventType,
		Payload:       model.JSONMap(o.Payload),
		Status:        "pending",
		NextRetryAt:   time.Now().UTC(),
	}
}

func (r *PaymentRepository) first(ctx context.Context, query string, args ...any) (*domain.Transaction, error) {
	var m model.Transaction
	err := r.db.WithContext(ctx).Where(query, args...).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

// --- mappers (domain <-> model) ---

func toModel(t *domain.Transaction) *model.Transaction {
	return &model.Transaction{
		ID:                t.ID,
		AppID:             t.AppID,
		ExternalReference: t.ExternalReference,
		IdempotencyKey:    t.IdempotencyKey,
		Status:            string(t.Status),
		Currency:          t.Currency,
		CurrencyExponent:  int16(t.CurrencyExponent),
		GrossAmount:       t.GrossAmount,
		FeeAmount:         t.FeeAmount,
		NetAmount:         t.NetAmount,
		RefundedAmount:    t.RefundedAmount,
		Gateway:           t.Gateway,
		GatewayTxnID:      t.GatewayTxnID,
		GatewayRequestID:  t.GatewayRequestID,
		PaymentURL:        t.PaymentURL,
		PaymentMethod:     t.PaymentMethod,
		CustomerRef:       t.CustomerRef,
		Metadata:          model.JSONMap(t.Metadata),
		Description:       t.Description,
		ExpiresAt:         t.ExpiresAt,
		PaidAt:            t.PaidAt,
		SettledAt:         t.SettledAt,
		FailedAt:          t.FailedAt,
		ExpiredAt:         t.ExpiredAt,
		CreatedAt:         t.CreatedAt,
		UpdatedAt:         t.UpdatedAt,
	}
}

func toDomain(m *model.Transaction) *domain.Transaction {
	return &domain.Transaction{
		ID:                m.ID,
		AppID:             m.AppID,
		ExternalReference: m.ExternalReference,
		IdempotencyKey:    m.IdempotencyKey,
		Status:            domain.Status(m.Status),
		Currency:          m.Currency,
		CurrencyExponent:  int(m.CurrencyExponent),
		GrossAmount:       m.GrossAmount,
		FeeAmount:         m.FeeAmount,
		NetAmount:         m.NetAmount,
		RefundedAmount:    m.RefundedAmount,
		Gateway:           m.Gateway,
		GatewayTxnID:      m.GatewayTxnID,
		GatewayRequestID:  m.GatewayRequestID,
		PaymentURL:        m.PaymentURL,
		PaymentMethod:     m.PaymentMethod,
		CustomerRef:       m.CustomerRef,
		Metadata:          map[string]any(m.Metadata),
		Description:       m.Description,
		ExpiresAt:         m.ExpiresAt,
		PaidAt:            m.PaidAt,
		SettledAt:         m.SettledAt,
		FailedAt:          m.FailedAt,
		ExpiredAt:         m.ExpiredAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func eventToModel(e *domain.TransactionEvent) *model.TransactionEvent {
	return &model.TransactionEvent{
		ID:             e.ID,
		TransactionID:  e.TransactionID,
		AppID:          e.AppID,
		EventType:      string(e.EventType),
		FromStatus:     string(e.FromStatus),
		ToStatus:       string(e.ToStatus),
		AmountMinor:    e.AmountMinor,
		Source:         e.Source,
		GatewayEventID: e.GatewayEventID,
		Payload:        model.JSONMap(e.Payload),
		OccurredAt:     e.OccurredAt,
		CreatedAt:      e.CreatedAt,
	}
}
