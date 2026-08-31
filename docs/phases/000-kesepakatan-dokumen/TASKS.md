# Phase 000 — Kesepakatan & Dokumen · Tasks

| # | Task | Issue | Status |
|---|---|---|---|
| 1 | Buat `docs/adr/README.md` — indeks ADR + kerangka penulisan | #23 | todo |
| 2 | ADR-0001..0003 — DOKU-only tanpa abstraksi gateway, Gin, layout handler/service/repository | #24 | todo |
| 3 | ADR-0004..0006 — `merchant`→`app`, kredensial DOKU per-app + fallback env, Hosted Checkout + `allowed_channels` | #25 | todo |
| 4 | ADR-0007..0009 — kontrak hook ke aplikasi, provisioning `adminctl`, data yang dipertahankan/dibuang | #26 | todo |
| 5 | Triase 15 issue `docs/issues/` + kolom hasil triase di indeksnya | #27 | todo |
| 6 | Banner status dokumen desain v1 + update `docs/phases/README.md` & `README.md` | #28 | todo |

## Definisi selesai per task

- Task 2–4: tiap ADR memuat Status, Tanggal, **Sumber** (brainstorming / jawaban
  user / asumsi), Konteks, Keputusan, Konsekuensi (termasuk yang negatif), dan
  Alternatif yang ditolak.
- Task 5: tiap file issue punya baris `Triase rombakan v2` + `Alasan triase`.
- Task 6: tidak ada dokumen v1 yang bisa terbaca sebagai kebenaran saat ini tanpa
  penanda.

## Catatan

- **Nol file `.go` disentuh** di phase ini.
- `docs/api-documentation/` dan `docs/postman/` tidak disentuh (disinkronkan di
  phase terakhir lewat `/api-docs`).
- Temuan di luar scope diparkir lewat `/park-issue`, tidak diperbaiki di sini.
