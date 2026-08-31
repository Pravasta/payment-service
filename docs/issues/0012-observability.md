# 0012 — Observability dasar (metrik, log, correlation)

- **Status:** done
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** 0006
- **Referensi:** detailed-design §12, architecture-review §9.10
- **Triase rombakan v2:** berlaku — middleware di-port di Phase 1, diperluas di Phase 7
- **Alasan triase:** Metrik, correlation id, dan log terstruktur tetap; hanya bentuk middleware-nya yang mengikuti Gin.

## Konteks

Tanpa observability, "pembayaran sukses tapi langganan tak aktif" baru ketahuan
dari komplain. Butuh metrik & log terstruktur sejak awal.

## Scope

- [x] Correlation id (`X-Request-Id`, sudah ada middleware) diteruskan ke log,
      gateway call, dan callback. — `AccessLog` melog `request_id` per request;
      callback membawa `X-Event-Id`.
- [x] Metrik (Prometheus `/metrics`): success rate per gateway (`gateway_charge_total`),
      latency create-charge (`gateway_charge_duration_seconds`), webhook gagal
      (`webhook_total{result="failed"}`), backlog & DLQ outbox (`outbox_backlog`).
- [x] Log terstruktur (slog) di titik kunci: create/webhook (access log di boundary
      HTTP), outbox & reconciler (worker).
- [x] Alert (dok): contoh PromQL di `docs/observability.md` (webhook gagal beruntun,
      outbox menumpuk, recon mismatch, dll).

## Acceptance criteria

- [x] `/metrics` mengekspos metrik inti (API & worker).
- [x] Correlation id muncul konsisten di log satu request end-to-end.
- [x] Dashboard/alert minimal terdokumentasi (`docs/observability.md`).

## Catatan implementasi

- Metrik di registry per-proses (bukan default global) → mudah diuji, tak bocor antar test.
- Instrumentasi adapter lewat **interface kecil di sisi konsumen** (`doku.Observer`,
  `outbox.MetricsRecorder`, `middleware.MetricsRecorder`) + functional options nil-safe,
  sehingga `internal/adapter/**` tidak meng-import `infrastructure/metrics` (arah
  dependensi Clean Architecture tetap ke dalam) dan konstruktor/test lama tak berubah.
- Worker punya endpoint `/metrics` sendiri (`METRICS_ADDR`, default `:9091`) karena
  tak punya HTTP server aplikasi; gauge backlog via `OutboxRepository.Counts`.
- "Umur transaksi pending" belum diekspos sebagai gauge — backlog outbox + reconcile
  mismatch sudah cukup untuk alerting awal; bisa ditambah bila perlu.

## File terkait

- `internal/infrastructure/logger/`
- `internal/infrastructure/metrics/` — baru
- `internal/adapter/http/middleware/`

## Di luar scope

- Tracing terdistribusi (OpenTelemetry) — bila diperlukan nanti.
