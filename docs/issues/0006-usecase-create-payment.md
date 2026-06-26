# 0006 — Use-case CreatePayment + `POST /v1/payments`

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0003, 0004, 0005
- **Referensi:** detailed-design §1, §4.1

## Konteks

Menyatukan alur create: validasi → idempotency → simpan txn (`created`) → panggil
`gateway.CreateCharge` → update `pending` + simpan `payment_url`/`expires_at` →
tulis `transaction_event` → balikan resource.

## Scope

- [x] `usecase.CreatePayment` implementasi penuh (gantikan TODO).
- [x] Tetapkan `currency_exponent` dari tabel referensi (IDR=0); default IDR.
- [x] Handler `POST /v1/payments`: parse DTO, panggil use-case, map ke response
      `201` (id, status, payment_url, expires_at, ...).
- [x] Idempotency: key sama → balikan resource sama tanpa charge ulang.
- [x] Tulis event `payment.created` lalu `payment.pending`.

## Acceptance criteria

- [x] Request valid → `201` + `payment_url`, status `pending`, row di DB.
- [x] Retry idempotency-key sama → resource sama, tidak ada txn/charge dobel.
- [x] `external_reference` dobel per app → ditolak (`409`).
- [x] Test use-case dengan gateway & repo di-mock (6 test).

## Catatan implementasi

- Idempotency dijaga dua lapis: middleware HTTP (issue 0004) + lookup
  `GetByIdempotencyKey` di usecase (safety net + diuji unit). external_reference
  duplikat (key beda) → `ErrDuplicateReference` (409).
- Gateway error → transisi `created → failed` (audit) lalu error dikembalikan.
- `transaction_event` ditulis best-effort (append-only); atomicity event+txn
  dalam satu DB-tx ditunda ke jalur webhook (issue 0008) yang membutuhkannya.
- Smoke test live ditunda: credential DOKU sandbox masih ditolak (spec §9.5).

## File terkait

- `internal/usecase/payment/service.go` (CreatePayment)
- `internal/adapter/http/handler/payment_handler.go` (Create)
- `internal/adapter/http/dto/` (request/response) — baru

## Di luar scope

- Webhook & callback (issue 0008/0009).
