// Package crypto menyediakan enkripsi AES-256-GCM untuk secret at-rest.
// Format ciphertext: nonce (12 byte) || ciphertext+GCM-tag.
// Jalur migrasi ke Vault/KMS hanya mengganti Encrypt/Decrypt; skema DB tidak berubah.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // GCM standard nonce
)

// ErrInvalidCiphertext dikembalikan ketika ciphertext terlalu pendek atau GCM auth tag gagal (tamper).
var ErrInvalidCiphertext = errors.New("crypto: ciphertext tidak valid atau kunci salah")

// KeyFromHex mem-parse hex string 64 karakter (32 byte) menjadi []byte kunci AES-256.
// Gunakan untuk mengonversi nilai PAYMENTS_MASTER_KEY dari env ke kunci siap pakai.
func KeyFromHex(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("crypto: PAYMENTS_MASTER_KEY bukan hex valid: %w", err)
	}
	if len(b) != keySize {
		return nil, fmt.Errorf("crypto: PAYMENTS_MASTER_KEY harus 32 byte (64 hex char), dapat %d byte", len(b))
	}
	return b, nil
}

// Encrypt mengenkripsi plaintext dengan AES-256-GCM menggunakan kunci 32 byte.
// Nonce 12 byte dibuat acak tiap panggilan — ciphertext selalu berbeda untuk plaintext yang sama.
// Output: nonce (12 byte) diikuti ciphertext+GCM-tag.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: buat cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: buat GCM: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: buat nonce: %w", err)
	}
	// Seal menambahkan GCM tag (16 byte) di akhir; nonce menjadi prefix.
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt mendekripsi output dari Encrypt.
// Mengembalikan ErrInvalidCiphertext bila ciphertext pendek, nonce salah,
// atau GCM auth tag gagal (tamper terdeteksi).
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: buat cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: buat GCM: %w", err)
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plaintext, nil
}
