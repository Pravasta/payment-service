package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeStore mensimulasikan DB store dengan map + mutex; Begin race-safe
// (meniru constraint UNIQUE app_id+key).
type fakeStore struct {
	mu       sync.Mutex
	records  map[string]*IdempotencyRecord // key: appID|key
	beginErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{records: make(map[string]*IdempotencyRecord)}
}

func (s *fakeStore) Begin(_ context.Context, appID uuid.UUID, key, hash string) (*IdempotencyRecord, bool, error) {
	if s.beginErr != nil {
		return nil, false, s.beginErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := appID.String() + "|" + key
	if existing, ok := s.records[k]; ok {
		cp := *existing
		return &cp, false, nil
	}
	rec := &IdempotencyRecord{ID: uuid.New(), RequestHash: hash, StatusCode: 0}
	s.records[k] = rec
	cp := *rec
	return &cp, true, nil
}

func (s *fakeStore) Complete(_ context.Context, id uuid.UUID, status int, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.records {
		if rec.ID == id {
			rec.StatusCode = status
			rec.ResponseBody = append([]byte(nil), body...)
			return nil
		}
	}
	return nil
}

func (s *fakeStore) Discard(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, rec := range s.records {
		if rec.ID == id {
			delete(s.records, k)
			return nil
		}
	}
	return nil
}

// reqWithAppID membuat request POST dengan app_id sudah di context (seolah lolos Authenticate).
func reqWithAppID(appID uuid.UUID, key, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/payments", strings.NewReader(body))
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	ctx := context.WithValue(r.Context(), ctxKeyAppID{}, appID)
	return r.WithContext(ctx)
}

func TestIdempotency_MissingHeader(t *testing.T) {
	mw := Idempotency(newFakeStore())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })

	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, reqWithAppID(uuid.New(), "", `{"a":1}`))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, ingin 400", w.Code)
	}
}

func TestIdempotency_NoAppID(t *testing.T) {
	mw := Idempotency(newFakeStore())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })

	r := httptest.NewRequest(http.MethodPost, "/v1/payments", strings.NewReader(`{}`))
	r.Header.Set("Idempotency-Key", "k1")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, r) // tanpa app_id di context

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status %d, ingin 500", w.Code)
	}
}

func TestIdempotency_FirstThenReplay(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store)

	var calls int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"txn_1"}`))
	})

	appID := uuid.New()

	// Request pertama → handler jalan, response di-cache.
	w1 := httptest.NewRecorder()
	mw(next).ServeHTTP(w1, reqWithAppID(appID, "key-A", `{"amount":1000}`))
	if w1.Code != http.StatusCreated {
		t.Fatalf("req1 status %d, ingin 201", w1.Code)
	}
	if body := w1.Body.String(); body != `{"id":"txn_1"}` {
		t.Errorf("req1 body %q", body)
	}
	if w1.Header().Get("Idempotent-Replay") == "true" {
		t.Error("req1 seharusnya bukan replay")
	}

	// Request kedua identik → replay, handler TIDAK dipanggil lagi.
	w2 := httptest.NewRecorder()
	mw(next).ServeHTTP(w2, reqWithAppID(appID, "key-A", `{"amount":1000}`))
	if w2.Code != http.StatusCreated {
		t.Fatalf("req2 status %d, ingin 201", w2.Code)
	}
	if body := w2.Body.String(); body != `{"id":"txn_1"}` {
		t.Errorf("req2 body %q, ingin sama dgn req1", body)
	}
	if w2.Header().Get("Idempotent-Replay") != "true" {
		t.Error("req2 seharusnya replay (header Idempotent-Replay: true)")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("handler dipanggil %d kali, ingin 1", got)
	}
}

func TestIdempotency_SameKeyDifferentPayload(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	})
	appID := uuid.New()

	w1 := httptest.NewRecorder()
	mw(next).ServeHTTP(w1, reqWithAppID(appID, "key-B", `{"amount":1000}`))
	if w1.Code != http.StatusCreated {
		t.Fatalf("req1 status %d", w1.Code)
	}

	// Key sama, payload beda → 409.
	w2 := httptest.NewRecorder()
	mw(next).ServeHTTP(w2, reqWithAppID(appID, "key-B", `{"amount":9999}`))
	if w2.Code != http.StatusConflict {
		t.Errorf("req2 status %d, ingin 409", w2.Code)
	}
}

func TestIdempotency_5xxNotCached(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store)

	var calls int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError) // gagal pertama
			return
		}
		w.WriteHeader(http.StatusCreated) // sukses pada retry
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	appID := uuid.New()

	w1 := httptest.NewRecorder()
	mw(next).ServeHTTP(w1, reqWithAppID(appID, "key-C", `{}`))
	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("req1 status %d, ingin 500", w1.Code)
	}

	// Retry harus menjalankan handler lagi (record 5xx dibuang).
	w2 := httptest.NewRecorder()
	mw(next).ServeHTTP(w2, reqWithAppID(appID, "key-C", `{}`))
	if w2.Code != http.StatusCreated {
		t.Errorf("req2 status %d, ingin 201 (retry sukses)", w2.Code)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("handler dipanggil %d kali, ingin 2", got)
	}
}

func TestIdempotency_BodyReadableDownstream(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store)

	var gotBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusCreated)
	})

	mw(next).ServeHTTP(httptest.NewRecorder(), reqWithAppID(uuid.New(), "key-D", `{"hello":"world"}`))
	if gotBody != `{"hello":"world"}` {
		t.Errorf("handler membaca body %q, ingin payload utuh (body harus di-restore)", gotBody)
	}
}

// TestIdempotency_ConcurrentSameKey memastikan side-effect tepat-sekali pada race.
func TestIdempotency_ConcurrentSameKey(t *testing.T) {
	store := newFakeStore()
	mw := Idempotency(store)

	var calls int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond) // perlebar window in-progress
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"txn_X"}`))
	})

	appID := uuid.New()
	const n = 12
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			mw(next).ServeHTTP(w, reqWithAppID(appID, "race-key", `{"amount":500}`))
			codes[idx] = w.Code
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("handler dipanggil %d kali pada race, ingin tepat 1 (tidak boleh duplikat side-effect)", got)
	}

	// Setiap response: 201 (pemenang/replay) atau 409 (in-progress). Tidak boleh selain itu.
	var ok, conflict int
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			ok++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("status tak terduga: %d", c)
		}
	}
	if ok == 0 {
		t.Error("setidaknya satu request harus sukses (201)")
	}
	if ok+conflict != n {
		t.Errorf("total response %d, ingin %d", ok+conflict, n)
	}
}
