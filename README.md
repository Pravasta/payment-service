# Payment Service

Service pembayaran terpusat (Go + PostgreSQL + Docker) untuk banyak aplikasi SaaS
internal, dengan DOKU sebagai gateway pertama di balik interface yang bisa
diperluas. Desain lengkap ada di `docs/`:

- `docs/brainstorming/` — ide & konteks awal, termasuk
  `0002-rombak-v2-doku-only.md` (arah rombakan v2: DOKU-only, payment + hook)
- `docs/phases/` — unit pekerjaan berjalan (PRD + TD + tasks per phase)
- `docs/result/architecture-review.md` — review arsitektur
- `docs/result/detailed-design.md` — desain detail (skema, API, state machine)
- `docs/result/doku-integration-spec.md` — fakta DOKU & dampaknya ke desain

## Arsitektur (Clean Architecture)

Dependensi selalu mengarah ke dalam (`cmd → adapter → usecase → domain`).
Domain tidak tahu apa-apa soal HTTP/GORM/DOKU.

```
cmd/
  api/         # entrypoint HTTP server
  worker/      # background: outbox dispatcher + reconciler
  migrate/     # GORM AutoMigrate
internal/
  domain/payment/      # [1] entities, state machine, PORTS (Repository, Gateway)
  usecase/payment/     # [2] application business rules (interactors)
  adapter/
    http/              # [3] delivery: router, handler, middleware
    repository/        # [3] GORM impl dari port Repository (+ model & mapper)
    gateway/doku/      # [3] DOKU adapter (impl port Gateway) + crypto signature
  infrastructure/
    config/            # [4] loader env (godotenv) — .env saat dev, env asli saat prod
    database/          # [4] koneksi GORM Postgres
    logger/            # [4] slog
.env.example           # template environment (.env di-gitignore)
.air.toml              # konfigurasi Air (live reload saat dev)
```

Pemetaan ke modul di `detailed-design.md §11.2`: `api/`→adapter/http,
`payment/`→domain+usecase, `gateway/`→adapter/gateway, `store/`→adapter/repository,
`platform/`→infrastructure, `outbox/` & `recon/`→cmd/worker.

## Menjalankan (lokal)

```bash
make setup        # salin .env.example -> .env, lalu sesuaikan
make db-up        # start Postgres 15-alpine via docker compose
make migrate      # buat skema (GORM AutoMigrate)
make run          # start API di :8080  (atau: make dev  -> live reload via Air)
curl localhost:8080/healthz
```

Atau semuanya via Docker:

```bash
make up           # build + start postgres & api
```

Konfigurasi semua lewat **environment variable** (lihat `.env.example`). Saat
development dimuat dari `.env` via godotenv; di produksi env di-set langsung.
Live reload saat ngoding pakai **Air** (`make dev`).

## Status implementasi

Scaffold — struktur, wiring, koneksi DB, dan util crypto DOKU (signature/digest,
ada test) sudah jalan. Endpoint `/v1/*` masih `501 Not Implemented`.
Urutan pengisian modul ada di `docs/result/doku-integration-spec.md §8`.

```bash
make test   # menjalankan test crypto DOKU
```
