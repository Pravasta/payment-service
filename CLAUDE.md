# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Bahasa

Jawab dan jelaskan dalam **Bahasa Indonesia**. Identifier, kode, dan istilah teknis tetap Bahasa Inggris.

## Apa ini

Payment Service terpusat (Go) untuk banyak aplikasi SaaS internal, dengan DOKU sebagai gateway pertama di balik interface yang bisa diperluas. Masih tahap awal — lihat `docs/` untuk desain dan `docs/issues/` untuk langkah implementasi yang dikerjakan **satu per satu**.

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
- **Format amount DOKU beda per-endpoint**: Checkout = integer rupiah (`"50000"`); VA/SNAP = string 2-desimal (`"50000.00"`). Diformat di adapter, bukan di domain.

## Arsitektur (Clean Architecture)

Dependensi mengarah ke dalam: `cmd → adapter → usecase → domain`.
- `internal/domain/` — entity, state machine, dan **port** (interface `Repository`, `Gateway`). Tanpa dependensi framework (no GORM/HTTP/DOKU).
- `internal/usecase/` — orkestrasi, hanya bergantung pada port domain.
- `internal/adapter/` — implementasi port: `http` (chi), `repository` (GORM), `gateway/doku`.
- `internal/infrastructure/` — config, database, logger.
- **Model GORM (`adapter/repository/model`) terpisah dari entity domain** — petakan via mapper, jangan pakai entity domain sebagai model DB.
- State pembayaran adalah **state machine**: hanya transisi maju yang sah (`payment.CanTransition`); `transaction_event` **append-only** (tidak pernah di-update/delete).

## Git

- Branch fitur `feat/...` (atau `fix/...`), commit gaya **Conventional Commits** (`feat:`, `fix:`, `docs:`), lalu PR ke `main`. Jangan commit langsung ke `main`.
- Push saat ini lewat HTTPS kredensial `gh` (SSH key belum terdaftar di GitHub).
