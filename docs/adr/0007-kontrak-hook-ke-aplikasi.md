# ADR-0007 — Kontrak hook Payment Service → aplikasi

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K7; bagian `GET /v1/events` = **asumsi (perlu konfirmasi)**

## Konteks

Scope service ini adalah "skema pembayaran **dan hook-nya**". Hook adalah setengah
dari nilai service ini: tanpa pemberitahuan yang andal, setiap aplikasi terpaksa
melakukan polling, dan status pembayaran jadi telat atau salah.

Kendalanya nyata: aplikasi penerima bisa sedang deploy, jaringan internal bisa
putus, dan DOKU bisa mengirim notifikasi ganda atau tidak berurutan.

## Keputusan

Kami memakai **push berbasis outbox dengan jaminan at-least-once**.

**Bentuk pengiriman**

- Payment Service `POST` ke `webhook_endpoint.url` milik app.
- Header `X-Event-Id` (untuk dedup) dan `X-Signature` (HMAC-SHA256 atas raw body
  memakai `signing_secret` per endpoint).
- App **wajib** membalas `2xx`. Selain itu dianggap gagal dan dijadwalkan ulang.

**Event type**

`payment.pending`, `payment.paid`, `payment.expired`, `payment.failed`,
`payment.refunded`, `payment.partially_refunded`.

**Keandalan**

- Event ditulis ke `notification_outbox` **dalam transaksi database yang sama**
  dengan perubahan state pembayaran. Kalau state berubah, event pasti tercatat;
  tidak ada state berubah tanpa event.
- Worker terpisah yang mengirim. Retry backoff: `0s → 30s → 2m → 10m → 1h → 6h`,
  lalu status `dead` (DLQ).
- `POST /v1/admin/outbox/{id}/replay` untuk mengirim ulang secara manual.

**At-least-once, bukan exactly-once.** Aplikasi penerima wajib idempoten terhadap
`event_id`. Ini konsekuensi yang harus ditulis di panduan integrasi, bukan detail
yang boleh dianggap sudah dimengerti.

### `GET /v1/events` — asumsi, perlu konfirmasi

Endpoint pull `GET /v1/events?since=…` direncanakan sebagai jaring pengaman agar app
bisa mengejar ketinggalan sendiri setelah gangguan panjang, tanpa perlu ada orang
yang menjalankan replay manual.

**Ini keputusan saya, bukan jawaban user.** Dijadwalkan sebagai **task opsional
terakhir di Phase 4** — dikerjakan kalau Phase 4 masih lapang, ditunda kalau tidak.
Alasan boleh ditunda: kombinasi outbox retry (sampai 6 jam) + replay manual sudah
menutup hampir semua kasus nyata.

## Konsekuensi

Lebih mudah:

- Aplikasi tidak perlu polling; status sampai dalam hitungan detik.
- Outbox dalam satu transaksi menghapus kelas bug "state berubah tapi app tidak
  pernah diberi tahu".
- DLQ membuat kegagalan terlihat, bukan hilang diam-diam.

Lebih sulit:

- **Beban idempotensi digeser ke aplikasi penerima.** Empat aplikasi harus
  mengimplementasikannya dengan benar; kalau satu lalai, akan ada efek ganda
  (mis. invoice ditandai lunas dua kali). Wajib ditegaskan di panduan integrasi.
- Perlu worker yang berjalan terus — satu proses lagi yang harus dipantau. Kalau
  worker mati, tidak ada event yang terkirim sementara API tetap terlihat sehat.
  Ini harus terlihat di metrik, bukan baru ketahuan dari keluhan.
- Rotasi `signing_secret` menuntut dukungan dua secret aktif sementara, kalau tidak
  rotasi berarti downtime bagi app tersebut.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Polling saja (app menanyakan status berkala) | latensi tinggi, membebani DB dan DOKU, dan tetap butuh reconciler; hook justru inti scope service ini |
| Kirim langsung saat webhook DOKU masuk (tanpa outbox) | kalau pengiriman gagal, event hilang selamanya; tidak ada retry maupun jejak |
| Message broker (NATS/Kafka) | ketergantungan infrastruktur baru untuk empat konsumen internal; outbox di Postgres yang sudah ada sudah cukup |
| Exactly-once delivery | tidak bisa dijamin lintas jaringan; berpura-pura bisa justru membuat app tidak menulis dedup |
