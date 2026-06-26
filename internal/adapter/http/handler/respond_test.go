package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pravasta/payment-service/internal/adapter/http/apperror"
)

// withCapturedLog mengganti slog default sementara untuk menangkap output.
func withCapturedLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestWriteError_Logs5xxWithUnderlyingCause(t *testing.T) {
	buf := withCapturedLog(t)

	r := httptest.NewRequest("POST", "/v1/payments", nil)
	w := httptest.NewRecorder()

	// Error tak dikenal (mis. gateway gagal) → dipetakan ke 500.
	writeError(w, r, errors.New("gateway create charge: doku: Invalid Client-Id"))

	if w.Code != 500 {
		t.Fatalf("status = %d, ingin 500", w.Code)
	}
	// Body ke client TIDAK membocorkan detail internal.
	if strings.Contains(w.Body.String(), "Invalid Client-Id") {
		t.Errorf("body client membocorkan detail internal: %s", w.Body.String())
	}
	// Tapi log server HARUS memuat penyebab asli.
	out := buf.String()
	for _, want := range []string{`"level":"ERROR"`, "Invalid Client-Id", `"path":"/v1/payments"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log tidak memuat %q; got:\n%s", want, out)
		}
	}
}

func TestWriteError_DoesNotLog4xx(t *testing.T) {
	buf := withCapturedLog(t)

	r := httptest.NewRequest("GET", "/v1/payments/x", nil)
	w := httptest.NewRecorder()

	writeError(w, r, apperror.BadRequest("id pembayaran tidak valid"))

	if w.Code != 400 {
		t.Fatalf("status = %d, ingin 400", w.Code)
	}
	if buf.Len() != 0 {
		t.Errorf("error 4xx seharusnya tidak dilog, tapi ada output:\n%s", buf.String())
	}
}
