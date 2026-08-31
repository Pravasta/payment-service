# 0005 — DOKU adapter: CreateCharge (Checkout) + signed request

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0002
- **Referensi:** doku-integration-spec §1, §2, §4
- **Triase rombakan v2:** berlaku — disederhanakan di Phase 2 (ADR-0001)
- **Alasan triase:** Hosted Checkout dikonfirmasi sebagai satu-satunya gaya integrasi (ADR-0006); yang dilepas hanya lapisan terjemahan canonical-nya.

## Konteks

`doku.CreateCharge` masih TODO. DOKU Checkout mengembalikan `payment.url` sinkron;
amount IDR integer; signature request keluar pakai skema HMAC-SHA256 komponen
(util di `crypto.go` sudah ada).

## Scope

- [x] Builder HTTP request bertanda tangan: header `Client-Id`, `Request-Id`,
      `Request-Timestamp`, `Digest`, `Signature` (pakai `Sign`/`Digest`/`ComponentString`).
- [x] `CreateCharge`: bangun body Checkout (`order.invoice_number` ←
      external_reference, `order.amount` integer rupiah, `payment.payment_due_date`),
      kirim, parse response (dibungkus objek `response`) → `ChargeResult{PaymentURL,
      GatewayRequestID, ExpiresAt(expired_datetime), Status}`.
- [x] Map error DOKU → `doku.Error` terstruktur (HTTP status + code + message).
- [~] Konfig kredensial: saat ini dari `Config` (env via `DOKU_*`). Pengambilan
      dari `gateway_account` terenkripsi menyusul saat usecase wiring (issue 0006).

## Acceptance criteria

- [x] Unit test signer request keluar (komponen string & header benar).
- [x] Test parsing response (fixture) → `ChargeResult` benar; `ExpiresAt` dari
      `expired_datetime` (RFC3339), fallback `expired_date` (WIB).
- [x] Smoke test `create_doku_direct_checkout` (MCP key `doku_…`, 2026-06-30):
      response sukses, `payment.url` + `expired_datetime` terverifikasi (spec §2/§9.5).

## File terkait

- `internal/adapter/gateway/doku/doku.go` (CreateCharge)
- `internal/adapter/gateway/doku/client.go` (HTTP signed request) — baru
- `internal/adapter/gateway/doku/crypto.go` (sudah ada)

## Di luar scope

- Channel non-Checkout (VA/QRIS) — menyusul.
