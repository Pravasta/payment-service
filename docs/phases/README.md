# Phases

Setiap phase adalah satu unit pekerjaan besar dengan **satu branch dan satu PR**
ke `main`. Dibuka lewat skill `/phase`, ditutup lewat skill `/phase-close`.

Struktur tiap phase:

```
docs/phases/NNN-slug/
  PRD.md      # kenapa & apa (masalah, user story, acceptance criteria, non-goals)
  TD.md       # bagaimana (desain teknis, skema, kontrak, urutan kerja, risiko)
  TASKS.md    # checklist granular + nomor issue GitHub
```

| Phase | Judul | Branch | Status | PR |
|---|---|---|---|---|
| [000](000-kesepakatan-dokumen/) | Kesepakatan & Dokumen (kunci ADR K1–K9, triase issue v1) | `docs/phase-000-kesepakatan-dokumen` | review | — |

Status: `planned` → `in-progress` → `review` → `done`.

Keputusan yang sudah dikunci ada di [`docs/adr/`](../adr/) — itu rujukan utama
saat menulis kode. Rencana phase 0–8 untuk rombakan v2 ada di
[`docs/brainstorming/0002-rombak-v2-doku-only.md` §10](../brainstorming/0002-rombak-v2-doku-only.md).

## Aturan

- Temuan/bug di tengah phase **diparkir** ke `docs/issues/` (skill `/park-issue`),
  tidak diperbaiki di tengah jalan.
- OpenAPI (`docs/api-documentation/`) & Postman (`docs/postman/`) **hanya**
  disinkronkan di phase terakhir, lewat skill `/api-docs`.
