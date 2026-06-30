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

func seedPendingOld(repo *mockRepo, reqID string, createdAt time.Time, expiresAt *time.Time) *domain.Transaction {
	txn := &domain.Transaction{
		ID:                uuid.New(),
		AppID:             uuid.New(),
		ExternalReference: "INV-SYNC-" + reqID,
		Status:            domain.StatusPending,
		Currency:          "IDR",
		GrossAmount:       150000,
		Gateway:           "doku",
		GatewayRequestID:  reqID,
		ExpiresAt:         expiresAt,
		CreatedAt:         createdAt,
	}
	_ = repo.Create(context.Background(), txn)
	return txn
}

func TestSyncPayment_UpdatesStatus(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{statusResult: domain.StatusResult{
		Status:        domain.StatusPaid,
		PaymentMethod: "VIRTUAL_ACCOUNT_BCA",
		AmountMinor:   150000,
	}}
	svc := usecase.NewService(repo, nil, gw)
	txn := seedPendingOld(repo, "req-1", time.Now().UTC(), nil)

	got, err := svc.SyncPayment(context.Background(), txn.AppID, txn.ID)
	if err != nil {
		t.Fatalf("SyncPayment: %v", err)
	}
	if got.Status != domain.StatusPaid {
		t.Errorf("status = %q, ingin paid", got.Status)
	}
	if gw.statusCalls != 1 {
		t.Errorf("GetStatus dipanggil %d kali, ingin 1", gw.statusCalls)
	}
	if len(repo.outbox) != 1 {
		t.Errorf("outbox = %d, ingin 1 (callback)", len(repo.outbox))
	}
}

func TestSyncPayment_RateLimited(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{statusResult: domain.StatusResult{Status: domain.StatusPending}}
	svc := usecase.NewService(repo, nil, gw)
	txn := seedPendingOld(repo, "req-2", time.Now().UTC(), nil)

	if _, err := svc.SyncPayment(context.Background(), txn.AppID, txn.ID); err != nil {
		t.Fatalf("sync pertama: %v", err)
	}
	// Panggilan kedua langsung → terkena rate-limit.
	_, err := svc.SyncPayment(context.Background(), txn.AppID, txn.ID)
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("ingin ErrRateLimited, dapat %v", err)
	}
}

func TestSyncPayment_NotFound(t *testing.T) {
	svc := usecase.NewService(newMockRepo(), nil, &mockGateway{})
	if _, err := svc.SyncPayment(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ingin ErrNotFound, dapat %v", err)
	}
}

func TestSyncPayment_TerminalSkipsGateway(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{statusResult: domain.StatusResult{Status: domain.StatusPaid}}
	svc := usecase.NewService(repo, nil, gw)
	// Transaksi terminal (failed) — sync tak boleh memanggil gateway.
	txn := &domain.Transaction{
		ID:                uuid.New(),
		AppID:             uuid.New(),
		ExternalReference: "INV-TERM-1",
		Status:            domain.StatusFailed,
		Currency:          "IDR",
		GrossAmount:       150000,
		Gateway:           "doku",
		CreatedAt:         time.Now().UTC(),
	}
	_ = repo.Create(context.Background(), txn)

	got, err := svc.SyncPayment(context.Background(), txn.AppID, txn.ID)
	if err != nil {
		t.Fatalf("SyncPayment (terminal): %v", err)
	}
	if got.Status != domain.StatusFailed {
		t.Errorf("status = %q, ingin tetap failed", got.Status)
	}
	if gw.statusCalls != 0 {
		t.Errorf("GetStatus dipanggil %d kali, ingin 0 (terminal di-skip)", gw.statusCalls)
	}
	if len(repo.events) != 0 {
		t.Errorf("tidak boleh ada event untuk no-op terminal, ada %d", len(repo.events))
	}
}

func TestReconcilePending_ClosesPaid(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{statusResult: domain.StatusResult{Status: domain.StatusPaid, AmountMinor: 150000}}
	svc := usecase.NewService(repo, nil, gw)
	old := time.Now().UTC().Add(-10 * time.Minute)
	txn := seedPendingOld(repo, "req-3", old, nil)

	changed, err := svc.ReconcilePending(context.Background(), 100)
	if err != nil {
		t.Fatalf("ReconcilePending: %v", err)
	}
	if changed != 1 {
		t.Errorf("changed = %d, ingin 1", changed)
	}
	if repo.byID[txn.ID].Status != domain.StatusPaid {
		t.Errorf("status = %q, ingin paid", repo.byID[txn.ID].Status)
	}
}

func TestReconcilePending_ExpiresPastExpiry(t *testing.T) {
	repo := newMockRepo()
	// Gateway masih pending, tapi sudah lewat expires_at → harus expired.
	gw := &mockGateway{statusResult: domain.StatusResult{Status: domain.StatusPending}}
	svc := usecase.NewService(repo, nil, gw)
	old := time.Now().UTC().Add(-10 * time.Minute)
	past := time.Now().UTC().Add(-1 * time.Minute)
	txn := seedPendingOld(repo, "req-4", old, &past)

	changed, err := svc.ReconcilePending(context.Background(), 100)
	if err != nil {
		t.Fatalf("ReconcilePending: %v", err)
	}
	if changed != 1 {
		t.Errorf("changed = %d, ingin 1", changed)
	}
	if repo.byID[txn.ID].Status != domain.StatusExpired {
		t.Errorf("status = %q, ingin expired", repo.byID[txn.ID].Status)
	}
}

func TestReconcilePending_NoChangeWhenStillPending(t *testing.T) {
	repo := newMockRepo()
	gw := &mockGateway{statusResult: domain.StatusResult{Status: domain.StatusPending}}
	svc := usecase.NewService(repo, nil, gw)
	old := time.Now().UTC().Add(-10 * time.Minute)
	future := time.Now().UTC().Add(30 * time.Minute)
	seedPendingOld(repo, "req-5", old, &future) // belum expired, gateway pending

	changed, err := svc.ReconcilePending(context.Background(), 100)
	if err != nil {
		t.Fatalf("ReconcilePending: %v", err)
	}
	if changed != 0 {
		t.Errorf("changed = %d, ingin 0 (tetap pending)", changed)
	}
	if len(repo.events) != 0 {
		t.Errorf("tidak boleh ada event saat tanpa perubahan, ada %d", len(repo.events))
	}
}
