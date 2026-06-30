# 0015 — DOKU GetStatus: signature GET (tanpa Digest) + amount angka

- **Status:** done
- **Prioritas:** high
- **Depends on:** 0010
- **Referensi:** verifikasi live run Postman 2026-06-30 (#8 Sync 500); `internal/adapter/gateway/doku/client.go`, `crypto.go`, `doku.go`

## Konteks

Setelah `.env` `DOKU_SECRET_KEY` dibetulkan (issue 0014), Create Payment hijau
(`201` + `payment_url`), tetapi **Sync (`POST /v1/payments/{id}/sync`) tetap 500**
untuk transaksi `pending`. Diagnosis live mengungkap **dua bug** di jalur
`GetStatus` (`GET /orders/v1/status/{invoice_number}`):

### Bug 1 — Signature GET menyertakan Digest (DOKU menolak)

Bukti live (diagnostic ke `api-sandbox.doku.com`):

```
[GET dengan Digest]  HTTP 400  {"error":{"code":"invalid_signature","message":"Invalid Header Signature"}}
[GET tanpa Digest]   HTTP 200  {"order":{...,"status":"ORDER_GENERATED"},"transaction":{"status":"PENDING"},...}
```

`doSigned` selalu menghitung `Digest(body)` dan menyertakan baris `Digest:` di
component string + header `Digest`, juga untuk **GET tanpa body**. DOKU
mengharuskan request **tanpa body TIDAK menyertakan Digest** (baik header maupun
komponen tanda tangan). CreateCharge (POST, ada body) tak terdampak → itu sebabnya
Create hijau tapi Sync gagal.

### Bug 2 — `order.amount` Check Status berupa ANGKA, struct membaca string

Response Check Status nyata: `"order":{"amount":50000,...}` (angka), sedangkan
Checkout/notifikasi memakai string (`"50000.00"`). Struct `statusResponse.Order.Amount`
bertipe `string` → `json.Unmarshal` gagal pada angka → GetStatus error → 500 (tetap
gagal walau signature sudah benar).

## Perbaikan

- **`crypto.go` `ComponentString`**: hilangkan baris `Digest` bila `digest == ""`.
  Backward-compatible — caller lama (POST, webhook) selalu mengirim digest non-kosong.
- **`client.go` `doSigned`**: hitung Digest & set header `Digest`/`Content-Type`
  **hanya** bila `len(body) > 0`. Untuk GET → tanda tangan tanpa Digest.
- **`doku.go` `statusResponse.Order.Amount`** → `json.RawMessage`; helper `rawAmount`
  menormalkan angka/“string” sebelum `parseAmountMinor` (tahan kedua bentuk).

## Verifikasi

- Live (adapter→DOKU sandbox): `GetStatus OK: status=pending amount=50000` (sebelumnya
  `Invalid Header Signature`).
- Unit test:
  - `TestComponentString_OmitsDigestWhenEmpty` (crypto_test.go).
  - `TestGetStatus_SignedAndParsed` diperbarui: assert signature **tanpa** Digest,
    header `Digest` **tidak diset**, fixture `order.amount` berupa **angka**.
- `/check` hijau.

## Acceptance criteria

- [x] `POST /sync` atas transaksi `pending` valid → bukan 500 (gateway menerima
      signature; response ter-parse).
- [x] GET DOKU ditandatangani tanpa Digest; POST tetap dengan Digest.
- [x] `order.amount` angka maupun string ter-parse benar.
- [x] `make test` hijau; `/check` lolos.

## Di luar scope

- `service.id`/`acquirer.id`/`channel.id` pada Check Status untuk order belum
  dibayar masih kosong (`""`) — wajar (belum ada channel final); tidak diubah.
- Pemetaan status DOKU `ORDER_GENERATED` (order belum dibayar) → saat ini
  `transaction.status` (`PENDING`) yang dipakai untuk mapping; sudah benar.
