---
name: park-issue
description: Parkir temuan/bug yang muncul di tengah pekerjaan ke docs/issues/ dengan nomor berurutan, tanpa memperbaikinya sekarang. Gunakan setiap kali menemukan bug, inkonsistensi, atau utang teknis yang di luar scope phase yang sedang dikerjakan.
---

Saat menemukan masalah di tengah phase: **catat, jangan perbaiki**. Memperbaiki
temuan sambil jalan membuat PR jadi campur aduk dan sulit direview manual.

## Kapan dipakai

Pakai skill ini saat menemukan: bug di kode existing, asumsi DOKU yang ternyata
salah, inkonsistensi kontrak API, utang teknis, test yang rapuh, atau kesempatan
refactor — **selama itu bukan bagian dari scope phase yang sedang dikerjakan**.

**Pengecualian — jangan diparkir, lapor ke user sekarang:** temuan yang membuat
phase berjalan tidak mungkin diselesaikan (blocker), atau masalah keamanan
(kebocoran secret, signature tidak diverifikasi, data lintas-app bocor). Untuk itu
berhenti dan minta keputusan user.

## Langkah

### 1. Cari nomor berikutnya

```bash
ls docs/issues | grep -Eo '^[0-9]{4}' | sort -n | tail -1
```

Nomor berikutnya = angka itu + 1, empat digit. Nama file:
`docs/issues/NNNN-<slug-kebab-case>.md`.

### 2. Tulis file issue

Ikuti format yang sudah dipakai issue lama (lihat `docs/issues/0013-*.md`):

```markdown
# NNNN — <Judul ringkas>

- **Status:** todo
- **Prioritas:** high | medium | low
- **Estimasi:** S | M | L
- **Depends on:** <nomor issue lain> | —
- **Ditemukan saat:** Phase NNN — <judul phase>
- **Referensi:** <file:baris, dokumen desain, PR>

## Konteks

Apa yang sedang dikerjakan saat temuan ini muncul.

## Temuan

Fakta, bukan dugaan. Sertakan bukti: potongan kode `file.go:12`, output error,
atau response DOKU yang sebenarnya.

## Dampak

Apa yang rusak / berisiko rusak, dan untuk siapa. Kalau belum berdampak nyata,
katakan itu.

## Usulan solusi

Arah perbaikan, bukan patch lengkap. Sebutkan alternatif kalau ada.

## Acceptance criteria

- [ ] ...
```

Isi dalam Bahasa Indonesia; identifier dan istilah teknis tetap Inggris.

### 3. Daftarkan ke indeks

Tambahkan baris ke tabel di `docs/issues/README.md`, mengikuti urutan nomor:

```markdown
| [NNNN](NNNN-slug.md) | <Judul> | <prioritas> | <depends on> |
```

### 4. Buat issue GitHub

```bash
gh issue create --title "NNNN — <Judul>" --body-file docs/issues/NNNN-slug.md
```

Tulis nomor issue GitHub kembali ke baris `- **Referensi:**` di file. Lewati
langkah ini kalau user bilang tidak perlu.

### 5. Commit dan lanjutkan

Commit terpisah dari pekerjaan phase supaya jejaknya jelas:

```bash
git add docs/issues && git commit -m "docs(issues): parkir NNNN — <judul>"
```

Lalu **kembali ke task phase yang sedang dikerjakan**. Sebutkan ke user dalam satu
kalimat bahwa temuan sudah diparkir sebagai issue NNNN dan pekerjaan dilanjutkan —
jangan berhenti untuk membahasnya.

## Kapan issue yang diparkir dikerjakan

Setelah phase selesai (lihat skill `/phase-close`). Tiap issue dikerjakan di branch
`fix/issue-NNNN-<slug>` sendiri dengan PR sendiri.
