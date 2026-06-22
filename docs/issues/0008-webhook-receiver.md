# 0008 — Webhook receiver DOKU + ParseWebhook + state machine

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** L
- **Depends on:** 0006
- **Referensi:** detailed-design §6.1, doku-integration-spec §1, §5, §9

## Konteks

Inti keandalan: terima notifikasi DOKU, verifikasi signature, dedup, terapkan
transisi state machine, dan tulis outbox — semua idempoten & tahan out-of-order.

## Scope

- [ ] `POST /v1/webhooks/doku`: simpan `webhook_inbox` (raw + headers + signature)
      **sebelum** verifikasi.
- [ ] `doku.ParseWebhook`: verifikasi signature (`VerifySignature`), parse body,
      `mapStatus`, isi `PaymentMethod` dari `channel.id`, `GatewayEventID`←`Request-Id`.
- [ ] Dedup `gateway_event_id` di `transaction_event` → ack & skip bila dobel.
- [ ] Dalam satu transaksi DB: validasi `CanTransition` → update `transaction` →
      insert `transaction_event` → insert `notification_outbox`.
- [ ] Transisi ilegal (mis. `expired` setelah `paid`) → di-log sebagai event,
      status tak berubah; tetap ack `2xx`.

## Acceptance criteria

- [ ] Signature invalid → `401`, `verified=false`, tidak mengubah status.
- [ ] Webhook dobel (Request-Id sama) → `2xx`, tanpa side-effect ganda.
- [ ] Webhook out-of-order → status final tidak rusak.
- [ ] `payment.paid` valid → status `paid`, event + outbox tertulis.
- [ ] Test untuk dedup, transisi legal/ilegal, signature.

## File terkait

- `internal/adapter/http/handler/payment_handler.go` (WebhookDOKU)
- `internal/adapter/gateway/doku/doku.go` (ParseWebhook)
- `internal/usecase/payment/` (apply webhook event, transaksi DB)
- `internal/adapter/repository/` (webhook_inbox, outbox, event)

## Di luar scope

- Pengiriman callback ke app (issue 0009).
