# Phase NNN — <Judul> · Technical Design

- **Status:** draft | approved
- **PRD:** [PRD.md](PRD.md)
- **Referensi desain:** `docs/brainstorming/…`, `docs/result/…`

## 1. Ringkasan pendekatan

Beberapa kalimat: bentuk solusinya seperti apa, kenapa itu yang dipilih.

## 2. Perubahan struktur / paket

File & paket yang dibuat, dipindah, atau dihapus. Jaga arah dependensi:
`cmd → handler → service → domain ← repository/gateway`.

## 3. Kontrak

- Endpoint (method, path, scope, Idempotency-Key)
- Request/response DTO
- Event yang dipancarkan ke aplikasi

## 4. Skema data

Tabel/kolom/index yang berubah. Uang selalu `bigint` minor unit.
Migrasi lewat GORM AutoMigrate (`make migrate`), bukan file SQL.

## 5. Integrasi DOKU

Endpoint DOKU yang dipakai, format amount per-endpoint, skema signature,
dan **bukti verifikasi** (MCP sandbox / response nyata). Jangan menulis adapter
dari asumsi dokumentasi saja.

## 6. Urutan kerja

Langkah kecil yang masing-masing meninggalkan repo dalam keadaan hijau.

1. ...

## 7. Strategi test

Unit (domain, gateway crypto), handler (httptest + Gin), integrasi (Postgres).
Sebutkan test lama yang harus tetap hijau sebagai jaring pengaman.

## 8. Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| | | |
