# DOKU Integration Spec — Jawaban §11 detailed-design

> Menjawab 6 poin terbuka di `detailed-design.md` §11 ("Yang Belum Final /
> Perlu Dicek ke Dokumen DOKU") dengan fakta dari dokumentasi DOKU.
> Sumber: schema tool **DOKU MCP server** (sandbox, `Client-Id: BRN-0276-…`) +
> dokumentasi resmi `developers.doku.com` / `docs.doku.com`.
> Status: masih desain — belum menulis kode. Tanggal: 2026-06-22.

## 0. Cara verifikasi & catatan sumber

- DOKU MCP server **✔ Connected** (`https://api-sandbox.doku.com/doku-mcp-server/mcp`,
  header `Client-Id`). Tool yang relevan: `create_doku_direct_checkout`,
  `create_virtual_account_payment`, `get_transaction_by_invoice_number`,
  `get_transaction_by_date_range`, `get_merchant_payment_methods`.
- **Tidak ada** tool refund maupun MCP "resource" dokumentasi → poin signature
  webhook & refund dijawab dari dokumentasi publik DOKU.
- **MCP live ✅ (2026-06-30).** Panggilan API live lewat MCP butuh header
  `Authorization: Basic base64(<mcpKey>:)`. **Penting:** `mcpKey` adalah kredensial
  **khusus MCP server** berprefix `doku_…` (±49 char) — **bukan** `DOKU_SECRET_KEY`
  Checkout (prefix `SK-…`) yang dipakai adapter Go. Dengan `doku_…` + Client-Id
  `BRN-0252-…`, call seperti `get_merchant_payment_methods` dan
  `create_doku_direct_checkout` sukses → fakta di dokumen ini **sudah diverifikasi
  empiris** (lihat contoh response §2).

---

## 1. Skema signature & header webhook DOKU  ✅ TERJAWAB

DOKU menandatangani HTTP Notification (dan kita menandatangani request keluar)
dengan skema **HMAC-SHA256 berbasis komponen** — bukan sekadar HMAC atas body.

**Header yang dikirim DOKU:**
| Header | Isi |
|---|---|
| `Client-Id` | id merchant (mis. `BRN-0276-…`) |
| `Request-Id` | id unik request (dipakai juga untuk dedup) |
| `Request-Timestamp` | ISO-8601 UTC (anti-replay) |
| `Signature` | `HMACSHA256=<base64>` |

**Cara verifikasi:**
1. `Digest = base64( SHA-256( raw_json_body ) )` — pakai **byte mentah body**,
   jangan re-serialize JSON.
2. Susun *component string* (newline `\n` sebagai pemisah, urutan persis ini):
   ```
   Client-Id:{clientId}
   Request-Id:{requestId}
   Request-Timestamp:{requestTimestamp}
   Request-Target:{requestTarget}     # path URL notifikasi kita, mis. /v1/webhooks/doku
   Digest:{digest}
   ```
3. `expected = "HMACSHA256=" + base64( HMAC-SHA256( secretKey, componentString ) )`
4. Bandingkan **constant-time** dengan header `Signature`.

**Dampak ke desain:**
- §6.1 langkah 2 sekarang konkret: simpan `raw_body` apa adanya (untuk hitung
  `Digest`) **sebelum** parse — sudah sesuai `webhook_inbox.raw_body (bytea)`.
- `transaction_event.gateway_event_id` ⇐ petakan ke `Request-Id` DOKU (dedup).
- Validasi `Request-Timestamp` terhadap jendela waktu (mis. ±5 menit) → tolak replay.
- Skema yang sama dipakai untuk **request keluar PS → DOKU** (kita yang membuat
  `Signature`), jadi satu util signer/verifier bisa dipakai dua arah.

---

## 2. `payment_url` sinkron & format `order_id`  ✅ TERJAWAB

- **`payment.url` dikembalikan SINKRON** pada response yang sama (HTTP 200,
  `message: ["SUCCESS"]`) dari create-order Checkout. App boleh langsung redirect.
  → asumsi alur §1 detailed-design **benar**.
- **Struktur body (live-confirmed 2026-06-30):** semua field dibungkus objek
  top-level **`response`** — bukan di level atas:

  ```json
  {
    "message": ["SUCCESS"],
    "invoiceNumber": "INV-...",
    "response": {
      "order":   { "amount": "50000", "invoice_number": "INV-...", "currency": "IDR",
                   "session_id": "125f8c84...27bc" },
      "payment": { "token_id": "125f8c84...781",
                   "url": "https://staging.doku.com/checkout-link-v2/125f8c84...781",
                   "payment_due_date": 60,
                   "expired_date": "20260701003444",          // yyyyMMddHHmmss WIB
                   "expired_datetime": "2026-06-30T17:34:44Z", // RFC3339 UTC
                   "type": "SALE" },
      "headers": { "request_id": "REQ-...", "signature": "HMACSHA256=...",
                   "client_id": "BRN-0252-..." }
    }
  }
  ```
- **`order.invoice_number`** = id milik merchant (analog `external_reference` kita):
  - string, **unik per merchant/request**, boleh alfanumerik + karakter spesial,
    **maks 64 char**. `INV-2026-000123` valid.
  - Bila kosong, DOKU MCP meng-autogenerate `AIO-xxxx-xxxxxxxxx`. **Kita selalu
    kirim sendiri** dari `external_reference` agar idempotency & rekonsiliasi rapi.
- Field response lain yang berguna: `response.order.amount`,
  `response.payment.expired_datetime`/`expired_date` (lihat §6),
  `response.headers.request_id` (korelasi/audit).

**Dampak ke desain:**
- Mapping: `transaction.external_reference` → `response.order.invoice_number`;
  `transaction.payment_url` ← `response.payment.url`; `transaction.gateway_txn_id`
  ← `response.payment.token_id` (atau `response.order.session_id`), **bukan**
  `Request-Id`. `response.headers.request_id` (`REQ-…`) adalah id request DOKU,
  terpisah dari token transaksi.
- Tidak perlu state khusus menunggu URL async — `created → pending` terjadi dalam
  satu panggilan sinkron.

---

## 3. Refund sinkron atau async  ✅ TERJAWAB — **KEDUANYA**

- Endpoint: `POST {base}/cancellation/credit-card/refund`
  (sandbox: `https://api-sandbox.doku.com/cancellation/credit-card/refund`).
- Param wajib: `order.invoice_number`, `payment.original_request_id`,
  `refund.amount`.
- **Online (SINKRON):** `VOID`, `PARTIAL_REFUND`, `FULL_REFUND` → hasil langsung
  di `refund.status`.
- **Manual (ASYNC):** `MANUAL_PARTIAL_REFUND`, `MANUAL_FULL_REFUND` → diproses
  DOKU Refund Ops, bisa beberapa hari, hasil final via **Refund Notification**.
- ⚠️ Path mengandung `credit-card` → endpoint refund ini berorientasi kartu.
  Refund untuk channel lain (VA/e-wallet/QRIS) bisa berbeda mekanisme/ketersediaan
  → **konfirmasi per channel** sebelum menjanjikan refund universal.

**Dampak ke desain:**
- State machine refund §3.2 (`requested→pending→succeeded/failed`) **tetap valid**
  dan memang harus mendukung dua jalur:
  - sync → langsung set `succeeded`/`failed` dari response.
  - async → tahan di `pending`, finalisasi dari Refund Notification / poll.
- Simpan `payment.original_request_id` di `transaction` (atau `refund`) — wajib
  untuk memanggil refund. Tambahkan kolom bila belum ada (mis.
  `transaction.gateway_request_id`).
- Adapter `Refund` DOKU harus memilih tipe (VOID vs PARTIAL/FULL) sesuai kondisi.

---

## 4. Format/exponent amount IDR  ✅ TERJAWAB — ada nuansa penting

- **DOKU Checkout** `order.amount` untuk IDR: **bilangan bulat rupiah, tanpa
  desimal**, hanya numerik, **maks 16 digit**. → konfirmasi penuh asumsi kita:
  IDR `currency_exponent = 0`, `amount_minor` = jumlah rupiah.
- ⚠️ **Tapi tidak seragam antar produk DOKU.** Schema tool VA
  (`create_virtual_account_payment`) menyatakan amount *"with 2 decimal, ISO 4217,
  example 11500.00"*. Jadi sebagian Direct/SNAP API memformat amount sebagai
  **string 2-desimal**.

**Dampak ke desain:**
- Penyimpanan kanonik kita (`amount_minor bigint`, exponent 0 untuk IDR) **tetap
  benar** sebagai source of truth.
- **Adapter DOKU bertanggung jawab memformat per-endpoint:**
  - Checkout → kirim integer apa adanya (`"50000"`).
  - VA/SNAP → format `amount_minor` + `.00` (`"50000.00"`).
  - Util `formatAmount(gatewayEndpoint, amount_minor, currency)` di adapter.
- Tambahkan validasi maks 16 digit.

---

## 5. Mekanisme `GetStatus` (reconciler & `/sync`)  ✅ TERJAWAB

Dua jalur tersedia:
1. **Check Status API:** `GET {base}/orders/v1/status/{invoice_number | Request-Id}`
   (sandbox: `api-sandbox.doku.com/orders/v1/status/...`).
   - Nilai status response: `SUCCESS`, `FAILED`, `REFUNDED`.
   - ⚠️ DOKU menyarankan **hit Check Status ≥ 60 detik setelah pembayaran selesai**
     (status mungkin belum final lebih awal).
2. **Report API (via MCP):** `get_transaction_by_invoice_number`,
   `get_transaction_by_date_range` — cocok untuk reconciler batch & dashboard.

**Status notifikasi DOKU lengkap (7 nilai) → mapping canonical (kita):**
| DOKU `transaction.status` | canonical PS | catatan |
|---|---|---|
| `PENDING` | `pending` | menunggu pembayaran |
| `REDIRECT` | `pending` | sedang 3DS/redirect — jangan dianggap final |
| `SUCCESS` | `paid` | |
| `FAILED` | `failed` | |
| `TIMEOUT` | `failed` | sesi habis tanpa selesai (perlakukan gagal) |
| `EXPIRED` | `expired` | lewat `expired_date` |
| `REFUNDED` | `refunded` / `partially_refunded` | lihat amount vs `gross_amount` |

**Dampak ke desain:**
- Interface `PaymentGateway.GetStatus(ref)` → implementasi Check Status API.
- Endpoint `/v1/payments/{id}/sync` & reconciler (§4.3, §6.3) memakai ini.
- **Rate-limit `/sync`**: selaras dengan saran 60 detik — selain throttle
  1×/10s, jangan harap perubahan status sebelum 60s pasca pembayaran.

---

## 6. Aturan expiry checkout DOKU vs `expires_at` internal  ✅ TERJAWAB

- **Default kedaluwarsa halaman Checkout = 60 menit** (cocok dengan default
  `expiry_minutes: 60` di API kita); response meng-echo `response.payment.payment_due_date: 60`.
- Response mengembalikan **dua** field expiry (live-confirmed 2026-06-30):
  - `response.payment.expired_datetime` — **RFC3339 UTC**, mis. `2026-06-30T17:34:44Z`.
  - `response.payment.expired_date` — `yyyyMMddHHmmss` zona **WIB (UTC+7)**,
    mis. `20260701003444` (= 00:34:44 WIB = 17:34:44 UTC).
  - ⚠️ **Tidak ada `expired_date_utc` maupun `peer_to_peer_info`** di response
    Checkout — asumsi lama (pra-kode) keliru.

**Dampak ke desain:**
- **`transaction.expires_at` diisi dari `expired_datetime`** (RFC3339 UTC, source of
  truth gateway), fallback parse `expired_date` (WIB) bila kosong — **bukan**
  dihitung lokal `now()+expiry_minutes`. Sesuai `parseCheckoutExpiry` di
  `internal/adapter/gateway/doku/doku.go`.
- Reconciler (§6.3) menandai `expired` hanya setelah melewati `expired_datetime`
  DOKU **dan** Check Status bukan `SUCCESS`.

---

## 7. Ringkasan perubahan terhadap `detailed-design.md`

| Area | Status | Aksi |
|---|---|---|
| Alur sinkron `payment_url` (§1) | Dikonfirmasi | tidak berubah |
| `webhook_inbox.raw_body` untuk Digest (§6.1) | Dikonfirmasi | tidak berubah |
| `gateway_event_id` ⇐ `Request-Id` (§2.6) | Diperjelas | dokumentasikan mapping |
| Format amount IDR (§7) | Dikonfirmasi (Checkout) | **tambah** util format per-endpoint (VA 2-desimal) |
| Refund (§3.2/§8) | Dikonfirmasi sync+async | **tambah** kolom `gateway_request_id`; handle 2 jalur; cek per-channel |
| `expires_at` (§2.5) | Diubah | isi dari `expired_datetime` (RFC3339) DOKU, bukan lokal |
| Struktur response Checkout (§2) | Dikoreksi | dibungkus objek `response.{order,payment,headers}` |
| `GetStatus` (§4.3/§6.3) | Dikonfirmasi | implement Check Status API; sadar jeda 60s |
| Auth PS→DOKU | Baru | signer HMAC-SHA256 komponen (sama dgn verifier webhook) |

---

## 8. Langkah berikutnya (urutan implementasi yang disarankan)

Menyambung §12 detailed-design, sekarang ter-anchor ke fakta DOKU:

1. **(Opsional, untuk demo MCP live)** Tambah header Authorization Basic ke MCP:
   ```bash
   ./scripts/encode-doku-key.sh           # hasilkan base64(secret_key:)
   claude mcp remove doku-mcp-server
   claude mcp add --transport http doku-mcp-server \
     "https://api-sandbox.doku.com/doku-mcp-server/mcp" \
     --header "Client-Id: BRN-0276-1776847997042" \
     --header "Authorization: Basic <ENCODED>"
   ```
   Lalu uji `get_merchant_payment_methods` & `create_doku_direct_checkout` sandbox.
2. **Skema DB + migrasi** (§2) — tambahkan `transaction.gateway_request_id`.
3. **Util crypto DOKU**: `Digest` + signer/verifier HMAC-SHA256 komponen (§1) —
   dipakai dua arah; tulis unit test dengan contoh dari sandbox.
4. **Adapter DOKU**:
   - `CreateCharge` → Checkout (format amount integer, kirim `invoice_number`,
     baca dari `response.{order,payment}`: simpan `payment.url`, `payment.token_id`,
     `payment.expired_datetime`).
   - `formatAmount` per-endpoint (Checkout integer / VA `.00`).
   - `GetStatus` → `GET /orders/v1/status/{invoice_number}` + mapping status.
   - `Refund` → `/cancellation/credit-card/refund` (pilih VOID/PARTIAL/FULL).
5. **Webhook receiver** `/v1/webhooks/doku`: simpan raw → verifikasi signature
   (§1) → dedup `Request-Id` → state machine → outbox.
6. **Outbox worker** (callback ke app) + **reconciler/poll** (Check Status, sadar
   jeda 60s) + endpoint `/sync` (rate-limit).
7. **Refund end-to-end** (sync set langsung; async tunggu Refund Notification).
8. **Observability** dasar (success rate, outbox lag, recon mismatch).

---

## 9. Sisa pertanyaan — sudah terjawab dari dokumentasi

### 9.1. Refund non-kartu — **per-channel, BUKAN endpoint kartu**  ✅
Tiap channel punya mekanisme refund sendiri; `/cancellation/credit-card/refund`
**tidak universal**:
| Channel | Endpoint refund |
|---|---|
| Kartu kredit | `POST /cancellation/credit-card/refund` |
| QRIS | `POST /snap-adapter/b2b/v1.0/qr/qr-mpm-refund` |
| E-wallet (mis. DANA) | `POST .../direct-debit/core/v1/debit/refund` (`additionalInfo.channel = EMONEY_DANA_SNAP`) |
| Virtual Account | **Tidak terdokumentasi** — kemungkinan tidak ada refund VA via API |

→ **Dampak desain:** adapter `Refund` DOKU harus memilih endpoint berdasarkan
`transaction.payment_method`/channel. Untuk MVP (fokus Checkout), refund VA
mungkin perlu jalur manual/Back Office → tandai sebagai keterbatasan, jangan
janjikan refund VA otomatis.

### 9.2. Daftar status notifikasi lengkap  ✅
7 nilai: `PENDING`, `REDIRECT`, `SUCCESS`, `FAILED`, `TIMEOUT`, `EXPIRED`,
`REFUNDED`. Mapping canonical lengkap ada di §5.

### 9.3. Notifikasi membawa payment method final?  ✅ YA
Notifikasi berisi tiga field path pembayaran:
- `service.id` — tipe layanan, mis. `VIRTUAL_ACCOUNT`, `CREDIT_CARD`.
- `channel.id` — channel spesifik, mis. `VIRTUAL_ACCOUNT_BCA`.
- `acquirer.id` — institusi, mis. `BCA`, `BANK_CIMB`.

→ **Dampak desain:** isi `transaction.payment_method` dari `channel.id` (atau
simpan ketiganya di `transaction_event.payload` untuk reporting).

### 9.4. Base URL & environment  ✅
- Sandbox: `https://api-sandbox.doku.com` · Produksi: `https://api.doku.com`.
- Auth request keluar PS→DOKU: skema **Signature HMAC-SHA256 komponen** (§1),
  sama dua arah. (Catatan: header `Authorization: Basic` yang dipakai adalah auth
  **MCP server**, bukan auth Direct API DOKU untuk adapter Go kita.)

### 9.5. Verifikasi empiris — **SELESAI (2026-06-30)** ✅
- Response Checkout sungguhan dikonfirmasi via `create_doku_direct_checkout`
  sandbox (`Client-Id BRN-0252-…`, MCP key `doku_…`): struktur `response.{order,
  payment,headers}`, `payment.url`, `payment.token_id`, `payment.expired_datetime`
  (RFC3339) + `expired_date` (WIB). Contoh lengkap di §2.
- Tersisa (opsional, butuh transaksi dibayar): bentuk nyata **payload notifikasi**
  (`service.id`/`channel.id`/`acquirer.id`, §9.3) & **refund per-channel** (§9.1)
  — saat ini dari dokumentasi, belum dari transaksi live.
