package middleware_test

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/Pravasta/payment-service/internal/adapter/http/middleware"
	appcrypto "github.com/Pravasta/payment-service/internal/infrastructure/crypto"
)

// --- helper: buat argon2id hash dalam format PHC ---

func hashArgon2id(secret string) string {
	salt := make([]byte, 16)
	rand.Read(salt) //nolint:errcheck
	const memory, iterations, parallelism, keyLen = 64 * 1024, 3, 4, 32
	h := argon2.IDKey([]byte(secret), salt, iterations, memory, parallelism, keyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(h),
	)
}

// --- stub AuthRepository ---

type stubRepo struct {
	cred *middleware.Credential
	err  error
}

func (s *stubRepo) FindCredentialByKeyID(_ context.Context, _ string) (*middleware.Credential, error) {
	return s.cred, s.err
}

// --- helper: buat request dengan Authorization header ---

func newRequest(keyID, secret string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	r.Header.Set("Authorization", "Bearer "+keyID+":"+secret)
	return r
}

// --- test: Authenticate middleware ---

func TestAuthenticate_ValidKey(t *testing.T) {
	secret := "my-super-secret"
	cred := &middleware.Credential{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		SecretHash: hashArgon2id(secret),
		Scopes:     []string{"payments:write"},
		Status:     "active",
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, nil)

	var gotAppID uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAppID = middleware.AppIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, newRequest("pk_live_test", secret))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, ingin 200; body: %s", w.Code, w.Body.String())
	}
	if gotAppID != cred.MerchantID {
		t.Errorf("AppIDFromContext: dapat %s, ingin %s", gotAppID, cred.MerchantID)
	}
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	repo := &stubRepo{}
	mw := middleware.Authenticate(repo, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

func TestAuthenticate_WrongSecret(t *testing.T) {
	cred := &middleware.Credential{
		SecretHash: hashArgon2id("correct-secret"),
		Status:     "active",
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, newRequest("pk_live_test", "wrong-secret"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

func TestAuthenticate_RevokedCredential(t *testing.T) {
	secret := "some-secret"
	cred := &middleware.Credential{
		SecretHash: hashArgon2id(secret),
		Status:     "revoked", // dicabut
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, newRequest("pk_live_test", secret))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

func TestAuthenticate_RepoError(t *testing.T) {
	repo := &stubRepo{err: fmt.Errorf("db down")}
	mw := middleware.Authenticate(repo, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, newRequest("pk_live_test", "any"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

// --- test: RequireScope ---

func TestRequireScope_HasScope(t *testing.T) {
	mw := middleware.RequireScope("payments:write")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r := httptest.NewRequest(http.MethodPost, "/v1/payments", nil)
	// inject scopes ke context seperti yang dilakukan Authenticate
	ctx := context.WithValue(r.Context(), struct{ key string }{"scopes"}, []string{"payments:write"})
	_ = ctx // context key private — test via end-to-end flow di bawah

	// Simulasi penuh: Authenticate → RequireScope
	secret := "s3cr3t"
	cred := &middleware.Credential{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		SecretHash: hashArgon2id(secret),
		Scopes:     []string{"payments:write", "payments:read"},
		Status:     "active",
	}
	repo := &stubRepo{cred: cred}

	chain := middleware.Authenticate(repo, nil)(mw(next))

	w := httptest.NewRecorder()
	req := newRequest("pk_live_test", secret)
	chain.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status %d, ingin 200; body: %s", w.Code, w.Body.String())
	}
}

func TestRequireScope_MissingScope(t *testing.T) {
	mw := middleware.RequireScope("payments:admin")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	secret := "s3cr3t"
	cred := &middleware.Credential{
		ID:         uuid.New(),
		MerchantID: uuid.New(),
		SecretHash: hashArgon2id(secret),
		Scopes:     []string{"payments:read"}, // tidak ada payments:admin
		Status:     "active",
	}
	repo := &stubRepo{cred: cred}
	chain := middleware.Authenticate(repo, nil)(mw(next))

	w := httptest.NewRecorder()
	chain.ServeHTTP(w, newRequest("pk_live_test", secret))

	if w.Code != http.StatusForbidden {
		t.Errorf("status %d, ingin 403", w.Code)
	}
}

// --- test: HMAC signing ---

func TestAuthenticate_HMACValid(t *testing.T) {
	// Siapkan master key dan signing key
	masterKey := make([]byte, 32)
	rand.Read(masterKey) //nolint:errcheck
	signingKey := make([]byte, 32)
	rand.Read(signingKey) //nolint:errcheck

	signingKeyEnc, err := appcrypto.Encrypt(masterKey, signingKey)
	if err != nil {
		t.Fatalf("enkripsi signing key: %v", err)
	}

	secret := "hmac-secret"
	cred := &middleware.Credential{
		ID:               uuid.New(),
		MerchantID:       uuid.New(),
		SecretHash:       hashArgon2id(secret),
		SigningSecretEnc: signingKeyEnc,
		Scopes:           []string{"payments:write"},
		Status:           "active",
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, masterKey)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	tsStr := strconv.FormatInt(time.Now().Unix(), 10)
	body := ""
	bodyDigest := sha256.Sum256([]byte(body))
	component := strings.Join([]string{tsStr, "GET", "/v1/payments", hex.EncodeToString(bodyDigest[:])}, "\n")
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(component))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	r := newRequest("pk_live_test", secret)
	r.Header.Set("X-Timestamp", tsStr)
	r.Header.Set("X-Signature", sig)

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status %d, ingin 200; body: %s", w.Code, w.Body.String())
	}
}

func TestAuthenticate_HMACExpiredTimestamp(t *testing.T) {
	masterKey := make([]byte, 32)
	rand.Read(masterKey) //nolint:errcheck
	signingKey := make([]byte, 32)
	rand.Read(signingKey) //nolint:errcheck

	signingKeyEnc, _ := appcrypto.Encrypt(masterKey, signingKey)
	secret := "hmac-secret"
	cred := &middleware.Credential{
		SecretHash:       hashArgon2id(secret),
		SigningSecretEnc: signingKeyEnc,
		Status:           "active",
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, masterKey)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// Timestamp 10 menit yang lalu → kedaluwarsa
	tsStr := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	r := newRequest("pk_live_test", secret)
	r.Header.Set("X-Timestamp", tsStr)
	r.Header.Set("X-Signature", "invalid-but-timestamp-checked-first")

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

func TestAuthenticate_HMACWrongSignature(t *testing.T) {
	masterKey := make([]byte, 32)
	rand.Read(masterKey) //nolint:errcheck
	signingKey := make([]byte, 32)
	rand.Read(signingKey) //nolint:errcheck

	signingKeyEnc, _ := appcrypto.Encrypt(masterKey, signingKey)
	secret := "hmac-secret"
	cred := &middleware.Credential{
		SecretHash:       hashArgon2id(secret),
		SigningSecretEnc: signingKeyEnc,
		Status:           "active",
	}
	repo := &stubRepo{cred: cred}
	mw := middleware.Authenticate(repo, masterKey)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	tsStr := strconv.FormatInt(time.Now().Unix(), 10)
	r := newRequest("pk_live_test", secret)
	r.Header.Set("X-Timestamp", tsStr)
	r.Header.Set("X-Signature", "signature-salah")

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status %d, ingin 401", w.Code)
	}
}

// --- test: error response JSON berisi field yang diharapkan ---

func TestErrorResponseFormat(t *testing.T) {
	repo := &stubRepo{}
	mw := middleware.Authenticate(repo, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	r := httptest.NewRequest(http.MethodGet, "/v1/payments", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r)

	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type %q, ingin application/json", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"code"`) || !strings.Contains(body, `"message"`) {
		t.Errorf("response JSON tidak punya field code/message: %s", body)
	}
}
