# 0002 — Enkripsi secret at-rest (AES-GCM + master key)

- **Status:** done
- **Prioritas:** high
- **Estimasi:** S
- **Depends on:** 0001
- **Referensi:** detailed-design §9, doku-integration-spec §8
- **Triase rombakan v2:** berlaku
- **Alasan triase:** Enkripsi AES-GCM justru jadi fondasi ADR-0005 (kredensial DOKU per-app terenkripsi); tidak ada yang berubah.

## Konteks

Secret gateway (DOKU client_id/secret) & signing secret app disimpan terenkripsi
di kolom `*_enc` (bytea), kuncinya dari `PAYMENTS_MASTER_KEY`. Jalur migrasi ke
Vault/KMS tidak mengubah skema.

## Scope

- [x] Paket `internal/infrastructure/crypto` (atau `platform/secret`): `Encrypt`,
      `Decrypt` AES-256-GCM dengan nonce acak per record.
- [x] Master key dibaca dari config (`PAYMENTS_MASTER_KEY`), validasi panjang
      (32 byte). Gagal boot bila kosong di `APP_ENV=production`.
- [x] Helper untuk simpan/baca `gateway_account.config_enc` &
      `api_credential.signing_secret_enc` / `webhook_endpoint.signing_secret_enc`.

## Acceptance criteria

- [x] Unit test round-trip encrypt→decrypt; ciphertext berbeda tiap enkripsi
      (nonce), decrypt benar.
- [x] Tamper ciphertext → decrypt gagal (GCM auth).
- [x] Boot menolak master key kosong/lemah di production.

## File terkait

- `internal/infrastructure/crypto/aesgcm.go` (+ test)
- `internal/infrastructure/config/config.go` (validasi master key)

## Di luar scope

- Integrasi Vault/KMS (didokumentasikan saja sebagai jalur migrasi).
