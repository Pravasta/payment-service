# Issues — Payment Service

Langkah implementasi dipecah menjadi unit kerja kecil agar dikerjakan **satu per
satu**. Urutan mengikuti dependensi (atas → bawah). Setiap issue punya scope,
acceptance criteria, dan referensi ke dokumen desain.

> Konvensi status: `todo` → `in-progress` → `done`. Update field **Status** di
> tiap file saat dikerjakan.

> Temuan yang muncul di tengah phase **diparkir** ke sini lewat skill
> `/park-issue` dan dikerjakan setelah phase selesai (skill `/phase-close`),
> masing-masing di branch `fix/issue-NNNN-*` dengan PR sendiri.

## Daftar issue

| # | Judul | Status | Triase rombakan v2 |
|---|---|---|---|
| [0001](0001-data-model-lengkap.md) | Lengkapi data model & migrasi (sisa tabel) | done | dikerjakan ulang di Phase 2 (ADR-0004, ADR-0009) |
| [0002](0002-enkripsi-secret-at-rest.md) | Enkripsi secret at-rest (AES-GCM + master key) | done | berlaku |
| [0003](0003-auth-middleware.md) | Auth middleware (API key + HMAC signing) | done | dikerjakan ulang di Phase 1 (ADR-0002) |
| [0004](0004-idempotency-middleware.md) | Idempotency middleware (write requests) | done | dikerjakan ulang di Phase 1 (ADR-0002) |
| [0005](0005-doku-create-charge.md) | DOKU adapter: CreateCharge (Checkout) + signed request | done | berlaku — disederhanakan di Phase 2 |
| [0006](0006-usecase-create-payment.md) | Use-case CreatePayment + `POST /v1/payments` | done | dikerjakan ulang di Phase 1 & 3 |
| [0007](0007-read-endpoints.md) | Endpoint baca: Get & List payment | done | dikerjakan ulang di Phase 3 |
| [0008](0008-webhook-receiver.md) | Webhook receiver DOKU + ParseWebhook + state machine | done | berlaku — diperluas di Phase 4 |
| [0009](0009-outbox-worker.md) | Outbox worker (callback ke app, retry/backoff/DLQ) | done | berlaku — dipindah di Phase 1 |
| [0010](0010-reconciler-sync.md) | Reconciler + `POST /sync` + DOKU GetStatus | done | berlaku — ditinjau di Phase 3 |
| [0011](0011-refund-end-to-end.md) | Refund end-to-end + DOKU Refund (per-channel) | done | berlaku — ditinjau di Phase 5 |
| [0012](0012-observability.md) | Observability dasar (metrik, log, correlation) | done | berlaku — diperluas di Phase 7 |
| [0013](0013-doku-doc-corrections.md) | Koreksi dokumen DOKU agar sesuai response live | done | berlaku |
| [0014](0014-postman-run-findings.md) | Temuan run Postman: Sync 500 guard + observability gagal-gateway | done | berlaku — diverifikasi di Phase 8 |
| [0015](0015-doku-getstatus-signature-amount.md) | DOKU GetStatus: signature GET tanpa Digest + amount angka | done | berlaku — jaga dari regresi di Phase 2 |

## Hasil triase rombakan v2 (Phase 000)

Ke-15 issue di atas sudah berstatus `done` saat rombakan v2 dimulai, jadi
triasenya menjawab pertanyaan yang berbeda: **apakah hasilnya masih berlaku di
bawah keputusan v2?** Tiga kemungkinan:

- **berlaku** — hasilnya utuh, tidak ada keputusan v2 yang membatalkannya;
- **dikerjakan ulang di Phase N** — hasilnya tetap dibutuhkan, tapi kodenya
  disentuh ulang oleh rombakan (pindah paket, ganti framework, ganti nama tabel);
- **superseded oleh ADR-NNNN** — sebagian hasilnya dibatalkan keputusan v2.

Alasan per issue ada di baris `Triase rombakan v2` di masing-masing file.
Tidak ada issue yang `dropped`.

## Referensi desain

- `docs/result/detailed-design.md` — skema, kontrak API, state machine
- `docs/result/doku-integration-spec.md` — fakta DOKU & dampak desain
- `docs/result/scaffold-foundation.md` — fondasi kode saat ini
- `docs/adr/` — keputusan rombakan v2 yang sudah dikunci
