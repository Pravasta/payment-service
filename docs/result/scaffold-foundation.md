# Scaffold & Fondasi Implementasi — Payment Service

> Dokumentasi kerangka kode awal (Clean Architecture, Go) yang menjadi dasar
> implementasi modul berikutnya. Belum ada business logic endpoint — lihat
> `docs/issues/` untuk langkah lanjutan yang dipecah satu per satu.
> Tanggal: 2026-06-22.

> ## ⚠️ Status: struktur folder digantikan rombakan v2 (2026-08-31)
>
> Layout `internal/adapter/…` + `internal/usecase/…` yang dijelaskan di sini
> digantikan layout flat by-layer `internal/{handler,service,repository,gateway,domain}`
> (ADR-0003), dan chi digantikan Gin (ADR-0002). Prinsip arah dependensi-nya tetap
> berlaku. Migrasinya dieksekusi di Phase 1.

## 1. Tujuan

Menyiapkan **struktur, wiring, dan fondasi teknis** yang rapi sebelum mengisi
logic. Yang sudah berdiri: layout Clean Architecture, koneksi DB (GORM),
konfigurasi berbasis environment, live-reload (Air), dan util kripto DOKU yang
sudah teruji. Endpoint bisnis sengaja masih kosong (`501`) agar dikerjakan
bertahap.

## 2. Stack & keputusan teknis

| Area | Pilihan | Catatan |
|---|---|---|
| Bahasa | Go 1.25 | sesuai brainstorming |
| HTTP router | `go-chi/chi/v5` | ringan, idiomatik, ramah middleware |
| ORM | `gorm.io/gorm` + driver `postgres` | sesuai permintaan |
| DB | PostgreSQL 15-alpine (Docker) | `docker-compose.yml` |
| Konfigurasi | **environment variable** via `joho/godotenv` | `.env` saat dev; env asli saat prod |
| Live reload | **Air** (`.air.toml`) | `make dev`, auto-rebuild saat save |
| Logging | `log/slog` (stdlib) | JSON di prod, text di dev |
| UUID | `google/uuid` | id semua entitas |
| JSONB | tipe custom `JSONMap` | tanpa dependensi tambahan |

> Catatan: konfigurasi memakai **env (godotenv)**, bukan file TOML. File TOML di
> repo (`.air.toml`) khusus untuk Air (live reload), bukan untuk app config.

## 3. Clean Architecture — aturan dependensi

Dependensi **selalu mengarah ke dalam**. Domain tidak tahu apa pun soal HTTP,
GORM, atau DOKU.

```
cmd  ──▶  adapter  ──▶  usecase  ──▶  domain
                 └────────────────────▲ (implementasi port)
infrastructure dipakai cmd untuk merakit (config, db, logger)
```

| Layer | Paket | Isi | Boleh impor |
|---|---|---|---|
| 1. Domain | `internal/domain/payment` | entity, state machine, **port** (`Repository`, `Gateway`) | std + uuid |
| 2. Use-case | `internal/usecase/payment` | interactor (orkestrasi) | domain |
| 3. Adapter | `internal/adapter/{http,repository,gateway/doku}` | delivery HTTP, GORM, adapter DOKU | usecase, domain |
| 4. Infra | `internal/infrastructure/{config,database,logger}` | framework & driver | std + lib |
| Entrypoint | `cmd/{api,worker,migrate}` | merakit & menjalankan | semua |

Pemetaan ke modul `detailed-design.md §11.2`: `api/`→`adapter/http`,
`payment/`→`domain`+`usecase`, `gateway/`→`adapter/gateway`,
`store/`→`adapter/repository`, `platform/`→`infrastructure`,
`outbox/`+`recon/`→`cmd/worker`.

## 4. Peta direktori

```
cmd/
  api/main.go            entrypoint HTTP server (+ graceful shutdown)
  worker/main.go         background: outbox + reconciler (loop scaffold)
  migrate/main.go        GORM AutoMigrate
internal/
  domain/payment/        transaction.go (entity+state machine), refund.go,
                         event.go, errors.go, repository.go (port), gateway.go (port)
  usecase/payment/       service.go (CreatePayment/Get/Sync/Refund — skeleton)
  adapter/
    http/                router.go, handler/, middleware/request_id.go
    repository/          payment_repository.go (impl port) + model/ (+ JSONMap)
    gateway/doku/        doku.go (impl port) + crypto.go (signature) + crypto_test.go
  infrastructure/
    config/config.go     loader env (godotenv)
    database/postgres.go koneksi GORM
    logger/logger.go     slog
.env.example             template environment (.env di-gitignore)
.air.toml                konfigurasi Air (live reload)
Dockerfile · docker-compose.yml · Makefile · README.md
```

## 5. Yang sudah berfungsi vs masih skeleton

**Berfungsi nyata:**
- Boot API + `/healthz`, `/readyz`, graceful shutdown.
- Koneksi GORM Postgres + pool; `cmd/migrate` AutoMigrate 3 model.
- Repository GORM penuh (CRUD + mapper domain↔model + JSONB).
- Util kripto DOKU: `Digest`, `Sign`, `VerifySignature`, `FormatAmount`
  (sesuai `doku-integration-spec.md §1/§4) — **ada unit test, lulus**.
- State machine canonical + transisi legal; `mapStatus` DOKU→canonical (7 status).
- Config env (godotenv) + override prod.

**Masih skeleton (lihat issues):**
- Endpoint `/v1/*` → `501 Not Implemented`.
- `usecase` method → `TODO`.
- DOKU `CreateCharge`/`ParseWebhook`/`GetStatus`/`Refund` → `TODO`.
- Tabel selain transaction/event/refund (merchant, api_credential,
  webhook_endpoint, gateway_account, webhook_inbox, notification_outbox).
- Enkripsi secret at-rest, auth/idempotency middleware, outbox worker, reconciler.

## 6. Menjalankan

```bash
make setup          # .env dari .env.example
make db-up          # Postgres 15-alpine
make migrate        # skema (AutoMigrate)
make run            # API :8080   |  make dev = live reload (Air)
make test           # test kripto DOKU
curl localhost:8080/healthz
```

Konfigurasi semua via env (lihat `.env.example`). Secret (DB password, DOKU
client/secret, master key) **tidak** disimpan di repo.

## 7. Verifikasi terakhir

`go build ./...` ✅ · `go vet ./...` ✅ · `go test ./...` ✅ ·
boot membaca `.env` ✅ (gagal hanya saat Postgres belum jalan, sesuai harapan).

## 8. Langkah berikutnya

Dipecah menjadi issue granular di **`docs/issues/`** agar dikerjakan satu per
satu. Lihat `docs/issues/README.md` untuk daftar & urutan.
