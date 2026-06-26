package payment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
	usecase "github.com/Pravasta/payment-service/internal/usecase/payment"
)

// seedPending menyisipkan transaksi pending dengan gateway_request_id tertentu.
func seedPending(repo *mockRepo, reqID string) *domain.Transaction {
	txn := &domain.Transaction{
		ID:                uuid.New(),
		AppID:             uuid.New(),
		ExternalReference: "INV-WH-1",
		Status:            domain.StatusPending,
		Currency:          "IDR",
		GrossAmount:       150000,
		Gateway:           "doku",
		GatewayRequestID:  reqID,
		CreatedAt:         time.Now().UTC(),
	}
	_ = repo.Create(context.Background(), txn)
	return txn
}

func TestWebhook_InvalidSignature(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{webhookErr: domain.ErrInvalidSignature}
	svc := usecase.NewService(repo, nil, gw)
	seedPending(repo, "req-1")

	err := svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Fatalf("ingin ErrInvalidSignature, dapat %v", err)
	}
	if len(repo.events) != 0 {
		t.Errorf("tidak boleh ada event saat signature invalid, ada %d", len(repo.events))
	}
}

func TestWebhook_PaidTransition(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{webhookEvent: domain.WebhookEvent{
		GatewayEventID:    "notif-1",
		OriginalRequestID: "req-1",
		Status:            domain.StatusPaid,
		PaymentMethod:     "VIRTUAL_ACCOUNT_BCA",
		AmountMinor:       150000,
		OccurredAt:        time.Now().UTC(),
	}}
	svc := usecase.NewService(repo, nil, gw)
	txn := seedPending(repo, "req-1")

	if err := svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	got := repo.byID[txn.ID]
	if got.Status != domain.StatusPaid {
		t.Errorf("status = %q, ingin paid", got.Status)
	}
	if got.PaidAt == nil {
		t.Error("paid_at nil")
	}
	if got.PaymentMethod != "VIRTUAL_ACCOUNT_BCA" {
		t.Errorf("payment_method = %q", got.PaymentMethod)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != domain.EventPaid {
		t.Errorf("events = %v, ingin [paid]", repo.eventTypes())
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, ingin 1", len(repo.outbox))
	}
}

func TestWebhook_DedupSameEventID(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{webhookEvent: domain.WebhookEvent{
		GatewayEventID:    "notif-dup",
		OriginalRequestID: "req-1",
		Status:            domain.StatusPaid,
		AmountMinor:       150000,
	}}
	svc := usecase.NewService(repo, nil, gw)
	seedPending(repo, "req-1")

	_ = svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{})
	_ = svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{}) // delivery dobel

	if len(repo.events) != 1 {
		t.Errorf("events = %d, ingin 1 (dedup)", len(repo.events))
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, ingin 1 (tidak dobel)", len(repo.outbox))
	}
}

func TestWebhook_OutOfOrderIllegal(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{webhookEvent: domain.WebhookEvent{
		GatewayEventID:    "notif-exp",
		OriginalRequestID: "req-1",
		Status:            domain.StatusExpired, // expired setelah paid → ilegal
		AmountMinor:       150000,
	}}
	svc := usecase.NewService(repo, nil, gw)
	txn := seedPending(repo, "req-1")
	txn.Status = domain.StatusPaid // sudah paid
	_ = repo.Update(context.Background(), txn)

	if err := svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{}); err != nil {
		t.Fatalf("handle: %v (harus di-ack)", err)
	}

	got := repo.byID[txn.ID]
	if got.Status != domain.StatusPaid {
		t.Errorf("status = %q, ingin tetap paid (transisi ilegal diabaikan)", got.Status)
	}
	// Event audit tercatat, tapi tanpa outbox.
	if len(repo.events) != 1 {
		t.Errorf("events = %d, ingin 1 (audit)", len(repo.events))
	}
	if len(repo.outbox) != 0 {
		t.Errorf("outbox = %d, ingin 0", len(repo.outbox))
	}
}

func TestWebhook_UnknownTransaction(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{webhookEvent: domain.WebhookEvent{
		GatewayEventID:    "notif-x",
		OriginalRequestID: "tidak-ada",
		Status:            domain.StatusPaid,
	}}
	svc := usecase.NewService(repo, nil, gw)

	// txn tak dikenal → di-ack (nil), tanpa event.
	if err := svc.HandleDOKUWebhook(context.Background(), domain.WebhookPayload{}); err != nil {
		t.Fatalf("ingin nil (ack), dapat %v", err)
	}
	if len(repo.events) != 0 {
		t.Errorf("events = %d, ingin 0", len(repo.events))
	}
}
