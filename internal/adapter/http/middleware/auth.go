package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
)

// AuthRepository adalah port yang dibutuhkan middleware untuk lookup credential.
// Interface ini diimplementasikan oleh repository.CredentialRepository.
// Didefinisikan di sisi consumer (middleware) sesuai prinsip "interface belongs to consumer".
type AuthRepository interface {
	FindCredentialByKeyID(ctx context.Context, keyID string) (*Credential, error)
}

// Credential adalah proyeksi api_credential yang dibutuhkan untuk auth.
// Dibuat minimalis — hanya kolom yang relevan untuk proses verifikasi.
type Credential struct {
	ID               uuid.UUID
	MerchantID       uuid.UUID // dipakai sebagai app_id di context
	SecretHash       string    // argon2id PHC hash dari secret
	SigningSecretEnc []byte    // AES-GCM encrypted HMAC key; nil → HMAC tidak divalidasi
	Scopes           []string  // mis. ["payments:write", "payments:read"]
	Status           string    // "active" | "revoked"
}

// context key types — unexported agar tidak konflik dengan paket lain.
type ctxKeyAppID struct{}
type ctxKeyScopes struct{}

// AppIDFromContext mengambil app_id (merchant_id) yang sudah terautentikasi.
// Mengembalikan uuid.Nil bila middleware Authenticate tidak dipasang di route tersebut.
func AppIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(ctxKeyAppID{}).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// ScopesFromContext mengambil daftar scope dari context.
func ScopesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(ctxKeyScopes{}).([]string); ok {
		return v
	}
	return nil
}

// timestampSkew adalah batas maksimal selisih X-Timestamp dengan waktu server (anti-replay).
const timestampSkew = 5 * time.Minute

// Authenticate memvalidasi Authorization header (Bearer <key_id>:<secret>).
//
// Format header: Authorization: Bearer <key_id>:<secret>
// Bila credential punya SigningSecretEnc (HMAC aktif), dua header tambahan wajib ada:
//   - X-Timestamp: Unix detik (string), mis. "1719100000"
//   - X-Signature: base64(HMAC-SHA256(signingKey, componentString))
//
// Component string HMAC: "{timestamp}\n{METHOD}\n{request_uri}\n{sha256_hex(body)}"
//
// masterKey adalah kunci AES-GCM untuk mendekripsi SigningSecretEnc; boleh nil
// bila tidak ada credential yang mengaktifkan HMAC (misal: environment development).
func Authenticate(repo AuthRepository, masterKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID, secret, ok := parseAuthHeader(r)
			if !ok {
				respondAuthError(w, r, apperror.Unauthorized(
					"header Authorization tidak valid; gunakan: Bearer <key_id>:<secret>",
				))
				return
			}

			cred, err := repo.FindCredentialByKeyID(r.Context(), keyID)
			if err != nil {
				// error DB ataupun not-found diperlakukan sama — jangan beri tahu mana yang salah.
				respondAuthError(w, r, apperror.Unauthorized("API key tidak ditemukan atau sudah dicabut"))
				return
			}
			if cred.Status != "active" {
				respondAuthError(w, r, apperror.Unauthorized("API key sudah dicabut"))
				return
			}

			if !verifyArgon2id(secret, cred.SecretHash) {
				respondAuthError(w, r, apperror.Unauthorized("API key tidak valid"))
				return
			}

			// HMAC request signing — hanya bila credential mengaktifkannya.
			if len(cred.SigningSecretEnc) > 0 {
				if len(masterKey) == 0 {
					respondAuthError(w, r, apperror.Internal())
					return
				}
				signingKey, decErr := crypto.Decrypt(masterKey, cred.SigningSecretEnc)
				if decErr != nil {
					respondAuthError(w, r, apperror.Internal())
					return
				}
				if hmacErr := verifyHMAC(r, signingKey); hmacErr != nil {
					respondAuthError(w, r, hmacErr)
					return
				}
			}

			ctx := context.WithValue(r.Context(), ctxKeyAppID{}, cred.MerchantID)
			ctx = context.WithValue(ctx, ctxKeyScopes{}, cred.Scopes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireScope menolak request dengan 403 bila scope yang dibutuhkan tidak ada di context.
// Harus dipasang setelah Authenticate.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(ScopesFromContext(r.Context()), scope) {
				respondAuthError(w, r, apperror.Forbidden(
					"akses ditolak: scope '"+scope+"' diperlukan",
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- helpers ---

// parseAuthHeader mem-parsing "Authorization: Bearer <key_id>:<secret>".
// Prefix "Bearer " opsional — mendukung "Authorization: <key_id>:<secret>" juga.
func parseAuthHeader(r *http.Request) (keyID, secret string, ok bool) {
	h := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.SplitN(h, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// verifyArgon2id memverifikasi secret terhadap hash PHC argon2id.
// Format PHC: $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<saltB64>$<hashB64>
func verifyArgon2id(secret, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	// hasil split: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}

	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(secret), salt, iterations, memory, parallelism, uint32(len(storedHash)))
	return hmac.Equal(computed, storedHash)
}

// verifyHMAC memverifikasi X-Signature dan X-Timestamp pada request.
// Body dibaca dan di-restore ke r.Body agar handler downstream bisa membacanya.
func verifyHMAC(r *http.Request, signingKey []byte) *apperror.AppError {
	tsStr := r.Header.Get("X-Timestamp")
	sigStr := r.Header.Get("X-Signature")
	if tsStr == "" || sigStr == "" {
		return apperror.Unauthorized("X-Timestamp dan X-Signature wajib ada untuk endpoint ini")
	}

	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return apperror.Unauthorized("X-Timestamp tidak valid; gunakan Unix detik (integer)")
	}
	diff := time.Since(time.Unix(tsUnix, 0))
	if diff < 0 {
		diff = -diff
	}
	if diff > timestampSkew {
		return apperror.Unauthorized(
			fmt.Sprintf("X-Timestamp sudah kedaluwarsa (selisih %.0f detik, maksimal %s)", diff.Seconds(), timestampSkew),
		)
	}

	// Baca body — restore agar handler hilir bisa baca.
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return apperror.Internal()
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	bodyDigest := sha256.Sum256(bodyBytes)
	component := strings.Join([]string{
		tsStr,
		r.Method,
		r.URL.RequestURI(),
		hex.EncodeToString(bodyDigest[:]),
	}, "\n")

	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(component))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sigStr)) {
		return apperror.Unauthorized("X-Signature tidak valid")
	}
	return nil
}

// respondAuthError menulis error response JSON langsung dari middleware.
// Tidak mengimpor paket handler (itu yang mengimpor middleware — tidak boleh sirkular).
func respondAuthError(w http.ResponseWriter, r *http.Request, ae *apperror.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(ae.HTTPStatus)
	_ = json.NewEncoder(w).Encode(struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	}{
		Code:      ae.Code,
		Message:   ae.Message,
		RequestID: RequestIDFromContext(r.Context()),
	})
}
