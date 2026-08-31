# ADR-0004 — `merchant` menjadi `app`

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K4

## Konteks

Entitas pemanggil Payment Service diberi nama `merchant` di desain v1. Nama itu
salah alamat: pemanggilnya adalah **aplikasi internal** (HR, KOL, Invoice, CRM),
bukan merchant. Merchant DOKU justru ada satu tingkat di atas — yaitu pemilik
service ini sendiri.

Akibatnya kalimat seperti "merchant mana yang punya transaksi ini" jadi ambigu:
merchant DOKU, atau aplikasi pemanggil? Ambiguitas ini akan makin mahal begitu
ADR-0005 memperkenalkan kredensial DOKU per-aplikasi, karena kedua konsep akan
muncul di satu kalimat yang sama.

## Keputusan

Kami mengganti nama entitas pemanggil dari `merchant` menjadi **`app`**.

- Tabel `merchant` → `app`; kolom `merchant_id` → `app_id` di seluruh tabel.
- `app.code` berisi identitas aplikasi: `hr`, `kol`, `invoice`, `crm`.
- Kolom `transaction.app_id` yang sudah ada tetap; namanya kini konsisten dengan
  tabel yang dirujuknya (sebelumnya `app_id` menunjuk ke tabel `merchant` — sumber
  kebingungan tersendiri).
- Semua query di-scope `app_id`. Tidak ada endpoint yang boleh mengembalikan data
  lintas-app.

Migrasi dieksekusi di Phase 2 lewat GORM AutoMigrate di atas volume Postgres yang
direset (lihat ADR-0005 §Konsekuensi dan jawaban user 2026-08-31).

## Konsekuensi

Lebih mudah:

- Tidak ada lagi ambiguitas antara "merchant DOKU" dan "aplikasi pemanggil".
- Ketidakcocokan `app_id` → tabel `merchant` hilang.

Lebih sulit:

- Rename menyentuh banyak file sekaligus: model GORM, mapper, repository, DTO,
  middleware auth, dan seluruh test yang menyebut `merchant`. Diff Phase 2 akan
  besar dan membosankan untuk direview.
- Kalau suatu saat service ini dipakai pihak di luar organisasi, istilah `app`
  akan terasa sempit. Diterima: itu bukan skenario yang direncanakan.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Tetap `merchant` | ambiguitas terus dibayar, dan makin parah setelah ADR-0005 |
| `tenant` | menyiratkan multi-tenancy pelanggan eksternal; yang dimaksud adalah aplikasi milik sendiri |
| `client` | bentrok dengan "HTTP client" dan dengan `Client-Id` milik DOKU |
| `consumer` | akurat tapi lebih panjang dan tidak dipakai dalam percakapan sehari-hari tim |
