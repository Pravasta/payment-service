# 0014 — Temuan run Postman: CreatePayment/Sync 500 + observability gagal-gateway

- **Status:** done
- **Prioritas:** high
- **Depends on:** 0006, 0010
- **Referensi:** hasil run koleksi Postman 2026-06-30; `internal/usecase/payment/sync.go`, `internal/adapter/gateway/doku/client.go`

## Konteks

Menjalankan seluruh koleksi Postman berurutan (2026-06-30). Ringkasan:

| # | Request | Status | Catatan |
|---|---|---|---|
| 1-3 | Health (`/healthz`,`/readyz`,`/metrics`) | 200 | ✅ |
| 4 | Create Payment | **500** | ❌ `internal_error` |
| 5-7 | Get by ID / by Ref / List | 200 | ✅ |
| 8 | Sync Payment | **500** | ❌ `internal_error` |
| 9-10 | Refund full/partial (atas txn `failed`) | 400 | ✅ `transaksi tidak dalam status yang bisa direfund` (benar) |
| 11 | Admin replay (id acak) | 404 | ✅ expected |
| 12 | Webhook (signature invalid) | 401 | ✅ expected |

## Akar masalah (utama — konfigurasi)

**`DOKU_SECRET_KEY` di `.env` tertukar dengan MCP key.** Saat memperbaiki MCP
(2026-06-30), `.env` `DOKU_SECRET_KEY` diisi key MCP berprefix **`doku_…` (49 char)**.
Padahal adapter Go memakai `DOKU_SECRET_KEY` untuk **menandatangani request Direct
API DOKU** (`client.go:33` → `Sign(cfg.SecretKey, …)`), yang butuh **Secret Key
Direct API / Checkout** (berbeda dari key MCP). Akibatnya signature salah → DOKU
tolak CreateCharge & GetStatus → transaksi `failed`/500.

**→ Dua key berbeda, jangan tertukar:**
| Pemakai | Key | Lokasi |
|---|---|---|
| MCP server (tool `mcp__doku-…`) | `doku_…` | header `Authorization: Basic` di config MCP (`claude mcp add`) |
| Adapter Go (Checkout/Direct API HMAC) | Secret Key Direct API (mis. `SK-…`) | `.env` `DOKU_SECRET_KEY` |

**Perbaikan langsung (oleh user, bukan kode):** kembalikan **Secret Key Direct API
DOKU** ke `.env` `DOKU_SECRET_KEY`; key `doku_…` cukup hidup di config MCP saja.
Lalu restart `make run` dan ulangi Create Payment.

## Temuan kode yang tetap perlu diperbaiki (independen dari key)

Bahkan setelah key dibetulkan, run ini menyingkap kelemahan nyata:

1. **`/sync` tidak punya guard status/gateway-ref → 500, bukan 4xx.**
   `SyncPayment` (`sync.go:46-58`) langsung memanggil `gateway.GetStatus` untuk
   transaksi **apa pun**, termasuk status **terminal** (`failed`/`expired`/
   `refunded`) dan transaksi **tanpa `gateway_request_id`** (yang gagal sebelum
   sampai gateway). Untuk txn `failed` tanpa ref → GetStatus error → 500.
   **Harusnya:** short-circuit untuk status terminal / tanpa gateway-ref →
   kembalikan transaksi apa adanya atau `409 conflict`/`422 unprocessable`, jangan
   panggil gateway dan jangan 500.

2. **Kegagalan gateway tidak terekam untuk diagnosis (observability gap).**
   Saat CreateCharge gagal, `transaction_event` `payment.failed` ditulis dengan
   `payload` **kosong** — alasan DOKU (signature/credential) tidak tersimpan. Untuk
   mendiagnosis harus mengintip log server + DB manual. **Harusnya:** simpan ringkasan
   error gateway (kode/pesan DOKU) ke `transaction_event.payload` (dan/atau log
   terstruktur yang gampang dikorelasi via `request_id`), tanpa membocorkan secret.

3. **(Minor) Konsistensi status code refund wrong-state.** Refund atas transaksi
   non-refundable mengembalikan `400 invalid_request`; tabel error di
   `docs/api-documentation` mengasosiasikan kasus state/refund dengan
   `422 unprocessable`. Pertimbangkan menyelaraskan (rendah prioritas).

## Scope

- [x] **Guard `/sync`:** short-circuit transaksi **terminal** (`failed`/`expired`/
      `refunded`) sebelum memanggil gateway → no-op idempoten (`sync.go`); unit test
      `TestSyncPayment_TerminalSkipsGateway`.
- [x] **Rekam error gateway:** `markFailed` mem-persist `{gateway, error}` ke
      `transaction_event.payload` saat CreateCharge gagal (tanpa secret); diuji di
      `TestCreatePayment_GatewayErrorMarksFailed`. Log 5xx di boundary HTTP sudah
      ada (PR #14) sebagai pelengkap.
- [~] (Opsional) Status code refund wrong-state (400 → 422): **ditunda** — `400
      invalid_request` defensibel untuk prakondisi state; tidak diubah agar tak
      memecah kontrak yang sudah ada.
- [x] Dokumentasikan pembedaan key MCP (`doku_…`) vs Direct API (`DOKU_SECRET_KEY`)
      di `.env.example`.

## Acceptance criteria

- [x] `POST /sync` atas transaksi `failed`/terminal → no-op `200` (bukan 500).
- [x] Kegagalan create-charge meninggalkan jejak alasan (`payload.error`) yang
      cukup untuk diagnosis tanpa membaca kode.
- [x] `make test` hijau; `/check` lolos.

## Catatan implementasi

- Guard sync hanya menyaring status **terminal**. Transaksi non-terminal tanpa
  `gateway_request_id` tetap boleh sync karena DOKU Check Status dapat memakai
  `invoice_number` (= `external_reference`).
- Reuse: `markFailed` kini menerima `cause error`; satu-satunya pemanggil
  (`CreatePayment`) meneruskan error gateway.

## Di luar scope

- Perbaikan kredensial `.env` itu sendiri — tindakan konfigurasi user, bukan kode.
- Verifikasi pembayaran end-to-end (butuh `payment_url` dibuka & dibayar di sandbox).
