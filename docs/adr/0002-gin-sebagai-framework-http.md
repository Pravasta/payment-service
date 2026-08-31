# ADR-0002 — Gin menggantikan chi sebagai framework HTTP

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K2

## Konteks

Delivery layer saat ini memakai `go-chi/chi/v5` dengan handler bergaya
`net/http` (`func(w http.ResponseWriter, r *http.Request)`) dan validasi request
ditulis manual di DTO.

Pemilik produk memakai Gin di aplikasi lain (HR, KOL, Invoice, CRM) dan ingin
kesamaan idiom lintas repo, supaya berpindah antar-proyek tidak menuntut ganti
kebiasaan.

## Keputusan

Kami memakai **Gin** sebagai framework HTTP, menggantikan chi seluruhnya di Phase 1.

Aturan pemakaian yang mengikat:

- Pakai `gin.New()`, **bukan** `gin.Default()`. Logger dan recovery bawaan Gin
  menulis ke stdout dengan format sendiri dan akan menabrak `slog` terstruktur serta
  correlation id yang sudah ada.
- `gin.SetMode(gin.ReleaseMode)` di luar development.
- Validasi request pakai tag `binding:"required,..."` (validator v10) lewat
  `ShouldBindJSON`, menggantikan pemeriksaan manual di DTO.
- Middleware idempotency perlu membaca body lebih dari sekali — pakai
  `ShouldBindBodyWith` atau simpan body di context. **Jangan `io.ReadAll` dua kali**;
  body request hanya bisa dibaca sekali.

## Konsekuensi

Lebih mudah:

- Idiom seragam dengan empat aplikasi pemanggil.
- Validasi deklaratif menghapus banyak kode pemeriksaan manual di DTO.
- Route group + middleware per-group memetakan scope auth dengan rapi.

Lebih sulit:

- **Delapan middleware harus ditulis ulang** ke `gin.HandlerFunc`: request-id,
  access log, metrics, recovery, auth, scope, idempotency, strip-slash. Ini
  pekerjaan mekanis tapi rawan salah — dan middleware auth/idempotency adalah dua
  tempat paling berbahaya untuk salah.
- Gin lebih "berpendapat" daripada chi; handler tidak lagi `http.HandlerFunc` biasa,
  sehingga menguji handler menuntut `gin.CreateTestContext`, bukan `httptest` polos.
- Risiko regresi diam-diam selama migrasi.

Mitigasi risiko regresi: **Phase 1 tidak menambah fitur apa pun.** Test yang sudah
ada (auth, idempotency, webhook, sync, refund) wajib tetap hijau tanpa diubah
ekspektasinya. Test yang harus diubah ekspektasinya adalah tanda ada perilaku yang
berubah — dan itu harus dibahas, bukan dilewatkan.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Tetap di chi | secara teknis baik-baik saja, tapi berbeda dari idiom empat aplikasi lain; nilai keseragaman lebih besar daripada biaya migrasi satu kali |
| Echo / Fiber | tidak dipakai di aplikasi lain; Fiber tidak memakai `net/http` sehingga ekosistem middleware standar tidak berlaku |
| `net/http` + `ServeMux` Go 1.22+ | routing sudah cukup, tapi harus menulis sendiri binding, validasi, dan rantai middleware — kerja lebih banyak, keseragaman tidak dapat |
