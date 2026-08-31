# 0007 — Endpoint baca: Get & List payment

- **Status:** done
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** 0006
- **Referensi:** detailed-design §4.2, §4.5
- **Triase rombakan v2:** dikerjakan ulang di Phase 3
- **Alasan triase:** `GET /v1/transactions` dilebur ke `GET /v1/payments` supaya hanya ada satu istilah di kontrak API v2.

## Konteks

App polling status & reporting butuh endpoint baca yang scoped per `app_id`.

## Scope

- [x] `GET /v1/payments/{id}` → representasi transaksi penuh.
- [x] `GET /v1/payments?external_reference=...` → cari by ref.
- [x] `GET /v1/transactions` → list + filter (`status`, `from`, `to`) +
      cursor-based pagination, hanya milik `app_id` pemanggil.
- [x] `usecase.GetPayment` (+ GetPaymentByReference + ListPayments) implementasi penuh.

## Acceptance criteria

- [x] Get by id/ref milik app lain → `404` (isolasi).
- [x] List ter-scope per app, pagination konsisten (keyset cursor).
- [x] Test use-case (isolasi, scoping, pagination 2 halaman, limit clamp).

## Catatan implementasi

- Pagination **keyset** (`(created_at, id) < cursor`), bukan OFFSET — stabil &
  efisien. Cursor opaque base64 (`created_at_nano:id`) di layer dto.
- `ListTransactions` ditambahkan ke port `domain.Repository`.
- Limit di-clamp di usecase (default 50, max 100); ambil `limit+1` utk `HasMore`.

## File terkait

- `internal/usecase/payment/service.go` (GetPayment + list)
- `internal/adapter/http/handler/payment_handler.go` (Get) + handler list
- `internal/adapter/repository/payment_repository.go` (query list + cursor)

## Di luar scope

- Reporting/agregasi (layer terpisah di masa depan).
