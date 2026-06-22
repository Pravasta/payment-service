# 0003 — Auth middleware (API key + HMAC signing)

- **Status:** todo
- **Prioritas:** high
- **Estimasi:** M
- **Depends on:** 0001, 0002
- **Referensi:** detailed-design §9, architecture-review §8.1

## Konteks

App memanggil PS dengan API key (`key_id` + secret di-hash argon2id), dengan opsi
HMAC request signing (`X-Signature` atas timestamp+method+path+body, anti-replay
via `X-Timestamp`). Hasil auth menetapkan `app_id` untuk scope semua query.

## Scope

- [ ] Middleware auth: validasi `Authorization` (key_id+secret) → lookup
      `api_credential` → cek argon2id hash + status active.
- [ ] (Opsional aktif untuk Invoice SaaS) verifikasi HMAC `X-Signature` +
      `X-Timestamp` (tolak skew > N menit).
- [ ] Set `app_id` & scopes ke context; helper `AppIDFromContext`.
- [ ] Enforce scope per endpoint (mis. `payments:write`).

## Acceptance criteria

- [ ] Request tanpa/`key` salah → `401`; scope kurang → `403`.
- [ ] HMAC salah / timestamp basi → `401`.
- [ ] Handler hilir bisa membaca `app_id` dari context.
- [ ] Unit test untuk verifikasi key & HMAC.

## File terkait

- `internal/adapter/http/middleware/auth.go`
- `internal/adapter/repository/` (lookup credential)
- `internal/adapter/http/router.go` (pasang middleware di grup `/v1`)

## Di luar scope

- Endpoint admin pembuatan/rotasi API key (issue terpisah bila perlu).
- mTLS (jangka panjang).
