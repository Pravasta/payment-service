# 0010 — Reconciler + `POST /sync` + DOKU GetStatus

- **Status:** todo
- **Prioritas:** medium
- **Estimasi:** M
- **Depends on:** 0008
- **Referensi:** detailed-design §4.3, §6.3, doku-integration-spec §5, §6

## Konteks

Jaring pengaman bila webhook hilang/telat. `GetStatus` ke DOKU
(`GET /orders/v1/status/{invoice}`), sadar saran "cek ≥60 detik setelah bayar".

## Scope

- [ ] `doku.GetStatus` implementasi (Check Status API + map status canonical).
- [ ] Reconciler (`cmd/worker`): untuk `pending` melewati ambang (mis. 5 menit)
      & belum `expires_at` → GetStatus → update via jalur yang sama (issue 0008).
- [ ] Yang lewat `expires_at` tanpa bayar → transisi `expired` + event + outbox.
- [ ] `POST /v1/payments/{id}/sync`: panggil GetStatus, rate-limited
      (mis. 1×/10s per txn), idempoten secara efek.

## Acceptance criteria

- [ ] `sync` memperbarui status bila berubah; rate-limit bekerja.
- [ ] Reconciler menutup transaksi yang webhook-nya hilang.
- [ ] Transisi via reconciler tetap lewat validasi state machine.
- [ ] Test map status & guard rate-limit.

## File terkait

- `internal/adapter/gateway/doku/doku.go` (GetStatus)
- `cmd/worker/main.go` (reconciler)
- `internal/adapter/http/handler/payment_handler.go` (Sync)

## Di luar scope

- Rekonsiliasi settlement report (fee/net) — fase berikutnya.
