# 0007 — Endpoint baca: Get & List payment

- **Status:** todo
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** 0006
- **Referensi:** detailed-design §4.2, §4.5

## Konteks

App polling status & reporting butuh endpoint baca yang scoped per `app_id`.

## Scope

- [ ] `GET /v1/payments/{id}` → representasi transaksi penuh.
- [ ] `GET /v1/payments?external_reference=...` → cari by ref.
- [ ] `GET /v1/transactions` → list + filter (`status`, `from`, `to`) +
      cursor-based pagination, hanya milik `app_id` pemanggil.
- [ ] `usecase.GetPayment` implementasi penuh.

## Acceptance criteria

- [ ] Get by id/ref milik app lain → `404` (isolasi).
- [ ] List ter-scope per app, pagination konsisten.
- [ ] Test handler + use-case.

## File terkait

- `internal/usecase/payment/service.go` (GetPayment + list)
- `internal/adapter/http/handler/payment_handler.go` (Get) + handler list
- `internal/adapter/repository/payment_repository.go` (query list + cursor)

## Di luar scope

- Reporting/agregasi (layer terpisah di masa depan).
