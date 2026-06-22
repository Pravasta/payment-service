# 0001 — Lengkapi data model & migrasi (sisa tabel)

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** —
- **Referensi:** detailed-design §2

## Konteks

Scaffold baru memodelkan `transaction`, `transaction_event`, dan `refund`. Tabel
lain di detailed-design §2 belum ada, padahal dibutuhkan auth, webhook, dan outbox.

## Scope

- [ ] GORM model + `TableName()` untuk: `merchant`, `api_credential`,
      `webhook_endpoint`, `gateway_account`, `webhook_inbox`, `notification_outbox`.
- [ ] Tambahkan ke `cmd/migrate` AutoMigrate.
- [ ] Index & unique constraint sesuai §2 (mis. `UNIQUE(app_id, external_reference)`,
      `UNIQUE(gateway_event_id) WHERE NOT NULL`).
- [ ] Pisahkan model DB dari entity domain bila modul terkait sudah butuh.

## Acceptance criteria

- [ ] `make migrate` membuat semua tabel tanpa error pada Postgres bersih.
- [ ] Constraint unik & index terbukti ada (cek `\d+ <table>` di psql).
- [ ] `go build ./...` & `go vet ./...` hijau.

## File terkait

- `internal/adapter/repository/model/*.go`
- `cmd/migrate/main.go`

## Di luar scope

- Repository/CRUD untuk tabel baru (menyusul di issue yang membutuhkannya).
- Migrasi SQL berversi (AutoMigrate cukup untuk tahap ini).
