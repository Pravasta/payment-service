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

// --- mock RefundRepository ---

type mockRefundRepo struct {
	byID    map[uuid.UUID]*domain.Refund
	byIdem  map[string]*domain.Refund
	events  []*domain.TransactionEvent
	outbox  []*domain.OutboxMessage
	applied int
}

func newMockRefundRepo() *mockRefundRepo {
	return &mockRefundRepo{byID: map[uuid.UUID]*domain.Refund{}, byIdem: map[string]*domain.Refund{}}
}

func (m *mockRefundRepo) Create(_ context.Context, r *domain.Refund) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	m.byID[r.ID] = r
	if r.IdempotencyKey != "" {
		m.byIdem[r.AppID.String()+"|"+r.IdempotencyKey] = r
	}
	return nil
}
func (m *mockRefundRepo) Update(_ context.Context, r *domain.Refund) error {
	m.byID[r.ID] = r
	return nil
}
func (m *mockRefundRepo) GetByID(_ context.Context, _, id uuid.UUID) (*domain.Refund, error) {
	if r, ok := m.byID[id]; ok {
		return r, nil
	}
	return nil, domain.ErrNotFound
}
func (m *mockRefundRepo) GetByIdempotencyKey(_ context.Context, appID uuid.UUID, key string) (*domain.Refund, error) {
	if r, ok := m.byIdem[appID.String()+"|"+key]; ok {
		return r, nil
	}
	return nil, domain.ErrNotFound
}
func (m *mockRefundRepo) ApplyRefundSucceeded(_ context.Context, refund *domain.Refund, _ *domain.Transaction, event *domain.TransactionEvent, outbox *domain.OutboxMessage) error {
	m.applied++
	m.byID[refund.ID] = refund
	m.events = append(m.events, event)
	if outbox != nil {
		m.outbox = append(m.outbox, outbox)
	}
	return nil
}

func seedPaid(repo *mockRepo, gross int64) *domain.Transaction {
	txn := &domain.Transaction{
		ID:                uuid.New(),
		AppID:             uuid.New(),
		ExternalReference: "INV-RF-1",
		Status:            domain.StatusPaid,
		Currency:          "IDR",
		GrossAmount:       gross,
		Gateway:           "doku",
		GatewayRequestID:  "req-paid-1",
		PaymentMethod:     "CREDIT_CARD",
		CreatedAt:         time.Now().UTC(),
	}
	_ = repo.Create(context.Background(), txn)
	return txn
}

func TestRefund_FullSync(t *testing.T) {
	repo := newMockRepo()
	refunds := newMockRefundRepo()
	gw := &mockGateway{refundResult: domain.RefundResult{Status: domain.RefundSucceeded, GatewayRefundID: "rfnd-gw-1"}}
	svc := usecase.NewService(repo, refunds, gw)
	txn := seedPaid(repo, 150000)

	rf, err := svc.Refund(context.Background(), usecase.RefundInput{
		AppID: txn.AppID, TransactionID: txn.ID, IdempotencyKey: "rf-1",
	})
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if rf.Status != domain.RefundSucceeded {
		t.Errorf("refund status = %q, ingin succeeded", rf.Status)
	}
	if rf.AmountMinor != 150000 {
		t.Errorf("amount = %d, ingin 150000 (full)", rf.AmountMinor)
	}
	got := repo.byID[txn.ID]
	if got.RefundedAmount != 150000 {
		t.Errorf("refunded_amount = %d, ingin 150000", got.RefundedAmount)
	}
	if got.Status != domain.StatusRefunded {
		t.Errorf("txn status = %q, ingin refunded", got.Status)
	}
	if refunds.applied != 1 || len(refunds.outbox) != 1 {
		t.Errorf("applied=%d outbox=%d, ingin 1/1", refunds.applied, len(refunds.outbox))
	}
}

func TestRefund_PartialSync(t *testing.T) {
	repo := newMockRepo()
	refunds := newMockRefundRepo()
	gw := &mockGateway{refundResult: domain.RefundResult{Status: domain.RefundSucceeded}}
	svc := usecase.NewService(repo, refunds, gw)
	txn := seedPaid(repo, 150000)

	rf, err := svc.Refund(context.Background(), usecase.RefundInput{
		AppID: txn.AppID, TransactionID: txn.ID, AmountMinor: 50000, IdempotencyKey: "rf-2",
	})
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if rf.AmountMinor != 50000 {
		t.Errorf("amount = %d", rf.AmountMinor)
	}
	got := repo.byID[txn.ID]
	if got.RefundedAmount != 50000 {
		t.Errorf("refunded_amount = %d, ingin 50000", got.RefundedAmount)
	}
	if got.Status != domain.StatusPartiallyRefunded {
		t.Errorf("txn status = %q, ingin partially_refunded", got.Status)
	}
}

func TestRefund_ExceedsRefundable(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, newMockRefundRepo(), &mockGateway{})
	txn := seedPaid(repo, 100000)

	_, err := svc.Refund(context.Background(), usecase.RefundInput{
		AppID: txn.AppID, TransactionID: txn.ID, AmountMinor: 150000,
	})
	if !errors.Is(err, domain.ErrRefundExceedsAmount) {
		t.Fatalf("ingin ErrRefundExceedsAmount, dapat %v", err)
	}
}

func TestRefund_NotRefundableState(t *testing.T) {
	repo := newMockRepo()
	svc := usecase.NewService(repo, newMockRefundRepo(), &mockGateway{})
	// transaksi pending → tidak bisa refund.
	txn := &domain.Transaction{ID: uuid.New(), AppID: uuid.New(), Status: domain.StatusPending, GrossAmount: 1000, Currency: "IDR"}
	_ = repo.Create(context.Background(), txn)

	_, err := svc.Refund(context.Background(), usecase.RefundInput{AppID: txn.AppID, TransactionID: txn.ID})
	if !errors.Is(err, domain.ErrNotRefundable) {
		t.Fatalf("ingin ErrNotRefundable, dapat %v", err)
	}
}

func TestRefund_Idempotent(t *testing.T) {
	repo := newMockRepo()
	refunds := newMockRefundRepo()
	gw := &mockGateway{refundResult: domain.RefundResult{Status: domain.RefundSucceeded}}
	svc := usecase.NewService(repo, refunds, gw)
	txn := seedPaid(repo, 150000)

	in := usecase.RefundInput{AppID: txn.AppID, TransactionID: txn.ID, AmountMinor: 50000, IdempotencyKey: "rf-dup"}
	first, err := svc.Refund(context.Background(), in)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.Refund(context.Background(), in)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("idempotent refund menghasilkan id berbeda")
	}
	if gw.refundCalls != 1 {
		t.Errorf("gateway refund dipanggil %d kali, ingin 1", gw.refundCalls)
	}
}

func TestRefund_AsyncPending(t *testing.T) {
	repo := newMockRepo()
	refunds := newMockRefundRepo()
	gw := &mockGateway{refundResult: domain.RefundResult{Status: domain.RefundPending}}
	svc := usecase.NewService(repo, refunds, gw)
	txn := seedPaid(repo, 150000)

	rf, err := svc.Refund(context.Background(), usecase.RefundInput{AppID: txn.AppID, TransactionID: txn.ID, IdempotencyKey: "rf-async"})
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if rf.Status != domain.RefundPending {
		t.Errorf("status = %q, ingin pending (async)", rf.Status)
	}
	// txn belum berubah sampai finalisasi.
	got := repo.byID[txn.ID]
	if got.RefundedAmount != 0 || got.Status != domain.StatusPaid {
		t.Errorf("txn berubah prematur: refunded=%d status=%q", got.RefundedAmount, got.Status)
	}
	if refunds.applied != 0 {
		t.Errorf("ApplyRefundSucceeded tidak boleh dipanggil utk async")
	}
}

func TestRefund_GatewayError(t *testing.T) {
	repo := newMockRepo()
	refunds := newMockRefundRepo()
	gw := &mockGateway{refundErr: errors.New("doku down")}
	svc := usecase.NewService(repo, refunds, gw)
	txn := seedPaid(repo, 150000)

	_, err := svc.Refund(context.Background(), usecase.RefundInput{AppID: txn.AppID, TransactionID: txn.ID, IdempotencyKey: "rf-err"})
	if err == nil {
		t.Fatal("ingin error dari gateway")
	}
	// refund row tercatat sebagai failed.
	rf := refunds.byIdem[txn.AppID.String()+"|rf-err"]
	if rf == nil || rf.Status != domain.RefundFailed {
		t.Errorf("refund harus failed, dapat %+v", rf)
	}
}
