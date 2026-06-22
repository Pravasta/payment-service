# 0006 — Use-case CreatePayment + `POST /v1/payments`

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0003, 0004, 0005
- **Referensi:** detailed-design §1, §4.1

## Konteks

Menyatukan alur create: validasi → idempotency → simpan txn (`created`) → panggil
`gateway.CreateCharge` → update `pending` + simpan `payment_url`/`expires_at` →
tulis `transaction_event` → balikan resource.

## Scope

- [ ] `usecase.CreatePayment` implementasi penuh (gantikan TODO).
- [ ] Tetapkan `currency_exponent` dari tabel referensi (IDR=0); default IDR.
- [ ] Handler `POST /v1/payments`: parse DTO, panggil use-case, map ke response
      `201` (id, status, payment_url, expires_at, ...).
- [ ] Idempotency: key sama → balikan resource sama (200) tanpa charge ulang.
- [ ] Tulis event `payment.created` lalu `payment.pending`.

## Acceptance criteria

- [ ] Request valid → `201` + `payment_url`, status `pending`, row di DB.
- [ ] Retry idempotency-key sama → resource sama, tidak ada txn/charge dobel.
- [ ] `external_reference` dobel per app → ditolak (`409`).
- [ ] Test use-case dengan gateway & repo di-mock.

## File terkait

- `internal/usecase/payment/service.go` (CreatePayment)
- `internal/adapter/http/handler/payment_handler.go` (Create)
- `internal/adapter/http/dto/` (request/response) — baru

## Di luar scope

- Webhook & callback (issue 0008/0009).
