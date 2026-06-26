package outbox_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
	"github.com/Pravasta/payment-service/internal/outbox"
)

type retryCall struct {
	id       uuid.UUID
	attempts int
	next     time.Time
}
type deadCall struct {
	id       uuid.UUID
	attempts int
}

type fakeStore struct {
	due         []outbox.Message
	endpoint    outbox.Endpoint
	endpointErr error
	delivered   []uuid.UUID
	retried     []retryCall
	dead        []deadCall
	replayed    []uuid.UUID
	fetched     bool
}

func (s *fakeStore) FetchDue(_ context.Context, _ int) ([]outbox.Message, error) {
	if s.fetched { // hanya satu batch agar test deterministik
		return nil, nil
	}
	s.fetched = true
	return s.due, nil
}
func (s *fakeStore) ActiveEndpoint(_ context.Context, _ uuid.UUID) (outbox.Endpoint, error) {
	return s.endpoint, s.endpointErr
}
func (s *fakeStore) MarkDelivered(_ context.Context, id uuid.UUID) error {
	s.delivered = append(s.delivered, id)
	return nil
}
func (s *fakeStore) MarkRetry(_ context.Context, id uuid.UUID, attempts int, next time.Time, _ string) error {
	s.retried = append(s.retried, retryCall{id, attempts, next})
	return nil
}
func (s *fakeStore) MarkDead(_ context.Context, id uuid.UUID, attempts int, _ string) error {
	s.dead = append(s.dead, deadCall{id, attempts})
	return nil
}
func (s *fakeStore) Replay(_ context.Context, id uuid.UUID) error {
	s.replayed = append(s.replayed, id)
	return nil
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func randKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func sampleMessage() outbox.Message {
	return outbox.Message{
		ID:            uuid.New(),
		AppID:         uuid.New(),
		TransactionID: uuid.New(),
		EventID:       "evt_123",
		EventType:     "payment.paid",
		Payload:       map[string]any{"payment": map[string]any{"id": "pay_1", "status": "paid"}},
	}
}

func TestDispatcher_DeliveredAndSigned(t *testing.T) {
	masterKey := randKey(t)
	signingSecret := []byte("merchant-signing-secret")
	enc, err := crypto.Encrypt(masterKey, signingSecret)
	if err != nil {
		t.Fatal(err)
	}

	var gotBody []byte
	var gotSig, gotEventID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-Signature")
		gotEventID = r.Header.Get("X-Event-Id")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	msg := sampleMessage()
	store := &fakeStore{due: []outbox.Message{msg}, endpoint: outbox.Endpoint{URL: srv.URL, SigningSecretEnc: enc}}
	d := outbox.NewDispatcher(store, masterKey, testLogger(), outbox.DefaultConfig())

	n, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("processed = %d, ingin 1", n)
	}
	if len(store.delivered) != 1 || store.delivered[0] != msg.ID {
		t.Fatalf("MarkDelivered tidak dipanggil benar: %v", store.delivered)
	}

	// Header X-Event-Id & X-Signature.
	if gotEventID != "evt_123" {
		t.Errorf("X-Event-Id = %q", gotEventID)
	}
	wantSig := outbox.Sign(signingSecret, gotBody)
	if gotSig != wantSig {
		t.Errorf("X-Signature tidak cocok (penerima tak bisa verifikasi)")
	}

	// Body memuat event_id, event_type, payment (detailed-design §4.7).
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("body bukan JSON: %v", err)
	}
	if body["event_id"] != "evt_123" || body["event_type"] != "payment.paid" {
		t.Errorf("body = %v", body)
	}
	if _, ok := body["payment"]; !ok {
		t.Error("body tidak memuat payment")
	}
}

func TestDispatcher_RetryOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	msg := sampleMessage() // Attempts = 0
	store := &fakeStore{due: []outbox.Message{msg}, endpoint: outbox.Endpoint{URL: srv.URL}}
	d := outbox.NewDispatcher(store, nil, testLogger(), outbox.DefaultConfig())

	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(store.delivered) != 0 {
		t.Error("tidak boleh delivered saat app gagal")
	}
	if len(store.retried) != 1 {
		t.Fatalf("MarkRetry = %d, ingin 1", len(store.retried))
	}
	if store.retried[0].attempts != 1 {
		t.Errorf("attempts = %d, ingin 1", store.retried[0].attempts)
	}
	if !store.retried[0].next.After(time.Now()) {
		t.Error("next_retry_at harus di masa depan (backoff)")
	}
}

func TestDispatcher_DeadAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := outbox.DefaultConfig()
	msg := sampleMessage()
	msg.Attempts = cfg.MaxAttempts - 1 // satu attempt lagi → dead
	store := &fakeStore{due: []outbox.Message{msg}, endpoint: outbox.Endpoint{URL: srv.URL}}
	d := outbox.NewDispatcher(store, nil, testLogger(), cfg)

	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(store.dead) != 1 {
		t.Fatalf("MarkDead = %d, ingin 1", len(store.dead))
	}
	if store.dead[0].attempts != cfg.MaxAttempts {
		t.Errorf("attempts = %d, ingin %d", store.dead[0].attempts, cfg.MaxAttempts)
	}
	if len(store.retried) != 0 {
		t.Error("tidak boleh retry setelah lewat batas")
	}
}

func TestDispatcher_NoEndpointRetried(t *testing.T) {
	msg := sampleMessage()
	store := &fakeStore{due: []outbox.Message{msg}, endpointErr: outbox.ErrNoEndpoint}
	d := outbox.NewDispatcher(store, nil, testLogger(), outbox.DefaultConfig())

	if _, err := d.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// Tanpa endpoint → diperlakukan gagal (retry), bukan delivered.
	if len(store.delivered) != 0 {
		t.Error("tidak boleh delivered tanpa endpoint")
	}
	if len(store.retried) != 1 {
		t.Errorf("MarkRetry = %d, ingin 1", len(store.retried))
	}
}

func TestDispatcher_Replay(t *testing.T) {
	store := &fakeStore{}
	d := outbox.NewDispatcher(store, nil, testLogger(), outbox.DefaultConfig())
	id := uuid.New()
	if err := d.Replay(context.Background(), id); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(store.replayed) != 1 || store.replayed[0] != id {
		t.Errorf("Replay tidak diteruskan ke store: %v", store.replayed)
	}
}
