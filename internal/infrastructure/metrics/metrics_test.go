package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// scrape menjalankan handler /metrics dan mengembalikan body teks Prometheus.
func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("scrape status = %d, ingin 200", rec.Code)
	}
	return rec.Body.String()
}

func TestMetrics_RecordsAndExposes(t *testing.T) {
	m := New()

	m.ObserveHTTP("POST", "/v1/payments", 201, 12*time.Millisecond)
	m.ObserveCharge("doku", true, 30*time.Millisecond)
	m.ObserveCharge("doku", false, 5*time.Millisecond)
	m.ObserveWebhook("doku", true)
	m.ObserveWebhook("doku", false)
	m.ObserveOutbox("delivered")
	m.ObserveOutbox("dead")
	m.SetOutboxBacklog(7, 2)
	m.ObserveReconcile(3, false)
	m.ObserveReconcile(0, true)

	body := scrape(t, m)

	wantContains := []string{
		`http_requests_total{method="POST",route="/v1/payments",status="201"} 1`,
		`gateway_charge_total{gateway="doku",result="success"} 1`,
		`gateway_charge_total{gateway="doku",result="error"} 1`,
		`webhook_total{gateway="doku",result="ok"} 1`,
		`webhook_total{gateway="doku",result="failed"} 1`,
		`outbox_dispatch_total{result="delivered"} 1`,
		`outbox_dispatch_total{result="dead"} 1`,
		`outbox_backlog{status="pending"} 7`,
		`outbox_backlog{status="dead"} 2`,
		`reconcile_changed_total 3`,
		`reconcile_errors_total 1`,
		`reconcile_runs_total 2`,
	}
	for _, w := range wantContains {
		if !strings.Contains(body, w) {
			t.Errorf("output /metrics tidak memuat %q", w)
		}
	}
}

func TestMetrics_HTTPRouteFallback(t *testing.T) {
	m := New()
	m.ObserveHTTP("GET", "", 404, time.Millisecond)
	body := scrape(t, m)
	if !strings.Contains(body, `route="unmatched"`) {
		t.Errorf("route kosong harus jadi \"unmatched\", body:\n%s", body)
	}
}
