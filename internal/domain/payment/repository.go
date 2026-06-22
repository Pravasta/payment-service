package payment

import (
	"context"

	"github.com/google/uuid"
)

// Repository adalah port persistensi (diimplementasi di adapter/repository pakai GORM).
// Domain & usecase hanya tahu interface ini, bukan GORM.
type Repository interface {
	Create(ctx context.Context, txn *Transaction) error
	Update(ctx context.Context, txn *Transaction) error
	GetByID(ctx context.Context, appID, id uuid.UUID) (*Transaction, error)
	GetByExternalReference(ctx context.Context, appID uuid.UUID, ref string) (*Transaction, error)
	GetByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*Transaction, error)
	GetByGatewayTxnID(ctx context.Context, gateway, gatewayTxnID string) (*Transaction, error)

	// AppendEvent menulis transaction_event (append-only). Idealnya dipanggil
	// dalam transaksi DB yang sama dengan Update (lihat detailed-design §6.1).
	AppendEvent(ctx context.Context, e *TransactionEvent) error
}

// RefundRepository adalah port persistensi untuk refund.
type RefundRepository interface {
	Create(ctx context.Context, r *Refund) error
	Update(ctx context.Context, r *Refund) error
	GetByID(ctx context.Context, appID, id uuid.UUID) (*Refund, error)
	GetByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*Refund, error)
}
