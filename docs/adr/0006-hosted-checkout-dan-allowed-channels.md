# ADR-0006 — Hosted Checkout, bukan direct API per channel

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K6; bagian `allowed_channels` = **asumsi (perlu konfirmasi)**

## Konteks

DOKU menyediakan dua gaya integrasi: **Hosted Checkout** (redirect ke halaman
pembayaran DOKU) dan **direct API per channel** (aplikasi merender UI sendiri lalu
memanggil endpoint spesifik per metode bayar).

Verifikasi live lewat MCP `doku-mcp-server` pada 2026-08-31 (sandbox
`BRN-0252-1782502630768`) menunjukkan akun ini mengaktifkan **30 channel dalam 6
kategori**:

| Kategori | Jumlah | Contoh |
|---|---|---|
| Virtual Account | 17 | BCA, Mandiri, BNI, BRI, BSI, Permata, CIMB, Danamon, BTN, Maybank, OCBC, BJB, Sinarmas, BNC, BSS, BPD Bali, DOKU |
| E-Wallet | 5 | OVO, DANA, ShopeePay, LinkAja, i.saku |
| BNPL / Installment | 4 | Kredivo, Akulaku, Indodana, BRI Ceria |
| Convenience Store | 2 | Alfamart, Indomaret |
| Credit Card | 1 | CREDIT_CARD |
| Payment Link | 1 | DOKU Customer Form |

Direct API berarti 6+ integrasi terpisah, masing-masing dengan payload, bentuk
notifikasi, dan mekanisme refund yang berbeda. Untuk kartu kredit, direct API juga
menarik beban kepatuhan **PCI-DSS** ke sisi kita karena data kartu melewati sistem
sendiri.

## Keputusan

Kami memakai **Hosted Checkout (redirect)** sebagai satu-satunya gaya integrasi.

- Satu integrasi membuat ke-30 channel langsung tersedia.
- UI pembayaran, alur 3DS kartu, dan penanganan kedaluwarsa ditangani DOKU.
- Data kartu tidak pernah menyentuh sistem ini → **di luar cakupan PCI-DSS**.
- Aplikasi menerima `payment_url` secara sinkron dan mengalihkan penggunanya ke sana.

Direct API per channel **tidak** dibangun sekarang. Kalau nanti dibutuhkan, yang
ditambahkan adalah channel spesifik dengan alasan produk yang jelas (misalnya VA
statis untuk pembayaran berulang), bukan seluruh katalog.

### `allowed_channels` — asumsi, perlu konfirmasi

Request `POST /v1/payments` menyediakan field opsional `allowed_channels` untuk
membatasi metode bayar yang muncul di halaman Checkout (mis. CRM hanya ingin VA dan
e-wallet). Kalau kosong, semua channel yang aktif di akun DOKU ditampilkan.

**Ini keputusan saya, bukan jawaban user.** Alasannya: biayanya satu field yang
diteruskan apa adanya ke payload Checkout DOKU, sementara menambahkannya belakangan
berarti mengubah kontrak API yang sudah dipakai empat aplikasi. Ditinjau ulang saat
Phase 3 kalau ternyata tidak dibutuhkan.

## Konsekuensi

Lebih mudah:

- Satu adapter, satu bentuk payload, satu bentuk notifikasi.
- Menambah channel baru = mengaktifkannya di dashboard DOKU, **tanpa deploy**.
- Beban PCI-DSS tidak pernah masuk.

Lebih sulit:

- **Kontrol UI hilang.** Pengguna keluar dari aplikasi ke halaman DOKU; tampilan dan
  branding mengikuti DOKU. Ini kompromi paling terasa dari keputusan ini.
- Alur pembayaran in-app (mis. tap VA langsung di aplikasi mobile) tidak mungkin
  tanpa menambah direct API belakangan.
- Bergantung pada ketersediaan halaman Checkout DOKU; kalau halaman itu bermasalah,
  tidak ada jalur cadangan.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Direct API per channel sejak awal | 6+ integrasi, dan CC direct menarik beban PCI-DSS; tidak sebanding untuk kebutuhan saat ini |
| Hosted Checkout + direct VA sekaligus | menggandakan jalur kode dan bentuk notifikasi sejak Phase 3 tanpa permintaan produk yang konkret |
| Payment Link (DOKU Customer Form) | ditujukan untuk penagihan manual, bukan integrasi programatik per transaksi |
