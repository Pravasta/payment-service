# 0011 — Refund end-to-end + DOKU Refund (per-channel)

- **Status:** todo
- **Prioritas:** medium
- **Estimasi:** L
- **Depends on:** 0008
- **Referensi:** detailed-design §3.2, §8, doku-integration-spec §3, §9.1

## Konteks

Refund DOKU bisa **sync** (VOID/PARTIAL/FULL) atau **async** (MANUAL_* via
notification), dan **endpoint berbeda per channel** (kartu/QRIS/e-wallet; VA
mungkin tak ada API).

## Scope

- [ ] `POST /v1/payments/{id}/refunds` (+ idempotency) → baris `refund` (`requested`).
- [ ] `usecase.Refund`: guard `amount <= refundable`, hanya `paid`/`settled`.
- [ ] `doku.Refund`: pilih endpoint per `payment_method`/channel & tipe
      (VOID/PARTIAL/FULL); kirim `original_request_id` (= `gateway_request_id`).
- [ ] Sync → set `succeeded`/`failed` langsung; async → `pending`, finalisasi via
      Refund Notification/poll.
- [ ] `succeeded`: naikkan `refunded_amount`, hitung ulang status
      (`partially_refunded`/`refunded`), event + callback.
- [ ] VA: tandai keterbatasan (tidak otomatis) bila tak didukung.

## Acceptance criteria

- [ ] Refund penuh & sebagian benar; `refunded_amount` akurat; status transaksi tepat.
- [ ] Refund > refundable → ditolak; non-`paid`/`settled` → ditolak.
- [ ] Idempotency-key refund mencegah dobel.
- [ ] Test alur sync & async (mock gateway).

## File terkait

- `internal/usecase/payment/service.go` (Refund)
- `internal/adapter/gateway/doku/doku.go` (Refund)
- `internal/adapter/http/handler/payment_handler.go` (Refund)
- `internal/adapter/repository/` (RefundRepository — implementasi port)

## Di luar scope

- Chargeback/dispute automation (jangka panjang).
