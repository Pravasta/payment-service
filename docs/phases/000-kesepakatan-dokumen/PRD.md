# Phase 000 — Kesepakatan & Dokumen · PRD

- **Status:** review
- **Branch:** `docs/phase-000-kesepakatan-dokumen`
- **Tracking issue:** #22
- **Depends on:** —

## 1. Masalah

Arah rombakan v2 sudah disepakati di `docs/brainstorming/0002-rombak-v2-doku-only.md`,
tapi bentuknya masih **brainstorming**: berisi opsi, rekomendasi, dan pertanyaan
terbuka. Dokumen semacam itu tidak bisa jadi rujukan saat menulis kode — setiap
kali ada keraguan, pembacanya harus menebak ulang mana opsi yang akhirnya dipilih.

Tiga hal yang membuat Phase 1 tidak aman dimulai sekarang:

1. **Keputusan K1–K9 belum dikunci.** Masih berupa "rekomendasi", bukan "keputusan".
2. **15 issue lama di `docs/issues/` belum ditriase.** Sebagian sudah `done`,
   sebagian jadi tidak relevan karena abstraksi multi-gateway dibuang, sebagian
   masih valid. Membawa semuanya masuk rombakan = membawa utang yang sudah lunas.
3. **Dokumen desain v1 masih terbaca seolah berlaku.** `detailed-design.md` dan
   `architecture-review.md` memuat multi-gateway dan fondasi dashboard revenue.
   Tanpa penanda, dokumen itu akan menyesatkan pengerjaan phase berikutnya.

## 2. Tujuan

- Setiap keputusan K1–K9 punya satu ADR bernomor: konteks, keputusan, konsekuensi.
- Setiap issue di `docs/issues/` punya keputusan eksplisit: `keep`, `superseded`,
  atau `dropped`, dengan alasannya.
- Dokumen desain v1 diberi penanda status sehingga tidak terbaca sebagai kebenaran
  saat ini.
- Phase 1 bisa dimulai tanpa satu pun pertanyaan arsitektural yang menggantung.

## 3. Non-goals

- **Tidak ada perubahan kode Go sama sekali.** Nol file `.go` disentuh.
- Tidak menulis ulang `detailed-design.md` — cukup diberi penanda status. Desain
  detail v2 tumbuh per-phase lewat TD masing-masing.
- Tidak menyelesaikan issue lama yang berstatus `keep` — itu dijadwalkan ke phase
  yang relevan, bukan dikerjakan di sini.
- Tidak menyentuh `docs/api-documentation/` dan `docs/postman/`.

## 4. User story / skenario

- Sebagai **pengembang yang membuka Phase 1**, saya ingin tahu persis framework,
  layout folder, dan batas abstraksi yang dipakai — tanpa membaca ulang seluruh
  brainstorming dan menebak mana yang jadi dipilih.
- Sebagai **reviewer PR**, saya ingin bisa menolak perubahan yang melanggar
  keputusan yang sudah dikunci, dengan menunjuk nomor ADR-nya.
- Sebagai **pemilik produk**, saya ingin tahu utang mana dari v1 yang masih harus
  dibayar dan mana yang hangus karena arah berubah.

## 5. Acceptance criteria

- [ ] `docs/adr/0001`–`0009` ada, masing-masing memuat: Status, Konteks, Keputusan,
      Konsekuensi (termasuk konsekuensi negatif yang diterima), dan Alternatif yang ditolak.
- [ ] Keempat jawaban user (akun DOKU per-app + fallback env; data Postgres boleh
      direset; urutan phase 0–8; rombak di repo ini) tercatat di ADR yang relevan.
- [ ] Dua asumsi yang diputuskan tanpa jawaban eksplisit (`GET /v1/events` ditunda
      ke Phase 4 sebagai opsional; `allowed_channels` disediakan sejak Phase 3)
      ditandai jelas sebagai **asumsi**, bukan keputusan user.
- [ ] Seluruh 15 issue di `docs/issues/` punya baris keputusan triase + alasan.
- [ ] `docs/issues/README.md` menampilkan kolom hasil triase.
- [ ] `detailed-design.md` dan `architecture-review.md` punya banner status di
      bagian paling atas.
- [ ] `docs/phases/README.md` terisi baris Phase 000.
- [ ] `gofmt -l .` bersih, `go build ./...` dan `go test ./...` sukses (tidak ada file Go yang berubah).
- [ ] `make lint` — **gagal, pre-existing**: 57 temuan yang jumlahnya identik di `main`. Diparkir sebagai issue 0017, bukan diperbaiki di phase dokumentasi.
- [ ] Tidak ada perubahan di `docs/api-documentation/` & `docs/postman/`.

## 6. Di luar cakupan / diparkir

- [0016](../../issues/0016-retensi-tabel-append-only.md) (#29) — kebijakan retensi
  `transaction_event` & `webhook_inbox`. Lahir dari ADR-0009 sendiri; dijadwalkan
  setelah Phase 8.
- [0017](../../issues/0017-lint-belum-bersih.md) (#30) — `make lint` gagal dengan 57
  temuan **pre-existing** (jumlah identik di `main`); Phase 000 nol menyentuh file
  Go. Dijadwalkan bersamaan Phase 1.

## 7. Catatan pelaksanaan

Satu asumsi di PRD ini meleset: triase diperkirakan akan memilah issue yang masih
terbuka, padahal **ke-15 issue sudah berstatus `done`**. Triasenya karena itu
menjawab pertanyaan lain — apakah hasilnya masih berlaku di bawah keputusan v2.
Hasilnya: 10 `berlaku`, 5 `dikerjakan ulang` (pindah paket/framework/nama tabel),
0 `dropped`.
