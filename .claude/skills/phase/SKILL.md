---
name: phase
description: Buka phase pekerjaan baru — buat folder docs/phases/NNN-slug/ berisi PRD, TD, dan TASKS, buat issue GitHub via gh, lalu buat branch kerja. Gunakan saat user minta "mulai phase", "kerjakan phase berikutnya", atau memulai unit pekerjaan besar.
---

Membuka phase baru. **Jangan menulis kode implementasi apa pun di skill ini** —
skill ini hanya menyiapkan dokumen, issue, dan branch. Semua output dokumen dalam
Bahasa Indonesia.

## Langkah

### 1. Pastikan titik awal bersih

```bash
git status --porcelain          # harus kosong
git switch main && git pull --ff-only
```

Kalau working tree kotor, **berhenti** dan tanyakan ke user mau di-commit, di-stash,
atau dibuang. Jangan pernah membuang perubahan user sendiri.

### 2. Tentukan nomor phase

```bash
ls docs/phases | grep -E '^[0-9]{3}-' | sort | tail -1
```

Nomor berikutnya = tiga digit berurutan (`000`, `001`, …). Slug: kebab-case pendek
dari judul phase (mis. `restruktur-gin`).

### 3. Tulis PRD & TD

Salin template lalu isi — jangan biarkan placeholder tersisa:

```bash
mkdir -p docs/phases/NNN-slug
cp docs/phases/TEMPLATE-PRD.md docs/phases/NNN-slug/PRD.md
cp docs/phases/TEMPLATE-TD.md  docs/phases/NNN-slug/TD.md
```

Sumber isinya: brainstorming yang relevan di `docs/brainstorming/` (§rencana phase),
`docs/result/detailed-design.md`, dan kondisi kode saat ini. Kalau ada bagian yang
tidak bisa Anda putuskan sendiri (keputusan produk/bisnis), tulis di PRD §"Pertanyaan
terbuka" dan **tanyakan ke user sebelum lanjut ke langkah 4**.

Untuk phase yang menyentuh DOKU: verifikasi dulu endpoint/payload lewat MCP
`doku-mcp-server` (sandbox) dan tulis buktinya di TD §5. Jangan merancang dari asumsi.

### 4. Buat `TASKS.md`

Pecah TD §6 jadi task kecil yang masing-masing meninggalkan repo dalam keadaan hijau
(build + test lulus). Format:

```markdown
# Phase NNN — <Judul> · Tasks

| # | Task | Issue | Status |
|---|---|---|---|
| 1 | ... | #— | todo |
```

### 5. Konfirmasi ke user

Tampilkan ringkasan: judul phase, tujuan, jumlah task, dan daftar judul task.
**Tunggu persetujuan user** sebelum membuat issue GitHub — issue itu outward-facing
dan merepotkan kalau salah.

### 6. Buat issue GitHub

Satu tracking issue untuk phase, lalu satu issue per task:

```bash
gh issue create --title "Phase NNN — <Judul>" \
  --body-file docs/phases/NNN-slug/PRD.md \
  --label "phase"

gh issue create --title "[Phase NNN] <task>" --body "<konteks + acceptance criteria>"
```

Kalau label belum ada, buat dulu (`gh label create phase`) atau jalankan tanpa
`--label`. Catat nomor issue kembali ke kolom `Issue` di `TASKS.md`, dan daftar
sub-issue ke body tracking issue.

### 7. Buat branch dan commit dokumen

```bash
git switch -c feat/phase-NNN-<slug>
git add docs/phases/NNN-slug
git commit -m "docs(phase-NNN): PRD, TD, dan task untuk <judul>"
```

Branch `feat/...` untuk fitur, `refactor/...` untuk restrukturisasi murni.

### 8. Update indeks

Tambahkan baris phase ke tabel di `docs/phases/README.md` (status `in-progress`).

### 9. Lapor

Sebutkan: nomor & judul phase, path dokumen, nomor tracking issue, nama branch,
dan task pertama yang akan dikerjakan. Jangan push dan jangan buat PR di sini —
PR dibuat di akhir phase lewat skill `/pr`.

## Aturan selama phase berjalan

- Temuan/bug di luar scope phase: **jangan diperbaiki**. Pakai skill `/park-issue`.
- `docs/api-documentation/openapi.yaml` dan `docs/postman/*`: **jangan disentuh**.
  Disinkronkan di phase terakhir lewat skill `/api-docs`.
- Jalankan `/check` sebelum tiap commit.
