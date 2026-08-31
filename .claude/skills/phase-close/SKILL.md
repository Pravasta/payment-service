---
name: phase-close
description: Tutup phase yang sudah di-merge — verifikasi acceptance criteria, tutup issue GitHub, update status dokumen phase, lalu kerjakan issue yang diparkir satu per satu. Gunakan saat user bilang phase sudah selesai/di-merge.
---

Dijalankan **setelah PR phase di-merge** dan `/after-merge` selesai.

## Langkah

### 1. Verifikasi acceptance criteria

Buka `docs/phases/NNN-slug/PRD.md` §5 dan cek satu per satu terhadap kode di `main`
yang sudah ter-merge. Centang yang benar-benar terpenuhi. Kalau ada yang **tidak**
terpenuhi:
- masih kecil dan jelas → jadikan issue baru lewat `/park-issue`;
- besar → sampaikan ke user bahwa phase belum layak ditutup, dengan alasannya.

Jangan mencentang sesuatu yang belum diverifikasi.

### 2. Tutup issue GitHub

```bash
gh issue list --search "Phase NNN" --state open
gh issue close <n> --comment "Selesai di PR #<pr>."
```

Tracking issue phase ditutup terakhir, dengan komentar ringkasan: apa yang selesai,
apa yang diparkir.

### 3. Update dokumen

- `docs/phases/NNN-slug/PRD.md` — Status → `done`, isi §6 dengan daftar issue yang
  diparkir selama phase.
- `docs/phases/NNN-slug/TASKS.md` — semua task `done`.
- `docs/phases/README.md` — status phase → `done` + nomor PR.

### 4. Ringkas temuan yang diparkir

```bash
grep -l "Ditemukan saat.*Phase NNN" docs/issues/*.md
```

Tampilkan ke user: nomor, judul, prioritas, dan rekomendasi urutan pengerjaan
(dependensi dulu, prioritas `high` dulu).

### 5. Kerjakan issue yang diparkir — satu per satu

Untuk setiap issue, **satu branch dan satu PR sendiri**:

```bash
git switch main && git pull --ff-only
git switch -c fix/issue-NNNN-<slug>
```

Kerjakan sesuai acceptance criteria di file issue → jalankan `/check` → tutup dengan
skill `/pr` (`Closes #<issue>`) → tunggu user merge → `/after-merge`. Baru lanjut ke
issue berikutnya. **Jangan menggabungkan beberapa issue dalam satu PR** kecuali user
meminta.

Setelah issue selesai, ubah `- **Status:** todo` → `done` di file issue-nya dan
perbarui `docs/issues/README.md`.

### 6. Siapkan phase berikutnya

Kalau masih ada phase tersisa di rencana (lihat brainstorming §10), tawarkan untuk
membukanya lewat skill `/phase`. Kalau semua phase sudah selesai, jalankan skill
`/api-docs` untuk menyinkronkan OpenAPI & Postman.
