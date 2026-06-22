// Package payment (usecase) berisi application business rules — orchestrasi
// antara domain, repository, dan gateway. Bergantung HANYA pada port domain.
package payment

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

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
// TODO(impl): idempotency lookup, simpan txn (created), panggil gateway.CreateCharge,
// update -> pending, tulis transaction_event, kembalikan resource.
func (s *Service) CreatePayment(ctx context.Context, in CreatePaymentInput) (*domain.Transaction, error) {
	return nil, errors.New("CreatePayment: belum diimplementasikan")
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
