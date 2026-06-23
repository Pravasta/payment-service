package crypto_test

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
)

func randKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return key
}

func TestRoundTrip(t *testing.T) {
	key := randKey(t)
	plaintext := []byte("super-secret-gateway-credential")

	ct1, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	ct2, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt kedua: %v", err)
	}

	// Nonce acak — ciphertext harus berbeda tiap enkripsi.
	if bytes.Equal(ct1, ct2) {
		t.Fatal("Encrypt dua kali menghasilkan ciphertext identik (nonce tidak acak?)")
	}

	pt1, err := crypto.Decrypt(key, ct1)
	if err != nil {
		t.Fatalf("Decrypt ct1: %v", err)
	}
	if !bytes.Equal(pt1, plaintext) {
		t.Errorf("Decrypt ct1: dapat %q, ingin %q", pt1, plaintext)
	}

	pt2, err := crypto.Decrypt(key, ct2)
	if err != nil {
		t.Fatalf("Decrypt ct2: %v", err)
	}
	if !bytes.Equal(pt2, plaintext) {
		t.Errorf("Decrypt ct2: dapat %q, ingin %q", pt2, plaintext)
	}
}

func TestTamperDetected(t *testing.T) {
	key := randKey(t)
	ct, err := crypto.Encrypt(key, []byte("nilai-rahasia"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ubah satu byte di akhir (GCM tag) — harus gagal auth.
	ct[len(ct)-1] ^= 0xFF

	_, err = crypto.Decrypt(key, ct)
	if err != crypto.ErrInvalidCiphertext {
		t.Fatalf("Decrypt setelah tamper: ingin ErrInvalidCiphertext, dapat %v", err)
	}
}

func TestShortCiphertext(t *testing.T) {
	key := randKey(t)
	_, err := crypto.Decrypt(key, []byte("pendek"))
	if err != crypto.ErrInvalidCiphertext {
		t.Fatalf("Decrypt ciphertext pendek: ingin ErrInvalidCiphertext, dapat %v", err)
	}
}

func TestWrongKey(t *testing.T) {
	key1, key2 := randKey(t), randKey(t)
	ct, err := crypto.Encrypt(key1, []byte("nilai"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, err = crypto.Decrypt(key2, ct)
	if err != crypto.ErrInvalidCiphertext {
		t.Fatalf("Decrypt kunci salah: ingin ErrInvalidCiphertext, dapat %v", err)
	}
}

func TestKeyFromHex(t *testing.T) {
	// valid: 64 hex char = 32 byte
	validHex := strings.Repeat("ab", 32)
	key, err := crypto.KeyFromHex(validHex)
	if err != nil {
		t.Fatalf("KeyFromHex valid: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("KeyFromHex: panjang %d, ingin 32", len(key))
	}

	// terlalu pendek
	if _, err := crypto.KeyFromHex("abcd"); err == nil {
		t.Error("KeyFromHex terlalu pendek: harusnya error")
	}

	// bukan hex
	if _, err := crypto.KeyFromHex(strings.Repeat("zz", 32)); err == nil {
		t.Error("KeyFromHex bukan hex: harusnya error")
	}

	// 63 hex char (bukan kelipatan 2 yang menghasilkan 32 byte)
	if _, err := crypto.KeyFromHex(strings.Repeat("a", 63)); err == nil {
		t.Error("KeyFromHex 63 char: harusnya error")
	}
}

func TestEmptyPlaintext(t *testing.T) {
	key := randKey(t)
	ct, err := crypto.Encrypt(key, []byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	pt, err := crypto.Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if !bytes.Equal(pt, []byte{}) {
		t.Errorf("Decrypt empty: dapat %v", pt)
	}
}
