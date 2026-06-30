# Issues — Payment Service

Langkah implementasi dipecah menjadi unit kerja kecil agar dikerjakan **satu per
satu**. Urutan mengikuti dependensi (atas → bawah). Setiap issue punya scope,
acceptance criteria, dan referensi ke dokumen desain.

> Konvensi status: `todo` → `in-progress` → `done`. Update field **Status** di
> tiap file saat dikerjakan.

## Daftar issue

| # | Judul | Prioritas | Depends on |
|---|---|---|---|
| [0001](0001-data-model-lengkap.md) | Lengkapi data model & migrasi (sisa tabel) | high | — |
| [0002](0002-enkripsi-secret-at-rest.md) | Enkripsi secret at-rest (AES-GCM + master key) | high | 0001 |
| [0003](0003-auth-middleware.md) | Auth middleware (API key + HMAC signing) | high | 0001, 0002 |
| [0004](0004-idempotency-middleware.md) | Idempotency middleware (write requests) | high | 0001 |
| [0005](0005-doku-create-charge.md) | DOKU adapter: CreateCharge (Checkout) + signed request | high | 0002 |
| [0006](0006-usecase-create-payment.md) | Use-case CreatePayment + `POST /v1/payments` | high | 0003, 0004, 0005 |
| [0007](0007-read-endpoints.md) | Endpoint baca: Get & List payment | medium | 0006 |
| [0008](0008-webhook-receiver.md) | Webhook receiver DOKU + ParseWebhook + state machine | high | 0006 |
| [0009](0009-outbox-worker.md) | Outbox worker (callback ke app, retry/backoff/DLQ) | high | 0008 |
| [0010](0010-reconciler-sync.md) | Reconciler + `POST /sync` + DOKU GetStatus | medium | 0008 |
| [0011](0011-refund-end-to-end.md) | Refund end-to-end + DOKU Refund (per-channel) | medium | 0008 |
| [0012](0012-observability.md) | Observability dasar (metrik, log, correlation) | medium | 0006 |
| [0013](0013-doku-doc-corrections.md) | Koreksi dokumen DOKU agar sesuai response live | medium | 0005 |
| [0014](0014-postman-run-findings.md) | Temuan run Postman: Sync 500 guard + observability gagal-gateway | high | 0006, 0010 |
| [0015](0015-doku-getstatus-signature-amount.md) | DOKU GetStatus: signature GET tanpa Digest + amount angka | high | 0010 |

## Referensi desain

- `docs/result/detailed-design.md` — skema, kontrak API, state machine
- `docs/result/doku-integration-spec.md` — fakta DOKU & dampak desain
- `docs/result/scaffold-foundation.md` — fondasi kode saat ini
