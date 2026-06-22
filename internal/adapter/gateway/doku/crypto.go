// Package doku adalah anti-corruption layer ke DOKU (implementasi domain.Gateway).
package doku

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// Digest menghitung header Digest DOKU: base64(SHA-256(raw body)).
// Pakai byte mentah body — jangan re-serialize JSON (doku-integration-spec §1).
func Digest(rawBody []byte) string {
	sum := sha256.Sum256(rawBody)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// ComponentString menyusun string yang ditandatangani (urutan & newline persis).
//
//	Client-Id:{clientId}
//	Request-Id:{requestId}
//	Request-Timestamp:{requestTimestamp}
//	Request-Target:{requestTarget}
//	Digest:{digest}
func ComponentString(clientID, requestID, requestTimestamp, requestTarget, digest string) string {
	return strings.Join([]string{
		"Client-Id:" + clientID,
		"Request-Id:" + requestID,
		"Request-Timestamp:" + requestTimestamp,
		"Request-Target:" + requestTarget,
		"Digest:" + digest,
	}, "\n")
}

// Sign menghasilkan header Signature: "HMACSHA256=" + base64(HMAC-SHA256(secret, component)).
func Sign(secretKey, component string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(component))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// VerifySignature memverifikasi signature webhook DOKU secara constant-time.
func VerifySignature(secretKey, signatureHeader string, p SignatureParams) bool {
	expected := Sign(secretKey, ComponentString(
		p.ClientID, p.RequestID, p.RequestTimestamp, p.RequestTarget, Digest(p.RawBody),
	))
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}

// SignatureParams adalah komponen untuk verifikasi signature webhook.
type SignatureParams struct {
	ClientID         string
	RequestID        string
	RequestTimestamp string
	RequestTarget    string // path URL notifikasi, mis. "/v1/webhooks/doku"
	RawBody          []byte
}

// FormatAmount memformat amount minor unit sesuai endpoint DOKU
// (doku-integration-spec §4): Checkout = integer rupiah; VA/SNAP = 2 desimal.
func FormatAmount(endpoint Endpoint, amountMinor int64, currency string) string {
	switch endpoint {
	case EndpointVA, EndpointSNAP:
		// IDR exponent 0 -> tampilkan "<rupiah>.00"
		return fmt.Sprintf("%d.00", amountMinor)
	default: // Checkout
		return fmt.Sprintf("%d", amountMinor)
	}
}

// Endpoint membedakan format/auth antar produk DOKU.
type Endpoint string

const (
	EndpointCheckout Endpoint = "checkout"
	EndpointVA       Endpoint = "va"
	EndpointSNAP     Endpoint = "snap"
)
