# Postman — Payment Service

Koleksi & environment untuk mencoba API secara lokal.

- `payment-service.postman_collection.json` — semua endpoint.
- `payment-service.local.postman_environment.json` — variabel untuk `localhost:8080`.

## Setup

```bash
make db-up          # Postgres (docker)
make migrate        # buat skema (GORM AutoMigrate)
go run ./cmd/seed   # buat merchant + API credential dev → cetak key_id/secret
make run            # jalankan API di :8080
```

`cmd/seed` membuat credential dev berikut (idempoten — aman dijalankan ulang):

| Field | Nilai default |
|---|---|
| `key_id` | `pk_demo_001` |
| `secret` | `sk_demo_secret_change_me` |
| `scopes` | `payments:read`, `payments:write`, `outbox:admin` |
| HMAC signing | **nonaktif** |

Bisa di-override via env: `SEED_KEY_ID`, `SEED_SECRET`, `SEED_MERCHANT_CODE`.

> Khusus development. Jangan pakai `cmd/seed` di produksi.

## Import di Postman

1. **Import** → pilih kedua file `*.json`.
2. Pilih environment **payment-service.local** di kanan atas.
3. Bila kamu meng-override key/secret saat seed, sesuaikan variabel `keyId`/`secret`.

## Autentikasi

Semua endpoint `/v1/*` butuh:

```
Authorization: Bearer <key_id>:<secret>
```

Koleksi sudah memasangnya di level koleksi memakai `{{keyId}}:{{secret}}`. Endpoint
**Health** dan **Webhook** memakai `noauth`.

### Idempotency

Endpoint tulis (`POST /v1/payments`, `POST /v1/payments/{id}/refunds`) wajib header
`Idempotency-Key`. Koleksi mengisinya dengan `{{$guid}}` (UUID baru tiap kirim).
Kirim ulang dengan key + payload sama → response identik (replay aman); payload
berbeda dengan key sama → `409 conflict`.

### HMAC request-signing (opsional)

Default seed tidak mengaktifkan HMAC, jadi `Authorization` saja cukup. Bila sebuah
credential punya `signing_secret_enc`, isi variabel environment `signingSecret`
dengan kunci HMAC mentahnya — pre-request script koleksi akan otomatis menambahkan:

- `X-Timestamp` (Unix detik)
- `X-Signature` = `base64(HMAC-SHA256(signingKey, component))`

dengan `component = "{timestamp}\n{METHOD}\n{request_uri}\n{sha256_hex(body)}"`.

## Alur uji cepat

1. **Payments → Create Payment** — membuat transaksi; `paymentId` & `externalReference`
   otomatis tersimpan ke variabel koleksi. (`payment_url` ada di response.)
2. **Get Payment by ID / by Reference**, **List Transactions** — baca kembali.
3. **Sync Payment** — refresh status dari gateway (rate-limited 10s/transaksi).
4. **Refunds** — full (`amount: 0`) atau partial.

> Catatan: Create/Sync/Refund memanggil DOKU sungguhan. Tanpa kredensial DOKU
> (`DOKU_CLIENT_ID`/`DOKU_SECRET_KEY`) yang valid, panggilan gateway gagal — auth,
> validasi, idempotency, dan jalur error tetap bisa diuji. Endpoint baca
> (`Get`/`List`) berfungsi penuh tanpa DOKU.

## Format error

Semua error berformat seragam:

```json
{ "code": "invalid_request", "message": "...", "request_id": "..." }
```

| HTTP | `code` | Contoh penyebab |
|---|---|---|
| 400 | `invalid_request` | body JSON tidak valid, parameter salah |
| 401 | `unauthorized` | Authorization/HMAC salah |
| 403 | `forbidden` | scope kurang |
| 404 | `not_found` | transaksi/outbox tidak ada |
| 409 | `conflict` | idempotency key dipakai ulang dengan payload beda; external_reference duplikat |
| 422 | `unprocessable` | refund melebihi sisa / channel tak mendukung refund (VA) |
| 429 | `rate_limited` | sync terlalu sering |
| 500 | `internal_error` | kesalahan tak terduga |

## Endpoint

| Method | Path | Scope | Catatan |
|---|---|---|---|
| GET | `/healthz` | — | liveness |
| GET | `/readyz` | — | readiness (ping DB) |
| GET | `/metrics` | — | Prometheus |
| POST | `/v1/payments` | `payments:write` | + Idempotency-Key |
| GET | `/v1/payments/{id}` | `payments:read` | |
| GET | `/v1/payments?external_reference=` | `payments:read` | |
| GET | `/v1/transactions` | `payments:read` | filter + cursor |
| POST | `/v1/payments/{id}/sync` | `payments:write` | rate-limited |
| POST | `/v1/payments/{id}/refunds` | `payments:write` | + Idempotency-Key |
| POST | `/v1/admin/outbox/{id}/replay` | `outbox:admin` | DLQ replay |
| POST | `/v1/webhooks/doku` | — | dipanggil gateway; HMAC DOKU |
