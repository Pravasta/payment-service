# Detailed Design — Payment Service (MVP)

> Lanjutan dari `architecture-review.md`. Masih tahap desain — **belum** menulis
> kode aplikasi. SQL/JSON di sini adalah *artefak desain* (kontrak & skema),
> bukan implementasi.
> Tanggal: 2026-06-22.

## 0. Keputusan yang Dikunci (dari §13 review)

| # | Keputusan | Implikasi desain |
|---|---|---|
| 1 | **DOKU Redirect / Hosted Checkout** | PS tidak pernah menyentuh data kartu → PCI scope minimal (SAQ-A). Flow berbasis `payment_url` + redirect. |
| 2 | **Webhook callback + polling**, idempoten | Notifikasi push via outbox **dan** endpoint read (pull). Plus endpoint `sync` untuk paksa refresh status dari gateway. |
| 3 | **Multi-currency, default IDR** | `amount_minor` (bigint) + `currency` + `currency_exponent`. IDR di praktik = 0 desimal. |
| 4 | **App pertama: Invoice SaaS** | Kontrak API divalidasi terhadap kasus invoice (1 invoice = 1 payment). |
| 5 | **Refund fungsional** | Endpoint + state machine + adapter refund DOKU aktif di MVP. |
| 6 | **Belum ada infra/secret manager** | Secret terenkripsi at-rest pakai master key dari env; jalur migrasi ke Vault/KMS disiapkan. |

---

## 1. Alur End-to-End (Redirect Checkout)

```
Invoice SaaS            Payment Service              DOKU                User
     │                       │                        │                   │
 1.  │ POST /v1/payments     │                        │                   │
     │ (Idempotency-Key)     │                        │                   │
     │──────────────────────▶│                        │                   │
     │                       │ 2. create txn (created)│                   │
     │                       │ 3. call Checkout API   │                   │
     │                       │───────────────────────▶│                   │
     │                       │ 4. payment_url         │                   │
     │                       │◀───────────────────────│                   │
     │ 5. { id, payment_url }│ (status: pending)      │                   │
     │◀──────────────────────│                        │                   │
     │ 6. redirect user ─────────────────────────────────────────────────▶│
     │                       │                        │ 7. user membayar  │
     │                       │                        │◀──────────────────│
     │                       │ 8. webhook (signed)    │                   │
     │                       │◀───────────────────────│                   │
     │                       │ 9. verify sig, simpan  │                   │
     │                       │    raw, update→paid,   │                   │
     │                       │    tulis outbox        │                   │
     │ 10a. callback (signed)│                        │                   │
     │◀──────────────────────│ (outbox worker, retry) │                   │
     │ 10b. atau app polling │                        │                   │
     │  GET /v1/payments/{id}│                        │                   │
     │──────────────────────▶│                        │                   │
```

Jaring pengaman: bila webhook (langkah 8) tak pernah datang, **reconciler**
mem-poll `GetStatus` ke DOKU untuk transaksi `pending` yang melewati ambang
waktu, lalu memicu update + outbox yang sama.

---

## 2. Data Model

Catatan umum: semua tabel punya `app_id` (discriminator multi-tenant), `id`
(UUID), `created_at`, `updated_at`. Uang **selalu** `bigint` minor unit.

### 2.1. `merchant` (aplikasi pemanggil)
```
merchant
  id            uuid pk
  code          text unique         -- "invoice-saas"
  name          text
  status        text                -- active | suspended
  created_at    timestamptz
```

### 2.2. `api_credential` (auth app → PS)
```
api_credential
  id            uuid pk
  merchant_id   uuid fk
  key_id        text unique         -- public, dikirim app (mis. "pk_live_xxx")
  secret_hash   text                -- argon2id(secret) — secret hanya tampil sekali saat dibuat
  signing_secret_enc bytea          -- untuk HMAC request signing (terenkripsi)
  scopes        text[]              -- ["payments:write","payments:read","refunds:write"]
  status        text                -- active | revoked
  created_at    timestamptz
```

### 2.3. `webhook_endpoint` (callback PS → app)
```
webhook_endpoint
  id            uuid pk
  merchant_id   uuid fk
  url           text
  signing_secret_enc bytea          -- PS menandatangani callback; app verifikasi
  status        text                -- active | disabled
```

### 2.4. `gateway_account` (kredensial PS → gateway)
```
gateway_account
  id            uuid pk
  gateway       text                -- "doku"
  environment   text                -- sandbox | production
  config_enc    bytea               -- client_id, secret_key DOKU (terenkripsi)
  status        text
  -- NOTE: dienkripsi dengan master key dari env (lihat §9)
```

### 2.5. `transaction` (state saat ini — source of truth fakta pembayaran)
```
transaction
  id                 uuid pk
  app_id             uuid fk -> merchant      -- scope semua query
  external_reference text                     -- id milik app (mis. invoice_id) — UNIQUE per app
  idempotency_key    text                     -- dari header; UNIQUE per app
  status             text                     -- canonical state machine (§3)
  -- uang
  currency           char(3)                  -- "IDR" (default), "USD", ...
  currency_exponent  smallint                 -- IDR=0, USD=2  (jumlah desimal)
  gross_amount       bigint                   -- minor unit (IDR: rupiah; USD: cent)
  fee_amount         bigint                   -- diisi saat settlement (default 0)
  net_amount         bigint                   -- gross - fee (diisi saat settlement)
  refunded_amount    bigint                   -- akumulasi refund (default 0)
  -- gateway
  gateway            text                     -- "doku"
  gateway_txn_id     text                     -- id transaksi di sisi DOKU
  payment_url        text                     -- url redirect checkout
  payment_method     text                     -- diisi setelah user memilih (va/qris/cc/...)
  -- konteks opaque untuk reporting (PS tidak menafsirkan)
  customer_ref       text                     -- id customer di app (opaque)
  metadata           jsonb                    -- label bebas: {"product":"invoice-pro", ...}
  description        text
  -- waktu lifecycle
  expires_at         timestamptz
  paid_at            timestamptz
  settled_at         timestamptz
  failed_at          timestamptz
  expired_at         timestamptz
  created_at         timestamptz
  updated_at         timestamptz

  UNIQUE (app_id, external_reference)
  UNIQUE (app_id, idempotency_key)
  INDEX  (app_id, status, created_at)
  INDEX  (gateway, gateway_txn_id)
```

### 2.6. `transaction_event` (APPEND-ONLY — audit & sumber reporting)
```
transaction_event
  id             uuid pk
  transaction_id uuid fk
  app_id         uuid fk
  event_type     text          -- payment.created | payment.pending | payment.paid
                                -- payment.failed | payment.expired
                                -- payment.refunded | payment.partially_refunded
  from_status    text
  to_status      text
  amount_minor   bigint        -- relevan utk refund
  source         text          -- "api" | "webhook" | "reconciler" | "manual"
  gateway_event_id text        -- dedup webhook DOKU
  payload        jsonb         -- snapshot data event
  occurred_at    timestamptz   -- waktu kejadian di sumber (utk urutan)
  created_at     timestamptz

  -- TIDAK PERNAH di-UPDATE / DELETE
  UNIQUE (gateway_event_id) WHERE gateway_event_id IS NOT NULL   -- idempotensi webhook
```

### 2.7. `webhook_inbox` (raw mentah dari gateway)
```
webhook_inbox
  id             uuid pk
  gateway        text
  signature      text
  headers        jsonb
  raw_body       bytea         -- apa adanya, untuk audit/replay
  verified       boolean
  processed      boolean
  transaction_id uuid          -- nullable sampai termatch
  received_at    timestamptz
```

### 2.8. `notification_outbox` (callback PS → app, reliable)
```
notification_outbox
  id             uuid pk
  app_id         uuid fk
  transaction_id uuid fk
  event_id       text          -- dikirim ke app utk dedup (UNIQUE)
  event_type     text
  payload        jsonb
  status         text          -- pending | delivered | failed | dead
  attempts       int
  next_retry_at  timestamptz
  last_error     text
  created_at     timestamptz
  delivered_at   timestamptz
```

### 2.9. `refund`
```
refund
  id              uuid pk
  app_id          uuid fk
  transaction_id  uuid fk
  idempotency_key text          -- UNIQUE per app
  amount_minor    bigint        -- partial diizinkan (<= sisa refundable)
  currency        char(3)
  status          text          -- requested | pending | succeeded | failed
  gateway_refund_id text
  reason          text
  created_at      timestamptz
  updated_at      timestamptz

  UNIQUE (app_id, idempotency_key)
```

---

## 3. State Machine

### 3.1. Payment (canonical, lintas gateway)
```
            ┌─────────┐
            │ created │  (txn dibuat, belum panggil gateway)
            └────┬────┘
                 │ checkout dibuat di gateway
            ┌────▼────┐
       ┌────│ pending │────┐──────────────┐
       │    └─────────┘    │              │
   user bayar          expiry/timeout   user batal/gagal
       │                   │              │
   ┌───▼──┐           ┌────▼────┐    ┌────▼───┐
   │ paid │           │ expired │    │ failed │
   └──┬───┘           └─────────┘    └────────┘
      │ settlement report
   ┌──▼─────┐
   │ settled│
   └──┬─────┘
      │ refund (penuh/sebagian)
   ┌──▼──────────────────┐   ┌──────────┐
   │ partially_refunded  │──▶│ refunded │
   └─────────────────────┘   └──────────┘
```

**Aturan transisi (anti out-of-order/duplikat):**
- Hanya transisi maju yang sah diterima; mis. webhook `expired` yang tiba
  setelah status `paid` → **diabaikan** (di-log sebagai event, status tak
  berubah).
- `paid → settled` dipicu rekonsiliasi settlement, bukan webhook user.
- `refunded_amount == gross_amount` ⇒ `refunded`; `0 < refunded_amount < gross`
  ⇒ `partially_refunded`.
- Terminal: `expired`, `failed`, `refunded`. `settled`/`paid` masih bisa →
  refund.

### 3.2. Refund
```
requested ──▶ pending ──▶ succeeded
                  └──────▶ failed
```
- `succeeded` menaikkan `transaction.refunded_amount` dan menulis
  `transaction_event` (`payment.refunded` / `payment.partially_refunded`).
- DOKU refund bisa async → status final dari webhook/poll refund.

---

## 4. Kontrak API (REST v1)

Header umum: `Authorization`, `Idempotency-Key` (untuk semua POST yang menulis),
`X-Signature` + `X-Timestamp` (jika HMAC signing diaktifkan), `X-Request-Id`.

### 4.1. Create Payment
```
POST /v1/payments
Idempotency-Key: 7c9e... (wajib)

{
  "external_reference": "INV-2026-000123",   // id invoice di app
  "amount": 150000,                           // minor unit; IDR → rupiah
  "currency": "IDR",
  "customer_ref": "cust_8842",
  "description": "Invoice Pro - Juni 2026",
  "expiry_minutes": 60,
  "return_url": "https://invoice.app/pay/INV-2026-000123/return",
  "metadata": { "product": "invoice-pro", "billing_cycle": "monthly" }
}
```
Response `201`:
```
{
  "id": "pay_3f2a...",
  "status": "pending",
  "external_reference": "INV-2026-000123",
  "amount": 150000,
  "currency": "IDR",
  "payment_url": "https://checkout.doku.com/...",   // app redirect ke sini
  "expires_at": "2026-06-22T03:45:00Z",
  "created_at": "2026-06-22T02:45:00Z"
}
```
Request ulang dengan `Idempotency-Key` sama → **mengembalikan resource yang
sama** (200), tidak membuat transaksi baru. (lihat §5)

### 4.2. Get Payment (polling, read murni / idempoten)
```
GET /v1/payments/{id}
GET /v1/payments?external_reference=INV-2026-000123
```
Response `200`: representasi transaksi penuh (status, amounts, refunded_amount,
timestamps, payment_method).

### 4.3. Force Sync (paksa refresh dari gateway — untuk polling agresif)
```
POST /v1/payments/{id}/sync
```
- Memanggil `GetStatus` ke DOKU, merekonsiliasi status, menulis event bila
  berubah. **Rate-limited** (mis. 1×/10 detik per txn) agar app yang polling
  ketat tidak membanjiri gateway. Aman dipanggil berulang (idempoten secara
  efek: hanya menulis event bila ada perubahan nyata).

### 4.4. Refund
```
POST /v1/payments/{id}/refunds
Idempotency-Key: a91f... (wajib)

{ "amount": 50000, "reason": "customer request" }   // amount opsional → full refund
```
Response `201`:
```
{
  "id": "rfnd_77c...",
  "transaction_id": "pay_3f2a...",
  "amount": 50000,
  "currency": "IDR",
  "status": "pending"
}
```
Validasi: `amount <= gross_amount - refunded_amount`. Refund penuh bila `amount`
diabaikan.

### 4.5. List Transactions (read/report, scoped per app)
```
GET /v1/transactions?status=paid&from=2026-06-01&to=2026-06-30&limit=50&cursor=...
```
Cursor-based pagination. Hanya transaksi milik `app_id` pemanggil.

### 4.6. Webhook receiver (gateway → PS, bukan untuk app)
```
POST /v1/webhooks/doku        // diverifikasi via signature DOKU
```

### 4.7. Bentuk callback PS → app (push)
PS mengirim ke `webhook_endpoint.url` milik app:
```
POST {app webhook url}
X-Signature: hmac-sha256(...)        // app verifikasi dgn signing_secret
X-Event-Id: evt_5a1...               // app dedup pakai ini

{
  "event_id": "evt_5a1...",
  "event_type": "payment.paid",
  "payment": {
    "id": "pay_3f2a...",
    "external_reference": "INV-2026-000123",
    "status": "paid",
    "amount": 150000, "currency": "IDR",
    "paid_at": "2026-06-22T03:10:00Z"
  }
}
```
App **wajib** merespons `2xx`; non-2xx/timeout → PS retry (lihat §6.2).

---

## 5. Idempotency (3 arah)

| Arah | Mekanisme | Perilaku |
|---|---|---|
| **App → PS** (create/refund) | Header `Idempotency-Key`, UNIQUE per `app_id` | Key sama + payload sama → balikan resource sama. Key sama + payload beda → `409 Conflict`. |
| **App → PS** (read/poll/sync) | GET murni read; `sync` rate-limited | Tak ada side-effect berbahaya saat dipanggil berulang. |
| **Gateway → PS** (webhook) | `gateway_event_id` UNIQUE di `transaction_event` | Webhook dobel → di-ack tapi skip side-effect. |
| **PS → App** (callback) | `event_id` unik dikirim ke app | App dedup; PS at-least-once. |

Penyimpanan idempotency: cukup constraint UNIQUE di DB (`app_id, idempotency_key`)
+ menyimpan response ringkas pertama untuk dikembalikan pada retry. Tidak perlu
store khusus untuk skala MVP.

---

## 6. Webhook, Outbox, dan Polling

### 6.1. Pemrosesan webhook DOKU (langkah 8–10)
1. Terima → simpan ke `webhook_inbox` (raw + signature) **sebelum** verifikasi.
2. Verifikasi signature DOKU. Gagal → tandai `verified=false`, return 401, alert.
3. Map ke `transaction` via `gateway_txn_id` / order id.
4. Cek `gateway_event_id` → bila sudah ada di `transaction_event`, ack & stop
   (dedup).
5. Dalam **satu transaksi DB**: validasi transisi state → update `transaction`
   → insert `transaction_event` → insert `notification_outbox`.
6. Ack `2xx` ke DOKU secepatnya (pemrosesan berat di luar request bila perlu).

### 6.2. Outbox worker (callback ke app)
- Loop: ambil `notification_outbox` `status=pending AND next_retry_at<=now`.
- Kirim HTTP bertanda tangan ke `webhook_endpoint`.
- Sukses (2xx) → `delivered`. Gagal → `attempts++`, `next_retry_at` dengan
  **exponential backoff + jitter** (mis. 10s, 30s, 2m, 10m, 1h, ...).
- Lewati batas attempt (mis. 12) → `dead` + alert (app bisa rekonsiliasi via
  polling). Sediakan endpoint admin "replay" untuk dead-letter.

### 6.3. Reconciler (jaring pengaman, scheduled job)
- Untuk `transaction` `pending` yang `created_at` > ambang (mis. 5 menit) dan
  belum `expires_at`: panggil `GetStatus` DOKU → update bila berubah (jalur sama
  §6.1 langkah 5).
- Untuk yang lewat `expires_at` tanpa pembayaran → transisi `expired` + event +
  outbox.
- (Fase berikut) rekonsiliasi settlement report untuk mengisi `fee_amount`,
  `net_amount`, dan `settled_at`.

---

## 7. Multi-Currency (default IDR)

- Simpan **`amount_minor` (bigint)** + **`currency` (char 3)** + **`currency_exponent`**.
- **Jebakan IDR:** secara ISO-4217 exponent IDR = 2, tetapi praktik pembayaran
  Indonesia & DOKU memakai **rupiah utuh (exponent 0)**. Jadi untuk IDR,
  `amount_minor` = jumlah rupiah, `currency_exponent = 0`. Untuk USD,
  `amount_minor` = cent, `exponent = 2`.
- Simpan tabel referensi exponent agar konsisten; tolak currency yang tak
  dikenal.
- API menerima `amount` dalam minor unit + `currency`; PS menetapkan
  `currency_exponent` dari tabel referensi (app tidak perlu mengirimnya).
- Default `currency = "IDR"` bila tidak dikirim.
- Jangan pernah menjumlahkan lintas-currency di reporting tanpa konversi
  eksplisit — selalu group by currency.

---

## 8. Refund (fungsional di MVP)

- Endpoint §4.4 → buat baris `refund` (`requested`) + idempotency.
- Panggil adapter `Refund` DOKU → `pending`.
- Hasil final via webhook/poll refund DOKU → `succeeded`/`failed`.
- Pada `succeeded`: naikkan `transaction.refunded_amount`, hitung ulang status
  transaksi (`partially_refunded`/`refunded`), tulis `transaction_event`, kirim
  callback (`payment.refunded`/`payment.partially_refunded`).
- Validasi guard: total refund tak boleh melebihi `gross_amount`; hanya
  transaksi `paid`/`settled` yang refundable.

---

## 9. Security & Secrets (tanpa secret manager dulu)

Karena belum ada infra (#6), pendekatan pragmatis yang aman dan mudah
ditingkatkan:

- **Auth app → PS:** API key (`key_id` + secret; secret di-hash argon2id). Sejak
  awal sediakan opsi **HMAC signing** (`X-Signature` atas timestamp+method+path+
  body, anti-replay via `X-Timestamp`). Aktifkan untuk Invoice SaaS.
- **Secret gateway & signing secret:** disimpan **terenkripsi at-rest**
  (AES-GCM) di kolom `*_enc`, kuncinya **master key dari environment variable**
  (`PAYMENTS_MASTER_KEY`) saat boot. Bukan ideal, tapi jauh lebih baik daripada
  plaintext dan **tidak mengubah skema** saat nanti pindah ke Vault/KMS — cukup
  ganti sumber master key.
- **Jalur migrasi (didokumentasikan, bukan dibangun sekarang):** master key →
  cloud KMS/Vault; tambahkan envelope encryption. Skema `*_enc` sudah siap.
- **Webhook DOKU:** verifikasi signature wajib; simpan raw; (opsional) IP allow-list.
- **Callback ke app:** ditandatangani HMAC dengan `webhook_endpoint.signing_secret`.
- **Audit:** `transaction_event` (append-only) + `webhook_inbox` (raw) +
  correlation id end-to-end.
- **PCI:** redirect checkout → PS tak pernah menerima PAN. Pastikan tak ada
  field kartu yang melewati PS.

---

## 10. Pemetaan untuk Invoice SaaS (app pertama)

- 1 invoice → 1 `transaction`, `external_reference = invoice_id`.
- Invoice SaaS memegang: status invoice, plan, customer. PS hanya tahu amount.
- Saat `payment.paid` diterima (callback atau polling), Invoice SaaS menandai
  invoice lunas — **idempoten** memakai `event_id`.
- Partial refund invoice → `POST /refunds` dengan `amount`.
- `metadata` membawa label reporting (mis. `{"product":"invoice-pro"}`) tanpa PS
  menafsirkannya.

---

## 11. Yang Belum Final / Perlu Dicek ke Dokumen DOKU

Sebelum/ saat implementasi, beberapa detail bergantung spesifik DOKU
(akan dikonfirmasi dari dokumentasi/akun DOKU):

1. Skema signature webhook & header persis DOKU (algoritma, field yang ditandatangani).
2. Apakah DOKU Checkout mengembalikan `payment_url` sinkron, dan format `order_id`.
3. Apakah refund DOKU sinkron atau async (memengaruhi penanganan status refund).
4. Field exponent/format amount yang diharapkan DOKU untuk IDR (asumsi: rupiah utuh).
5. Mekanisme `GetStatus` (untuk reconciler & endpoint `sync`).
6. Aturan expiry checkout di DOKU vs `expires_at` internal.

---

## 12. Langkah Berikutnya (saat Anda siap)

Urutan implementasi yang saya sarankan (tetap menunggu aba-aba Anda):
1. Skema DB + migrasi (tabel §2).
2. Auth (API key/HMAC) + middleware idempotency.
3. Use-case `CreatePayment` + interface `PaymentGateway` + adapter DOKU (Checkout).
4. Webhook receiver + verifikasi + state machine + outbox.
5. Outbox worker (callback) + reconciler/poll + endpoint `sync`.
6. Refund end-to-end.
7. Observability dasar (metrik success rate, outbox lag, recon mismatch).

Beri tahu bila Anda ingin saya lanjut ke **diagram ERD/sequence formal**,
**spesifikasi OpenAPI**, atau langsung mulai **implementasi** modul pertama.
