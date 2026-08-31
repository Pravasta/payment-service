# ADR-0008 — Provisioning lewat CLI `adminctl`

- **Status:** accepted
- **Tanggal:** 2026-08-31
- **Sumber:** brainstorming §K8

## Konteks

Rombakan v2 membuang dashboard dari scope. Tapi provisioning tetap harus terjadi:
mendaftarkan app baru, menerbitkan API key, mengatur URL webhook, merotasi secret.
Tanpa jalur resmi, pekerjaan itu akan dikerjakan lewat `INSERT` SQL manual — cara
paling mudah untuk memasukkan data yang tidak konsisten ke tabel yang memegang
kredensial.

`cmd/seed` yang ada sekarang hanya mengisi data contoh untuk development, bukan
alat operasional.

## Keputusan

Kami membuat CLI **`cmd/adminctl`** sebagai satu-satunya jalur provisioning resmi.

```
adminctl app create   --code crm --name "CRM SaaS"
adminctl key issue    --app crm --scopes payments:read,payments:write
adminctl key revoke   --key-id pk_live_xxx
adminctl hook set     --app crm --url https://crm.internal/hooks/payment
adminctl hook rotate  --app crm
adminctl txn get      --app crm --ref INV-2026-000123
```

Aturan:

- Secret **hanya ditampilkan sekali** saat dibuat. Yang tersimpan di DB adalah
  hash argon2id (API secret) atau ciphertext AES-GCM (signing secret).
- CLI memakai jalur kode yang sama dengan service (repository + crypto), bukan SQL
  mentah — supaya validasi dan enkripsi tidak bisa terlewat.
- `cmd/seed` dihapus; kebutuhan data development dipenuhi lewat `adminctl`.

Dibangun di **Phase 6**, setelah alur pembayaran dan hook berjalan.

## Konsekuensi

Lebih mudah:

- Provisioning konsisten dan bisa diaudit; tidak ada `INSERT` manual ke tabel kredensial.
- Enkripsi dan hashing tidak mungkin terlewat karena memakai jalur kode yang sama.
- Bisa dipanggil dari skrip deployment.

Lebih sulit:

- **Butuh akses shell ke server** (atau container) untuk setiap operasi. Tidak ada
  yang bisa dikerjakan orang non-teknis. Diterima: satu-satunya operatornya adalah
  pemilik sistem sendiri.
- Tidak ada UI untuk melihat daftar app/kredensial sekilas; harus lewat perintah.
- Satu binary lagi yang harus di-build dan didistribusikan bersama service.
- Sampai Phase 6 selesai, provisioning masih manual. Perlu diingat kalau CRM ingin
  integrasi lebih awal.

## Alternatif yang ditolak

| Alternatif | Alasan ditolak |
|---|---|
| Endpoint HTTP admin (`POST /v1/admin/apps`) | menambah permukaan serang yang bisa dicapai dari jaringan untuk operasi paling sensitif di sistem; CLI hanya bisa dijalankan oleh yang sudah punya akses shell |
| Dashboard web | keluar dari scope v2 secara eksplisit |
| SQL manual / migrasi seed | melewati enkripsi dan hashing; cara paling mudah membocorkan atau merusak kredensial |
| Pertahankan `cmd/seed` dan kembangkan | seed dirancang untuk data contoh yang boleh dibuang, bukan untuk operasi produksi |
