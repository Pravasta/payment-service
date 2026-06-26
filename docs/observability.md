# Observability

Panduan metrik, log, dan correlation id untuk Payment Service
(implementasi issue 0012; referensi detailed-design §12, architecture-review §9.10).

## Endpoint metrik

- **API server**: `GET /metrics` pada port HTTP utama (`SERVER_PORT`, default `8080`).
  Tanpa auth — batasi aksesnya di level jaringan (hanya scrape internal).
- **Worker**: `GET /metrics` pada `METRICS_ADDR` (default `:9091`). Worker tidak
  punya HTTP server aplikasi, jadi endpoint metriknya berdiri sendiri.

Format Prometheus (text exposition). Setiap proses memakai registry sendiri dan
juga mengekspos metrik runtime Go & proses (goroutine, GC, memori, CPU).

Contoh `scrape_configs` Prometheus:

```yaml
scrape_configs:
  - job_name: payment-api
    static_configs: [{ targets: ["payment-api:8080"] }]
  - job_name: payment-worker
    static_configs: [{ targets: ["payment-worker:9091"] }]
```

## Metrik inti

| Metrik | Tipe | Label | Sumber | Arti |
|---|---|---|---|---|
| `http_requests_total` | counter | `method`, `route`, `status` | API | Volume & success-rate request HTTP (pakai pola route chi, kardinaltias rendah). |
| `http_request_duration_seconds` | histogram | `method`, `route` | API | Latency request HTTP. |
| `gateway_charge_total` | counter | `gateway`, `result` | API/worker | Create-charge ke gateway per hasil (`success`/`error`). |
| `gateway_charge_duration_seconds` | histogram | `gateway` | API/worker | Latency create-charge. |
| `webhook_total` | counter | `gateway`, `result` | API | Webhook diparse per hasil (`ok`/`failed`; `failed` = signature/parse gagal). |
| `outbox_dispatch_total` | counter | `result` | worker | Hasil pengiriman callback (`delivered`/`retry`/`dead`). |
| `outbox_backlog` | gauge | `status` | worker | Jumlah pesan outbox tertahan (`pending`/`dead`), di-refresh tiap siklus reconcile. |
| `reconcile_runs_total` | counter | — | worker | Jumlah siklus reconciler. |
| `reconcile_changed_total` | counter | — | worker | Transaksi yang status-nya diperbaiki reconciler (mismatch ditemukan). |
| `reconcile_errors_total` | counter | — | worker | Siklus reconciler yang gagal. |

Plus metrik bawaan `go_*` dan `process_*`.

## Correlation id (log)

- Middleware `RequestID` memastikan tiap request punya `X-Request-Id` (di-generate
  bila tak ada), dipantulkan kembali di response header, dan disimpan di context.
- Middleware `AccessLog` menulis satu baris log terstruktur (slog) per request:
  `request_id`, `method`, `path`, `status`, `bytes`, `duration_ms`. Status ≥ 500
  dilog sebagai `Error`, selain itu `Info`. Dengan `request_id` ini satu request
  bisa ditelusuri end-to-end.
- Worker melog kejadian kunci (outbox dispatch/retry/dead, reconciler) lewat slog.
  Callback ke app membawa `X-Event-Id` (correlation milik callback) untuk dedup.

## Alert minimal (contoh PromQL)

> Sesuaikan threshold dengan baseline trafik nyata.

**Error rate create-charge tinggi** — pembayaran gagal di gateway:
```promql
sum(rate(gateway_charge_total{result="error"}[5m]))
  / sum(rate(gateway_charge_total[5m])) > 0.1
```

**Webhook gagal beruntun** — signature/parse bermasalah (kemungkinan salah secret
atau serangan):
```promql
increase(webhook_total{result="failed"}[10m]) > 5
```

**Outbox menumpuk** — callback ke app tertahan:
```promql
outbox_backlog{status="pending"} > 100
```

**Dead-letter bertambah** — ada callback yang menyerah total (butuh replay manual):
```promql
increase(outbox_dispatch_total{result="dead"}[15m]) > 0
```

**Reconcile mismatch tinggi** — banyak status tidak sinkron dengan gateway
(indikasi webhook hilang):
```promql
increase(reconcile_changed_total[15m]) > 10
```

**Reconciler error** — polling gateway gagal:
```promql
increase(reconcile_errors_total[15m]) > 0
```

**Latency HTTP p99 tinggi**:
```promql
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, route)) > 1
```

## Di luar scope (saat ini)

- Tracing terdistribusi (OpenTelemetry) — ditambah bila lintas-service makin kompleks.
- Dashboard Grafana siap pakai — metrik di atas sudah cukup sebagai dasar panel.
