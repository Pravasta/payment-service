# 0005 — DOKU adapter: CreateCharge (Checkout) + signed request

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0002
- **Referensi:** doku-integration-spec §1, §2, §4

## Konteks

`doku.CreateCharge` masih TODO. DOKU Checkout mengembalikan `payment.url` sinkron;
amount IDR integer; signature request keluar pakai skema HMAC-SHA256 komponen
(util di `crypto.go` sudah ada).

## Scope

- [x] Builder HTTP request bertanda tangan: header `Client-Id`, `Request-Id`,
      `Request-Timestamp`, `Digest`, `Signature` (pakai `Sign`/`Digest`/`ComponentString`).
- [x] `CreateCharge`: bangun body Checkout (`order.invoice_number` ←
      external_reference, `order.amount` integer rupiah, `payment.payment_due_date`),
      kirim, parse response → `ChargeResult{PaymentURL, GatewayRequestID, ExpiresAt(expired_date_utc), Status}`.
- [x] Map error DOKU → `doku.Error` terstruktur (HTTP status + code + message).
- [~] Konfig kredensial: saat ini dari `Config` (env via `DOKU_*`). Pengambilan
      dari `gateway_account` terenkripsi menyusul saat usecase wiring (issue 0006).

## Acceptance criteria

- [x] Unit test signer request keluar (komponen string & header benar).
- [x] Test parsing response (fixture) → `ChargeResult` benar; `ExpiresAt` dari
      `expired_date_utc`.
- [~] Smoke test `create_doku_direct_checkout` ditunda: credential MCP sandbox
      masih ditolak DOKU (spec §9.5). Parser dibuat toleran utk verifikasi nanti.

## File terkait

- `internal/adapter/gateway/doku/doku.go` (CreateCharge)
- `internal/adapter/gateway/doku/client.go` (HTTP signed request) — baru
- `internal/adapter/gateway/doku/crypto.go` (sudah ada)

## Di luar scope

- Channel non-Checkout (VA/QRIS) — menyusul.
