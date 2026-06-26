package payment

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ListFilter adalah kriteria query list transaksi (detailed-design §4.5).
// Selalu ter-scope per AppID. Cursor (CursorCreated+CursorID) mengikuti urutan
// (created_at DESC, id DESC) untuk pagination yang stabil.
type ListFilter struct {
	AppID         uuid.UUID
	Status        Status     // "" = semua status
	From          *time.Time // created_at >= From
	To            *time.Time // created_at <= To
	Limit         int        // jumlah baris yang diminta
	CursorCreated *time.Time // ambil yang lebih lama dari cursor ini
	CursorID      *uuid.UUID
}

// Repository adalah port persistensi (diimplementasi di adapter/repository pakai GORM).
// Domain & usecase hanya tahu interface ini, bukan GORM.
type Repository interface {
	Create(ctx context.Context, txn *Transaction) error
	Update(ctx context.Context, txn *Transaction) error
	GetByID(ctx context.Context, appID, id uuid.UUID) (*Transaction, error)
	GetByExternalReference(ctx context.Context, appID uuid.UUID, ref string) (*Transaction, error)
	GetByIdempotencyKey(ctx context.Context, appID uuid.UUID, key string) (*Transaction, error)
	GetByGatewayTxnID(ctx context.Context, gateway, gatewayTxnID string) (*Transaction, error)

	// ListTransactions mengembalikan transaksi sesuai filter, urut created_at DESC,
	// id DESC. Mengembalikan tepat sebanyak ListFilter.Limit (caller meminta +1
	// untuk mendeteksi adanya halaman berikutnya).
	ListTransactions(ctx context.Context, f ListFilter) ([]*Transaction, error)

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
