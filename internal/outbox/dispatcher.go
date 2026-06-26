// Package outbox mengirim callback PS → app dari notification_outbox dengan
// retry, exponential backoff + jitter, dan dead-letter (detailed-design §6.2).
package outbox

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Pravasta/payment-service/internal/infrastructure/crypto"
)

// Message adalah satu baris notification_outbox yang siap dikirim.
type Message struct {
	ID            uuid.UUID
	AppID         uuid.UUID // merchant_id — untuk lookup webhook_endpoint
	TransactionID uuid.UUID
	EventID       string
	EventType     string
	Payload       map[string]any
	Attempts      int
}

// Endpoint adalah tujuan callback milik merchant.
type Endpoint struct {
	URL              string
	SigningSecretEnc []byte // AES-GCM; di-decrypt dengan master key untuk HMAC
}

// ErrNoEndpoint dikembalikan bila merchant tidak punya webhook_endpoint aktif.
var ErrNoEndpoint = errors.New("outbox: tidak ada webhook_endpoint aktif untuk merchant")

// ErrNotReplayable dikembalikan saat Replay dipanggil untuk id yang tidak ada
// atau tidak berstatus dead.
var ErrNotReplayable = errors.New("outbox: pesan tidak ditemukan atau bukan dead-letter")

// Store adalah port persistensi outbox (diimplementasi repository GORM).
type Store interface {
	FetchDue(ctx context.Context, limit int) ([]Message, error)
	ActiveEndpoint(ctx context.Context, merchantID uuid.UUID) (Endpoint, error)
	MarkDelivered(ctx context.Context, id uuid.UUID) error
	MarkRetry(ctx context.Context, id uuid.UUID, attempts int, nextRetryAt time.Time, lastErr string) error
	MarkDead(ctx context.Context, id uuid.UUID, attempts int, lastErr string) error
	Replay(ctx context.Context, id uuid.UUID) error
}

// Config menyetel perilaku dispatcher.
type Config struct {
	MaxAttempts int           // lewat ini → dead (mis. 12)
	BaseBackoff time.Duration // backoff awal (mis. 10s)
	MaxBackoff  time.Duration // cap backoff (mis. 1h)
	BatchSize   int           // jumlah pesan per RunOnce
	HTTPTimeout time.Duration
}

// DefaultConfig mengembalikan nilai default yang masuk akal untuk MVP.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 12,
		BaseBackoff: 10 * time.Second,
		MaxBackoff:  time.Hour,
		BatchSize:   50,
		HTTPTimeout: 10 * time.Second,
	}
}

// MetricsRecorder adalah port observability opsional untuk dispatcher.
// Dideklarasikan di sisi konsumen agar package outbox tidak meng-import metrics.
type MetricsRecorder interface {
	ObserveOutbox(result string) // result: delivered/retry/dead
}

// Option mengonfigurasi Dispatcher saat konstruksi (functional options).
type Option func(*Dispatcher)

// WithMetrics memasang recorder metrik. Tanpa ini, instrumentasi di-skip (nil-safe).
func WithMetrics(m MetricsRecorder) Option {
	return func(d *Dispatcher) { d.metrics = m }
}

// Dispatcher memproses pesan outbox.
type Dispatcher struct {
	store     Store
	masterKey []byte
	log       *slog.Logger
	client    *http.Client
	cfg       Config
	metrics   MetricsRecorder

	// injectable untuk test deterministik.
	now  func() time.Time
	rand func() float64
}

// NewDispatcher membuat dispatcher. masterKey boleh nil bila tidak ada endpoint
// yang memerlukan signing secret (mis. dev).
func NewDispatcher(store Store, masterKey []byte, log *slog.Logger, cfg Config, opts ...Option) *Dispatcher {
	if cfg.MaxAttempts == 0 {
		cfg = DefaultConfig()
	}
	d := &Dispatcher{
		store:     store,
		masterKey: masterKey,
		log:       log,
		client:    &http.Client{Timeout: cfg.HTTPTimeout},
		cfg:       cfg,
		now:       time.Now,
		rand:      rand.Float64,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// recordOutbox mencatat hasil dispatch ke metrik bila recorder terpasang.
func (d *Dispatcher) recordOutbox(result string) {
	if d.metrics != nil {
		d.metrics.ObserveOutbox(result)
	}
}

// RunOnce memproses satu batch pesan yang due. Mengembalikan jumlah yang diproses.
func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	msgs, err := d.store.FetchDue(ctx, d.cfg.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("outbox: fetch due: %w", err)
	}
	for _, m := range msgs {
		d.process(ctx, m)
	}
	return len(msgs), nil
}

// process mengirim satu pesan & memperbarui statusnya (delivered/retry/dead).
func (d *Dispatcher) process(ctx context.Context, m Message) {
	if err := d.deliver(ctx, m); err != nil {
		attempts := m.Attempts + 1
		if attempts >= d.cfg.MaxAttempts {
			d.recordOutbox("dead")
			if e := d.store.MarkDead(ctx, m.ID, attempts, err.Error()); e != nil {
				d.log.Error("outbox: mark dead gagal", "id", m.ID, "err", e)
			}
			d.log.Error("outbox: pesan dead (lewat batas attempt)", "id", m.ID, "event_id", m.EventID, "err", err)
			return
		}
		d.recordOutbox("retry")
		next := d.now().UTC().Add(d.backoff(attempts))
		if e := d.store.MarkRetry(ctx, m.ID, attempts, next, err.Error()); e != nil {
			d.log.Error("outbox: mark retry gagal", "id", m.ID, "err", e)
		}
		d.log.Warn("outbox: kirim gagal, dijadwalkan retry", "id", m.ID, "attempt", attempts, "next_retry_at", next, "err", err)
		return
	}
	d.recordOutbox("delivered")
	if err := d.store.MarkDelivered(ctx, m.ID); err != nil {
		d.log.Error("outbox: mark delivered gagal", "id", m.ID, "err", err)
	}
}

// deliver mengirim HTTP POST bertanda tangan ke webhook_endpoint app.
func (d *Dispatcher) deliver(ctx context.Context, m Message) error {
	ep, err := d.store.ActiveEndpoint(ctx, m.AppID)
	if err != nil {
		return err
	}

	body, err := buildBody(m)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-Id", m.EventID)

	if len(ep.SigningSecretEnc) > 0 {
		secret, derr := crypto.Decrypt(d.masterKey, ep.SigningSecretEnc)
		if derr != nil {
			return fmt.Errorf("decrypt signing secret: %w", derr)
		}
		req.Header.Set("X-Signature", Sign(secret, body))
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("kirim callback: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback ditolak: http %d", resp.StatusCode)
	}
	return nil
}

// Replay menjadwalkan ulang pesan dead untuk dikirim lagi (admin dead-letter).
func (d *Dispatcher) Replay(ctx context.Context, id uuid.UUID) error {
	return d.store.Replay(ctx, id)
}

// backoff menghitung jeda retry: BaseBackoff * 2^(attempts-1), di-cap MaxBackoff,
// ditambah jitter 0..50% (hindari thundering herd).
func (d *Dispatcher) backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	b := d.cfg.BaseBackoff
	for i := 1; i < attempts; i++ {
		b *= 2
		if b >= d.cfg.MaxBackoff || b <= 0 {
			b = d.cfg.MaxBackoff
			break
		}
	}
	jitter := time.Duration(d.rand() * 0.5 * float64(b))
	return b + jitter
}

// buildBody menyusun body callback (detailed-design §4.7): event_id + event_type
// + isi Payload (mis. {"payment": {...}}).
func buildBody(m Message) ([]byte, error) {
	out := map[string]any{
		"event_id":   m.EventID,
		"event_type": m.EventType,
	}
	for k, v := range m.Payload {
		out[k] = v
	}
	return json.Marshal(out)
}

// Sign menghasilkan X-Signature: base64(HMAC-SHA256(secret, body)). Penerima
// memverifikasi dengan signing_secret miliknya.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
