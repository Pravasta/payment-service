# 0008 — Webhook receiver DOKU + ParseWebhook + state machine

- **Status:** done
- **Prioritas:** high
- **Estimasi:** L
- **Depends on:** 0006
- **Referensi:** detailed-design §6.1, doku-integration-spec §1, §5, §9
- **Triase rombakan v2:** berlaku — dipindah di Phase 1, diperluas di Phase 4
- **Alasan triase:** Verifikasi signature dan state machine tetap sama; yang berubah hanya lokasi paket dan gaya handler.

## Konteks

Inti keandalan: terima notifikasi DOKU, verifikasi signature, dedup, terapkan
transisi state machine, dan tulis outbox — semua idempoten & tahan out-of-order.

## Scope

- [x] `POST /v1/webhooks/doku`: simpan `webhook_inbox` (raw + headers + signature)
      **sebelum** verifikasi.
- [x] `doku.ParseWebhook`: verifikasi signature (`VerifySignature`), parse body,
      `mapStatus`, isi `PaymentMethod` dari `channel.id`, `GatewayEventID`←`Request-Id`.
- [x] Dedup `gateway_event_id` di `transaction_event` → ack & skip bila dobel.
- [x] Dalam satu transaksi DB: validasi `CanTransition` → update `transaction` →
      insert `transaction_event` → insert `notification_outbox` (`ApplyWebhook`).
- [x] Transisi ilegal (mis. `expired` setelah `paid`) → di-log sebagai event audit,
      status tak berubah; tetap ack `2xx`.

## Acceptance criteria

- [x] Signature invalid → `401`, `verified=false`, tidak mengubah status.
- [x] Webhook dobel (Request-Id sama) → `2xx`, tanpa side-effect ganda.
- [x] Webhook out-of-order → status final tidak rusak.
- [x] `payment.paid` valid → status `paid`, event + outbox tertulis.
- [x] Test untuk dedup, transisi legal/ilegal, signature.

## Catatan implementasi

- Port `Repository` diperluas (bukan dependency baru) agar `Service`/wiring
  tak berubah; `ApplyWebhook` menulis txn+event+outbox dalam satu `db.Transaction`.
- Match transaksi: prioritas `gateway_request_id` (== DOKU `original_request_id`,
  globally unique) lalu `gateway_txn_id` — hindari ambiguitas `external_reference`
  yang hanya unik per app.
- Txn tak dikenal / duplikat / transisi ilegal → tetap ack 2xx (cegah retry DOKU
  tak berujung); transisi ilegal dicatat sebagai event audit (`applied:false`).
- Pengiriman callback (outbox worker) = issue 0009.

## File terkait

- `internal/adapter/http/handler/payment_handler.go` (WebhookDOKU)
- `internal/adapter/gateway/doku/doku.go` (ParseWebhook)
- `internal/usecase/payment/` (apply webhook event, transaksi DB)
- `internal/adapter/repository/` (webhook_inbox, outbox, event)

## Di luar scope

- Pengiriman callback ke app (issue 0009).
