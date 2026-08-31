# Brainstorming Rombakan v2 — Payment Service (DOKU-only)

- **Tanggal:** 2026-08-31
- **Status:** draft untuk direview — belum ada keputusan yang dikunci
- **Menggantikan arah:** `docs/brainstorming/brainstorming.md` (v1, multi-gateway + dashboard revenue)
- **Konsumen service:** HR, KOL, Invoice, dan **CRM** (aplikasi yang sedang dibangun)

---

## 1. Ringkasan eksekutif

Rombakan ini **mempersempit** Payment Service, bukan memperbesarnya.

| Aspek | v1 (sekarang) | v2 (rombakan) |
|---|---|---|
| Gateway | DOKU + abstraksi untuk Midtrans/Xendit/Tripay | **DOKU saja**, tanpa lapisan multi-gateway |
| Cakupan | Payment + fondasi revenue platform + dashboard | **Payment + hook (webhook/callback) saja** |
| Framework HTTP | chi | **Gin** |
| Penamaan layer | `handler → usecase → repository` | **`handler → service → repository`** |
| Dashboard/analytics | direncanakan | **dibuang** (tidak ada, tidak disiapkan) |
| Provisioning app | implisit lewat dashboard | **CLI `adminctl`** (tanpa dashboard) |

Alasan utama: perizinan gateway lain ribet dan Anda hanya siap dengan DOKU.
Abstraksi multi-gateway yang tidak akan pernah dipakai adalah biaya rancang,
biaya baca, dan biaya test yang tidak menghasilkan apa-apa. Rombakan ini
menukar keluwesan hipotetis itu dengan **kejelasan dan kecepatan**.

Rombakan ini **bukan tulis-ulang dari nol**. Bagian yang sudah terbukti hidup
(adapter DOKU, signature HMAC, state machine, outbox, enkripsi AES-GCM)
dipertahankan dan dipindahkan. Lihat §11.

---

## 2. Masalah dengan desain v1

Bukan karena v1 salah, tapi karena asumsinya berubah.

1. **Abstraksi multi-gateway tidak berbayar.** Port `payment.Gateway`
   (`CreateCharge`/`ParseWebhook`/`GetStatus`/`Refund`) memaksa setiap fakta DOKU
   diterjemahkan ke bentuk "canonical" lalu diterjemahkan balik di adapter.
   Dengan satu gateway, lapisan terjemahan ini murni overhead — dan sudah terbukti
   bocor: `gateway_request_id`, `RefundType` (VOID/PARTIAL/FULL), dan format amount
   per-endpoint semuanya konsep DOKU yang naik ke domain.
2. **Scope creep ke revenue platform.** v1 menyimpan fondasi untuk dashboard
   revenue lintas aplikasi. Itu produk lain. Selama dia menempel di sini, setiap
   keputusan payment ikut dipertimbangkan dari sisi reporting.
3. **Istilah `merchant` menyesatkan.** Pemanggil service ini bukan merchant, tapi
   **aplikasi internal** Anda (HR/KOL/Invoice/CRM). Merchant DOKU-nya justru Anda
   sendiri, satu tingkat di atas.
4. **`usecase` bukan kosakata yang Anda pakai.** Anda berpikir dalam
   `handler → service → repository`. Struktur folder sebaiknya mengikuti kepala
   pemiliknya.
5. **Utang yang menumpuk di `docs/issues/`.** 15 issue, sebagian koreksi atas
   asumsi DOKU yang salah. Rombakan adalah momen tepat untuk memangkasnya.

---

## 3. Prinsip & batas scope

**Prinsip inti:** Payment Service tahu *ada sejumlah uang yang harus dibayar*,
dan tidak pernah tahu *untuk apa*.

### Di dalam scope (in)

- Membuat pembayaran ke DOKU dan mengembalikan `payment_url`.
- Menyimpan state pembayaran + jejak event (append-only).
- Menerima webhook DOKU, memverifikasi signature, memutakhirkan state.
- **Hook keluar**: mengirim callback ke aplikasi asal secara andal (retry, DLQ, signature).
- Sinkronisasi status atas permintaan (`/sync`) dan reconciler terjadwal.
- Refund (penuh & sebagian).
- Idempotency di tiga arah.
- Provisioning aplikasi & kredensial lewat CLI.

### Di luar scope (out) — eksplisit, jangan diselundupkan masuk

- Produk, paket, harga, diskon, pajak, kupon → milik aplikasi asal.
- Invoice, langganan, siklus tagihan, dunning → milik aplikasi asal.
- Customer/tenant/organization → PS hanya menyimpan `customer_ref` opaque.
- Dashboard, laporan, agregasi revenue, chart.
- Gateway selain DOKU.
- Payout/settlement ke rekening, rekonsiliasi bank.
- Ledger akuntansi double-entry.

> Aturan uji cepat: kalau sebuah field membuat PS harus **menafsirkan** arti bisnis
> dari uang itu, field tersebut di luar scope. Simpan di `metadata` (opaque) atau
> jangan simpan sama sekali.

---

## 4. Keputusan arsitektur (opsi + rekomendasi)

### K1 — Seberapa jauh membuang abstraksi gateway?

| Opsi | Isi | Trade-off |
|---|---|---|
| A. Buang total | Service memanggil `doku.Client` langsung | Paling sederhana; tapi service jadi sulit di-test tanpa HTTP mock |
| B. **Satu interface tipis (rekomendasi)** | `service` bergantung pada interface `DokuGateway` yang **bicara istilah DOKU**, bukan istilah canonical | Tetap mudah di-test (fake gateway), tanpa lapisan terjemahan ganda |
| C. Pertahankan port canonical | seperti sekarang | Biaya tanpa manfaat selama gateway hanya satu |

**Rekomendasi: B.** Interface tetap ada **untuk seam testing**, bukan untuk
portabilitas gateway. Konsekuensinya boleh jujur menyebut `RequestID`,
`InvoiceNumber`, `ChannelID` di signature-nya.

Yang ikut dibuang bersama abstraksi:
- kolom `transaction.gateway` (selalu `"doku"`),
- tabel `gateway_account.gateway`,
- pemetaan status berlapis (`DOKU → canonical → DOKU`) — cukup satu arah.

Yang **tetap dipertahankan**: `payment.Status` canonical internal
(`created/pending/paid/settled/expired/failed/partially_refunded/refunded`) dan
`CanTransition`. Ini bukan abstraksi gateway — ini **kontrak ke aplikasi Anda**,
dan justru harus stabil meski istilah DOKU berubah.

### K2 — chi → Gin

Diminta. Dampak nyata yang harus dianggarkan:

- 8 middleware ditulis ulang ke `gin.HandlerFunc`: request-id, access log, metrics,
  recovery, auth (API key + scope), idempotency, strip-slash.
- Handler berubah dari `(w, r)` ke `*gin.Context`; binding pakai
  `ShouldBindJSON` + tag `binding:"required"` (validator v10) — ini justru
  **menghapus** banyak validasi manual di DTO sekarang.
- Hati-hati: jangan pakai `gin.Default()` (logger+recovery bawaan) karena kita
  punya slog terstruktur sendiri. Pakai `gin.New()` + middleware sendiri, dan set
  `gin.SetMode(gin.ReleaseMode)` di produksi.
- Idempotency middleware perlu membaca-ulang body → pakai
  `c.ShouldBindBodyWith` atau simpan body di context; jangan `io.ReadAll` dua kali.

### K3 — Layout folder

| Opsi | Bentuk | Catatan |
|---|---|---|
| A. Minimal | tetap `adapter/…`, rename `usecase`→`service` | perubahan kecil, tapi jalur `handler→service→repository` tidak terbaca dari struktur |
| B. **Flat by-layer (rekomendasi)** | `internal/{handler,service,repository,gateway,domain,infrastructure}` | persis mencerminkan alur yang Anda inginkan; tetap Clean Architecture karena arah dependensi tidak berubah |

**Rekomendasi: B** (detail tree di §5). Clean Architecture dijaga oleh **arah
dependensi**, bukan oleh nama folder `adapter/`. Aturan yang tetap berlaku:
`domain` tidak boleh meng-import Gin, GORM, atau DOKU.

### K4 — `merchant` → `app`

Rename entitas pemanggil jadi `app` dengan `code` = `hr` | `kol` | `invoice` | `crm`.
Semua query di-scope `app_id`. Satu app = satu set kredensial + satu (atau lebih)
webhook endpoint. Nol ambiguitas dengan "merchant DOKU".

### K5 — Kredensial DOKU: satu untuk semua app, atau per-app?

Ini **keputusan bisnis, bukan teknis**: apakah dana HR, KOL, Invoice, dan CRM
masuk ke satu akun DOKU yang sama, atau perlu dipisah per produk untuk settlement?

| Opsi | Implikasi |
|---|---|
| A. Satu akun DOKU (env `DOKU_CLIENT_ID`/`DOKU_SECRET_KEY`) | paling sederhana; tabel `gateway_account` **dibuang**; semua dana bercampur di satu rekening settlement |
| B. **Per-app (rekomendasi)** | tabel `gateway_account` tetap ada, di-scope `app_id`, secret terenkripsi AES-GCM; **fallback ke env** kalau app belum punya baris sendiri |

**Rekomendasi: B dengan fallback.** Biayanya kecil (tabelnya sudah ada dan sudah
terenkripsi), dan mengubahnya nanti setelah ada transaksi produksi jauh lebih mahal.
→ **Perlu konfirmasi Anda** (lihat §15).

### K6 — Metode pembayaran: Checkout atau direct per-channel?

Sandbox Anda (live via MCP, `BRN-0252-…`) mengaktifkan **30 channel / 6 kategori**:

| Kategori | Channel |
|---|---|
| Virtual Account | 17 bank (BCA, Mandiri, BNI, BRI, BSI, Permata, CIMB, Danamon, BTN, Maybank, OCBC, BJB, Sinarmas, BNC, BSS, BPD Bali, DOKU) |
| E-Wallet | OVO, DANA, ShopeePay, LinkAja, i.saku |
| BNPL / Installment | Kredivo, Akulaku, Indodana, BRI Ceria |
| Convenience Store | Alfamart, Indomaret |
| Credit Card | CREDIT_CARD |
| Payment Link | DOKU Customer Form |

| Opsi | Kerja | Hasil |
|---|---|---|
| A. **Hosted Checkout / redirect (rekomendasi)** | 1 integrasi | ke-30 channel langsung tersedia; UI & 3DS ditangani DOKU; **tidak menyentuh PCI-DSS** |
| B. Direct API per channel | 6+ integrasi, tiap channel beda payload/notifikasi/refund | kontrol UI penuh; CC direct menarik beban PCI ke Anda |

**Rekomendasi: A.** Tambahkan field opsional `allowed_channels` di request agar app
bisa membatasi (mis. CRM hanya VA + QRIS). Direct API ditunda sampai ada alasan
produk yang konkret — dan kalau perlu, cukup channel tertentu saja (mis. VA statis),
bukan semuanya.

### K7 — Bentuk "hook" ke aplikasi

Ini setengah dari nilai service ini, jadi jangan setengah-setengah:

- **Push (utama):** PS `POST` ke `webhook_endpoint.url` milik app, ditandatangani
  HMAC-SHA256 dengan secret per-endpoint, header `X-Event-Id` untuk dedup.
- **At-least-once, bukan exactly-once.** App **wajib** idempoten atas `event_id`.
- **Outbox + retry backoff** (mis. 0s, 30s, 2m, 10m, 1h, 6h → DLQ), status
  `pending/sent/failed/dead`.
- **Pull (jaring pengaman):** `GET /v1/events?since=…` supaya app bisa mengejar
  ketinggalan tanpa perlu Anda melakukan replay manual.
- **Replay manual:** `POST /v1/admin/outbox/{id}/replay` (sudah ada, dipertahankan).
- Event: `payment.pending`, `payment.paid`, `payment.expired`, `payment.failed`,
  `payment.refunded`, `payment.partially_refunded`.

### K8 — Provisioning tanpa dashboard

Karena dashboard dibuang, provisioning butuh jalur lain: **`cmd/adminctl`**.

```
adminctl app create   --code crm --name "CRM SaaS"
adminctl key issue    --app crm --scopes payments:read,payments:write
adminctl key revoke   --key-id pk_live_xxx
adminctl hook set     --app crm --url https://crm.internal/hooks/payment
adminctl hook rotate  --app crm
adminctl txn get      --app crm --ref INV-2026-000123
```

Secret hanya ditampilkan sekali saat dibuat. Ini juga menggantikan `cmd/seed`.

### K9 — Data: apa yang dibuang, apa yang tetap

- **Tetap:** `transaction_event` (append-only). Murah, dan ini satu-satunya cara
  menjawab "kenapa transaksi ini jadi failed" tiga bulan kemudian. Bukan analytics.
- **Tetap:** `webhook_inbox` (payload mentah DOKU) — bukti saat sengketa.
- **Dibuang:** semua rencana tabel/kolom agregasi revenue.
- **Dibuang:** kolom `gateway` generik.

---

## 5. Target struktur kode

```
cmd/
  api/            # Gin HTTP server
  worker/         # outbox dispatcher + reconciler
  migrate/        # GORM AutoMigrate
  adminctl/       # provisioning CLI (pengganti dashboard)  [baru]
internal/
  domain/
    payment/      # entity, Status + CanTransition, error, PORT (Repository, DokuGateway)
  handler/        # [Gin] router.go, payment_handler.go, webhook_handler.go, health, admin
    dto/          # request/response + tag binding
    middleware/   # requestid, accesslog, metrics, recovery, auth, idempotency
  service/
    payment/      # CreatePayment, GetPayment, ListPayments, Sync, Refund, HandleWebhook
    notification/ # penyusunan event + dispatch outbox
  repository/
    postgres/     # implementasi GORM
      model/      # model DB (TERPISAH dari entity domain) + mapper
  gateway/
    doku/         # client HTTP, signature HMAC, format amount, mapper status
  infrastructure/
    config/ database/ logger/ metrics/ crypto/
```

Arah dependensi (tidak berubah dari Clean Architecture):
`cmd → handler → service → domain ← repository/gateway`.
`domain` tidak meng-import apa pun dari Gin/GORM/DOKU.

---

## 6. Data model (ramping)

| Tabel | Peran | Perubahan vs v1 |
|---|---|---|
| `app` | aplikasi pemanggil (hr/kol/invoice/crm) | rename dari `merchant` |
| `api_credential` | auth app → PS (key_id + argon2id hash + scopes) | tetap |
| `webhook_endpoint` | tujuan hook PS → app + signing secret | tetap |
| `gateway_account` | kredensial DOKU per app (terenkripsi), fallback env | kolom `gateway` dibuang (lihat K5) |
| `transaction` | state pembayaran saat ini | kolom `gateway` dibuang; `gateway_request_id` tetap (wajib untuk GetStatus & Refund) |
| `transaction_event` | append-only, audit | tetap |
| `webhook_inbox` | payload mentah DOKU + dedup `gateway_event_id` | tetap |
| `notification_outbox` | hook keluar + retry/DLQ | tetap |
| `refund` | refund penuh/sebagian | tetap |
| `idempotency_key` | replay-safe untuk endpoint tulis | tetap |

Aturan uang yang **tidak berubah**: selalu `bigint` minor unit, tidak pernah float.
IDR `currency_exponent = 0`. Format amount ke DOKU berbeda per-endpoint
(Checkout integer `"50000"`, VA/SNAP dua desimal `"50000.00"`) dan **diformat di
gateway layer, bukan di domain**.

---

## 7. Kontrak API v1 (usulan)

| Method | Path | Scope | Idempotency-Key | Catatan |
|---|---|---|---|---|
| POST | `/v1/payments` | `payments:write` | wajib | balikan `payment_url` |
| GET | `/v1/payments/{id}` | `payments:read` | — | |
| GET | `/v1/payments?external_reference=…` | `payments:read` | — | |
| GET | `/v1/payments` (list + filter + cursor) | `payments:read` | — | untuk rekonsiliasi app |
| POST | `/v1/payments/{id}/sync` | `payments:write` | — | rate-limited 1×/10s per txn |
| POST | `/v1/payments/{id}/refunds` | `refunds:write` | wajib | |
| GET | `/v1/events?since=…` | `events:read` | — | **baru** — pull fallback untuk hook |
| POST | `/v1/admin/outbox/{id}/replay` | `outbox:admin` | — | |
| POST | `/v1/webhooks/doku` | — (HMAC DOKU) | — | inbound dari gateway |
| GET | `/healthz`, `/readyz`, `/metrics` | — | — | |

Perubahan dari v1: `GET /v1/transactions` dilebur ke `GET /v1/payments`
(satu istilah saja: **payment**), dan `GET /v1/events` ditambahkan.

---

## 8. Reliability & idempotency (tiga arah)

| Arah | Kunci | Perilaku |
|---|---|---|
| App → PS | header `Idempotency-Key` (unik per app) | request ulang mengembalikan resource yang sama, tidak membuat transaksi baru |
| DOKU → PS | `gateway_event_id` (Request-Id notifikasi) | webhook duplikat di-drop; state machine forward-only menolak event out-of-order (mis. `expired` setelah `paid`) |
| PS → App | `event_id` di body + `X-Event-Id` | at-least-once; app wajib dedup |

Jaring pengaman berlapis: webhook (utama) → `/sync` on-demand (app yang polling) →
reconciler terjadwal (menyapu transaksi `pending` yang lewat `expires_at`).

---

## 9. Security

- **App → PS:** API key (`key_id` + secret, hash argon2id) + scope per kredensial.
  HMAC request signing opsional (`X-Signature` + `X-Timestamp`, window ±5 menit).
- **DOKU → PS:** verifikasi `Signature: HMACSHA256=…` atas component string
  (`Client-Id`, `Request-Id`, `Request-Timestamp`, `Request-Target`, `Digest`).
  Tolak sebelum parsing body.
- **PS → App:** HMAC-SHA256 per webhook endpoint, secret bisa dirotasi
  (dukung dua secret aktif saat rotasi).
- **At-rest:** semua secret gateway & signing terenkripsi AES-GCM dengan
  `PAYMENTS_MASTER_KEY` dari env. Tidak pernah masuk log, tidak pernah di-commit.
- **Log:** tidak boleh memuat secret, header `Authorization`, atau body webhook mentah
  yang berisi data kartu.

---

## 10. Rencana phase

Setiap phase = **satu branch + satu PR** ke `main`, direview manual oleh Anda.
Setiap phase dibuka dengan PRD + TD di `docs/phases/NNN-slug/` dan issue GitHub.

| Phase | Judul | Deliverable | Exit criteria |
|---|---|---|---|
| **0** | Kesepakatan & dokumen | PRD+TD rombakan, ADR keputusan K1–K9 terkunci, `docs/issues/` lama ditriase | Anda approve arah; issue lama ditandai keep/drop |
| **1** | Restruktur + Gin | chi→Gin, layout `handler/service/repository`, semua test hijau | **tanpa perubahan perilaku**; `make test` + `make lint` bersih |
| **2** | Simplifikasi DOKU-only | buang abstraksi multi-gateway, `merchant`→`app`, migrasi skema | skema baru ter-migrate; adapter DOKU bicara istilah DOKU |
| **3** | Core payment flow | create (Checkout) / get / list / sync + state machine | happy path sandbox end-to-end |
| **4** | Hook system | webhook DOKU in, outbox out, retry/DLQ, signature, `GET /v1/events` | callback sampai ke app dummy; retry & DLQ terbukti |
| **5** | Refund | refund penuh & sebagian, per-channel | refund sandbox berhasil, state konsisten |
| **6** | `adminctl` + rotasi secret | provisioning CLI | app CRM bisa di-provision tanpa SQL manual |
| **7** | Hardening & observability | rate limit, metrik, structured log, audit lint | lint bersih, metrik terekspos |
| **8** | Dokumentasi akhir | OpenAPI + Postman + panduan integrasi (CRM lebih dulu) | koleksi Postman jalan penuh terhadap sandbox |

**Catatan alur kerja:** temuan/bug yang muncul di tengah phase **tidak langsung
diperbaiki** — diparkir sebagai issue bernomor di `docs/issues/` dan dikerjakan
setelah phase selesai. OpenAPI & Postman **hanya** disentuh di Phase 8.

---

## 11. Yang dipertahankan dari kode sekarang

Rombakan ini memindahkan dan menyederhanakan, bukan membuang:

| Aset | Nasib |
|---|---|
| `gateway/doku/crypto.go` (signature HMAC) | **dipertahankan utuh** — sudah live-verified |
| `gateway/doku/doku.go` (Checkout, GetStatus, Refund) | dipertahankan, disederhanakan (buang lapisan canonical) |
| `domain/payment` state machine + `CanTransition` | dipertahankan |
| `outbox/dispatcher.go` | dipindah ke `service/notification` |
| `infrastructure/crypto` (AES-GCM) | dipertahankan utuh |
| `middleware/*` (auth, idempotency, requestid, metrics) | logikanya dipertahankan, **di-port ke Gin** |
| `repository/*` + `model/*` (GORM) | dipertahankan, disesuaikan skema baru |
| `cmd/seed` | digantikan `cmd/adminctl` |
| Test yang ada (crypto, webhook, sync, refund) | dipertahankan sebagai jaring pengaman selama migrasi Gin |

Perkiraan kasar: ~70% kode existing terpakai lagi.

---

## 12. Risiko & mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Migrasi Gin merusak perilaku diam-diam | tinggi | Phase 1 **tanpa perubahan fitur**; test lama harus tetap hijau sebelum apa pun ditambah |
| Rename tabel `merchant`→`app` di DB yang sudah ada data | sedang | belum ada data produksi → `down -v` + AutoMigrate ulang; kalau sudah ada, tulis migrasi eksplisit |
| Terlalu bergantung pada DOKU (vendor lock-in) | diterima sadar | konsekuensi keputusan bisnis; `payment.Status` canonical internal tetap menjaga kontrak ke app |
| Asumsi DOKU salah lagi (seperti `expired_date_utc`) | sedang | verifikasi tiap endpoint lewat MCP sandbox **sebelum** menulis adapter, bukan sesudah |
| Scope diam-diam melebar ke dashboard | sedang | §3 "out of scope" dijadikan checklist review PR |
| Utang 15 issue lama terbawa | rendah | ditriase eksplisit di Phase 0 |

---

## 13. Non-goals (ditegaskan ulang)

Tidak dibangun sekarang **dan tidak disiapkan tempatnya**: dashboard, analytics,
gateway kedua, payout, ledger, langganan, invoice, manajemen produk/harga,
multi-currency di luar IDR (struktur `currency_exponent` sudah ada, tapi tidak
diuji di luar IDR).

---

## 14. Pertanyaan terbuka — butuh keputusan Anda

1. **K5 — akun DOKU:** satu akun untuk semua app, atau per-app? (rekomendasi: per-app dengan fallback env)
2. **Data existing:** ada data yang harus diselamatkan, atau boleh reset volume Postgres saat Phase 2?
3. **CRM duluan:** apakah Phase 3–4 harus dipacu supaya CRM bisa integrasi lebih awal, meski refund (Phase 5) belum ada?
4. **`GET /v1/events`:** perlu dari awal, atau cukup push + replay manual dulu?
5. **`allowed_channels`:** perlu pembatasan channel per app/transaksi, atau semua channel selalu tersedia?
6. **Repo:** rombak di repo ini (rekomendasi, riwayat & issue terjaga) atau repo baru?

---

## 15. Referensi

- `docs/brainstorming/brainstorming.md` — brainstorming v1
- `docs/result/detailed-design.md` — desain detail v1 (masih relevan untuk data model & state machine)
- `docs/result/doku-integration-spec.md` — fakta DOKU terverifikasi (signature, amount, expiry, refund)
- `docs/issues/` — 15 issue v1 yang perlu ditriase di Phase 0
- DOKU sandbox via MCP `doku-mcp-server` — daftar channel aktif (§K6), diverifikasi 2026-08-31
