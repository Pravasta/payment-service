# API Documentation — Payment Service

Panduan alur penggunaan API end-to-end, dari membuat pembayaran hingga refund.
Spesifikasi mesin (Swagger/OpenAPI) ada di [`openapi.yaml`](./openapi.yaml).

- **Base URL (lokal):** `http://localhost:8080`
- **Uang:** selalu *minor unit* integer. IDR `currency_exponent = 0` (rupiah utuh) → `amount: 50000` = Rp50.000.
- **Format waktu:** RFC3339 UTC.

## Daftar isi

1. [Persiapan](#1-persiapan)
2. [Autentikasi](#2-autentikasi)
3. [Idempotency & correlation id](#3-idempotency--correlation-id)
4. [Alur utama: create → bayar → notifikasi](#4-alur-utama-create--bayar--notifikasi)
5. [Membaca status pembayaran](#5-membaca-status-pembayaran)
6. [Sinkronisasi manual](#6-sinkronisasi-manual)
7. [Refund](#7-refund)
8. [Webhook & callback ke app](#8-webhook--callback-ke-app)
9. [State machine pembayaran](#9-state-machine-pembayaran)
10. [Format error](#10-format-error)
11. [Melihat Swagger](#11-melihat-swagger)

---

## 1. Persiapan

```bash
make db-up          # Postgres (docker)
make migrate        # buat skema (GORM AutoMigrate)
go run ./cmd/seed   # buat merchant + API credential dev → cetak key_id/secret
make run            # API di :8080
```

`cmd/seed` mencetak kredensial dev (default `pk_demo_001` / `sk_demo_secret_change_me`,
scope penuh, HMAC nonaktif). Detail di [`docs/postman/README.md`](../postman/README.md).

> Catatan: Create/Sync/Refund memanggil DOKU sungguhan — perlu `DOKU_CLIENT_ID`/
> `DOKU_SECRET_KEY` sandbox yang valid di `.env`. Endpoint baca tidak butuh DOKU.

## 2. Autentikasi

Semua endpoint `/v1/*`:

```
Authorization: Bearer <key_id>:<secret>
```

Scope per credential: `payments:read`, `payments:write`, `outbox:admin`.

Bila credential mengaktifkan HMAC request-signing, tambah `X-Timestamp` (Unix detik)
dan `X-Signature` = `base64(HMAC-SHA256(signingKey, component))` dengan
`component = "{ts}\n{METHOD}\n{request_uri}\n{sha256_hex(body)}"`. Default seed tidak
memakai HMAC.

## 3. Idempotency & correlation id

- Endpoint tulis (`POST /payments`, `POST /payments/{id}/refunds`) **wajib** header
  `Idempotency-Key`. Kirim ulang dengan key + payload sama → response identik;
  key sama tapi payload beda → `409 conflict`.
- Setiap response membawa `X-Request-Id`. Kirim header yang sama untuk menelusuri
  satu request di log (muncul sebagai `request_id`).

## 4. Alur utama: create → bayar → notifikasi

```
App                     Payment Service                 DOKU
 |  POST /v1/payments         |                           |
 | -------------------------> |  simpan txn (created)     |
 |                            | --- Checkout ----------->  |
 |                            | <-- payment_url ----------  |
 | <-- 201 {payment_url} ---- |  txn → pending            |
 |                            |                           |
 | (redirect user ke payment_url, user membayar)          |
 |                            | <== webhook notifikasi === |
 |                            |  txn → paid (+event)      |
 |                            |  jadwalkan callback ──┐   |
 | <== callback (X-Event-Id) =========================┘   |
```

**Create payment:**

```bash
curl -sS -X POST http://localhost:8080/v1/payments \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{
    "external_reference": "demo-0001",
    "amount": 50000,
    "currency": "IDR",
    "customer_ref": "cust-001",
    "description": "Demo order",
    "expiry_minutes": 60,
    "return_url": "https://example.com/return",
    "metadata": { "order_id": "demo-123" }
  }'
```

Respon `201`:

```json
{
  "id": "9f1c2a3b-...",
  "status": "pending",
  "external_reference": "demo-0001",
  "amount": 50000,
  "currency": "IDR",
  "refunded_amount": 0,
  "payment_url": "https://staging.doku.com/checkout-link-v2/...",
  "expires_at": "2026-06-27T03:00:00Z",
  "created_at": "2026-06-27T02:00:00Z"
}
```

Arahkan pengguna ke `payment_url` untuk menyelesaikan pembayaran. Status berubah
menjadi `paid` setelah DOKU mengirim webhook (lihat §8).

## 5. Membaca status pembayaran

```bash
# berdasarkan id
curl -sS http://localhost:8080/v1/payments/{id} \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me"

# berdasarkan external_reference
curl -sS "http://localhost:8080/v1/payments?external_reference=demo-0001" \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me"

# list + pagination (keyset)
curl -sS "http://localhost:8080/v1/transactions?status=pending&limit=50" \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me"
```

List mengembalikan `{ items, has_more, next_cursor }`. Untuk halaman berikutnya,
pakai `next_cursor` ke parameter `cursor`. Filter opsional: `status`, `from`, `to`
(YYYY-MM-DD atau RFC3339), `limit` (default 50, maks 100).

## 6. Sinkronisasi manual

Bila webhook telat/hilang, paksa refresh status dari gateway:

```bash
curl -sS -X POST http://localhost:8080/v1/payments/{id}/sync \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me"
```

Rate-limited per transaksi (min 10 detik) → `429` bila terlalu cepat. Worker juga
menjalankan reconciler otomatis untuk transaksi `pending` yang sudah lama.

## 7. Refund

Hanya transaksi `paid` / `settled` / `partially_refunded`.

```bash
# refund penuh (amount: 0)
curl -sS -X POST http://localhost:8080/v1/payments/{id}/refunds \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{ "amount": 0, "reason": "customer request" }'

# refund sebagian
curl -sS -X POST http://localhost:8080/v1/payments/{id}/refunds \
  -H "Authorization: Bearer pk_demo_001:sk_demo_secret_change_me" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{ "amount": 20000, "reason": "partial refund" }'
```

Respon `201`:

```json
{
  "id": "rfnd-...",
  "transaction_id": "9f1c2a3b-...",
  "amount": 20000,
  "currency": "IDR",
  "status": "succeeded",
  "reason": "partial refund"
}
```

- **Sync** (kartu/QRIS/e-wallet): `status` langsung `succeeded`/`failed`. Refund
  sukses menaikkan `refunded_amount` dan memindahkan transaksi ke
  `partially_refunded`/`refunded`.
- **Async**: `status` `pending`, difinalisasi lewat notifikasi (jalur webhook).
- **VA** tidak mendukung API refund → `422 unprocessable` (refund manual).
- `amount` melebihi sisa refundable → `422`.

## 8. Webhook & callback ke app

- **Gateway → PS:** DOKU memanggil `POST /v1/webhooks/doku` (tanpa auth API key;
  diverifikasi via HMAC signature DOKU). PS mencocokkan transaksi via
  `original_request_id`, memperbarui status (append `transaction_event`), dan
  menjadwalkan callback.
- **PS → App:** worker mengirim callback bertanda tangan ke `webhook_endpoint`
  milik merchant dengan header `X-Event-Id` (untuk dedup) dan `X-Signature`
  (`base64(HMAC-SHA256(signing_secret, body))`). Retry dengan backoff; gagal
  permanen → dead-letter (bisa di-replay via endpoint admin).

Verifikasi callback di sisi app:

```
X-Signature == base64(HMAC-SHA256(signing_secret_app, raw_body))
```

## 9. State machine pembayaran

Hanya transisi maju yang sah (`payment.CanTransition`):

```
created ──▶ pending ──▶ paid ──▶ settled
   │           │          │  └─────────────┐
   ▼           ▼          ▼                 ▼
 failed     expired   partially_refunded ──▶ refunded
                          ▲                 ▲
                          └── (refund) ─────┘
```

- Terminal: `failed`, `expired`, `refunded`.
- `partially_refunded` boleh kembali ke dirinya sendiri (refund berulang) hingga
  habis → `refunded`.

## 10. Format error

Semua error seragam:

```json
{ "code": "not_found", "message": "data tidak ditemukan", "request_id": "..." }
```

| HTTP | `code` | Penyebab umum |
|---|---|---|
| 400 | `invalid_request` | body/parameter tidak valid |
| 401 | `unauthorized` | Authorization/HMAC salah |
| 403 | `forbidden` | scope kurang |
| 404 | `not_found` | transaksi/outbox tidak ada |
| 409 | `conflict` | idempotency key dipakai ulang beda payload; external_reference duplikat |
| 422 | `unprocessable` | refund melebihi sisa / channel tak mendukung refund (VA) |
| 429 | `rate_limited` | sync terlalu sering |
| 500 | `internal_error` | kesalahan tak terduga (detail ada di log server, bukan response) |

## 11. Melihat Swagger

Folder ini sudah berisi **`index.html`** (Swagger UI siap pakai) yang memuat
`openapi.yaml` di sebelahnya.

### Lokal

```bash
# dari root repo — server statis sederhana
python3 -m http.server 8081 --directory docs/api-documentation
# buka http://localhost:8081
```

> Buka lewat HTTP server (bukan `file://`) agar `fetch('./openapi.yaml')` tidak
> diblokir browser.

### GitHub Pages

`index.html` memuat spec via path **relatif** (`./openapi.yaml`), jadi langsung
jalan di Pages. Pilih salah satu cara:

1. **Settings → Pages → Source: `Deploy from a branch` → Branch `main` / folder `/docs`.**
   Swagger akan tampil di:
   `https://<user>.github.io/<repo>/api-documentation/`
2. Atau salin `index.html` + `openapi.yaml` ke root branch `gh-pages`.

### Alternatif tanpa file ini

- **Swagger Editor online:** <https://editor.swagger.io> → File → Import file → `openapi.yaml`.
- **Redoc:** `npx @redocly/cli preview-docs docs/api-documentation/openapi.yaml`

### Catatan "Try it out"

Tombol *Try it out* mengirim request ke server pada `servers:` (default
`http://localhost:8080`). Bila Swagger dibuka dari domain GitHub Pages sementara
API berjalan di `localhost`, browser akan memblokir karena **CORS** (server ini
belum memasang header CORS) — gunakan Postman atau `curl` untuk eksekusi nyata,
sedangkan Swagger Pages untuk eksplorasi/dokumentasi. Saat `index.html` & API
disajikan dari origin yang sama, *Try it out* berfungsi penuh.

Untuk mencoba request langsung dari Postman, lihat koleksi di
[`docs/postman/`](../postman/).
