---
name: check
description: Jalankan pemeriksaan cepat sebelum commit untuk repo Go ini — build, vet, test, dan cek format. Gunakan saat user minta "cek", "verify", atau sebelum membuat commit/PR.
---

Jalankan pemeriksaan berikut dari root repo, lalu laporkan hasilnya ringkas dalam Bahasa Indonesia.

1. **Format** — `gofmt -l .`
   - Jika ada file ter-list (belum terformat), jalankan `gofmt -w` pada file tersebut dan sebutkan file yang dirapikan.
2. **Build** — `go build ./...`
3. **Vet** — `go vet ./...`
4. **Test** — `go test ./...`
5. **Lint** — `make lint` (golangci-lint). Jika binary belum terpasang, lewati dan sebutkan cara install: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`.

Aturan:
- Jalankan tetap walau ada langkah yang gagal, lalu rangkum SEMUA kegagalan (jangan berhenti di error pertama), kecuali build gagal total sehingga vet/test tak bermakna.
- Test yang butuh Postgres (integrasi) memerlukan DB hidup (`make db-up`); test unit (mis. crypto DOKU) tidak. Jika test gagal karena DB mati, sebutkan itu, bukan dianggap bug kode.
- Akhiri dengan status singkat: ✅ semua lolos, atau daftar yang perlu diperbaiki.
- Jangan commit apa pun — ini hanya pemeriksaan.
