# 0004 — Idempotency middleware (write requests)

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0001
- **Referensi:** detailed-design §5

## Konteks

Semua POST yang menulis (create payment, refund) wajib `Idempotency-Key`, UNIQUE
per `app_id`. Key sama + payload sama → balikan resource sama. Key sama + payload
beda → `409 Conflict`.

## Scope

- [x] Middleware/util idempotency memanfaatkan constraint UNIQUE
      `(app_id, idempotency_key)` (tabel khusus `idempotency_key`).
- [x] Simpan ringkasan response pertama untuk dikembalikan saat retry.
- [x] Deteksi konflik payload (hash method+uri+body) → `409`.
- [x] Validasi header `Idempotency-Key` wajib pada endpoint tulis.

## Acceptance criteria

- [x] Dua request identik dengan key sama → satu resource, response konsisten
      (replay + header `Idempotent-Replay: true`).
- [x] Key sama payload beda → `409`.
- [x] Tanpa header pada endpoint tulis → `400`.
- [x] Aman pada race (uji konkuren `-race`, 12 goroutine) — handler tepat 1x.

## Catatan implementasi

- Dipilih **middleware-level response cache** (pola Stripe) dengan tabel khusus
  `idempotency_key`, bukan integrasi per-usecase. Alasan: generik & reusable
  untuk create + refund tanpa menyentuh usecase, dan menyimpan response final.
- Race-safety via `UNIQUE(app_id, idempotency_key)` + `gorm.ErrDuplicatedKey`
  (perlu `TranslateError: true` di GORM config).
- Handler `5xx` → record dibuang agar client boleh retry; `<500` di-cache.
- `sync` (POST) dikecualikan: idempoten natural & rate-limited (§4.3).
- DB unique constraint pada `transaction`/`refund` tetap jadi jaring pengaman
  lapis kedua saat usecase write diimplementasikan (issue 0006/0011).

## File terkait

- `internal/adapter/http/middleware/idempotency.go` (+ test)
- `internal/adapter/repository/idempotency_repository.go`
- `internal/adapter/repository/model/idempotency_key.go`
- `internal/infrastructure/database/postgres.go` (TranslateError)
- `internal/adapter/http/router.go`, `cmd/api/main.go`, `cmd/migrate/main.go`

## Di luar scope

- Store idempotency khusus (Redis) — constraint DB cukup untuk MVP.
