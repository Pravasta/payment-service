# Architecture Decision Records

Satu keputusan arsitektur = satu file bernomor. ADR dipakai (bukan bab tambahan di
dokumen desain) karena keputusan punya **umur dan status sendiri**: sebuah ADR bisa
jadi `superseded` tanpa membuat seluruh dokumen desain ikut usang, dan alasan
"kenapa dulu diputuskan begitu" tetap terbaca bertahun kemudian.

ADR **tidak pernah diedit isinya setelah `accepted`**. Kalau keputusannya berubah,
tulis ADR baru dan ubah status yang lama jadi `superseded by ADR-NNNN`.

## Indeks

| # | Keputusan | Status | Sumber |
|---|---|---|---|
| [0001](0001-doku-only-tanpa-abstraksi-gateway.md) | DOKU-only tanpa abstraksi multi-gateway | accepted | brainstorming §K1 |
| [0002](0002-gin-sebagai-framework-http.md) | Gin menggantikan chi | accepted | brainstorming §K2 |
| [0003](0003-layout-handler-service-repository.md) | Layout flat by-layer: handler → service → repository | accepted | brainstorming §K3 |
| [0004](0004-merchant-menjadi-app.md) | `merchant` menjadi `app` | accepted | brainstorming §K4 |
| [0005](0005-kredensial-doku-per-app.md) | Kredensial DOKU per-app dengan fallback env | accepted | jawaban user |
| [0006](0006-hosted-checkout-dan-allowed-channels.md) | Hosted Checkout, bukan direct API per channel | accepted | brainstorming §K6 + **asumsi** |
| [0007](0007-kontrak-hook-ke-aplikasi.md) | Kontrak hook PS → aplikasi | accepted | brainstorming §K7 + **asumsi** |
| [0008](0008-provisioning-via-adminctl.md) | Provisioning lewat CLI `adminctl` | accepted | brainstorming §K8 |
| [0009](0009-data-yang-dipertahankan-dan-dibuang.md) | Data yang dipertahankan dan dibuang | accepted | brainstorming §K9 |

## Kerangka penulisan

```markdown
# ADR-NNNN — <Judul keputusan>

- **Status:** accepted | superseded by ADR-NNNN | deprecated
- **Tanggal:** YYYY-MM-DD
- **Sumber:** brainstorming §K<n> | jawaban user | asumsi (perlu konfirmasi)

## Konteks
Fakta dan kendala yang berlaku saat keputusan diambil. Bukan opini.

## Keputusan
Satu kalimat tegas dan aktif: "Kami memakai X." Lalu detail secukupnya.

## Konsekuensi
Yang jadi lebih mudah **dan** yang jadi lebih sulit. Konsekuensi negatif wajib
ditulis — ADR tanpa konsekuensi negatif biasanya berarti trade-off-nya belum
dipikirkan.

## Alternatif yang ditolak
Opsi lain + alasan penolakan, supaya tidak diperdebatkan ulang tiap beberapa bulan.
```

## Arti field `Sumber`

| Nilai | Arti |
|---|---|
| `brainstorming §K<n>` | rekomendasi di dokumen brainstorming yang di-review dan di-merge user |
| `jawaban user` | user menjawabnya secara eksplisit; jangan diubah tanpa bertanya lagi |
| `asumsi (perlu konfirmasi)` | **keputusan sementara yang diambil tanpa jawaban user** — paling rapuh, tinjau ulang lebih dulu saat ada keraguan |
