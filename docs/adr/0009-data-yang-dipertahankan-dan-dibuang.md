# ADR-0009 — Data yang dipertahankan dan yang dibuang

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K9

## Konteks

Desain v1 menyimpan data dengan dua motif berbeda yang tercampur: **kebenaran
operasional** (apa yang terjadi pada sebuah pembayaran) dan **fondasi analytics**
(bahan mentah untuk dashboard revenue lintas aplikasi).

Rombakan v2 membuang dashboard. Pertanyaannya: apakah data yang dulu dikumpulkan
untuk dashboard ikut dibuang?

Godaan yang harus ditolak ada di dua arah. Membuang semuanya berarti kehilangan
jejak audit yang justru paling dibutuhkan saat ada sengketa pembayaran.
Mempertahankan semuanya berarti dashboard masuk lagi lewat pintu belakang, dan
setiap keputusan desain kembali ikut mempertimbangkan reporting.

## Keputusan

Kami memisahkan berdasarkan **motifnya**, bukan berdasarkan bentuk datanya.

**Dipertahankan — karena audit, bukan analytics**

| Tabel | Alasan |
|---|---|
| `transaction_event` (append-only) | satu-satunya cara menjawab "kenapa transaksi ini jadi `failed`" tiga bulan kemudian. Murah: satu baris per perubahan state. Tidak pernah di-`UPDATE` atau di-`DELETE`. |
| `webhook_inbox` (payload mentah DOKU) | bukti saat terjadi sengketa dengan gateway. Tanpa payload asli, klaim "DOKU mengirim status X" tidak bisa dibuktikan. |
| `refund` | bagian dari kebenaran operasional pembayaran. |

**Dibuang**

| Apa | Alasan |
|---|---|
| Kolom `transaction.gateway` | selalu bernilai `"doku"` (konsekuensi ADR-0001). |
| Kolom `gateway` di `gateway_account` | sama. |
| Seluruh rencana tabel/kolom agregasi revenue | keluar dari scope v2. |
| Endpoint reporting/agregasi | app melakukan agregasinya sendiri dari `GET /v1/payments`. |

**Aturan yang mengikat ke depan**

- Menyimpan sebuah field harus bisa dijawab dengan "dibutuhkan untuk memproses atau
  mengaudit pembayaran ini". Kalau jawabannya "nanti berguna untuk laporan", jangan
  disimpan.
- Konteks bisnis dari aplikasi masuk ke `metadata` (JSONB) sebagai data **opaque**.
  Payment Service tidak pernah menafsirkan, memvalidasi, atau meng-query isinya
  sebagai kriteria bisnis.
- Uang selalu `bigint` minor unit, tidak pernah float. IDR `currency_exponent = 0`.

## Konsekuensi

Lebih mudah:

- Jejak audit tetap utuh untuk sengketa dan investigasi.
- Skema tetap ramping; tidak ada kolom yang ada "untuk berjaga-jaga".
- Batas scope punya aturan uji yang konkret saat review PR.

Lebih sulit:

- `transaction_event` dan `webhook_inbox` **tumbuh tanpa batas**. Belum ada
  kebijakan retensi atau partisi. Pada volume sekarang tidak masalah, tapi ini utang
  yang akan ditagih. Diparkir sebagai issue, bukan dikerjakan sekarang.
- Kalau dashboard tetap diinginkan suatu saat, datanya ada tapi tidak dalam bentuk
  yang siap di-query — perlu ETL sendiri. Itu memang konsekuensi yang dipilih.
- `metadata` yang opaque berarti pertanyaan seperti "berapa revenue dari produk X"
  tidak bisa dijawab service ini. Aplikasi asal yang menjawabnya.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Buang `transaction_event` juga (dashboard sudah tidak ada) | mencampuradukkan audit dengan analytics; jejak audit hilang justru saat paling dibutuhkan, dan biayanya sangat murah |
| Pertahankan semua fondasi analytics v1 | dashboard masuk lagi lewat pintu belakang; setiap keputusan desain kembali terbebani pertimbangan reporting |
| Tambahkan kolom agregat (`total_paid_per_app`) sekarang | data turunan yang bisa dihitung ulang; menyimpannya berarti menciptakan sumber kebenaran kedua yang bisa melenceng |
