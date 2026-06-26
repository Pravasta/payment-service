package doku

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

// newTestAdapter membuat adapter deterministik yang menunjuk ke httptest server.
func newTestAdapter(srv *httptest.Server, reqID string, ts time.Time) *Adapter {
	a := New(Config{
		BaseURL:   srv.URL,
		ClientID:  "BRN-0276-TEST",
		SecretKey: "SK-test-secret",
	})
	a.client = srv.Client()
	a.now = func() time.Time { return ts }
	a.newRequestID = func() string { return reqID }
	return a
}

func TestCreateCharge_SignedRequestAndParse(t *testing.T) {
	const reqID = "req-fixed-123"
	fixedTS := time.Date(2026, 6, 26, 5, 0, 0, 0, time.UTC)

	var (
		gotPath    string
		gotHeaders http.Header
		gotBody    []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": ["SUCCESS"],
			"response": {
				"order": {"invoice_number": "INV-2026-001", "amount": "50000", "session_id": "sess-xyz"},
				"payment": {
					"token_id": "tok-abc",
					"url": "https://app-sandbox.doku.com/checkout/link/abc123",
					"expired_date": "20260626130000",
					"expired_datetime": "2026-06-26T06:00:00Z"
				}
			}
		}`))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, reqID, fixedTS)

	res, err := a.CreateCharge(context.Background(), domain.ChargeRequest{
		ExternalReference: "INV-2026-001",
		AmountMinor:       50000,
		Currency:          "IDR",
		CustomerName:      "Budi",
		CustomerEmail:     "budi@example.com",
		ExpiryMinutes:     60,
		ReturnURL:         "https://merchant.test/return",
	})
	if err != nil {
		t.Fatalf("CreateCharge error: %v", err)
	}

	// --- Request-Target & header signature ---
	if gotPath != checkoutPath {
		t.Errorf("path = %q, ingin %q", gotPath, checkoutPath)
	}
	if got := gotHeaders.Get("Client-Id"); got != "BRN-0276-TEST" {
		t.Errorf("Client-Id = %q", got)
	}
	if got := gotHeaders.Get("Request-Id"); got != reqID {
		t.Errorf("Request-Id = %q, ingin %q", got, reqID)
	}
	wantTS := fixedTS.Format(dokuTimeFormat)
	if got := gotHeaders.Get("Request-Timestamp"); got != wantTS {
		t.Errorf("Request-Timestamp = %q, ingin %q", got, wantTS)
	}

	// Digest harus base64(SHA256(body mentah)) dari body yang diterima server.
	wantDigest := Digest(gotBody)
	if got := gotHeaders.Get("Digest"); got != wantDigest {
		t.Errorf("Digest = %q, ingin %q", got, wantDigest)
	}

	// Signature harus cocok dengan komponen yang dihitung ulang.
	wantSig := Sign("SK-test-secret", ComponentString("BRN-0276-TEST", reqID, wantTS, checkoutPath, wantDigest))
	if got := gotHeaders.Get("Signature"); got != wantSig {
		t.Errorf("Signature = %q, ingin %q", got, wantSig)
	}

	// --- body request ---
	var sentBody checkoutRequest
	if err := json.Unmarshal(gotBody, &sentBody); err != nil {
		t.Fatalf("body terkirim bukan JSON valid: %v", err)
	}
	if sentBody.Order.Amount != 50000 {
		t.Errorf("order.amount = %d, ingin 50000 (JSON number)", sentBody.Order.Amount)
	}
	if sentBody.Order.InvoiceNumber != "INV-2026-001" {
		t.Errorf("order.invoice_number = %q", sentBody.Order.InvoiceNumber)
	}
	if sentBody.Order.Currency != "IDR" {
		t.Errorf("order.currency = %q", sentBody.Order.Currency)
	}
	if sentBody.Payment.PaymentDueDate != 60 {
		t.Errorf("payment.payment_due_date = %d, ingin 60", sentBody.Payment.PaymentDueDate)
	}

	// amount harus JSON number (tanpa kutip), bukan string.
	if !bodyHasNumericAmount(t, gotBody) {
		t.Error("order.amount harus JSON number, bukan string")
	}

	// --- hasil parsing ---
	if res.PaymentURL != "https://app-sandbox.doku.com/checkout/link/abc123" {
		t.Errorf("PaymentURL = %q", res.PaymentURL)
	}
	if res.GatewayRequestID != reqID {
		t.Errorf("GatewayRequestID = %q, ingin %q (Request-Id yg kita kirim)", res.GatewayRequestID, reqID)
	}
	if res.GatewayTxnID != "tok-abc" {
		t.Errorf("GatewayTxnID = %q, ingin token_id 'tok-abc'", res.GatewayTxnID)
	}
	if res.Status != domain.StatusPending {
		t.Errorf("Status = %q, ingin pending", res.Status)
	}
	if res.ExpiresAt == nil {
		t.Fatal("ExpiresAt nil, ingin dari expired_datetime")
	}
	wantExp := time.Date(2026, 6, 26, 6, 0, 0, 0, time.UTC)
	if !res.ExpiresAt.Equal(wantExp) {
		t.Errorf("ExpiresAt = %v, ingin %v (expired_datetime)", res.ExpiresAt.UTC(), wantExp)
	}
}

// bodyHasNumericAmount memeriksa order.amount diserialisasi sebagai number.
func bodyHasNumericAmount(t *testing.T, body []byte) bool {
	t.Helper()
	var m struct {
		Order struct {
			Amount json.RawMessage `json:"amount"`
		} `json:"order"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// number tidak diawali kutip.
	return len(m.Order.Amount) > 0 && m.Order.Amount[0] != '"'
}

func TestCreateCharge_DOKUError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_AMOUNT","message":["amount tidak valid"]}}`))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, "req-1", time.Now())

	_, err := a.CreateCharge(context.Background(), domain.ChargeRequest{
		ExternalReference: "INV-X",
		AmountMinor:       1000,
		Currency:          "IDR",
	})
	if err == nil {
		t.Fatal("ingin error, dapat nil")
	}
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("ingin *doku.Error, dapat %T: %v", err, err)
	}
	if de.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d", de.HTTPStatus)
	}
	if de.Code != "INVALID_AMOUNT" {
		t.Errorf("Code = %q", de.Code)
	}
	if de.Message != "amount tidak valid" {
		t.Errorf("Message = %q", de.Message)
	}
}

func TestCreateCharge_Validation(t *testing.T) {
	a := New(Config{BaseURL: "http://unused", ClientID: "c", SecretKey: "s"})

	cases := []struct {
		name string
		req  domain.ChargeRequest
	}{
		{"amount nol", domain.ChargeRequest{ExternalReference: "INV", AmountMinor: 0}},
		{"amount negatif", domain.ChargeRequest{ExternalReference: "INV", AmountMinor: -5}},
		{"amount kebesaran", domain.ChargeRequest{ExternalReference: "INV", AmountMinor: maxAmount + 1}},
		{"extref kosong", domain.ChargeRequest{ExternalReference: "", AmountMinor: 1000}},
		{"extref kepanjangan", domain.ChargeRequest{ExternalReference: string(make([]byte, 65)), AmountMinor: 1000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.CreateCharge(context.Background(), tc.req); err == nil {
				t.Errorf("ingin error untuk %s", tc.name)
			}
		})
	}
}

func TestParseCheckoutExpiry_WIBFallback(t *testing.T) {
	// Hanya expired_date (WIB) tersedia → harus dikonversi ke UTC (kurang 7 jam).
	var p checkoutResponse
	p.Response.Payment.ExpiredDate = "20260626130000" // 13:00 WIB
	got := parseCheckoutExpiry(p)
	if got == nil {
		t.Fatal("ExpiresAt nil")
	}
	want := time.Date(2026, 6, 26, 6, 0, 0, 0, time.UTC) // 06:00 UTC
	if !got.Equal(want) {
		t.Errorf("ExpiresAt = %v, ingin %v", got.UTC(), want)
	}
}

func TestFirstMessage(t *testing.T) {
	if got := firstMessage(json.RawMessage(`["a","b"]`)); got != "a" {
		t.Errorf("array: %q", got)
	}
	if got := firstMessage(json.RawMessage(`"solo"`)); got != "solo" {
		t.Errorf("string: %q", got)
	}
	if got := firstMessage(json.RawMessage(``)); got != "" {
		t.Errorf("empty: %q", got)
	}
}
