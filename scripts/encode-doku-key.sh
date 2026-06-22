#!/usr/bin/env bash
#
# encode-doku-key.sh
# Encode DOKU API key menjadi nilai header HTTP Basic auth.
#
# Format DOKU: Authorization: Basic base64(<API_KEY>:)   <- perhatikan titik dua di akhir
# (API key sebagai username, password kosong — pola HTTP Basic auth).
#
# Key dibaca secara tersembunyi (tidak tampil di layar, tidak masuk shell history)
# dan tidak pernah dicetak kembali dalam bentuk mentah.

set -euo pipefail

printf 'Masukkan DOKU API Key (input disembunyikan): ' >&2
# -s: silent (tidak echo), -r: jangan tafsirkan backslash
read -rs API_KEY
printf '\n' >&2

if [[ -z "${API_KEY}" ]]; then
  printf 'Error: API key kosong.\n' >&2
  exit 1
fi

# Encode "<API_KEY>:" -> base64 (tanpa newline). printf (bukan echo) agar tak ada \n nyasar.
ENCODED="$(printf '%s' "${API_KEY}:" | base64 | tr -d '\n')"

# Bersihkan key mentah dari memori shell secepatnya.
unset API_KEY

printf '\n=== Hasil ===\n'
printf 'Encoded (base64)      : %s\n' "${ENCODED}"
printf 'Header Authorization  : Basic %s\n' "${ENCODED}"
printf '\nUntuk dipakai sebagai env var (jalankan baris ini di shell Anda):\n'
printf '  export DOKU_ENCODED_KEY=%s\n' "${ENCODED}"
printf '\nVerifikasi (harus muncul key Anda diakhiri tanda titik dua ":"):\n'
printf '  printf %%s "%s" | base64 --decode; echo\n' "${ENCODED}"
