# 0016 — Kebijakan retensi `transaction_event` & `webhook_inbox`

- **Status:** todo
- **Prioritas:** low
- **Estimasi:** M
- **Depends on:** ADR-0009
- **Ditemukan saat:** Phase 000 — Kesepakatan & Dokumen
- **GitHub issue:** #29
- **Referensi:** `docs/adr/0009-data-yang-dipertahankan-dan-dibuang.md` §Konsekuensi
- **Triase rombakan v2:** keep → dijadwalkan setelah Phase 8
- **Alasan triase:** utang yang lahir dari keputusan v2 sendiri, bukan warisan v1.

## Konteks

ADR-0009 memutuskan `transaction_event` (append-only) dan `webhook_inbox` (payload
mentah DOKU) dipertahankan untuk keperluan audit dan bukti sengketa. Keputusan itu
tepat, tapi menciptakan konsekuensi yang ADR-nya sendiri catat sebagai utang.

## Temuan

Kedua tabel **tumbuh tanpa batas dan tidak pernah dipangkas**:

- `transaction_event` bertambah satu baris setiap perubahan state pembayaran
  (`created` → `pending` → `paid` → …), jadi beberapa baris per transaksi.
- `webhook_inbox` menyimpan **payload mentah** setiap notifikasi DOKU, termasuk
  notifikasi duplikat yang di-drop oleh dedup. Ukurannya per baris jauh lebih besar
  daripada `transaction_event`.

Belum ada kebijakan retensi, arsip, maupun partisi tabel di desain v1 maupun v2.

## Dampak

**Belum ada dampak nyata** pada volume sekarang (masih sandbox, jumlah transaksi
kecil). Ini dicatat supaya tidak ditemukan sebagai kejutan.

Yang akan terasa lebih dulu, sesuai urutan kemungkinannya:

1. Ukuran disk Postgres tumbuh terus, didominasi `webhook_inbox`.
2. Backup makin lama dan makin besar, padahal isinya sebagian besar payload lama
   yang tidak pernah dibaca lagi.
3. Query audit yang men-scan rentang waktu makin lambat kalau index-nya tidak pas.

## Usulan solusi

Arah, bukan patch — dipilih saat issue ini dikerjakan:

- **Retensi berbasis waktu**: hapus `webhook_inbox` yang lebih tua dari N bulan
  (mis. 12 bulan, menyesuaikan kebutuhan sengketa DOKU); pertahankan
  `transaction_event` lebih lama karena jauh lebih ramping.
- **Arsip sebelum hapus**: dump ke object storage dingin, bukan langsung `DELETE`.
- **Partisi per bulan** (`PARTITION BY RANGE (created_at)`) supaya pemangkasan jadi
  `DROP PARTITION`, bukan `DELETE` massal yang membebani autovacuum.

Perlu diputuskan lebih dulu: **berapa lama bukti pembayaran wajib disimpan** —
pertanyaan kepatuhan, bukan pertanyaan teknis.

## Acceptance criteria

- [ ] Periode retensi ditetapkan untuk `transaction_event` dan `webhook_inbox`,
      dengan alasan yang tertulis
- [ ] Mekanisme pemangkasan berjalan otomatis (job di worker atau cron), dengan metrik
      jumlah baris yang dipangkas
- [ ] Ada jalur arsip atau keputusan eksplisit bahwa data lama boleh hilang
- [ ] Terdokumentasi di `docs/adr/` sebagai ADR baru kalau mengubah keputusan ADR-0009
