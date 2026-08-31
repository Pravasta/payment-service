# ADR-0005 — Kredensial DOKU per-app dengan fallback ke env

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** **jawaban user** (2026-08-31)

## Konteks

Ini pertanyaan bisnis, bukan teknis: apakah dana dari HR, KOL, Invoice, dan CRM
masuk ke satu akun DOKU yang sama, atau perlu terpisah per produk untuk keperluan
settlement?

Dua pilihan yang diajukan:

- satu akun DOKU untuk semua app — kredensial cukup dari env, tabel
  `gateway_account` dibuang;
- akun per-app — tabel `gateway_account` dipertahankan dan di-scope `app_id`.

Yang penting: **arah keputusan ini sulit dibalik.** Memecah satu akun jadi
beberapa setelah ada transaksi produksi berarti memecah riwayat settlement dan
merekonsiliasi ulang; menggabungkan beberapa jadi satu jauh lebih murah.

## Keputusan

Kami memakai **kredensial DOKU per-app, dengan fallback ke env**. Dipilih user
pada 2026-08-31.

- Tabel `gateway_account` dipertahankan, di-scope `app_id`.
- `config_enc` (Client-Id + Secret Key DOKU) terenkripsi AES-GCM dengan
  `PAYMENTS_MASTER_KEY` dari env — mekanisme yang sudah ada dan sudah teruji.
- Kolom `gateway` dibuang dari tabel ini (konsekuensi ADR-0001).
- **Fallback:** app yang belum punya baris `gateway_account` memakai kredensial env
  (`DOKU_CLIENT_ID` / `DOKU_SECRET_KEY`). Ini membuat development dan sandbox tetap
  sederhana — tidak perlu provisioning DB hanya untuk menjalankan service lokal.
- Resolusi kredensial terjadi di service layer per transaksi, bukan sekali saat
  boot. Client DOKU tidak boleh menyimpan kredensial sebagai state global.

## Konsekuensi

Lebih mudah:

- Settlement per produk bisa dipisah tanpa migrasi data belakangan.
- Rotasi kredensial bisa dilakukan per app, tidak memaksa semua app ikut berhenti.
- Development tetap ringan berkat fallback env.

Lebih sulit:

- Client DOKU tidak bisa lagi jadi singleton dengan kredensial tetap. Perlu
  resolusi + cache kredensial per app, plus invalidasi cache saat kredensial dirotasi.
- Jalur fallback berarti ada **dua sumber kredensial**. Kalau logging-nya tidak
  jelas, memburu "kenapa app ini pakai akun DOKU yang salah" jadi menyakitkan.
  Mitigasi: log sumber kredensial (`db` atau `env`) beserta `app_id` di level info
  saat charge dibuat — **tanpa pernah menulis nilai kredensialnya**.
- Satu jalur kode lagi yang harus di-test.

Konsekuensi turunan yang perlu dicatat untuk Phase 2: user menyatakan **data
Postgres existing boleh direset** (`docker compose down -v` + AutoMigrate ulang)
karena isinya hanya data uji sandbox. Jadi tidak perlu migrasi eksplisit untuk
perubahan skema di ADR-0004 dan ADR-0005.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Satu akun DOKU untuk semua app (env saja) | paling sederhana, tapi mencampur dana empat produk di satu rekening settlement; memisahnya nanti jauh lebih mahal daripada biaya tabel ini sekarang |
| Per-app tanpa fallback env | memaksa provisioning DB hanya untuk menjalankan service secara lokal; menghambat development tanpa manfaat nyata |
| Simpan kredensial di env per app (`DOKU_CRM_CLIENT_ID`, …) | tidak bisa dirotasi tanpa deploy ulang, dan jumlah variabel env tumbuh seiring jumlah app |
