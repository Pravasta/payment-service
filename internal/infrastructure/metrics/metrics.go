// Package metrics menyediakan koleksi metrik Prometheus untuk observability
// (detailed-design §12). Instance dibuat sekali di entrypoint (cmd) lalu
// di-inject ke layer yang relevan lewat interface kecil yang dideklarasikan di
// sisi konsumen — sehingga adapter tidak perlu meng-import package infrastructure
// ini langsung (menjaga arah dependensi Clean Architecture).
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics memegang seluruh collector pada registry-nya sendiri (bukan default
// global) agar mudah diuji dan tidak bocor antar instance.
type Metrics struct {
	reg *prometheus.Registry

	httpRequests *prometheus.CounterVec   // per method, route, status
	httpDuration *prometheus.HistogramVec // latency request HTTP

	chargeTotal    *prometheus.CounterVec   // create-charge per gateway, result
	chargeDuration *prometheus.HistogramVec // latency create-charge per gateway

	webhookTotal *prometheus.CounterVec // webhook diterima per gateway, result

	outboxTotal   *prometheus.CounterVec // hasil dispatch: delivered/retry/dead
	outboxBacklog *prometheus.GaugeVec   // jumlah pesan tertahan per status

	reconcileRuns    prometheus.Counter // siklus reconciler yang berjalan
	reconcileChanged prometheus.Counter // transaksi yang berubah status (mismatch ditemukan)
	reconcileErrors  prometheus.Counter // siklus reconciler yang gagal
}

// New membuat seluruh collector dan meregistrasinya. Menyertakan collector
// runtime Go & proses untuk metrik dasar (goroutine, GC, memori, CPU).
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Jumlah request HTTP yang diproses.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Durasi pemrosesan request HTTP (detik).",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		chargeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gateway_charge_total",
			Help: "Jumlah percobaan create-charge ke gateway, per hasil (success/error).",
		}, []string{"gateway", "result"}),
		chargeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gateway_charge_duration_seconds",
			Help:    "Durasi create-charge ke gateway (detik).",
			Buckets: prometheus.DefBuckets,
		}, []string{"gateway"}),
		webhookTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "webhook_total",
			Help: "Jumlah webhook gateway yang diparse, per hasil (ok/failed).",
		}, []string{"gateway", "result"}),
		outboxTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "outbox_dispatch_total",
			Help: "Jumlah pemrosesan pesan outbox, per hasil (delivered/retry/dead).",
		}, []string{"result"}),
		outboxBacklog: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "outbox_backlog",
			Help: "Jumlah pesan outbox yang tertahan, per status (pending/dead).",
		}, []string{"status"}),
		reconcileRuns: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "reconcile_runs_total",
			Help: "Jumlah siklus reconciler yang dijalankan.",
		}),
		reconcileChanged: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "reconcile_changed_total",
			Help: "Jumlah transaksi yang status-nya diperbaiki reconciler (mismatch).",
		}),
		reconcileErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "reconcile_errors_total",
			Help: "Jumlah siklus reconciler yang gagal.",
		}),
	}

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.httpRequests, m.httpDuration,
		m.chargeTotal, m.chargeDuration,
		m.webhookTotal,
		m.outboxTotal, m.outboxBacklog,
		m.reconcileRuns, m.reconcileChanged, m.reconcileErrors,
	)
	return m
}

// Handler mengembalikan handler HTTP untuk endpoint /metrics (format Prometheus).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// ObserveHTTP mencatat satu request HTTP yang selesai.
func (m *Metrics) ObserveHTTP(method, route string, status int, dur time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(dur.Seconds())
}

// ObserveCharge mencatat satu percobaan create-charge ke gateway.
func (m *Metrics) ObserveCharge(gateway string, success bool, dur time.Duration) {
	m.chargeTotal.WithLabelValues(gateway, resultLabel(success)).Inc()
	m.chargeDuration.WithLabelValues(gateway).Observe(dur.Seconds())
}

// ObserveWebhook mencatat satu webhook gateway yang diparse (ok = verifikasi &
// parsing berhasil).
func (m *Metrics) ObserveWebhook(gateway string, ok bool) {
	result := "failed"
	if ok {
		result = "ok"
	}
	m.webhookTotal.WithLabelValues(gateway, result).Inc()
}

// ObserveOutbox mencatat hasil pemrosesan satu pesan outbox
// (result: delivered/retry/dead).
func (m *Metrics) ObserveOutbox(result string) {
	m.outboxTotal.WithLabelValues(result).Inc()
}

// SetOutboxBacklog menyetel gauge jumlah pesan outbox tertahan (dipanggil
// periodik oleh worker dari hasil query COUNT).
func (m *Metrics) SetOutboxBacklog(pending, dead int64) {
	m.outboxBacklog.WithLabelValues("pending").Set(float64(pending))
	m.outboxBacklog.WithLabelValues("dead").Set(float64(dead))
}

// ObserveReconcile mencatat satu siklus reconciler: jumlah transaksi yang
// berubah status (mismatch) dan apakah siklus gagal.
func (m *Metrics) ObserveReconcile(changed int, failed bool) {
	m.reconcileRuns.Inc()
	if failed {
		m.reconcileErrors.Inc()
		return
	}
	if changed > 0 {
		m.reconcileChanged.Add(float64(changed))
	}
}

func resultLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}
