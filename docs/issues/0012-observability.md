# 0012 — Observability dasar (metrik, log, correlation)

- **Status:** todo
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** 0006
- **Referensi:** detailed-design §12, architecture-review §9.10

## Konteks

Tanpa observability, "pembayaran sukses tapi langganan tak aktif" baru ketahuan
dari komplain. Butuh metrik & log terstruktur sejak awal.

## Scope

- [ ] Correlation id (`X-Request-Id`, sudah ada middleware) diteruskan ke log,
      gateway call, dan callback.
- [ ] Metrik (mis. Prometheus `/metrics`): success rate per gateway, latency
      create-charge, jumlah webhook gagal, umur transaksi pending, ukuran outbox/DLQ.
- [ ] Log terstruktur (slog) di titik kunci: create, webhook, outbox, reconciler.
- [ ] Alert (dok): webhook gagal beruntun, outbox menumpuk, recon mismatch.

## Acceptance criteria

- [ ] `/metrics` mengekspos metrik inti.
- [ ] Correlation id muncul konsisten di log satu request end-to-end.
- [ ] Dashboard/alert minimal terdokumentasi.

## File terkait

- `internal/infrastructure/logger/`
- `internal/infrastructure/metrics/` — baru
- `internal/adapter/http/middleware/`

## Di luar scope

- Tracing terdistribusi (OpenTelemetry) — bila diperlukan nanti.
