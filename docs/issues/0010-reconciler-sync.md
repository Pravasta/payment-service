# 0010 — Reconciler + `POST /sync` + DOKU GetStatus

- **Status:** done
- **Prioritas:** medium
- **Estimasi:** M
- **Depends on:** 0008
- **Referensi:** detailed-design §4.3, §6.3, doku-integration-spec §5, §6

## Konteks

Jaring pengaman bila webhook hilang/telat. `GetStatus` ke DOKU
(`GET /orders/v1/status/{invoice}`), sadar saran "cek ≥60 detik setelah bayar".

## Scope

- [x] `doku.GetStatus` implementasi (Check Status API + map status canonical).
- [x] Reconciler (`cmd/worker`): untuk `pending` melewati ambang (5 menit)
      → GetStatus → update via `applyDetectedStatus` (atomic, sama spt 0008).
- [x] Yang lewat `expires_at` tanpa bayar → transisi `expired` + event + outbox.
- [x] `POST /v1/payments/{id}/sync`: panggil GetStatus, rate-limited
      (1×/10s per txn), idempoten secara efek.

## Acceptance criteria

- [x] `sync` memperbarui status bila berubah; rate-limit bekerja.
- [x] Reconciler menutup transaksi yang webhook-nya hilang.
- [x] Transisi via reconciler tetap lewat validasi state machine.
- [x] Test map status & guard rate-limit.

## Catatan implementasi

- `postSigned` digeneralisasi jadi `doSigned` (POST/GET) — satu signer.
- `applyDetectedStatus` (polling) berbeda dari webhook: hanya bertindak saat
  transisi legal, tanpa event no-op; reuse `ApplyWebhook` (atomic + outbox).
- Rate-limit `/sync` in-memory per instance (cukup MVP); lintas-instance →
  kolom `last_synced_at`/Redis (follow-up).
- Worker: reconciler ticker 60s berdampingan outbox dispatcher 5s.

## File terkait

- `internal/adapter/gateway/doku/doku.go` (GetStatus)
- `cmd/worker/main.go` (reconciler)
- `internal/adapter/http/handler/payment_handler.go` (Sync)

## Di luar scope

- Rekonsiliasi settlement report (fee/net) — fase berikutnya.
