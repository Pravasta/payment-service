// Package payment (usecase) berisi application business rules — orchestrasi
// antara domain, repository, dan gateway. Bergantung HANYA pada port domain.
package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

const gatewayDOKU = "doku"

// currencyExponent mengembalikan jumlah desimal minor unit untuk currency
// (tabel referensi sederhana). IDR = 0 (rupiah utuh). ok=false bila tak didukung.
func currencyExponent(currency string) (int, bool) {
	switch currency {
	case "IDR":
		return 0, true
	default:
		return 0, false
	}
}

// Service meng-orchestrate use-case pembayaran (CreatePayment, GetPayment, Sync, Refund).
type Service struct {
	repo    domain.Repository
	refunds domain.RefundRepository
	gateway domain.Gateway
}

func NewService(repo domain.Repository, refunds domain.RefundRepository, gw domain.Gateway) *Service {
	return &Service{repo: repo, refunds: refunds, gateway: gw}
}

// CreatePaymentInput adalah input use-case create payment (DTO usecase, bukan HTTP).
type CreatePaymentInput struct {
	AppID             uuid.UUID
	IdempotencyKey    string
	ExternalReference string
	AmountMinor       int64
	Currency          string
	CustomerRef       string
	Description       string
	ExpiryMinutes     int
	ReturnURL         string
	Metadata          map[string]any
}

// CreatePayment membuat transaksi + memanggil gateway checkout (detailed-design §1).
//
// Alur: validasi → idempotency lookup → simpan txn (created) →
// gateway.CreateCharge → update (pending + payment_url/expires_at) →
// tulis transaction_event. Mengembalikan resource transaksi.
func (s *Service) CreatePayment(ctx context.Context, in CreatePaymentInput) (*domain.Transaction, error) {
	// 1. Validasi input.
	if in.AmountMinor <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	currency := in.Currency
	if currency == "" {
		currency = "IDR"
	}
	exponent, ok := currencyExponent(currency)
	if !ok {
		return nil, domain.ErrUnsupportedCurrency
	}
	if in.ExternalReference == "" {
		return nil, fmt.Errorf("%w: external_reference wajib", domain.ErrInvalidAmount)
	}

	// 2. Idempotency: key sama → kembalikan resource yang sudah ada (tanpa charge ulang).
	if in.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, in.AppID, in.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}

	// external_reference UNIQUE per app — tolak duplikat dengan key berbeda.
	if _, err := s.repo.GetByExternalReference(ctx, in.AppID, in.ExternalReference); err == nil {
		return nil, domain.ErrDuplicateReference
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// 3. Simpan transaksi awal (status: created).
	now := time.Now().UTC()
	txn := &domain.Transaction{
		ID:                uuid.New(),
		AppID:             in.AppID,
		ExternalReference: in.ExternalReference,
		IdempotencyKey:    in.IdempotencyKey,
		Status:            domain.StatusCreated,
		Currency:          currency,
		CurrencyExponent:  exponent,
		GrossAmount:       in.AmountMinor,
		Gateway:           gatewayDOKU,
		CustomerRef:       in.CustomerRef,
		Description:       in.Description,
		Metadata:          in.Metadata,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, txn); err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	s.appendEvent(ctx, txn, domain.EventCreated, "", domain.StatusCreated, nil)

	// 4. Panggil gateway (DOKU Checkout) — payment_url sinkron.
	res, err := s.gateway.CreateCharge(ctx, domain.ChargeRequest{
		ExternalReference: in.ExternalReference,
		AmountMinor:       in.AmountMinor,
		Currency:          currency,
		CurrencyExponent:  exponent,
		CustomerRef:       in.CustomerRef,
		Description:       in.Description,
		ExpiryMinutes:     in.ExpiryMinutes,
		ReturnURL:         in.ReturnURL,
		Metadata:          in.Metadata,
	})
	if err != nil {
		// Gateway gagal → transisi created → failed (audit), kembalikan error.
		s.markFailed(ctx, txn)
		return nil, fmt.Errorf("gateway create charge: %w", err)
	}

	// 5. Update transaksi → pending + simpan hasil gateway.
	txn.GatewayTxnID = res.GatewayTxnID
	txn.GatewayRequestID = res.GatewayRequestID
	txn.PaymentURL = res.PaymentURL
	txn.ExpiresAt = res.ExpiresAt
	txn.Status = domain.StatusPending
	txn.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, txn); err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}
	s.appendEvent(ctx, txn, domain.EventPending, domain.StatusCreated, domain.StatusPending, nil)

	return txn, nil
}

// markFailed mentransisikan transaksi ke failed (best-effort audit saat gateway error).
func (s *Service) markFailed(ctx context.Context, txn *domain.Transaction) {
	if !domain.CanTransition(txn.Status, domain.StatusFailed) {
		return
	}
	from := txn.Status
	now := time.Now().UTC()
	txn.Status = domain.StatusFailed
	txn.FailedAt = &now
	txn.UpdatedAt = now
	_ = s.repo.Update(ctx, txn)
	s.appendEvent(ctx, txn, domain.EventFailed, from, domain.StatusFailed, nil)
}

// appendEvent menulis transaction_event (append-only, audit trail). Best-effort:
// kegagalan menulis event tidak membatalkan operasi utama (di-recover reconciler).
func (s *Service) appendEvent(ctx context.Context, txn *domain.Transaction, t domain.EventType, from, to domain.Status, payload map[string]any) {
	now := time.Now().UTC()
	_ = s.repo.AppendEvent(ctx, &domain.TransactionEvent{
		ID:            uuid.New(),
		TransactionID: txn.ID,
		AppID:         txn.AppID,
		EventType:     t,
		FromStatus:    from,
		ToStatus:      to,
		AmountMinor:   txn.GrossAmount,
		Source:        "api",
		Payload:       payload,
		OccurredAt:    now,
		CreatedAt:     now,
	})
}

// GetPayment mengembalikan transaksi (read murni). TODO(impl).
func (s *Service) GetPayment(ctx context.Context, appID, id uuid.UUID) (*domain.Transaction, error) {
	return nil, errors.New("GetPayment: belum diimplementasikan")
}

// SyncPayment memaksa refresh status dari gateway (rate-limited). TODO(impl).
func (s *Service) SyncPayment(ctx context.Context, appID, id uuid.UUID) (*domain.Transaction, error) {
	return nil, errors.New("SyncPayment: belum diimplementasikan")
}

// RefundInput adalah input use-case refund.
type RefundInput struct {
	AppID          uuid.UUID
	TransactionID  uuid.UUID
	IdempotencyKey string
	AmountMinor    int64 // 0 = full refund
	Reason         string
}

// Refund membuat & mengeksekusi refund (detailed-design §8). TODO(impl).
func (s *Service) Refund(ctx context.Context, in RefundInput) (*domain.Refund, error) {
	return nil, errors.New("Refund: belum diimplementasikan")
}
