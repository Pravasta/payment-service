# 0009 — Outbox worker (callback ke app, retry/backoff/DLQ)

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0008
- **Referensi:** detailed-design §4.7, §6.2
- **Triase rombakan v2:** berlaku — dipindah ke `service/notification` di Phase 1
- **Alasan triase:** Kontrak retry/DLQ-nya justru dikunci jadi keputusan resmi di ADR-0007.

## Konteks

`notification_outbox` menjamin callback ke app at-least-once meski app sedang
down. Worker mengirim HTTP bertanda tangan dengan retry & dead-letter.

## Scope

- [x] Worker (`cmd/worker`) loop: ambil `notification_outbox`
      `status=pending AND next_retry_at<=now`.
- [x] Kirim POST ke `webhook_endpoint.url` dengan `X-Signature` (HMAC pakai
      `signing_secret` per merchant, di-decrypt AES-GCM) + `X-Event-Id`.
- [x] Sukses `2xx` → `delivered`. Gagal → `attempts++`, `next_retry_at` dengan
      exponential backoff + jitter.
- [x] Lewati batas attempt → `dead`. Endpoint admin replay dead-letter
      (`POST /v1/admin/outbox/{id}/replay`).

## Acceptance criteria

- [x] Callback terkirim & ditandai `delivered`; payload sesuai §4.7.
- [x] App down → retry sesuai backoff; tidak hilang.
- [x] Melewati batas → `dead`, bisa di-replay.
- [x] Signature callback bisa diverifikasi penerima (test).

## Catatan implementasi

- Paket `internal/outbox` (Dispatcher + port Store); repo `OutboxRepository`.
- Signature callback: `base64(HMAC-SHA256(signing_secret, raw_body))`.
- MVP single worker (FetchDue tanpa row-lock); multi-worker → tambah
  `SELECT ... FOR UPDATE SKIP LOCKED` (follow-up).
- Admin replay pakai auth merchant + scope `outbox:admin` (admin API khusus
  belum didesain).

## File terkait

- `cmd/worker/main.go`
- `internal/usecase/` atau `internal/outbox/` (dispatcher)
- `internal/adapter/repository/` (outbox queries)

## Di luar scope

- Message broker (ditunda; outbox cukup untuk MVP).
