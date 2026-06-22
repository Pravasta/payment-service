# 0004 — Idempotency middleware (write requests)

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0001
- **Referensi:** detailed-design §5

## Konteks

Semua POST yang menulis (create payment, refund) wajib `Idempotency-Key`, UNIQUE
per `app_id`. Key sama + payload sama → balikan resource sama. Key sama + payload
beda → `409 Conflict`.

## Scope

- [ ] Middleware/util idempotency memanfaatkan constraint UNIQUE
      `(app_id, idempotency_key)` di tabel terkait.
- [ ] Simpan ringkasan response pertama untuk dikembalikan saat retry.
- [ ] Deteksi konflik payload (mis. hash request body) → `409`.
- [ ] Validasi header `Idempotency-Key` wajib pada endpoint tulis.

## Acceptance criteria

- [ ] Dua request identik dengan key sama → satu resource, response konsisten.
- [ ] Key sama payload beda → `409`.
- [ ] Tanpa header pada endpoint tulis → `400`.
- [ ] Aman pada race (uji konkuren) — tidak membuat duplikat.

## File terkait

- `internal/adapter/http/middleware/idempotency.go`
- `internal/usecase/payment/` (integrasi lookup by idempotency key)

## Di luar scope

- Store idempotency khusus (Redis) — constraint DB cukup untuk MVP.
