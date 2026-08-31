# Phase 000 — Kesepakatan & Dokumen · Technical Design

- **Status:** approved
- **PRD:** [PRD.md](PRD.md)
- **Referensi desain:** `docs/brainstorming/0002-rombak-v2-doku-only.md` (§4 K1–K9, §10, §14)

## 1. Ringkasan pendekatan

Phase murni dokumentasi. Pola yang dipakai: **satu keputusan = satu ADR bernomor**
(Architecture Decision Record, format Nygard yang dipangkas). ADR dipilih daripada
menambah bab di `detailed-design.md` karena keputusan punya **umur dan status
sendiri** — sebuah ADR bisa jadi `superseded` tanpa membuat seluruh dokumen desain
ikut usang, dan riwayat "kenapa dulu diputuskan begitu" tetap terbaca.

Brainstorming tetap ada apa adanya sebagai jejak proses; ADR yang jadi rujukan.

## 2. Perubahan struktur / paket

Tidak ada paket Go yang berubah. Struktur dokumen:

```
docs/
  adr/                       # [baru]
    README.md                # indeks + cara menulis ADR baru
    0001-doku-only-tanpa-abstraksi-gateway.md
    0002-gin-sebagai-framework-http.md
    0003-layout-handler-service-repository.md
    0004-merchant-menjadi-app.md
    0005-kredensial-doku-per-app.md
    0006-hosted-checkout-dan-allowed-channels.md
    0007-kontrak-hook-ke-aplikasi.md
    0008-provisioning-via-adminctl.md
    0009-data-yang-dipertahankan-dan-dibuang.md
  phases/000-kesepakatan-dokumen/{PRD,TD,TASKS}.md
```

## 3. Kontrak

Tidak ada endpoint yang berubah di phase ini. ADR-0007 **mendokumentasikan**
kontrak hook yang akan diimplementasikan di Phase 4; itu spesifikasi, bukan kode.

## 4. Skema data

Tidak ada. ADR-0005 dan ADR-0009 memutuskan bentuk skema target; migrasinya
dieksekusi di Phase 2.

Keputusan yang berdampak ke Phase 2 dan perlu dicatat sekarang: **data Postgres
existing boleh direset** (`docker compose down -v` + AutoMigrate ulang), karena
isinya hanya data uji sandbox. Ini menghapus kebutuhan menulis migrasi rename
`merchant` → `app` secara eksplisit.

## 5. Integrasi DOKU

Tidak ada panggilan DOKU baru di phase ini. Fakta DOKU yang sudah terverifikasi
dan dipakai sebagai dasar ADR-0006:

- Sandbox `BRN-0252-…` mengaktifkan **30 channel / 6 kategori** (verifikasi live
  via MCP `doku-mcp-server`, 2026-08-31) — dasar keputusan memakai Hosted Checkout.
- Body sukses Checkout membungkus `order`/`payment` di objek top-level `response`.
- `expired_date_utc` **tidak ada** di respons nyata; pakai `payment.expired_datetime`.
- Format amount berbeda per-endpoint (Checkout integer, VA/SNAP dua desimal).

Sumber: `docs/result/doku-integration-spec.md` + `docs/issues/0013`.

## 6. Format ADR

Setiap file memakai kerangka yang sama, singkat dan bisa dibaca dalam satu menit:

```markdown
# ADR-NNNN — <Judul keputusan>

- **Status:** accepted | superseded by ADR-NNNN | deprecated
- **Tanggal:** YYYY-MM-DD
- **Sumber:** brainstorming §K<n>  |  jawaban user  |  asumsi (perlu konfirmasi)

## Konteks
Fakta dan kendala yang berlaku saat keputusan diambil. Bukan opini.

## Keputusan
Satu kalimat tegas, kalimat aktif: "Kami memakai X." Lalu detail secukupnya.

## Konsekuensi
Yang jadi lebih mudah **dan** yang jadi lebih sulit. Konsekuensi negatif wajib
ditulis — ADR tanpa konsekuensi negatif biasanya berarti trade-off-nya belum dipikirkan.

## Alternatif yang ditolak
Opsi lain + alasan penolakan, supaya tidak diperdebatkan ulang tiap beberapa bulan.
```

**Penandaan sumber itu penting.** ADR-0006 (bagian `allowed_channels`) dan ADR-0007
(bagian `GET /v1/events`) berasal dari **asumsi saya**, bukan jawaban eksplisit user.
Keduanya harus ditandai `asumsi (perlu konfirmasi)` supaya saat review terlihat mana
yang benar-benar sudah disepakati.

## 7. Metode triase issue

Untuk tiap file `docs/issues/NNNN-*.md`, tambahkan satu baris di bawah `Status`:

```markdown
- **Triase rombakan v2:** keep → Phase N | superseded oleh ADR-NNNN | dropped
- **Alasan triase:** <satu kalimat>
```

Kriteria:

| Hasil | Kapan dipakai |
|---|---|
| `keep → Phase N` | masih valid di arsitektur v2, dijadwalkan ke phase yang relevan |
| `superseded oleh ADR-NNNN` | masalahnya hilang atau berubah bentuk karena keputusan v2 |
| `dropped` | menyangkut hal yang keluar dari scope v2 (multi-gateway, dashboard) |
| `done` (tetap) | sudah selesai di v1 dan tetap berlaku — cukup dikonfirmasi |

Issue yang sudah `done` **tidak dibuka ulang** kecuali keputusan v2 membatalkannya.

## 8. Urutan kerja

Tiap langkah meninggalkan repo dalam keadaan bisa di-commit:

1. `docs/adr/README.md` — indeks + kerangka ADR.
2. ADR-0001 s/d 0003 (struktural: gateway, framework, layout).
3. ADR-0004 s/d 0006 (model app, kredensial, channel).
4. ADR-0007 s/d 0009 (hook, provisioning, data).
5. Triase 15 issue + kolom hasil triase di `docs/issues/README.md`.
6. Banner status di `detailed-design.md` & `architecture-review.md`; update
   `docs/phases/README.md` dan tautan ADR di `README.md`.

## 9. Strategi test

Tidak ada test otomatis untuk dokumen. Verifikasi yang dijalankan:

- `gofmt -l .` bersih dan `go build ./...` sukses — membuktikan nol file Go tersentuh.
- `git diff --stat main...HEAD` — memastikan tidak ada perubahan di
  `docs/api-documentation/`, `docs/postman/`, atau `internal/`.
- Cek silang manual: setiap K1–K9 di brainstorming §4 punya tepat satu ADR;
  setiap file di `docs/issues/` punya baris triase.

## 10. Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| ADR jadi salinan brainstorming, tidak menambah kejelasan | sedang | ADR wajib memuat **konsekuensi negatif** dan **alternatif yang ditolak** — dua bagian yang tidak ada di brainstorming |
| Asumsi saya terbaca sebagai keputusan user | sedang | field `Sumber:` di tiap ADR; dua ADR ditandai `asumsi (perlu konfirmasi)` |
| Triase terlalu agresif membuang issue yang masih valid | sedang | tiap `dropped`/`superseded` wajib punya alasan satu kalimat yang bisa dibantah saat review |
| Phase dokumentasi memakan waktu tanpa hasil terlihat | rendah | dibatasi 5 task; tidak ada penulisan ulang `detailed-design.md` |
