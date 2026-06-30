package doku

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

func TestGetStatus_SignedAndParsed(t *testing.T) {
	const reqID = "req-status-1"
	fixedTS := time.Date(2026, 6, 26, 7, 0, 0, 0, time.UTC)

	var gotMethod, gotPath, gotSig, gotDigest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotSig = r.Header.Get("Signature")
		gotDigest = r.Header.Get("Digest")
		w.Header().Set("Content-Type", "application/json")
		// Bentuk nyata Check Status: order.amount berupa ANGKA (bukan string).
		_, _ = w.Write([]byte(`{
			"order": {"invoice_number": "INV-2026-001", "amount": 150000, "status": "ORDER_GENERATED"},
			"transaction": {"status": "SUCCESS"},
			"channel": {"id": "VIRTUAL_ACCOUNT_BCA"}
		}`))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, reqID, fixedTS)

	res, err := a.GetStatus(context.Background(), domain.StatusRef{ExternalReference: "INV-2026-001"})
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, ingin GET", gotMethod)
	}
	if gotPath != "/orders/v1/status/INV-2026-001" {
		t.Errorf("path = %q", gotPath)
	}
	// Signature GET: TANPA Digest (request tanpa body). DOKU menolak "Invalid
	// Header Signature" bila Digest disertakan untuk GET.
	wantSig := Sign("SK-test-secret", ComponentString("BRN-0276-TEST", reqID, fixedTS.Format(dokuTimeFormat), "/orders/v1/status/INV-2026-001", ""))
	if gotSig != wantSig {
		t.Errorf("Signature tidak cocok")
	}
	if gotDigest != "" {
		t.Errorf("header Digest tidak boleh diset untuk GET, dapat %q", gotDigest)
	}

	if res.Status != domain.StatusPaid {
		t.Errorf("Status = %q, ingin paid", res.Status)
	}
	if res.PaymentMethod != "VIRTUAL_ACCOUNT_BCA" {
		t.Errorf("PaymentMethod = %q", res.PaymentMethod)
	}
	if res.AmountMinor != 150000 {
		t.Errorf("AmountMinor = %d", res.AmountMinor)
	}
}

func TestGetStatus_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":["order tidak ditemukan"]}}`))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, "req-x", time.Now())
	if _, err := a.GetStatus(context.Background(), domain.StatusRef{ExternalReference: "INV-X"}); err == nil {
		t.Fatal("ingin error untuk HTTP 404")
	}
}
