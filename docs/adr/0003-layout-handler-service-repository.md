# ADR-0003 — Layout flat by-layer: handler → service → repository

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K3

## Konteks

Layout v1 mengikuti penamaan Clean Architecture kanonik:
`internal/adapter/http`, `internal/usecase/payment`, `internal/adapter/repository`,
`internal/adapter/gateway/doku`.

Pemilik produk berpikir dan berbicara dalam istilah **handler → service →
repository**. Setiap kali membaca kode, ada langkah terjemahan mental
"usecase itu service" dan "adapter itu handler/repository". Biaya kecil, tapi
dibayar terus-menerus dan tanpa imbalan.

## Keputusan

Kami memakai layout **flat by-layer**:

```
internal/
  domain/         # entity, state machine, PORT (interface Repository & gateway)
  handler/        # Gin: router, handler, middleware, dto
  service/        # orkestrasi
  repository/     # implementasi GORM + model DB + mapper
  gateway/doku/   # client DOKU + signature HMAC
  infrastructure/ # config, database, logger, metrics, crypto
```

Clean Architecture tetap berlaku dan dijaga oleh **arah dependensi**, bukan oleh
nama folder `adapter/`:

```
cmd → handler → service → domain ← repository / gateway
```

Aturan yang tidak boleh dilanggar:

- `internal/domain/` **tidak boleh meng-import** Gin, GORM, atau paket DOKU apa pun.
- `internal/handler/` tidak boleh meng-import `repository` atau `gateway` langsung;
  ia hanya bicara ke `service`.
- Model GORM di `repository/.../model` tetap **terpisah** dari entity domain,
  dipetakan lewat mapper.

## Konsekuensi

Lebih mudah:

- Struktur folder langsung membaca alur yang ada di kepala pemiliknya; tidak ada
  terjemahan mental.
- Pendatang baru menemukan file yang dicari dari nama layer-nya.

Lebih sulit:

- Nama `adapter/` yang mengelompokkan "semua yang menyentuh dunia luar" hilang.
  `handler`, `repository`, dan `gateway` kini sejajar padahal ketiganya sama-sama
  adapter secara konseptual.
- **Aturan arah dependensi jadi konvensi, bukan sesuatu yang terlihat dari struktur.**
  Lebih mudah tidak sengaja meng-import `repository` dari `handler`. Ini harus
  dijaga saat review PR — dan kalau pelanggaran terulang, tambahkan
  `depguard` di `.golangci.yml`.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Tetap `adapter/…`, hanya rename `usecase` → `service` | perubahan paling kecil, tapi jalur handler → service → repository tetap tidak terbaca dari struktur; masalah aslinya tidak selesai |
| Package-by-feature (`internal/payment/{handler,service,repo}`) | bagus untuk domain yang banyak, tapi service ini praktis punya satu domain (payment); hasilnya cuma satu folder tebal dengan indirection tambahan |
| Tetap seperti sekarang | membiarkan biaya terjemahan mental dibayar terus |
