---
name: pr
description: Tutup pekerjaan dengan Pull Request ke main — jalankan pemeriksaan, commit gaya Conventional Commits, push branch, lalu buka PR via gh untuk direview manual user. Gunakan saat sebuah phase/issue selesai atau user minta "buat PR".
---

Menutup satu unit pekerjaan. **Jangan pernah merge, dan jangan pernah push langsung
ke `main`.** User yang mereview dan merge secara manual.

## Prasyarat

```bash
git branch --show-current
```

Kalau hasilnya `main`, **berhenti**. Buat branch dulu (`feat/…`, `fix/…`,
`refactor/…`, `docs/…`) dan pindahkan perubahan ke sana.

## Langkah

### 1. Pemeriksaan

Jalankan skill `/check` (gofmt, build, vet, test, lint). **Semua harus lulus.**
Kalau ada yang gagal:
- gagal karena kode yang Anda tulis → perbaiki sekarang, ini bagian dari pekerjaan;
- gagal karena hal lain di luar scope → parkir lewat `/park-issue`, dan sebutkan
  di badan PR;
- test integrasi gagal karena Postgres mati → nyalakan (`make db-up`) lalu ulangi;
  jangan laporkan lulus kalau sebenarnya tidak dijalankan.

### 2. Review diff sendiri

```bash
git status --porcelain
git diff --stat
git diff
```

Cek: tidak ada secret/`.env`, tidak ada file sementara, tidak ada `fmt.Println`
sisa debug, dan tidak ada perubahan di `docs/api-documentation/` atau `docs/postman/`
(kecuali memang sedang menjalankan `/api-docs`).

### 3. Commit

Gaya **Conventional Commits**, pesan Bahasa Indonesia, scope diisi:

```
feat(payment): endpoint create payment via DOKU Checkout
fix(doku): GetStatus GET sign tanpa Digest
refactor(handler): pindah router chi ke Gin
docs(phase-001): PRD dan TD restruktur Gin
```

Pecah jadi beberapa commit logis kalau perubahannya beragam (mis. refactor terpisah
dari fitur). Jangan pakai `git add -A` membabi buta — periksa dulu daftar filenya.

### 4. Push

Remote `origin` memakai SSH, tapi SSH key **belum** terdaftar di GitHub —
`git push origin` akan gagal dengan `Permission denied (publickey)`. Push lewat
HTTPS (kredensial `gh`):

```bash
git push https://github.com/Pravasta/payment-service.git HEAD:"$(git branch --show-current)"
```

### 5. Buka PR

```bash
gh pr create --base main --title "<tipe>(<scope>): <ringkasan>" --body-file <file>
```

Badan PR (Bahasa Indonesia):

```markdown
## Ringkasan
2–4 kalimat: apa yang berubah dan kenapa.

## Phase / Issue
- Phase NNN — <judul> (`docs/phases/NNN-slug/`)
- Closes #<n>, Closes #<n>

## Perubahan utama
- ...

## Cara verifikasi
Perintah yang bisa user jalankan sendiri, dan apa yang seharusnya terlihat.

## Hasil pemeriksaan
- `gofmt` / `go build` / `go vet` / `go test` / `make lint` — hasilnya apa

## Temuan yang diparkir
- docs/issues/NNNN — <judul> (tidak diperbaiki di PR ini, sesuai aturan phase)

## Catatan untuk reviewer
Bagian yang paling perlu mata Anda, keputusan yang masih bisa diperdebatkan,
atau hal yang sengaja ditunda.
```

Pakai `Closes #n` hanya untuk issue yang benar-benar tuntas di PR ini.

### 6. Lapor

Berikan URL PR ke user dan sebutkan bahwa PR menunggu review manual.
**Berhenti di sini.** Jangan merge, jangan hapus branch, jangan `git pull` —
tunggu user memberi tahu bahwa PR sudah di-merge (lanjut ke skill `/after-merge`).
