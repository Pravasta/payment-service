package doku

import (
	"context"
	"testing"

	domain "github.com/Pravasta/payment-service/internal/domain/payment"
)

const webhookBody = `{
	"order": {"invoice_number": "INV-WH-1", "amount": "150000.00"},
	"transaction": {"status": "SUCCESS", "date": "2026-06-26T13:00:00Z", "original_request_id": "req-create-1"},
	"service": {"id": "VIRTUAL_ACCOUNT"},
	"channel": {"id": "VIRTUAL_ACCOUNT_BCA"},
	"acquirer": {"id": "BCA"}
}`

func signedWebhook(t *testing.T, secret, clientID, reqID, ts, target string, body []byte) domain.WebhookPayload {
	t.Helper()
	sig := Sign(secret, ComponentString(clientID, reqID, ts, target, Digest(body)))
	return domain.WebhookPayload{
		Headers: map[string]string{
			"Client-Id":         clientID,
			"Request-Id":        reqID,
			"Request-Timestamp": ts,
			"Signature":         sig,
		},
		RawBody: body,
		URLPath: target,
	}
}

func TestParseWebhook_ValidSignature(t *testing.T) {
	const secret = "SK-webhook-secret"
	a := New(Config{ClientID: "BRN-0276", SecretKey: secret})

	raw := signedWebhook(t, secret, "BRN-0276", "notif-1", "2026-06-26T13:00:01Z", "/v1/webhooks/doku", []byte(webhookBody))

	evt, err := a.ParseWebhook(context.Background(), raw)
	if err != nil {
		t.Fatalf("ParseWebhook error: %v", err)
	}

	if evt.GatewayEventID != "notif-1" {
		t.Errorf("GatewayEventID = %q, ingin notif-1 (Request-Id)", evt.GatewayEventID)
	}
	if evt.ExternalReference != "INV-WH-1" {
		t.Errorf("ExternalReference = %q", evt.ExternalReference)
	}
	if evt.OriginalRequestID != "req-create-1" {
		t.Errorf("OriginalRequestID = %q", evt.OriginalRequestID)
	}
	if evt.Status != domain.StatusPaid {
		t.Errorf("Status = %q, ingin paid (SUCCESS)", evt.Status)
	}
	if evt.PaymentMethod != "VIRTUAL_ACCOUNT_BCA" {
		t.Errorf("PaymentMethod = %q, ingin channel.id", evt.PaymentMethod)
	}
	if evt.AmountMinor != 150000 {
		t.Errorf("AmountMinor = %d, ingin 150000 (dari '150000.00')", evt.AmountMinor)
	}
}

func TestParseWebhook_InvalidSignature(t *testing.T) {
	a := New(Config{ClientID: "BRN-0276", SecretKey: "SK-correct"})

	// Ditandatangani dengan secret berbeda → harus ditolak.
	raw := signedWebhook(t, "SK-wrong", "BRN-0276", "notif-2", "2026-06-26T13:00:01Z", "/v1/webhooks/doku", []byte(webhookBody))

	_, err := a.ParseWebhook(context.Background(), raw)
	if err != domain.ErrInvalidSignature {
		t.Fatalf("ingin ErrInvalidSignature, dapat %v", err)
	}
}

func TestParseAmountMinor(t *testing.T) {
	cases := map[string]int64{"50000": 50000, "50000.00": 50000, "": 0, "abc": 0, "150000.99": 150000}
	for in, want := range cases {
		if got := parseAmountMinor(in); got != want {
			t.Errorf("parseAmountMinor(%q) = %d, ingin %d", in, got, want)
		}
	}
}
