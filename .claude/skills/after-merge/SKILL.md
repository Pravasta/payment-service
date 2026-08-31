---
name: after-merge
description: Bersih-bersih setelah user merge PR secara manual — kembali ke main, pull, dan hapus branch lokal & remote. Gunakan saat user bilang PR sudah di-merge atau minta "hapus branch dan pull".
---

Dipakai **hanya setelah user memberi tahu bahwa PR sudah di-merge**. Jangan
dijalankan atas inisiatif sendiri.

## Langkah

### 1. Pastikan PR memang sudah merged

```bash
gh pr status
gh pr view "$(git branch --show-current)" --json number,state,mergedAt
```

Kalau `state` bukan `MERGED`, **berhenti** dan beri tahu user — jangan hapus branch
yang masih punya pekerjaan belum masuk `main`.

### 2. Simpan nama branch, lalu kembali ke main

```bash
BRANCH="$(git branch --show-current)"
git status --porcelain      # harus kosong; kalau ada sisa, tanyakan ke user dulu
git switch main
git pull --ff-only
```

### 3. Hapus branch

```bash
git branch -d "$BRANCH"                  # -d, bukan -D: gagal kalau ada commit belum ter-merge
git push origin --delete "$BRANCH"       # lewati kalau GitHub sudah menghapusnya otomatis
git fetch --prune
```

Kalau `git branch -d` menolak, **jangan paksa dengan `-D`**. Laporkan ke user bahwa
ada commit yang belum masuk `main`.

### 4. Verifikasi & lapor

```bash
git log --oneline -3
git branch -a | head -20
```

Laporkan: commit terbaru di `main`, branch yang dihapus, dan langkah berikutnya
(task phase selanjutnya, atau issue yang diparkir kalau phase sudah selesai).
