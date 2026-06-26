package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// RefundInput adalah input use-case refund.
type RefundInput struct {
	AppID          uuid.UUID
	TransactionID  uuid.UUID
	IdempotencyKey string
	AmountMinor    int64 // 0 = full refund (sisa refundable)
	Reason         string
}

// Refund membuat & mengeksekusi refund (detailed-design §8):
// guard state & amount → buat baris refund (requested) → panggil gateway →
// sync: finalisasi (succeeded + refunded_amount + status + event + callback);
// async: tahan di pending (finalisasi via Refund Notification/poll).
func (s *Service) Refund(ctx context.Context, in RefundInput) (*domain.Refund, error) {
	txn, err := s.repo.GetByID(ctx, in.AppID, in.TransactionID)
	if err != nil {
		return nil, err
	}

	// Idempotency: key sama → kembalikan refund yang sudah ada.
	if in.IdempotencyKey != "" {
		existing, err := s.refunds.GetByIdempotencyKey(ctx, in.AppID, in.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}

	// Guard state: hanya transaksi paid/settled/partially_refunded yang refundable.
	switch txn.Status {
	case domain.StatusPaid, domain.StatusSettled, domain.StatusPartiallyRefunded:
	default:
		return nil, domain.ErrNotRefundable
	}

	// Guard amount.
	refundable := txn.GrossAmount - txn.RefundedAmount
	amount := in.AmountMinor
	if amount == 0 {
		amount = refundable // full refund = sisa refundable
	}
	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if amount > refundable {
		return nil, domain.ErrRefundExceedsAmount
	}

	// Buat baris refund (requested).
	now := time.Now().UTC()
	refund := &domain.Refund{
		ID:             uuid.New(),
		AppID:          in.AppID,
		TransactionID:  txn.ID,
		IdempotencyKey: in.IdempotencyKey,
		AmountMinor:    amount,
		Currency:       txn.Currency,
		Status:         domain.RefundRequested,
		Reason:         in.Reason,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.refunds.Create(ctx, refund); err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	// Tentukan tipe: full bila menutup seluruh gross & belum ada refund sebelumnya.
	rtype := domain.RefundTypePartial
	if amount == txn.GrossAmount && txn.RefundedAmount == 0 {
		rtype = domain.RefundTypeFull
	}

	// Panggil gateway.
	res, err := s.gateway.Refund(ctx, domain.RefundRequest{
		ExternalReference: txn.ExternalReference,
		OriginalRequestID: txn.GatewayRequestID,
		AmountMinor:       amount,
		Currency:          txn.Currency,
		Reason:            in.Reason,
		Type:              rtype,
		PaymentMethod:     txn.PaymentMethod,
	})
	if err != nil {
		s.markRefundFailed(ctx, refund)
		return nil, fmt.Errorf("gateway refund: %w", err)
	}

	refund.GatewayRefundID = res.GatewayRefundID
	switch res.Status {
	case domain.RefundSucceeded:
		if err := s.finalizeRefund(ctx, txn, refund, amount); err != nil {
			return nil, err
		}
	case domain.RefundPending:
		refund.Status = domain.RefundPending
		refund.UpdatedAt = time.Now().UTC()
		if err := s.refunds.Update(ctx, refund); err != nil {
			return nil, err
		}
	default: // failed
		s.markRefundFailed(ctx, refund)
	}
	return refund, nil
}

// finalizeRefund menerapkan refund sukses secara atomik: naikkan refunded_amount,
// hitung ulang status transaksi, tulis event + callback (detailed-design §8).
func (s *Service) finalizeRefund(ctx context.Context, txn *domain.Transaction, refund *domain.Refund, amount int64) error {
	now := time.Now().UTC()
	refund.Status = domain.RefundSucceeded
	refund.UpdatedAt = now

	from := txn.Status
	txn.RefundedAmount += amount
	newStatus := domain.StatusPartiallyRefunded
	if txn.RefundedAmount >= txn.GrossAmount {
		newStatus = domain.StatusRefunded
	}
	if domain.CanTransition(from, newStatus) {
		txn.Status = newStatus
	}
	txn.UpdatedAt = now

	event := &domain.TransactionEvent{
		ID:            uuid.New(),
		TransactionID: txn.ID,
		AppID:         txn.AppID,
		EventType:     eventTypeForStatus(txn.Status),
		FromStatus:    from,
		ToStatus:      txn.Status,
		AmountMinor:   amount,
		Source:        "api",
		OccurredAt:    now,
		CreatedAt:     now,
	}
	return s.refunds.ApplyRefundSucceeded(ctx, refund, txn, event, newOutbox(txn, event))
}

func (s *Service) markRefundFailed(ctx context.Context, refund *domain.Refund) {
	refund.Status = domain.RefundFailed
	refund.UpdatedAt = time.Now().UTC()
	_ = s.refunds.Update(ctx, refund)
}
