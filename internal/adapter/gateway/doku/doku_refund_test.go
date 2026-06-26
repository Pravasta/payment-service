package doku

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

func TestRefundEndpoint_Selection(t *testing.T) {
	cases := map[string]struct {
		path string
		ok   bool
	}{
		"CREDIT_CARD":         {"/cancellation/credit-card/refund", true},
		"QRIS":                {"/snap-adapter/b2b/v1.0/qr/qr-mpm-refund", true},
		"EMONEY_DANA_SNAP":    {"/direct-debit/core/v1/debit/refund", true},
		"VIRTUAL_ACCOUNT_BCA": {"", false},                                // VA tidak didukung
		"":                    {"/cancellation/credit-card/refund", true}, // default kartu
	}
	for pm, want := range cases {
		path, ok := refundEndpoint(pm)
		if ok != want.ok || path != want.path {
			t.Errorf("refundEndpoint(%q) = (%q,%v), ingin (%q,%v)", pm, path, ok, want.path, want.ok)
		}
	}
}

func TestRefund_VAUnsupported(t *testing.T) {
	a := New(Config{ClientID: "BRN", SecretKey: "SK"})
	_, err := a.Refund(context.Background(), domain.RefundRequest{PaymentMethod: "VIRTUAL_ACCOUNT_BCA"})
	if err != domain.ErrRefundNotSupported {
		t.Fatalf("ingin ErrRefundNotSupported, dapat %v", err)
	}
}

func TestRefund_CardSyncSuccess(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"refund":{"id":"rfnd-1","status":"SUCCESS"}}`))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, "req-rf", time.Now())
	res, err := a.Refund(context.Background(), domain.RefundRequest{
		ExternalReference: "INV-1",
		OriginalRequestID: "orig-1",
		AmountMinor:       50000,
		Currency:          "IDR",
		Type:              domain.RefundTypePartial,
		PaymentMethod:     "CREDIT_CARD",
	})
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if gotPath != "/cancellation/credit-card/refund" {
		t.Errorf("path = %q", gotPath)
	}
	if res.Status != domain.RefundSucceeded {
		t.Errorf("status = %q, ingin succeeded", res.Status)
	}
	if res.GatewayRefundID != "rfnd-1" {
		t.Errorf("GatewayRefundID = %q", res.GatewayRefundID)
	}
}
