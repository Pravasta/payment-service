# 0005 — DOKU adapter: CreateCharge (Checkout) + signed request

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0002
- **Referensi:** doku-integration-spec §1, §2, §4

## Konteks

`doku.CreateCharge` masih TODO. DOKU Checkout mengembalikan `payment.url` sinkron;
amount IDR integer; signature request keluar pakai skema HMAC-SHA256 komponen
(util di `crypto.go` sudah ada).

## Scope

- [ ] Builder HTTP request bertanda tangan: header `Client-Id`, `Request-Id`,
      `Request-Timestamp`, `Digest`, `Signature` (pakai `Sign`/`Digest`/`ComponentString`).
- [ ] `CreateCharge`: bangun body Checkout (`order.invoice_number` ←
      external_reference, `order.amount` ← FormatAmount Checkout, `payment.payment_due_date`),
      kirim, parse response → `ChargeResult{PaymentURL, GatewayRequestID, ExpiresAt(expired_date_utc), Status}`.
- [ ] Map error DOKU → error domain yang jelas.
- [ ] Konfig kredensial dari `gateway_account` (didekripsi, issue 0002) atau env.

## Acceptance criteria

- [ ] Unit test signer request keluar (komponen string & header benar).
- [ ] Test parsing response (fixture) → `ChargeResult` benar; `ExpiresAt` dari
      `expired_date_utc`.
- [ ] (Bila credential sandbox valid) smoke test `create_doku_direct_checkout`.

## File terkait

- `internal/adapter/gateway/doku/doku.go` (CreateCharge)
- `internal/adapter/gateway/doku/client.go` (HTTP signed request) — baru
- `internal/adapter/gateway/doku/crypto.go` (sudah ada)

## Di luar scope

- Channel non-Checkout (VA/QRIS) — menyusul.
