# 0017 — `make lint` belum bersih (57 temuan pre-existing)

- **Status:** todo
- **Prioritas:** medium
- **Estimasi:** S
- **Depends on:** —
- **Ditemukan saat:** Phase 000 — Kesepakatan & Dokumen
- **GitHub issue:** #30
- **Referensi:** `.golangci.yml`, output `make lint`
- **Triase rombakan v2:** keep → dikerjakan bersamaan Phase 1
- **Alasan triase:** Phase 1 memindahkan hampir seluruh file Go, jadi memperbaikinya di sana jauh lebih murah daripada sekarang.

## Konteks

Saat menutup Phase 000 (phase dokumentasi, **nol file `.go` disentuh**),
`make lint` gagal. Verifikasi silang di branch `main` memberi jumlah temuan yang
**persis sama**, jadi ini bukan regresi dari Phase 000 melainkan kondisi yang sudah
ada sebelumnya.

Aturan alur kerja di `CLAUDE.md` menyebut "lint harus hijau sebelum PR". Selama
issue ini belum selesai, aturan itu **tidak bisa dipenuhi oleh PR mana pun** —
itulah alasan ini perlu dibereskan lebih awal, bukan karena 57 temuannya berbahaya.

## Temuan

57 temuan, dua kelompok:

**misspell (7)** — false positive. Linter membaca komentar Bahasa Indonesia sebagai
bahasa Inggris yang salah eja:

```
internal/adapter/gateway/doku/crypto.go:1:55: `implementasi` is a misspelling of `implements`
internal/adapter/http/router.go:70:6:        `Operasional` is a misspelling of `Operational`
```

Repo ini memang menulis komentar dalam Bahasa Indonesia (aturan di `CLAUDE.md`),
jadi `misspell` dengan locale default akan terus salah tuduh.

**revive (50)** — sebagian besar sepele dan sah:

- `exported … should have comment or be unexported` — mayoritas temuan; kebanyakan
  method `TableName()` pada model GORM dan beberapa var/error yang di-export.
- `parameter 'r' seems to be unused, consider removing or renaming it as _` (4×).
- `redefinition of the built-in function min` (1×).

## Dampak

Tidak ada bug yang terbukti dari temuan ini — semuanya soal gaya dan dokumentasi.

Dampak nyatanya pada proses: **lint yang selalu merah kehilangan fungsinya sebagai
sinyal.** Kalau `make lint` gagal di setiap PR, temuan baru yang benar-benar penting
akan tenggelam di antara 57 temuan lama, dan kebiasaan yang terbentuk adalah
mengabaikan hasilnya.

## Usulan solusi

1. **misspell** — matikan untuk komentar, atau kecualikan lewat `exclusions` di
   `.golangci.yml`. Linter ejaan Bahasa Inggris tidak cocok untuk basis kode yang
   komentarnya Bahasa Indonesia. Ini keputusan konfigurasi, bukan perbaikan kode.
2. **revive** — perbaiki betulan (tambah komentar doc, rename parameter tak terpakai
   jadi `_`, ganti nama fungsi `min`). Jumlahnya banyak tapi tiap perbaikan sepele.
3. Kerjakan **bersamaan Phase 1**: phase itu memindahkan hampir semua file Go ke
   layout baru, jadi menambahkan komentar doc sambil memindahkan file jauh lebih
   murah daripada dua kali menyentuh file yang sama.

## Acceptance criteria

- [ ] `make lint` keluar dengan status 0
- [ ] Konfigurasi `misspell` disesuaikan untuk komentar Bahasa Indonesia, dengan
      alasan tertulis di `.golangci.yml`
- [ ] Temuan `revive` diperbaiki di kode, bukan dibungkam lewat exclusion
- [ ] Tidak ada `//nolint` yang ditambahkan tanpa alasan tertulis
