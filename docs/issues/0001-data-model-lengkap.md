# 0001 — Lengkapi data model & migrasi (sisa tabel)

- **Status:** done
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** —
- **Referensi:** detailed-design §2
- **Triase rombakan v2:** dikerjakan ulang di Phase 2 — sebagian superseded oleh ADR-0004 & ADR-0009
- **Alasan triase:** Tabel-tabelnya tetap dibutuhkan, tapi `merchant` menjadi `app` dan kolom `gateway` dibuang, jadi skemanya disentuh ulang.

## Konteks

Scaffold baru memodelkan `transaction`, `transaction_event`, dan `refund`. Tabel
lain di detailed-design §2 belum ada, padahal dibutuhkan auth, webhook, dan outbox.

## Scope

- [x] GORM model + `TableName()` untuk: `merchant`, `api_credential`,
      `webhook_endpoint`, `gateway_account`, `webhook_inbox`, `notification_outbox`.
- [x] Tambahkan ke `cmd/migrate` AutoMigrate.
- [x] Index & unique constraint sesuai §2 (`UNIQUE(app_id, external_reference)`,
      partial `UNIQUE(gateway_event_id) WHERE <> ''`, dll).
- [x] Pisahkan model DB dari entity domain (model package terpisah, sudah dipakai).

## Acceptance criteria

- [x] `make migrate` membuat 9 tabel tanpa error pada Postgres bersih.
- [x] Constraint unik & index terbukti ada; dedup partial index diuji fungsional
      (banyak `''` lolos, duplikat non-kosong ditolak).
- [x] `go build ./...` & `go vet ./...` hijau.

## Catatan implementasi

- `transaction_event.gateway_event_id` bertipe `string` (default `''`) → partial
  unique index dibuat eksplisit di `cmd/migrate` dengan `WHERE gateway_event_id <> ''`
  (padanan "NOT NULL" untuk kolom string; tag GORM tak bisa partial index).
- `api_credential.scopes` disimpan via `serializer:json` (bukan native `text[]`)
  agar tanpa dependensi array driver; cukup untuk MVP.
- Secret (`signing_secret_enc`, `config_enc`) kolom `bytea` — enkripsi diisi di issue 0002.

## File terkait

- `internal/adapter/repository/model/*.go`
- `cmd/migrate/main.go`

## Di luar scope

- Repository/CRUD untuk tabel baru (menyusul di issue yang membutuhkannya).
- Migrasi SQL berversi (AutoMigrate cukup untuk tahap ini).
