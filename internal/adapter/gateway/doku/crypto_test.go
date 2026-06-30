package doku

import "testing"

func TestDigest(t *testing.T) {
	// SHA-256("") base64 = 47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=
	got := Digest([]byte(""))
	want := "47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="
	if got != want {
		t.Fatalf("Digest empty = %q, want %q", got, want)
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	secret := "SK-test-secret"
	body := []byte(`{"order":{"invoice_number":"INV-1"}}`)
	p := SignatureParams{
		ClientID:         "BRN-0276-1",
		RequestID:        "req-123",
		RequestTimestamp: "2026-06-22T03:10:00Z",
		RequestTarget:    "/v1/webhooks/doku",
		RawBody:          body,
	}
	sig := Sign(secret, ComponentString(p.ClientID, p.RequestID, p.RequestTimestamp, p.RequestTarget, Digest(body)))

	if !VerifySignature(secret, sig, p) {
		t.Fatal("VerifySignature gagal untuk signature yang valid")
	}
	if VerifySignature("wrong-secret", sig, p) {
		t.Fatal("VerifySignature seharusnya gagal untuk secret salah")
	}
}

func TestComponentString_OmitsDigestWhenEmpty(t *testing.T) {
	base := "Client-Id:CID\nRequest-Id:RID\nRequest-Timestamp:TS\nRequest-Target:/t"

	// GET tanpa body → tanpa baris Digest (DOKU menolak bila disertakan).
	if got := ComponentString("CID", "RID", "TS", "/t", ""); got != base {
		t.Errorf("tanpa digest:\n got=%q\nwant=%q", got, base)
	}
	// Dengan body → baris Digest disertakan (perilaku POST tetap).
	if got := ComponentString("CID", "RID", "TS", "/t", "DG"); got != base+"\nDigest:DG" {
		t.Errorf("dengan digest = %q", got)
	}
}

func TestFormatAmount(t *testing.T) {
	if got := FormatAmount(EndpointCheckout, 50000, "IDR"); got != "50000" {
		t.Errorf("Checkout amount = %q, want \"50000\"", got)
	}
	if got := FormatAmount(EndpointVA, 50000, "IDR"); got != "50000.00" {
		t.Errorf("VA amount = %q, want \"50000.00\"", got)
	}
}
