---
name: api-docs
description: Sinkronkan OpenAPI (docs/api-documentation/openapi.yaml) dan koleksi Postman dengan route yang benar-benar ada di kode. Dijalankan HANYA di akhir, setelah semua phase selesai — jangan dipakai di tengah phase.
---

Dokumentasi API sengaja **dibiarkan usang selama phase berjalan** supaya tiap PR
tetap fokus dan tidak penuh diff YAML. Skill ini yang melunasinya di akhir.

Jalankan hanya kalau: semua phase sudah `done`, atau user meminta eksplisit.

## Langkah

### 1. Ambil kebenaran dari kode, bukan dari dokumen lama

```bash
rg -n 'router\.|r\.(GET|POST|PUT|PATCH|DELETE)|Group\(' internal/handler/
```

Susun daftar route sebenarnya: method, path, middleware (auth/scope/idempotency),
DTO request, DTO response, dan kode status yang mungkin. Baca juga
`internal/handler/dto/` untuk field dan aturan `binding`.

### 2. Perbarui OpenAPI

`docs/api-documentation/openapi.yaml`. Yang harus benar dan sering salah:

- Header wajib: `Authorization`, `Idempotency-Key` (endpoint tulis), `X-Request-Id`.
- Scope per endpoint didokumentasikan di deskripsi.
- **Uang: integer minor unit** (IDR = rupiah utuh, `currency_exponent` 0) — jangan
  pernah `number`/float di schema.
- Bentuk error konsisten dengan `internal/handler/apperror` yang sebenarnya.
- Contoh response untuk kasus sukses **dan** kasus umum gagal (401, 409 idempotency
  conflict, 422 validasi, 502 gateway).
- Webhook keluar PS → app didokumentasikan (bentuk body + header `X-Event-Id`,
  `X-Signature`), meski bukan endpoint yang di-serve service ini.

Validasi kalau tersedia: `npx --yes @redocly/cli lint docs/api-documentation/openapi.yaml`.
Kalau tidak ada jaringan/npx, minimal cek YAML-nya parse.

### 3. Perbarui Postman

`docs/postman/payment-service.postman_collection.json` dan
`payment-service.local.postman_environment.json`:

- Satu folder per area (Payments, Refunds, Events, Webhooks, Admin, Health).
- Variabel environment: `base_url`, `api_key_id`, `api_secret`, `idempotency_key`.
- Pre-request script untuk `Idempotency-Key` acak dan HMAC signature bila dipakai.
- Test script minimal per request: cek status code + simpan `payment_id` ke variabel
  supaya request berikutnya bisa berantai.

Kalau bisa, jalankan terhadap sandbox:
`npx --yes newman run docs/postman/payment-service.postman_collection.json -e docs/postman/payment-service.local.postman_environment.json`

### 4. Temuan saat menyusun

Kalau ternyata kode dan desain tidak konsisten (mis. field beda nama, status code
tidak sesuai), **jangan diam-diam menyesuaikan dokumen ke kode yang salah**.
Dokumentasikan apa adanya, lalu parkir temuannya lewat `/park-issue`.

### 5. PR terpisah

```bash
git switch -c docs/sync-openapi-postman
```

Commit `docs(api): sinkronkan OpenAPI & koleksi Postman dengan route final`,
lalu tutup dengan skill `/pr`. Jangan campur dengan perubahan kode.
