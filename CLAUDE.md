# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Bahasa

Jawab dan jelaskan dalam **Bahasa Indonesia**. Identifier, kode, dan istilah teknis tetap Bahasa Inggris.

## Apa ini

Payment Service terpusat (Go) untuk aplikasi SaaS internal (HR, KOL, Invoice, CRM),
dengan **DOKU sebagai satu-satunya gateway**. Scope-nya sempit dan disengaja:
**skema pembayaran + hook (webhook/callback) ke aplikasi asal**. Apa yang dibayar
adalah urusan aplikasi pemanggil.

Sedang dalam **rombakan v2** — lihat `docs/brainstorming/0002-rombak-v2-doku-only.md`
untuk arah, keputusan, dan rencana phase. `docs/` berisi desain, `docs/phases/`
berisi unit pekerjaan berjalan, `docs/issues/` berisi temuan yang diparkir.

## Alur kerja wajib

Selalu ikuti urutan ini. Skill-nya ada di `.claude/skills/`.

1. **Buka phase** — `/phase`: buat `docs/phases/NNN-slug/` (PRD + TD + TASKS),
   buat issue GitHub via `gh`, lalu buat branch kerja. Tidak ada kode ditulis
   sebelum PRD & TD ada dan disetujui user.
2. **Kerjakan task** satu per satu; jalankan `/check` sebelum tiap commit.
3. **Temuan di tengah jalan** — `/park-issue`: catat ke `docs/issues/NNNN-*.md`
   dengan nomor berurutan, **jangan diperbaiki sekarang**. Pengecualian hanya untuk
   blocker dan masalah keamanan — itu dilaporkan ke user seketika.
4. **Tutup dengan PR** — `/pr`: check, commit Conventional Commits, push, buka PR ke
   `main`. **Jangan merge.** User mereview dan merge manual.
5. **Setelah user bilang sudah di-merge** — `/after-merge`: kembali ke `main`, pull,
   hapus branch lokal & remote.
6. **Tutup phase** — `/phase-close`: verifikasi acceptance criteria, tutup issue
   GitHub, lalu kerjakan issue yang diparkir satu per satu (masing-masing branch
   `fix/issue-NNNN-*` + PR sendiri).
7. **Setelah semua phase selesai** — `/api-docs`: baru sinkronkan OpenAPI & Postman.

Aturan yang tidak bisa ditawar:
- **Jangan pernah commit langsung ke `main`.** Selalu branch baru.
- **Jangan pernah merge PR sendiri.**
- **`docs/api-documentation/` dan `docs/postman/` tidak disentuh selama phase berjalan.**
- **Lint dan test harus hijau sebelum PR** (`make lint`, `make test`, `gofmt -l .`).

## Perintah

Pakai `make` (lihat `Makefile`):
- `make run` — jalankan API; `make dev` — API dengan live reload (Air, butuh `air` terinstall)
- `make migrate` — buat/sesuaikan skema via **GORM AutoMigrate** (`cmd/migrate`) — bukan file SQL
- `make db-up` / `make up` — Postgres saja / semua service (docker compose)
- `make test` — `go test ./...`
- `make lint` — golangci-lint (config `.golangci.yml`; binary perlu diinstall: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- `/check` (skill) — build + vet + test + lint + cek gofmt sebelum commit

## Konfigurasi

- **Semua config via environment variable**, dibaca `joho/godotenv` (`internal/infrastructure/config`). Salin `.env.example` → `.env` (gitignored). **Bukan file TOML.**
- `.air.toml` adalah konfigurasi **Air** (live reload), **bukan** config aplikasi — jangan bingung.
- Secret (DB password, `DOKU_*`, `PAYMENTS_MASTER_KEY`) hanya lewat env; jangan commit.

## Gotcha penting

- **Ganti kredensial Postgres tidak otomatis berlaku.** `POSTGRES_USER/PASSWORD/DB` di `docker-compose.yml` hanya dipakai saat volume `pgdata` pertama kali dibuat. Setelah diubah, wajib reset volume: `docker compose down -v && docker compose up -d postgres` (menghapus data).
- **Uang selalu integer minor unit** (`bigint`), tidak pernah float. IDR `currency_exponent = 0` (rupiah utuh).
- **Format amount DOKU beda per-endpoint**: Checkout = integer rupiah (`"50000"`); VA/SNAP = string 2-desimal (`"50000.00"`). Diformat di gateway layer, bukan di domain.
- **Jangan merancang integrasi DOKU dari asumsi.** Verifikasi endpoint & payload
  lewat MCP `doku-mcp-server` (sandbox) dulu, tulis buktinya di TD. Beberapa asumsi
  dokumentasi sudah terbukti salah (mis. `expired_date_utc` tidak ada; body sukses
  Checkout dibungkus objek `response`).

## Arsitektur (Clean Architecture)

Alur selalu **Handler → Service → Repository**, dengan arah dependensi ke dalam:
`cmd → handler → service → domain ← repository/gateway`.

- `internal/domain/` — entity, state machine, dan **port** (interface `Repository`, gateway). Tanpa dependensi framework (no GORM/HTTP/DOKU).
- `internal/service/` — orkestrasi, hanya bergantung pada port domain.
- `internal/handler/` — delivery HTTP (**Gin**): router, handler, middleware, DTO.
- `internal/repository/` — implementasi GORM.
- `internal/gateway/doku/` — client DOKU + signature HMAC.
- `internal/infrastructure/` — config, database, logger, metrics, crypto.
- **Model GORM (`repository/.../model`) terpisah dari entity domain** — petakan via mapper, jangan pakai entity domain sebagai model DB.
- State pembayaran adalah **state machine**: hanya transisi maju yang sah (`payment.CanTransition`); `transaction_event` **append-only** (tidak pernah di-update/delete).

> Catatan transisi: layout & framework di atas adalah **target rombakan v2**
> (Phase 1–2). Kode saat ini masih memakai chi dan folder `internal/adapter/…` +
> `internal/usecase/…`. Ikuti struktur yang benar-benar ada saat membaca kode;
> ikuti target di atas saat menulis kode baru dalam phase restrukturisasi.

## Git

- Branch fitur `feat/...` (atau `fix/...`, `refactor/...`, `docs/...`), commit gaya **Conventional Commits** (`feat:`, `fix:`, `docs:`), lalu PR ke `main`. Jangan commit langsung ke `main`.
- PR dibuat lewat `gh` (sudah ter-autentikasi). Review & merge dilakukan user secara manual.
- **Push lewat HTTPS**, bukan SSH — SSH key belum terdaftar di GitHub sehingga
  `git push origin` gagal (`Permission denied (publickey)`). Pakai:
  `git push https://github.com/Pravasta/payment-service.git HEAD:<branch>`
