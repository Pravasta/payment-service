package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type fakeRecorder struct {
	method string
	route  string
	status int
	called int
}

func (f *fakeRecorder) ObserveHTTP(method, route string, status int, _ time.Duration) {
	f.method, f.route, f.status = method, route, status
	f.called++
}

func TestMetricsMiddleware_UsesRoutePattern(t *testing.T) {
	rec := &fakeRecorder{}
	r := chi.NewRouter()
	r.Use(Metrics(rec))
	r.Get("/v1/payments/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/v1/payments/abc", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	if rec.called != 1 {
		t.Fatalf("ObserveHTTP dipanggil %d kali, ingin 1", rec.called)
	}
	if rec.route != "/v1/payments/{id}" {
		t.Errorf("route = %q, ingin pola chi /v1/payments/{id}", rec.route)
	}
	if rec.method != "GET" || rec.status != 200 {
		t.Errorf("method/status = %s/%d, ingin GET/200", rec.method, rec.status)
	}
}

func TestAccessLog_IncludesRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(RequestID) // menyuntik correlation id
	r.Use(AccessLog(log))
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Request-Id", "corr-123")
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, want := range []string{`"request_id":"corr-123"`, `"status":200`, `"path":"/healthz"`} {
		if !strings.Contains(out, want) {
			t.Errorf("access log tidak memuat %q; got:\n%s", want, out)
		}
	}
}
