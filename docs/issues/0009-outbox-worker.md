# 0009 — Outbox worker (callback ke app, retry/backoff/DLQ)

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0008
- **Referensi:** detailed-design §4.7, §6.2

## Konteks

`notification_outbox` menjamin callback ke app at-least-once meski app sedang
down. Worker mengirim HTTP bertanda tangan dengan retry & dead-letter.

## Scope

- [ ] Worker (`cmd/worker`) loop: ambil `notification_outbox`
      `status=pending AND next_retry_at<=now`.
- [ ] Kirim POST ke `webhook_endpoint.url` dengan `X-Signature` (HMAC pakai
      `signing_secret` per merchant) + `X-Event-Id`.
- [ ] Sukses `2xx` → `delivered`. Gagal → `attempts++`, `next_retry_at` dengan
      exponential backoff + jitter.
- [ ] Lewati batas attempt → `dead` + alert. Endpoint admin "replay" dead-letter.

## Acceptance criteria

- [ ] Callback terkirim & ditandai `delivered`; payload sesuai §4.7.
- [ ] App down → retry sesuai backoff; tidak hilang.
- [ ] Melewati batas → `dead`, bisa di-replay.
- [ ] Signature callback bisa diverifikasi penerima (test).

## File terkait

- `cmd/worker/main.go`
- `internal/usecase/` atau `internal/outbox/` (dispatcher)
- `internal/adapter/repository/` (outbox queries)

## Di luar scope

- Message broker (ditunda; outbox cukup untuk MVP).
