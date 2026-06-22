// Package repository mengimplementasi port domain.Repository memakai GORM.
package repository

import (
	"context"
	"errors"

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

func (r *PaymentRepository) AppendEvent(ctx context.Context, e *domain.TransactionEvent) error {
	m := eventToModel(e)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
		e.ID = m.ID
	}
	return r.db.WithContext(ctx).Create(m).Error
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
