# 0013 — Koreksi dokumen DOKU agar sesuai response live

- **Status:** done
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** 0005 (sudah done; ini koreksi dokumentasinya)
- **Referensi:** detailed-design §11, doku-integration-spec §2/§6, kode `internal/adapter/gateway/doku/doku.go` (PR #15)

## Konteks

`docs/result/doku-integration-spec.md` ditulis **2026-06-22, sebelum kode adapter
ada**, berdasarkan asumsi schema. Saat integrasi live (2026-06-27, PR #15) dan
verifikasi ulang via MCP (2026-06-30), beberapa asumsi itu **terbukti salah**.
Kode `doku.go` sudah diperbaiki, tetapi **dokumen desain masih memuat fakta usang**
sehingga menyesatkan untuk pengembangan berikutnya (mis. adapter channel lain).

Bukti live (response `create_doku_direct_checkout`, sandbox `BRN-0252-…`,
2026-06-30) — payload sukses membungkus semua di objek top-level `response`:

```json
{
  "message": ["SUCCESS"],
  "invoiceNumber": "DOC-CHECK-20260630-01",
  "response": {
    "order": { "amount": "50000", "invoice_number": "...", "currency": "IDR",
               "session_id": "125f8c84...27bc" },
    "payment": {
      "token_id": "125f8c84...781",
      "url": "https://staging.doku.com/checkout-link-v2/125f8c84...781",
      "payment_due_date": 60,
      "expired_date": "20260701003444",          // yyyyMMddHHmmss, WIB (UTC+7)
      "expired_datetime": "2026-06-30T17:34:44Z", // RFC3339 UTC
      "type": "SALE"
    },
    "headers": { "request_id": "REQ-...", "signature": "HMACSHA256=...",
                 "client_id": "BRN-0252-..." }
  }
}
```

## Temuan yang perlu dikoreksi

| # | Lokasi (spec) | Klaim usang | Fakta live |
|---|---|---|---|
| 1 | §2 ln 65 | response `result/payment.status` di level atas | dibungkus objek `response` → `response.payment.status`/`response.order` |
| 2 | §6 ln 167-168, §2 ln 73 | expiry di `peer_to_peer_info.expired_date(_utc)` | field-nya `response.payment.expired_date` (`yyyyMMddHHmmss` WIB) & `expired_datetime` (RFC3339 UTC); **`peer_to_peer_info` & `expired_date_utc` tidak ada** |
| 3 | §6 ln 171/174, §7 ln 188 | `expires_at` diisi dari `expired_date_utc` | diisi dari `expired_datetime` (RFC3339), fallback `expired_date` (WIB) — sesuai `parseCheckoutExpiry` di kode |
| 4 | §2 ln 78-80 | `gateway_txn_id ← Request-Id` | `gateway_txn_id ← payment.token_id` / `order.session_id`; `headers.request_id` (`REQ-…`) terpisah |
| 5 | §8 ln 213, §9.5 ln 263 | menyebut `expired_date_utc` sebagai field yang disimpan/diverifikasi | ganti ke `expired_datetime` |
| 6 | §0 ln 17-21, §9.5 ln 264-267 | MCP "credential ditolak / belum bisa transaksi" | MCP **live OK** pakai key **`doku_…`** (bukan Secret Key Checkout `SK-…`); §9.5 verifikasi empiris sudah selesai |

Juga di `docs/issues/0005-doku-create-charge.md` (ln 21, 30): ganti
`ExpiresAt(expired_date_utc)` → `expired_datetime` agar selaras (issue sudah done,
hanya catatan dokumentasi).

## Scope

- [x] Update `docs/result/doku-integration-spec.md` §2, §6, §7, §8, §9.5 sesuai
      tabel di atas; sertakan contoh response live sebagai referensi.
- [x] Update catatan §0 + §9.5 bahwa MCP sudah live (key `doku_…`, Client-Id
      `BRN-0252-…`), verifikasi empiris response Checkout **selesai**.
- [x] Sinkronkan penyebutan `expired_date_utc` di `docs/issues/0005-doku-create-charge.md`.
- [x] Catatan: `headers.signature` & `headers.request_id` ikut di body response
      Checkout (berguna untuk korelasi/audit) — masuk contoh §2.
- [x] Refresh koleksi Postman (`dokuClientId` → `BRN-0252-…`, payload webhook
      diperkaya `service.id`/`acquirer.id`) untuk testing.

## Acceptance criteria

- [x] Tidak ada lagi `expired_date_utc` maupun `peer_to_peer_info` di `docs/`
      (`grep -rn` bersih) kecuali dalam konteks "field ini TIDAK ada".
- [x] Spec mendeskripsikan struktur `response.{order,payment,headers}` dengan benar
      dan konsisten dengan `internal/adapter/gateway/doku/doku.go`.
- [x] Catatan MCP mencerminkan status live + jenis key yang benar.

## Di luar scope

- Perubahan kode adapter — **tidak ada**; `doku.go` sudah benar sejak PR #15.
  Issue ini murni koreksi dokumen.
- Verifikasi response notifikasi/webhook & refund per-channel secara live —
  bisa jadi issue terpisah bila ingin bukti empiris (saat ini dari dokumentasi).
