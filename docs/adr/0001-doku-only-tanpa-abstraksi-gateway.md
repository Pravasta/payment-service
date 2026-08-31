# ADR-0001 — DOKU-only tanpa abstraksi multi-gateway

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K1

## Konteks

Desain v1 memasang port `payment.Gateway` dengan empat method canonical
(`CreateCharge`, `ParseWebhook`, `GetStatus`, `Refund`) supaya Midtrans, Xendit,
atau Tripay bisa ditambahkan tanpa mengubah aplikasi pemanggil.

Dua fakta membatalkan asumsi itu:

1. Perizinan gateway lain merepotkan; pemilik produk hanya siap dengan DOKU dan
   tidak punya rencana konkret menambah gateway kedua.
2. Abstraksinya sudah terbukti bocor. Konsep khas DOKU naik ke domain:
   `gateway_request_id` (wajib untuk GetStatus & Refund), `RefundType`
   (`VOID`/`PARTIAL_REFUND`/`FULL_REFUND`), dan format amount yang berbeda
   per-endpoint. Ini bukan model canonical — ini model DOKU yang menyamar.

Abstraksi yang menyembunyikan satu implementasi tidak menyembunyikan apa pun; ia
hanya menambah satu lapisan terjemahan yang harus dibaca, di-test, dan dijaga.

## Keputusan

Kami membuang lapisan abstraksi multi-gateway dan memakai **satu interface tipis
yang jujur bicara istilah DOKU**, semata-mata sebagai seam untuk testing.

Konkretnya:

- Interface gateway boleh menyebut `RequestID`, `InvoiceNumber`, `ChannelID` di
  signature-nya. Tidak ada lagi kewajiban memetakan ke nama netral.
- Kolom `transaction.gateway` dibuang (nilainya selalu `"doku"`).
- Pemetaan status jadi satu arah saja: DOKU → status internal. Tidak ada lagi
  terjemahan balik.
- **`payment.Status` dan `payment.CanTransition` tetap dipertahankan.** Ini bukan
  abstraksi gateway — ini **kontrak ke HR/KOL/Invoice/CRM**, dan justru harus stabil
  meski istilah DOKU berubah.

## Konsekuensi

Lebih mudah:

- Satu lapisan terjemahan hilang; membaca alur dari handler ke DOKU jadi lurus.
- Fakta DOKU yang aneh (bungkus `response`, format amount per-endpoint) tidak perlu
  lagi diselundupkan ke model canonical.
- Test lebih sedikit: tidak ada lagi test untuk pemetaan bolak-balik.

Lebih sulit — **diterima secara sadar**:

- **Vendor lock-in ke DOKU.** Menambah gateway kedua nanti berarti pekerjaan nyata,
  bukan sekadar menulis adapter baru. Ini konsekuensi langsung dari keputusan bisnis
  bahwa hanya DOKU yang tersedia.
- Kalau DOKU mengubah kontraknya, perubahan itu merambat lebih jauh ke dalam
  daripada di desain v1.

Mitigasi atas keduanya: `payment.Status` yang tetap canonical menjaga agar kontrak
ke aplikasi pemanggil tidak ikut berubah.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Buang interface sepenuhnya, service memanggil `doku.Client` langsung | service jadi hanya bisa di-test dengan HTTP mock; seam testing terlalu murah untuk dikorbankan |
| Pertahankan port canonical seperti v1 | biaya baca dan biaya test tanpa manfaat selama gateway hanya satu; sudah terbukti bocor |
| Buang juga `payment.Status`, pakai status DOKU apa adanya | status DOKU akan bocor ke API publik yang dipakai empat aplikasi; perubahan istilah di DOKU jadi breaking change untuk mereka |
