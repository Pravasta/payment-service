package payment

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// HandleDOKUWebhook memproses notifikasi DOKU end-to-end (detailed-design §6.1):
//
//  1. simpan webhook_inbox (raw) SEBELUM verifikasi
//  2. verifikasi signature + parse (gateway.ParseWebhook)
//  3. match transaction (gateway_request_id / gateway_txn_id)
//  4. dedup via gateway_event_id
//  5. dalam satu transaksi DB: update txn + transaction_event + outbox
//
// Mengembalikan domain.ErrInvalidSignature bila signature tidak valid (→ 401).
// Untuk kasus lain yang "aman di-ack" (txn tak dikenal, duplikat, transisi ilegal)
// mengembalikan nil agar handler membalas 2xx dan DOKU berhenti retry.
func (s *Service) HandleDOKUWebhook(ctx context.Context, raw domain.WebhookPayload) error {
	// 1. Simpan inbox apa adanya (audit/replay) sebelum verifikasi.
	inbox := &domain.WebhookInboxRecord{
		Gateway:    gatewayDOKU,
		Signature:  raw.Headers["Signature"],
		Headers:    headersToMap(raw.Headers),
		RawBody:    raw.RawBody,
		ReceivedAt: time.Now().UTC(),
	}
	_ = s.repo.SaveWebhookInbox(ctx, inbox)

	// 2. Verifikasi signature + parse body.
	evt, err := s.gateway.ParseWebhook(ctx, raw)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidSignature) {
			inbox.Verified = false
			_ = s.repo.UpdateWebhookInbox(ctx, inbox)
		}
		return err
	}
	inbox.Verified = true

	// 3. Match transaksi.
	txn, err := s.matchTransaction(ctx, evt)
	if err != nil {
		inbox.Processed = true
		_ = s.repo.UpdateWebhookInbox(ctx, inbox)
		if errors.Is(err, domain.ErrNotFound) {
			return nil // txn tak dikenal → ack, hindari retry tak berujung
		}
		return err
	}
	inbox.TransactionID = &txn.ID

	// 4. Dedup: gateway_event_id sudah pernah dicatat → ack & skip.
	if evt.GatewayEventID != "" {
		exists, err := s.repo.EventExists(ctx, evt.GatewayEventID)
		if err != nil {
			return err
		}
		if exists {
			inbox.Processed = true
			_ = s.repo.UpdateWebhookInbox(ctx, inbox)
			return nil
		}
	}

	// 5. Terapkan transisi (atomic).
	if err := s.applyWebhookTransition(ctx, txn, evt); err != nil {
		return err
	}
	inbox.Processed = true
	_ = s.repo.UpdateWebhookInbox(ctx, inbox)
	return nil
}

// matchTransaction mencari transaksi target: prioritas gateway_request_id
// (== DOKU original_request_id, globally unique), fallback gateway_txn_id.
func (s *Service) matchTransaction(ctx context.Context, evt domain.WebhookEvent) (*domain.Transaction, error) {
	if evt.OriginalRequestID != "" {
		t, err := s.repo.GetByGatewayRequestID(ctx, gatewayDOKU, evt.OriginalRequestID)
		if err == nil {
			return t, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	if evt.GatewayTxnID != "" {
		t, err := s.repo.GetByGatewayTxnID(ctx, gatewayDOKU, evt.GatewayTxnID)
		if err == nil {
			return t, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	return nil, domain.ErrNotFound
}

// applyWebhookTransition menulis perubahan (atomic). Transisi legal → update status
// + outbox; transisi ilegal/out-of-order/no-op → catat event audit (dedup) tanpa
// mengubah status & tanpa outbox (tetap di-ack).
func (s *Service) applyWebhookTransition(ctx context.Context, txn *domain.Transaction, evt domain.WebhookEvent) error {
	from := txn.Status
	legal := evt.Status != "" && evt.Status != from && domain.CanTransition(from, evt.Status)

	event := &domain.TransactionEvent{
		ID:             uuid.New(),
		TransactionID:  txn.ID,
		AppID:          txn.AppID,
		EventType:      eventTypeForStatus(evt.Status),
		FromStatus:     from,
		AmountMinor:    evt.AmountMinor,
		Source:         "webhook",
		GatewayEventID: evt.GatewayEventID,
		Payload:        evt.Raw,
		OccurredAt:     evt.OccurredAt,
		CreatedAt:      time.Now().UTC(),
	}

	if !legal {
		event.ToStatus = from // status tidak berubah
		if event.Payload == nil {
			event.Payload = map[string]any{}
		}
		event.Payload["observed_status"] = string(evt.Status)
		event.Payload["applied"] = false
		return s.repo.ApplyWebhook(ctx, txn, event, nil)
	}

	applyStatusFields(txn, evt.Status, evt.PaymentMethod, evt.OccurredAt)
	txn.Status = evt.Status
	txn.UpdatedAt = time.Now().UTC()
	event.ToStatus = evt.Status

	return s.repo.ApplyWebhook(ctx, txn, event, newOutbox(txn, event))
}

// applyDetectedStatus menerapkan status yang TERDETEKSI dari polling (sync /
// reconciler). Berbeda dengan webhook: hanya bertindak bila ada transisi legal
// (tanpa mencatat event no-op), dan tidak melakukan dedup gateway_event_id.
// Mengembalikan changed=true bila status berubah.
func (s *Service) applyDetectedStatus(ctx context.Context, txn *domain.Transaction, newStatus domain.Status, paymentMethod string, amountMinor int64, source string, raw map[string]any) (bool, error) {
	if newStatus == "" || newStatus == txn.Status || !domain.CanTransition(txn.Status, newStatus) {
		return false, nil
	}
	from := txn.Status
	now := time.Now().UTC()
	applyStatusFields(txn, newStatus, paymentMethod, now)
	txn.Status = newStatus
	txn.UpdatedAt = now

	event := &domain.TransactionEvent{
		ID:            uuid.New(),
		TransactionID: txn.ID,
		AppID:         txn.AppID,
		EventType:     eventTypeForStatus(newStatus),
		FromStatus:    from,
		ToStatus:      newStatus,
		AmountMinor:   amountMinor,
		Source:        source,
		Payload:       raw,
		OccurredAt:    now,
		CreatedAt:     now,
	}
	return true, s.repo.ApplyWebhook(ctx, txn, event, newOutbox(txn, event))
}

// newOutbox membangun pesan outbox callback dari transaksi & event.
func newOutbox(txn *domain.Transaction, event *domain.TransactionEvent) *domain.OutboxMessage {
	return &domain.OutboxMessage{
		ID:            uuid.New(),
		AppID:         txn.AppID,
		TransactionID: txn.ID,
		EventID:       "evt_" + uuid.NewString(),
		EventType:     string(event.EventType),
		Payload:       buildCallbackPayload(txn),
	}
}

// applyStatusFields mengisi timestamp/field turunan sesuai status baru.
func applyStatusFields(txn *domain.Transaction, newStatus domain.Status, paymentMethod string, occurredAt time.Time) {
	now := occurredAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	switch newStatus {
	case domain.StatusPaid:
		txn.PaidAt = &now
		if paymentMethod != "" {
			txn.PaymentMethod = paymentMethod
		}
	case domain.StatusFailed:
		txn.FailedAt = &now
	case domain.StatusExpired:
		txn.ExpiredAt = &now
	case domain.StatusSettled:
		txn.SettledAt = &now
	case domain.StatusRefunded:
		txn.RefundedAmount = txn.GrossAmount
	}
}

// buildCallbackPayload menyusun body callback PS → app (detailed-design §4.7).
func buildCallbackPayload(txn *domain.Transaction) map[string]any {
	payment := map[string]any{
		"id":                 txn.ID.String(),
		"external_reference": txn.ExternalReference,
		"status":             string(txn.Status),
		"amount":             txn.GrossAmount,
		"currency":           txn.Currency,
	}
	if txn.PaidAt != nil {
		payment["paid_at"] = txn.PaidAt.UTC().Format(time.RFC3339)
	}
	return map[string]any{"payment": payment}
}

// eventTypeForStatus memetakan status canonical → tipe event.
func eventTypeForStatus(s domain.Status) domain.EventType {
	switch s {
	case domain.StatusPaid:
		return domain.EventPaid
	case domain.StatusFailed:
		return domain.EventFailed
	case domain.StatusExpired:
		return domain.EventExpired
	case domain.StatusRefunded:
		return domain.EventRefunded
	case domain.StatusPartiallyRefunded:
		return domain.EventPartiallyRefunded
	default:
		return domain.EventPending
	}
}

func headersToMap(h map[string]string) map[string]any {
	out := make(map[string]any, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}
