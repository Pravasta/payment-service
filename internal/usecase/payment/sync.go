package payment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// reconcileThreshold: transaksi pending lebih tua dari ini menjadi kandidat
// reconciler (detailed-design §6.3).
const reconcileThreshold = 5 * time.Minute

// rateLimiter membatasi aksi per-key (mis. /sync per transaksi) dengan jendela
// waktu minimum. In-memory, cukup untuk MVP single-instance.
type rateLimiter struct {
	mu   sync.Mutex
	last map[uuid.UUID]time.Time
	min  time.Duration
	now  func() time.Time
}

func newRateLimiter(min time.Duration) *rateLimiter {
	return &rateLimiter{last: make(map[uuid.UUID]time.Time), min: min, now: time.Now}
}

// allow melaporkan apakah aksi untuk key boleh berjalan; bila ya, catat waktunya.
func (l *rateLimiter) allow(key uuid.UUID) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if t, ok := l.last[key]; ok && now.Sub(t) < l.min {
		return false
	}
	l.last[key] = now
	return true
}

// SyncPayment memaksa refresh status dari gateway (detailed-design §4.3).
// Rate-limited per transaksi; aman dipanggil berulang (hanya menulis event/outbox
// bila ada perubahan status). Mengembalikan transaksi terkini.
func (s *Service) SyncPayment(ctx context.Context, appID, id uuid.UUID) (*domain.Transaction, error) {
	txn, err := s.repo.GetByID(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	// Status terminal (failed/expired/refunded) tidak bisa berubah lagi → tak perlu
	// memanggil gateway. Kembalikan transaksi apa adanya (no-op idempoten); ini
	// mencegah error gateway/transisi pada transaksi final berubah jadi 500.
	if domain.IsTerminal(txn.Status) {
		return txn, nil
	}
	if !s.syncLimiter.allow(id) {
		return nil, domain.ErrRateLimited
	}

	if _, err := s.reconcileTransaction(ctx, txn, "sync"); err != nil {
		return nil, err
	}
	return txn, nil
}

// ReconcilePending menjalankan reconciler (detailed-design §6.3): untuk transaksi
// pending yang sudah melewati ambang, tanyakan status ke gateway dan terapkan
// perubahan (termasuk expired bila lewat expires_at). Mengembalikan jumlah yang
// statusnya berubah.
func (s *Service) ReconcilePending(ctx context.Context, limit int) (int, error) {
	cutoff := time.Now().UTC().Add(-reconcileThreshold)
	txns, err := s.repo.ListPendingForReconcile(ctx, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("reconcile: list pending: %w", err)
	}
	changed := 0
	for _, txn := range txns {
		ok, err := s.reconcileTransaction(ctx, txn, "reconciler")
		if err != nil {
			// jangan hentikan batch karena satu transaksi gagal.
			continue
		}
		if ok {
			changed++
		}
	}
	return changed, nil
}

// reconcileTransaction menanyakan status ke gateway lalu menerapkan transisi.
// source = "sync" | "reconciler". Bila gateway tetap pending tetapi sudah lewat
// expires_at → paksa expired (state machine tetap divalidasi).
func (s *Service) reconcileTransaction(ctx context.Context, txn *domain.Transaction, source string) (bool, error) {
	res, err := s.gateway.GetStatus(ctx, domain.StatusRef{
		ExternalReference: txn.ExternalReference,
		GatewayRequestID:  txn.GatewayRequestID,
	})
	if err != nil {
		return false, fmt.Errorf("gateway get status: %w", err)
	}

	newStatus := res.Status
	if newStatus == domain.StatusPending && txn.ExpiresAt != nil && time.Now().UTC().After(*txn.ExpiresAt) {
		newStatus = domain.StatusExpired
	}

	return s.applyDetectedStatus(ctx, txn, newStatus, res.PaymentMethod, res.AmountMinor, source, res.Raw)
}
