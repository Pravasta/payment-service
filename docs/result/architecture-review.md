# Architecture Review — Payment Service & Revenue Platform

> Peran: Software Architect.
> Status: Diskusi arsitektur (belum implementasi).
> Tanggal: 2026-06-22.

Dokumen ini menjawab seluruh poin di `docs/brainstorming/brainstorming.md`:
review ide awal, kritik, alternatif desain, trade-off, rekomendasi MVP, evolusi
6–24 bulan, dan hal-hal penting yang berpotensi terlewat.

---

## 0. Ringkasan Eksekutif (TL;DR)

1. **Instinct utama Anda benar.** Memisahkan pembayaran ke satu service terpusat
   yang *tidak tahu business logic* adalah keputusan arsitektur yang tepat dan
   akan terbukti benar dalam 1–2 tahun ke depan. Pertahankan.

2. **Satu kesalahan paling mahal yang harus dihindari sejak hari pertama:**
   memperlakukan tabel transaksi sebagai *mutable state* dan men-*overwrite*
   status. Sebaliknya, **simpan setiap perubahan sebagai event yang immutable
   (append-only).** Data yang tidak Anda simpan hari ini tidak bisa diciptakan
   ulang besok. Ini fondasi untuk revenue dashboard Anda.

3. **Jangan bangun dashboard/analytics di dalam Payment Service.** Payment
   Service cukup menjadi *source of truth untuk fakta pembayaran* dan
   memancarkan event. Reporting adalah service terpisah (bisa ditunda), tapi
   *data mentahnya* harus ditangkap sejak awal.

4. **Untuk MVP: modular monolith, bukan microservices.** Satu service Go,
   satu PostgreSQL, satu gateway (DOKU) di balik sebuah interface. Hindari
   message broker, multiple service, dan event bus untuk sekarang — tapi
   desain *boundary*-nya seolah-olah suatu hari akan dipecah.

5. **Tiga hal teknis yang non-negotiable sejak MVP:** (a) uang disimpan sebagai
   integer minor unit + currency, jangan pernah float; (b) idempotency key di
   setiap operasi tulis; (c) verifikasi signature di webhook gateway dan jangan
   pernah percaya `amount`/`status` dari pihak luar tanpa rekonsiliasi.

---

## 1. Review Ide Awal — Apa yang Sudah Benar

| Keputusan Anda | Penilaian | Alasan |
|---|---|---|
| Centralized payment service, app tidak integrasi langsung ke gateway | ✅ Tepat | Kredensial gateway terpusat, satu titik audit, satu tempat menambah gateway, app tidak perlu PCI/compliance burden. |
| Payment Service tidak tahu business logic (plan, pricing, tenant) | ✅ Tepat | Ini yang membuat service *reusable* lintas Invoice/POS/HR/Payroll. Begitu PS tahu "subscription", ia berhenti generik. |
| Subscription, plan, customer tetap di app asal | ✅ Tepat | Ownership ada di domain yang mengerti maknanya. PS hanya proses uang. |
| Abstraksi gateway untuk masa depan (Midtrans/Xendit/Tripay) | ✅ Tepat arah | Asal diimplementasi sebagai *anti-corruption layer*, bukan sekadar `if gateway == "doku"`. |
| MVP satu gateway dulu, hindari over-engineering | ✅ Sehat | Tapi "fondasi yang baik" perlu definisi konkret — lihat §10. |
| Stack Go + PostgreSQL + Docker + REST | ✅ Cocok | Go sangat pas untuk service I/O-bound + concurrency webhook. Postgres cukup sampai skala besar. |

**Kesimpulan review:** desainnya bukan hanya "tidak salah", tapi memang ini
pola yang dipakai perusahaan payment dewasa (Stripe, Adyen, Midtrans internal).
Yang perlu diperketat adalah *bagaimana* boundary dan data model-nya, bukan
*apakah* boundary-nya benar.

---

## 2. Kritik — Bagian yang Berpotensi Menjadi Masalah

### 2.1. "Payment Service tidak tahu apa-apa" terlalu ekstrem untuk visi revenue
Ada tension nyata di dokumen Anda: di satu sisi PS harus *buta* terhadap
business logic; di sisi lain Anda mau dashboard revenue per aplikasi, per
produk, pertumbuhan, dsb.

Jika PS benar-benar tidak menyimpan *konteks* apa pun, Anda tidak akan bisa
membuat laporan "revenue POS SaaS bulan ini" tanpa menggabungkan data dari
banyak app. Solusinya bukan membuat PS paham business logic, melainkan:

> PS tetap buta terhadap **makna**, tapi menyimpan **label/metadata opaque**
> yang dikirim app: `app_id`, `product_code`, `customer_ref`, `metadata` (JSON
> bebas). PS tidak menafsirkannya — ia hanya menyimpan & meng-*group by*-nya.

Ini membuat PS tetap generik **dan** menjadi sumber data analitik yang kaya.

### 2.2. Risiko menjadikan PS sebagai "satu-satunya source of truth revenue"
Hati-hati dengan istilah *revenue*. Yang dimiliki PS adalah **fakta pembayaran**
(uang masuk/keluar, kapan, lewat gateway apa, sukses/gagal). Itu **bukan** sama
dengan *recognized revenue* secara akuntansi:

- Pembayaran sukses ≠ revenue (ada refund, chargeback, partial refund).
- Gross amount ≠ net (gateway memotong fee — lihat §9.4).
- Subscription dibayar di muka 12 bulan ≠ revenue bulan ini (revenue recognition).

**Kritik:** kalau dashboard Anda menyebut angka PS sebagai "revenue", suatu hari
angkanya tidak akan cocok dengan pembukuan. Posisikan PS sebagai *source of
truth untuk cash/payment events*, dan biarkan layer reporting/finance yang
menafsirkannya menjadi revenue.

### 2.3. Webhook forwarding ke app adalah titik kegagalan yang sering diremehkan
Alur "gateway → PS → app" punya dua hop yang bisa gagal independen. Pertanyaan
yang belum terjawab di brainstorming: *apa yang terjadi kalau app Anda sedang
down saat PS mencoba mengirim notifikasi status?* Tanpa **outbox + retry**, Anda
akan kehilangan notifikasi dan status app jadi tidak konsisten dengan PS. Ini
penyebab #1 keluhan "pembayaran sukses tapi langganan tidak aktif".

### 2.4. Status pembayaran bukan boolean — ini state machine
Brainstorming menyebut "pending/success/failed", tapi realita gateway lebih
kaya: `created → pending → authorized → paid → settled`, plus
`expired/failed/cancelled/refunded/partially_refunded/disputed`. Kalau model
status tidak dirancang sebagai **state machine eksplisit** dengan transisi yang
diizinkan, Anda akan dapat bug seperti webhook `expired` datang *setelah* `paid`
(out-of-order) dan menimpa status yang benar. Lihat §9.2.

### 2.5. Idempotency & out-of-order webhook tidak boleh "ditambahkan nanti"
Gateway *akan* mengirim webhook yang sama berkali-kali dan kadang tidak urut.
Klien Anda *akan* me-retry request. Kalau idempotency bukan bagian dari desain
inti sejak awal, retrofit-nya menyakitkan dan rawan double-charge / double-grant.

### 2.6. "Tidak over-engineering" bisa salah arah jika diterjemahkan sebagai "skip fondasi"
Over-engineering yang harus dihindari: microservices, Kafka, CQRS, multi-region
sekarang. Tapi *append-only event log*, *idempotency*, *integer money*, dan
*signature verification* **bukan** over-engineering — itu garis dasar payment
system. Membedakan keduanya adalah inti dokumen ini.

---

## 3. Scope & Responsibility (Topic 1)

### 3.1. Tanggung jawab Payment Service (DO own)
- Menerima permintaan pembayaran dan membuat **payment transaction**.
- Abstraksi & komunikasi ke payment gateway (create charge, redirect/VA/QRIS).
- Menerima & memverifikasi **webhook** dari gateway.
- Mengelola **state machine** status pembayaran (canonical, lintas gateway).
- **Idempotency**, retry, dan reliable delivery notifikasi ke app (outbox).
- **Audit trail & event log** immutable untuk setiap transaksi.
- **Rekonsiliasi** status PS dengan settlement report gateway.
- Refund / partial refund (mengeksekusi perintah, bukan memutuskan kebijakan).
- Menyimpan metadata opaque dari app untuk keperluan reporting.

### 3.2. Tanggung jawab app pemanggil (PS must NOT own)
- Plan, pricing, diskon, kupon, tax rules.
- Subscription lifecycle (aktif, grace period, dunning).
- Tenant / organization / customer sebagai entitas bisnis.
- Keputusan *apa* yang terjadi setelah pembayaran sukses (grant akses, dll).
- Invoice/billing semantik (PS hanya tahu "ada amount X harus dibayar").

### 3.3. Yang sebaiknya TIDAK dimiliki PS
- Data kartu / kredensial pembayaran end-user (jaga **PCI scope** = nol dengan
  selalu pakai hosted page / redirect / VA gateway — jangan pernah sentuh PAN).
- Logika revenue recognition / akuntansi (itu domain finance/reporting).
- UI dashboard bisnis (itu service/aplikasi lain).

**Prinsip pemandu:** *Payment Service memindahkan uang dan melaporkan fakta. Ia
tidak pernah memutuskan apa arti uang itu bagi bisnis.*

---

## 4. Data Ownership (Topic 2)

### 4.1. Pembagian ideal

| Data | Pemilik | Catatan |
|---|---|---|
| `transaction` (amount, currency, status, gateway) | **Payment Service** | Source of truth fakta pembayaran. |
| `transaction_event` (append-only log) | **Payment Service** | Immutable. Tidak pernah di-update. |
| `webhook_inbox` (raw payload mentah) | **Payment Service** | Simpan apa adanya untuk audit/replay. |
| `refund` | **Payment Service** | Terhubung ke transaksi. |
| `gateway_config` / credentials | **Payment Service** | Terenkripsi / secret manager. |
| `app` / `merchant` + API credential | **Payment Service** | Identitas pemanggil. |
| Plan, pricing, subscription | **App asal** | PS hanya terima `amount`. |
| Customer, tenant, organization | **App asal** | PS hanya simpan `customer_ref` opaque. |
| Invoice / order semantics | **App asal** | PS terima `external_reference_id`. |

### 4.2. Kontrak penghubung (yang membuat ownership tetap bersih)
Dua field ini adalah "lem" antara PS dan app, dan harus dirancang serius:

- **`external_reference_id`** — ID milik app (mis. invoice id / order id).
  Dipakai sebagai **idempotency anchor**: app boleh kirim request yang sama dua
  kali, PS mengembalikan transaksi yang sama, bukan membuat dobel.
- **`app_id` / `merchant_id`** — mengidentifikasi app pemanggil; semua query &
  reporting di-*scope* per ini. Juga basis multi-tenancy & isolasi kredensial.

> Aturan emas: PS tidak boleh menyimpan data yang *bisa berubah maknanya* di
> sisi app (mis. nama plan). Kalau perlu untuk reporting, simpan sebagai
> snapshot/label opaque di `metadata`, dengan kesadaran bahwa itu copy, bukan
> source of truth.

---

## 5. Multi-Application Architecture (Topic 3)

### 5.1. Model: "App sebagai merchant/client"
Perlakukan setiap aplikasi SaaS sebagai sebuah **merchant** (atau "client")
internal di PS:

```
merchant (app_id, name, status)
  └── api_credential (key_id, hashed_secret, scopes, status)
  └── webhook_endpoint (url, signing_secret)   ← untuk callback ke app
```

- Identifikasi transaksi antar app: **selalu** lewat `app_id` + `external_ref`.
- Isolasi: query, rate limit, kredensial, dan webhook endpoint semuanya
  per-merchant. Satu app tidak bisa melihat transaksi app lain.
- Reusability dijaga dengan: API yang **generik** (tidak ada field spesifik
  domain seperti `plan_name` di level kontrak — itu masuk `metadata`).

### 5.2. Multi-tenancy: cukup shared-schema dengan kolom `app_id`
Untuk skala Anda (beberapa app internal), **shared database + shared schema +
discriminator column `app_id`** sudah cukup dan paling sederhana. Schema-per-app
atau DB-per-app adalah over-engineering sekarang; pertimbangkan hanya jika ada
kebutuhan isolasi/compliance keras (mis. data residency) di masa depan.

---

## 6. Payment Gateway Strategy (Topic 4)

### 6.1. Pola: Adapter + Anti-Corruption Layer
Definisikan satu interface domain (konseptual, bukan kode):

```
PaymentGateway:
  CreateCharge(canonicalRequest)  -> canonicalChargeResult
  ParseWebhook(rawPayload, sig)   -> canonicalEvent
  Refund(canonicalRefundRequest)  -> canonicalRefundResult
  GetStatus(gatewayTxnId)         -> canonicalStatus   // untuk polling/recon
```

Setiap gateway (DOKU, lalu Midtrans/Xendit/Tripay) adalah satu adapter yang
menerjemahkan dunia gateway ↔ model **canonical** internal Anda. Inti PS hanya
bicara model canonical, tidak pernah tahu detail DOKU.

### 6.2. Tantangan utama saat jumlah gateway bertambah (yang sering mengejutkan)
1. **Status mapping berbeda-beda.** Setiap gateway punya status & sebutan
   sendiri. Anda butuh tabel pemetaan eksplisit → canonical status.
2. **Format & mekanisme webhook berbeda** (header signature, algoritma HMAC,
   field, retry behaviour). Verifikasi signature unik per gateway.
3. **Payment method tidak seragam** (VA, QRIS, e-wallet, card, retail outlet).
   Model `payment_method` Anda harus cukup longgar.
4. **Refund semantics berbeda** (ada yang async, partial tidak didukung, dll).
5. **Settlement & fee berbeda** (timing pencairan, struktur fee).
6. **Sandbox vs production** beda kredensial & endpoint — pisahkan environment.

> Rekomendasi: walau MVP hanya DOKU, **tetap letakkan DOKU di balik interface**
> sejak awal. Biaya membuat 1 interface + 1 adapter sangat kecil, tapi memaksa
> boundary yang benar dan menjadi cetakan untuk gateway berikutnya.

### 6.3. Routing (untuk nanti, jangan sekarang)
Suatu hari Anda mungkin mau memilih gateway berdasarkan biaya/keberhasilan/jenis
metode. Siapkan *tempatnya* (field `gateway` per transaksi sudah ada), tapi
jangan bangun routing engine di MVP.

---

## 7. Integration Pattern (Topic 5)

### 7.1. Webhook-first, polling sebagai jaring pengaman
- **Primary:** terima webhook dari gateway (cepat, real-time).
- **Fallback:** background job yang **poll `GetStatus`** untuk transaksi yang
  masih `pending` melewati ambang waktu (webhook bisa hilang/telat). Ini juga
  menutup kasus webhook gateway gagal terkirim.
- **Reconciliation:** job harian membandingkan dengan settlement report gateway
  (kebenaran uang yang benar-benar masuk), lihat §9.5.

### 7.2. Notifikasi ke app — pakai Transactional Outbox
Jangan kirim HTTP ke app langsung di dalam handler webhook. Pola yang andal:

1. Webhook gateway masuk → simpan raw ke `webhook_inbox`.
2. Dalam **satu transaksi DB**: update transaksi + tulis baris ke
   `notification_outbox`.
3. Worker terpisah membaca outbox → kirim callback ke app → tandai terkirim.
4. Gagal? **retry dengan exponential backoff**, lalu **dead-letter** + alert.

Ini menjamin: kalau DB commit, notifikasi *pasti* akhirnya terkirim
(at-least-once), bahkan jika app sedang down. App wajib memperlakukan callback
secara **idempotent** (kirim `event_id`).

### 7.3. Idempotency (dua arah)
- **App → PS:** header `Idempotency-Key` (atau pakai `external_reference_id`).
  Request ulang dengan key sama → kembalikan hasil yang sama, jangan buat baru.
- **Gateway → PS:** simpan `gateway_event_id`; jika sudah pernah diproses,
  acknowledge tapi skip side-effect (deduplikasi).
- **PS → App:** sertakan `event_id` unik agar app bisa deduplikasi.

### 7.4. Out-of-order protection
Setiap event punya timestamp/sequence dari gateway. State machine hanya
menerima transisi maju yang valid; webhook "expired" yang datang setelah "paid"
harus **ditolak/diabaikan**, bukan menimpa.

### 7.5. Event-driven: ya secara konsep, tidak secara infrastruktur (untuk MVP)
Model data event-driven (append-only `transaction_event` + outbox) memberi Anda
99% manfaat event-driven **tanpa** Kafka/NATS. Message broker ditunda sampai
benar-benar ada banyak consumer (reporting service, dll) — lihat §11.

---

## 8. Security (Topic 6)

### 8.1. Service-to-service auth (app → PS)
- **MVP:** API key dengan **key_id + secret**, secret disimpan ter-*hash*
  (argon2/bcrypt) di PS. Kirim via header `Authorization`.
- **Lebih baik (mudah ditambah):** **HMAC request signing** — app
  menandatangani (method + path + body + timestamp) dengan secret; PS verifikasi
  + tolak timestamp basah (anti-replay). Ini mencegah penyalahgunaan jika key
  bocor di log dan menutup replay attack.
- **Jangka panjang:** mTLS antar service internal / mesh.
- Scope per credential (mis. hanya boleh create payment untuk `app_id` sendiri).

### 8.2. Webhook verification (gateway → PS)
- Verifikasi **signature** setiap webhook sesuai mekanisme tiap gateway
  (DOKU punya skema signature sendiri — verifikasi wajib).
- **Jangan pernah** percaya `amount`/`status` dari payload mentah sebagai
  kebenaran final tanpa: (a) signature valid, dan idealnya (b) konfirmasi ke
  `GetStatus` untuk transaksi bernilai besar.
- Whitelist IP gateway bila tersedia (defense in depth, bukan pengganti
  signature).

### 8.3. Callback verification (PS → app)
- PS menandatangani callback dengan `signing_secret` per merchant; app
  memverifikasinya. Dua arah aman.

### 8.4. Auditability & traceability
- `transaction_event` append-only = audit trail alami.
- Simpan `webhook_inbox` mentah untuk forensik & replay.
- **Correlation/request ID** mengalir dari app → PS → gateway → callback.
- Secret di **secret manager** (Vault/cloud KMS), bukan di kode/env plain.
- Masking PII & secret di log.

---

## 9. Hal Penting yang Mungkin Belum Dipikirkan (Topic 7) — *baca bagian ini*

Ini bagian dengan nilai tertinggi: hal-hal yang tidak ada di brainstorming tapi
akan menjadi masalah nyata.

### 9.1. Representasi uang
- **Simpan sebagai integer minor unit** (mis. rupiah = `amount_cents bigint`)
  + `currency` (ISO-4217). **Jangan pernah `float`/`double`** — pembulatan akan
  merusak pembukuan. Ini kesalahan #1 di payment system pemula.

### 9.2. State machine status yang eksplisit
Definisikan canonical status + transisi legal, contoh:
```
created → pending → paid → settled
created → pending → expired
created → pending → failed
paid → refunded / partially_refunded
paid → disputed → ...
```
Transisi ilegal ditolak. Ini mencegah kelas bug out-of-order/duplikat.

### 9.3. Refund, partial refund, chargeback/dispute
Brainstorming hanya membahas "pembayaran masuk". Realita: ada uang keluar.
Model refund sejak desain (walau eksekusinya ditunda) agar tidak perlu bedah
besar nanti. Chargeback/dispute memengaruhi angka revenue Anda secara langsung.

### 9.4. Gateway fee — gross vs net
Gateway memotong fee (mis. % + flat per transaksi). "Revenue" yang benar adalah
**net**. Simpan `gross_amount`, `fee_amount`, `net_amount` per transaksi (fee
sering baru pasti saat settlement). Tanpa ini dashboard revenue Anda akan
overstate.

### 9.5. Settlement & rekonsiliasi
Status "paid" di gateway ≠ uang sudah ada di rekening bank Anda. Ada jeda
settlement (T+1, T+2…). Untuk dashboard yang jujur, bedakan **paid** vs
**settled**, dan jalankan rekonsiliasi terhadap laporan settlement gateway untuk
menangkap selisih (uang hilang/ganda/fee tak terduga).

### 9.6. Expiry / timeout pembayaran
VA/QRIS punya masa berlaku. PS harus punya konsep expiry + job yang menandai
transaksi `expired`, dan memberi tahu app.

### 9.7. PCI-DSS scope
Dengan **selalu** memakai hosted page / redirect / VA / QRIS dari gateway dan
**tidak pernah** menyentuh data kartu, Anda menjaga PCI scope minimal (SAQ-A).
Jangan pernah membuat form kartu sendiri yang mengirim PAN ke PS.

### 9.8. Test/sandbox vs production
Pisahkan kredensial, data, dan idealnya environment. Banyak insiden karena
transaksi test bocor ke data produksi/reporting.

### 9.9. Data retention & compliance
Catatan finansial biasanya wajib disimpan lama (sering 5–10 tahun). Jangan
desain yang menghapus/agregasi-destruktif data transaksi. Append-only membantu.

### 9.10. Observability sejak awal
- Metrik: success rate per gateway, latency create-charge, jumlah webhook
  gagal, umur transaksi pending, ukuran dead-letter queue.
- **Alert** untuk: webhook gagal beruntun, outbox menumpuk, recon mismatch.
- Tanpa ini, "pembayaran sukses tapi langganan tak aktif" baru ketahuan dari
  komplain user.

### 9.11. (Jangka panjang) Double-entry ledger
Saat Anda serius ke "revenue platform", pertimbangkan **ledger double-entry**
yang immutable sebagai sumber kebenaran finansial (debit/kredit per akun). Ini
standar emas sistem finansial dan membuat audit/rekonsiliasi/multi-currency jauh
lebih waras. **Bukan untuk MVP**, tapi `transaction_event` append-only Anda
adalah batu loncatan ke sana.

---

## 10. Revenue & Analytics + Pertanyaan Dashboard (Topic 8 & bagian Dashboard)

Menjawab langsung pertanyaan di brainstorming:

**Q: Apakah PS sebaiknya jadi source of truth utama untuk seluruh transaksi?**
Ya — untuk **fakta pembayaran** (payment events). Tidak — untuk **revenue
akuntansi** (itu turunan yang ditafsirkan layer lain). PS adalah sumber data
*hulu* yang andal; reporting service yang mengubahnya jadi metrik bisnis.

**Q: Data apa yang perlu disimpan sejak awal agar dashboard masa depan aman?**
Tangkap ini dari hari pertama (banyak yang mahal/mustahil di-backfill):
- `app_id`, `external_reference_id`, `customer_ref` (opaque), `metadata` JSON.
- `gross_amount`, `fee_amount`, `net_amount`, `currency`.
- `gateway`, `payment_method`, `gateway_txn_id`.
- Timestamp lengkap: `created_at`, `paid_at`, `settled_at`, `failed_at`,
  `expired_at`, `refunded_at`.
- `status` canonical **+ seluruh riwayat di `transaction_event`** (append-only).
- Raw webhook di `webhook_inbox`.

**Q: Bagaimana PS tetap fokus pembayaran tapi mendukung reporting?**
PS hanya **menyimpan & memancarkan** data kaya di atas; PS **tidak** menghitung
MRR/growth/cohort. Perhitungan itu milik reporting layer yang membaca data PS.

**Q: Dashboard/reporting bagian dari PS atau service terpisah?**
**Terpisah** (di masa depan). Tapi *jangan* bangun service terpisah itu
sekarang. Untuk MVP, dashboard kecil boleh membaca **read-replica**/endpoint
read PS. Saat kebutuhan analitik tumbuh, pindahkan ke service + datastore
sendiri yang mengonsumsi event PS.

**Q: Trade-off tiap pendekatan?**

| Pendekatan reporting | Pro | Kontra | Kapan |
|---|---|---|---|
| Query langsung tabel PS | Paling simpel, 0 infra baru | Beban analitik mengganggu jalur pembayaran | ❌ hindari di produksi |
| **Read-replica PS untuk read/report** | Simpel, isolasi beban baca, real-time-ish | Masih skema OLTP, query berat tetap ada | ✅ **MVP & early growth** |
| Reporting service + datastore sendiri (konsumsi event) | Skalabel, schema dioptimasi analitik, tahan beban | Infra + pipeline + eventual consistency | 🔜 saat data/among-consumer tumbuh |
| Data warehouse / OLAP (mis. kolomnar) | Analitik berat & historis cepat | Operasional & biaya, latency batch | 🔮 saat skala besar |

> Pola evolusi yang aman: **mulai dari read-replica → pindah ke reporting
> service berbasis event → warehouse**, tanpa pernah mengubah PS core. Itu
> mungkin *justru karena* Anda menangkap event lengkap sejak awal.

---

## 11. Rekomendasi Arsitektur MVP (Realistis)

### 11.1. Bentuk
- **Satu service Go** (modular monolith), **satu PostgreSQL**, **satu gateway
  (DOKU)** di balik interface, **REST**, **Docker**.
- **Tanpa** message broker, **tanpa** microservices, **tanpa** reporting service
  terpisah, **tanpa** routing engine.

### 11.2. Modul internal (boundary jelas, walau satu binary)
```
api/            → HTTP handlers, auth (API key/HMAC), validasi, idempotency
payment/        → domain: transaction, state machine, use-cases
gateway/        → interface PaymentGateway + adapter DOKU (anti-corruption)
webhook/        → terima + verifikasi signature + simpan raw + proses
outbox/         → notification dispatcher ke app (retry/backoff/DLQ)
recon/          → polling status + (nanti) rekonsiliasi settlement
store/          → repository PostgreSQL
platform/       → logging, config, secret, observability
```
Pisahkan modul ini dengan disiplin → saat perlu dipecah jadi service, garis
potongnya sudah ada.

### 11.3. Skema data inti (konseptual)
- `merchant` (app), `api_credential`, `webhook_endpoint`
- `transaction` (status canonical, amounts, currency, gateway, refs, timestamps)
- `transaction_event` (**append-only**, immutable)
- `webhook_inbox` (raw payload + signature + processed flag)
- `notification_outbox` (callback ke app + status delivery + attempts)
- `refund` (siapkan tabelnya, eksekusi boleh menyusul)

### 11.4. Endpoint REST minimal
```
POST /v1/payments              # create payment (idempotent)
GET  /v1/payments/{id}         # status by PS id
GET  /v1/payments?ref=...      # cari by external_reference_id
POST /v1/payments/{id}/refund  # (opsional MVP / siapkan)
POST /v1/webhooks/doku         # receiver webhook DOKU
GET  /v1/transactions          # list utk read/report (scoped per app)
```

### 11.5. Non-negotiable di MVP (rekap)
1. Integer money + currency.
2. Idempotency key di setiap write.
3. Append-only `transaction_event`.
4. Signature verification webhook + simpan raw.
5. State machine status eksplisit.
6. Outbox + retry untuk callback ke app.
7. Polling fallback untuk transaksi pending.
8. Correlation ID + observability dasar.

### 11.6. Eksplisit DITUNDA (anti over-engineering)
Message broker, reporting service terpisah, gateway routing, ledger
double-entry, multi-region, schema-per-tenant, dispute automation, admin UI
penuh. Semua punya "tempat" di desain tapi tidak dibangun sekarang.

---

## 12. Roadmap Evolusi 6–24 Bulan

**Fase 0 — MVP (bulan 0–3)**
DOKU only, 1–2 app pertama, fondasi §11. Tujuan: pembayaran andal + data lengkap.

**Fase 1 — Hardening & gateway kedua (bulan 3–6)**
Tambah Midtrans/Xendit (validasi bahwa abstraksi benar — gateway kedua selalu
membongkar asumsi yang salah). Rekonsiliasi settlement otomatis. Refund penuh.
HMAC signing. Observability + alerting matang.

**Fase 2 — Reporting layer (bulan 6–12)**
Read-replica → reporting read API. Dashboard internal (service/aplikasi
terpisah) membaca metrik: revenue per app, harian/bulanan, success/failed/
pending, statistik gateway, growth. PS tetap tak tersentuh logikanya.

**Fase 3 — Event backbone (bulan 12–18)**
Saat consumer event bertambah (reporting, notifikasi, anti-fraud), perkenalkan
message broker + outbox→broker. Reporting service punya datastore sendiri.

**Fase 4 — Revenue/Finance platform (bulan 18–24)**
Ledger double-entry sebagai sumber kebenaran finansial, fee/net akurat,
multi-currency, revenue recognition, payout tracking. Gateway routing/failover
berbasis biaya & success rate. Inilah "ekosistem revenue" di visi Anda.

> Benang merah: setiap fase **menambah lapisan di sekeliling** PS core yang
> stabil, bukan membongkarnya. Itu hanya mungkin jika boundary & data event
> benar sejak Fase 0.

---

## 13. Daftar Keputusan yang Perlu Anda Konfirmasi

Sebelum masuk desain detail/implementasi, ada beberapa hal yang butuh keputusan
Anda (jawaban memengaruhi desain):

1. **Model integrasi pembayaran DOKU** yang dipakai (redirect/hosted checkout vs
   VA vs QRIS vs e-wallet) — memengaruhi flow & PCI scope.
2. **Mode notifikasi ke app**: callback webhook (push) saja, atau juga sediakan
   polling endpoint (pull) untuk app yang tidak bisa expose webhook?
3. **Multi-currency**: hanya IDR untuk sekarang, atau siapkan multi-currency
   sejak skema?
4. **Siapa app pertama** yang akan integrasi (Invoice/POS/HR)? Berguna untuk
   memvalidasi kontrak API dengan kasus nyata.
5. **Refund di MVP**: cukup siapkan tabel, atau perlu eksekusi refund fungsional
   sejak awal?
6. **Lingkungan**: apakah sudah ada infra (cloud/secret manager) yang menentukan
   pilihan untuk auth & secret?

---

## 14. Penutup

Ide Anda sehat dan arah arsitekturnya benar. Risiko terbesar Anda **bukan**
salah memilih boundary (itu sudah tepat), melainkan **kehilangan data dan
keandalan di detail**: money sebagai float, status sebagai mutable boolean,
webhook tanpa idempotency/verifikasi, dan notifikasi tanpa outbox. Perbaiki
empat hal itu di MVP, jaga PS tetap buta terhadap business logic tapi kaya akan
event data, dan visi revenue platform Anda bisa tumbuh berlapis tanpa rewrite.

Saya siap lanjut ke **desain detail** (skema tabel final, kontrak API,
state-machine diagram, flow webhook+outbox) begitu keputusan di §13 ditentukan —
tetap tanpa menulis kode sampai Anda minta.
