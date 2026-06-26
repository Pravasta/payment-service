package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Pravasta/payment-service/internal/adapter/repository/model"
	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// RefundRepository implementasi GORM dari domain.RefundRepository.
type RefundRepository struct {
	db *gorm.DB
}

func NewRefundRepository(db *gorm.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

var _ domain.RefundRepository = (*RefundRepository)(nil)

func (r *RefundRepository) Create(ctx context.Context, rf *domain.Refund) error {
	m := refundToModel(rf)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
		rf.ID = m.ID
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *RefundRepository) Update(ctx context.Context, rf *domain.Refund) error {
	return r.db.WithContext(ctx).Save(refundToModel(rf)).Error
}

func (r *RefundRepository) GetByID(ctx context.Context, appID, id uuid.UUID) (*domain.Refund, error) {
	return r.firstRefund(ctx, "app_id = ? AND id = ?", appID, id)
}

func (r *RefundRepository) GetByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*domain.Refund, error) {
	return r.firstRefund(ctx, "app_id = ? AND idempotency_key = ?", appID, key)
}

// ApplyRefundSucceeded menulis finalisasi refund secara atomik (satu DB-tx).
func (r *RefundRepository) ApplyRefundSucceeded(ctx context.Context, refund *domain.Refund, txn *domain.Transaction, event *domain.TransactionEvent, outbox *domain.OutboxMessage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(refundToModel(refund)).Error; err != nil {
			return err
		}
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

func (r *RefundRepository) firstRefund(ctx context.Context, query string, args ...any) (*domain.Refund, error) {
	var m model.Refund
	err := r.db.WithContext(ctx).Where(query, args...).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return refundToDomain(&m), nil
}

func refundToModel(rf *domain.Refund) *model.Refund {
	return &model.Refund{
		ID:              rf.ID,
		AppID:           rf.AppID,
		TransactionID:   rf.TransactionID,
		IdempotencyKey:  rf.IdempotencyKey,
		AmountMinor:     rf.AmountMinor,
		Currency:        rf.Currency,
		Status:          string(rf.Status),
		GatewayRefundID: rf.GatewayRefundID,
		Reason:          rf.Reason,
		CreatedAt:       rf.CreatedAt,
		UpdatedAt:       rf.UpdatedAt,
	}
}

func refundToDomain(m *model.Refund) *domain.Refund {
	return &domain.Refund{
		ID:              m.ID,
		AppID:           m.AppID,
		TransactionID:   m.TransactionID,
		IdempotencyKey:  m.IdempotencyKey,
		AmountMinor:     m.AmountMinor,
		Currency:        m.Currency,
		Status:          domain.RefundStatus(m.Status),
		GatewayRefundID: m.GatewayRefundID,
		Reason:          m.Reason,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
