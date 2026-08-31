# 0011 — Refund end-to-end + DOKU Refund (per-channel)

- **Status:** done
- **Prioritas:** medium
- **Estimasi:** L
- **Depends on:** 0008
- **Referensi:** detailed-design §3.2, §8, doku-integration-spec §3, §9.1
- **Triase rombakan v2:** berlaku — ditinjau di Phase 5
- **Alasan triase:** ADR-0001 justru melegalkan `RefundType` bicara istilah DOKU, jadi tidak perlu lagi disamarkan sebagai model canonical.

## Konteks

Refund DOKU bisa **sync** (VOID/PARTIAL/FULL) atau **async** (MANUAL_* via
notification), dan **endpoint berbeda per channel** (kartu/QRIS/e-wallet; VA
mungkin tak ada API).

## Scope

- [x] `POST /v1/payments/{id}/refunds` (+ idempotency) → baris `refund` (`requested`).
- [x] `usecase.Refund`: guard `amount <= refundable`, hanya `paid`/`settled`/`partially_refunded`.
- [x] `doku.Refund`: pilih endpoint per `payment_method`/channel & tipe
      (PARTIAL/FULL); kirim `original_request_id` (= `gateway_request_id`).
- [x] Sync → set `succeeded`/`failed` langsung; async → `pending`, finalisasi via
      Refund Notification/poll (jalur webhook 0008).
- [x] `succeeded`: naikkan `refunded_amount`, hitung ulang status
      (`partially_refunded`/`refunded`), event + callback (atomic).
- [x] VA: ditolak `ErrRefundNotSupported` (refund manual via Back Office).

## Acceptance criteria

- [x] Refund penuh & sebagian benar; `refunded_amount` akurat; status transaksi tepat.
- [x] Refund > refundable → ditolak; non-`paid`/`settled` → ditolak.
- [x] Idempotency-key refund mencegah dobel.
- [x] Test alur sync & async (mock gateway).

## Catatan implementasi

- `ApplyRefundSucceeded` menulis refund+txn+event+outbox dalam satu DB-tx.
- `RefundRepository` di-wire ke `cmd/api` & `cmd/worker` (sebelumnya `nil`).
- Finalisasi refund async (Refund Notification) memanfaatkan jalur webhook
  0008; wiring penuh notifikasi refund DOKU menyusul pasca verifikasi sandbox.

## File terkait

- `internal/usecase/payment/service.go` (Refund)
- `internal/adapter/gateway/doku/doku.go` (Refund)
- `internal/adapter/http/handler/payment_handler.go` (Refund)
- `internal/adapter/repository/` (RefundRepository — implementasi port)

## Di luar scope

- Chargeback/dispute automation (jangka panjang).
