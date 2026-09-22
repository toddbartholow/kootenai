// Package metrics provides Prometheus instrumentation for the labctl API.
//
// Built on github.com/prometheus/client_golang/prometheus. Each *Metrics
// instance owns its own *prometheus.Registry, so production uses a single
// instance attached to *Server while tests can create isolated registries
// without "already registered" panics.
//
// The middleware (Metrics.Middleware) tracks every HTTP request; the scrape
// endpoint (Metrics.Handler) serves OpenMetrics text by default and a JSON
// snapshot (Metrics.Snapshot) when "?format=json" or "Accept: application/json"
// is supplied. The standard `go_*` and `process_*` collectors are registered
// automatically so runtime/process metrics work without hand-rolled GC stats.
package metrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

// namespace prefixes every application metric (e.g. "labctl_http_requests_total")
// so they don't collide with `go_*` / `process_*` runtime collectors.
const namespace = "labctl"

// Metrics holds the prometheus collectors for a single labctl instance.
// All methods are safe for concurrent use.
type Metrics struct {
	registry *prometheus.Registry

	// HTTP
	httpRequestsTotal   prometheus.Counter
	httpRequestDuration prometheus.Histogram
	httpResponsesTotal  prometheus.Counter
	httpResponsesByCode *prometheus.CounterVec
	activeConnections   prometheus.Gauge

	// Pods / VMs
	podsCreated         prometheus.Counter
	podsDestroyed       prometheus.Counter
	activePods          prometheus.Gauge
	vmOperationDuration *prometheus.HistogramVec

	// Sessions
	sessionsActive  prometheus.Gauge
	sessionsCreated prometheus.Counter
	sessionsEnded   prometheus.Counter

	// Checkpoints
	checkpointsPassed prometheus.Counter
	checkpointsFailed prometheus.Counter

	// Redis
	redisOperations prometheus.Counter
	redisCacheHits  prometheus.Counter
	redisCacheMiss  prometheus.Counter

	// Database
	dbQueriesTotal  prometheus.Counter
	dbQueryDuration prometheus.Histogram

	// i18n telemetry
	i18nCatalogMisses prometheus.Counter

	// Reconcile freshness. Written only by Server.reconcileMetricsOnce, and
	// only when every gauge in the pass was set.
	reconcileSuccessTime prometheus.Gauge

	// Process uptime (in seconds, computed at scrape time).
	startTime time.Time
}

// New constructs a *Metrics with a fresh prometheus.Registry. The Go and
// process collectors are registered automatically so runtime metrics
// (`go_goroutines`, `process_memory_*`, etc.) are exposed by Handler().
func New() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry:  registry,
		startTime: time.Now(),

		httpRequestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests received (counted on entry).",
		}),
		httpRequestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "Latency of HTTP requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		}),
		httpResponsesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_responses_total",
			Help:      "Total HTTP responses written (counted on exit).",
		}),
		httpResponsesByCode: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_responses_by_code_total",
			Help:      "HTTP responses split by status code.",
		}, []string{"code"}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_active_connections",
			Help:      "Current number of in-flight HTTP requests.",
		}),

		podsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pods_created_total",
			Help:      "Total pods created.",
		}),
		podsDestroyed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pods_destroyed_total",
			Help:      "Total pods destroyed.",
		}),
		activePods: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "pods_active",
			Help:      "Pods holding hypervisor resources (every status except destroyed), reconciled from the database every 60s.",
		}),
		vmOperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "vm_operation_duration_seconds",
			Help:      "Latency of VM operations (start/stop/snapshot/reset/...).",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),

		sessionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sessions_active",
			Help:      "Lab sessions that have not ended, reconciled from the database every 60s.",
		}),
		sessionsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sessions_created_total",
			Help:      "Total lab sessions created.",
		}),
		sessionsEnded: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sessions_ended_total",
			Help:      "Total lab sessions ended.",
		}),

		checkpointsPassed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "checkpoints_passed_total",
			Help:      "Total checkpoint evaluations that passed.",
		}),
		checkpointsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "checkpoints_failed_total",
			Help:      "Total checkpoint evaluations that failed.",
		}),

		redisOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "redis_operations_total",
			Help:      "Total Redis operations performed.",
		}),
		redisCacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "redis_cache_hits_total",
			Help:      "Total Redis cache hits.",
		}),
		redisCacheMiss: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "redis_cache_miss_total",
			Help:      "Total Redis cache misses.",
		}),

		dbQueriesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "db_queries_total",
			Help:      "Total database queries executed.",
		}),
		dbQueryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "db_query_duration_seconds",
			Help:      "Latency of database queries.",
			Buckets:   prometheus.DefBuckets,
		}),

		reconcileSuccessTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "metrics_reconcile_success_timestamp_seconds",
			Help:      "Unix time of the last fully successful gauge reconcile against the database.",
		}),

		i18nCatalogMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "i18n_catalog_misses_total",
			Help:      "Catalog lookups that fell back to the message ID (broken/missing key).",
		}),
	}

	registry.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.httpResponsesTotal,
		m.httpResponsesByCode,
		m.activeConnections,
		m.podsCreated,
		m.podsDestroyed,
		m.activePods,
		m.vmOperationDuration,
		m.sessionsActive,
		m.sessionsCreated,
		m.sessionsEnded,
		m.checkpointsPassed,
		m.checkpointsFailed,
		m.redisOperations,
		m.redisCacheHits,
		m.redisCacheMiss,
		m.dbQueriesTotal,
		m.dbQueryDuration,
		m.i18nCatalogMisses,
	)
	// Standard runtime + process collectors give us go_goroutines,
	// go_memstats_*, process_resident_memory_bytes, etc. Reading them at
	// scrape time is cheaper than the hand-rolled MemStats snapshot we had
	// before because client_golang batches the syscall.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	// Reconcile freshness. A permanently failing reconcile (bad credentials,
	// schema drift, statement timeout) would otherwise freeze pods_active and
	// sessions_active at a plausible-looking value with nothing to alert on.
	registry.MustRegister(m.reconcileSuccessTime)

	// Custom uptime gauge so dashboards can plot startup time without doing
	// `time() - process_start_time_seconds` arithmetic.
	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "uptime_seconds",
		Help:      "Seconds since the metrics package was constructed.",
	}, func() float64 { return time.Since(m.startTime).Seconds() }))

	return m
}

// I18nCatalogMiss increments the i18n catalog-miss counter. Wire from
// i18n.SetMissHandler at startup.
func (m *Metrics) I18nCatalogMiss() { m.i18nCatalogMisses.Inc() }

// HTTPRequestStarted should be called when an HTTP request enters the server.
func (m *Metrics) HTTPRequestStarted() {
	m.httpRequestsTotal.Inc()
	m.activeConnections.Inc()
}

// HTTPRequestFinished should be called when an HTTP request leaves the server.
func (m *Metrics) HTTPRequestFinished(duration time.Duration) {
	m.activeConnections.Dec()
	m.httpRequestDuration.Observe(duration.Seconds())
	m.httpResponsesTotal.Inc()
}

// HTTPResponseByCode records a response by its status code label.
func (m *Metrics) HTTPResponseByCode(statusCode int) {
	m.httpResponsesByCode.WithLabelValues(strconv.Itoa(statusCode)).Inc()
}

// VMOperationDuration records the latency of a VM operation (start, stop,
// snapshot, reset, ...). The operation name becomes the `operation` label.
func (m *Metrics) VMOperationDuration(operation string, duration time.Duration) {
	m.vmOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// The pods_active and sessions_active gauges are deliberately NOT maintained
// by the counter methods below. Both were originally incremented and
// decremented in pairs at the call sites, which cannot be kept correct here:
// pods are created down three paths and destroyed down two, a failed
// provision leaves a row that is later destroyed (decrementing something that
// was never incremented), a partially-failed destroy returns before
// decrementing at all, and any in-process gauge resets to zero on restart
// while the rows survive in Postgres. The gauges are therefore owned by the
// reconcile loop, which reads the authoritative counts from the database --
// see Server.runMetricsReconcile and SetActivePods/SetActiveSessions.
//
// The counters are monotonic, so they cannot drift the way a gauge does. They
// still count events rather than entities: a caller that replays a request
// counts it twice, which is why DestroyPod guards on already-destroyed.

// PodCreated records a pod creation.
func (m *Metrics) PodCreated() { m.podsCreated.Inc() }

// PodDestroyed records a pod destruction.
func (m *Metrics) PodDestroyed() { m.podsDestroyed.Inc() }

// SessionCreated records a session creation.
func (m *Metrics) SessionCreated() { m.sessionsCreated.Inc() }

// SessionEnded records a session end.
func (m *Metrics) SessionEnded() { m.sessionsEnded.Inc() }

// SessionsEnded records n session ends at once, for the bulk stale-session
// sweeps which end an unbounded number of sessions in a single statement.
func (m *Metrics) SessionsEnded(n int64) {
	if n > 0 {
		m.sessionsEnded.Add(float64(n))
	}
}

// CheckpointPassed records a passed checkpoint.
func (m *Metrics) CheckpointPassed() { m.checkpointsPassed.Inc() }

// CheckpointFailed records a failed checkpoint.
func (m *Metrics) CheckpointFailed() { m.checkpointsFailed.Inc() }

// RedisOperation records a redis op.
func (m *Metrics) RedisOperation() { m.redisOperations.Inc() }

// RedisCacheHit records a cache hit.
func (m *Metrics) RedisCacheHit() { m.redisCacheHits.Inc() }

// RedisCacheMiss records a cache miss.
func (m *Metrics) RedisCacheMiss() { m.redisCacheMiss.Inc() }

// DBQuery records a database query (count + duration).
func (m *Metrics) DBQuery(duration time.Duration) {
	m.dbQueriesTotal.Inc()
	m.dbQueryDuration.Observe(duration.Seconds())
}

// ReconcileSucceeded stamps the time of a fully successful reconcile pass.
// Alert on `time() - labctl_metrics_reconcile_success_timestamp_seconds > 300`
// to catch a reconcile that is failing silently.
func (m *Metrics) ReconcileSucceeded(now time.Time) {
	m.reconcileSuccessTime.Set(float64(now.Unix()))
}

// SetActivePods sets the active-pods gauge from a reconcile against the
// database. This is the only writer of that gauge.
func (m *Metrics) SetActivePods(count int64) { m.activePods.Set(float64(count)) }

// SetActiveSessions sets the active-sessions gauge from a reconcile against
// the database. This is the only writer of that gauge.
func (m *Metrics) SetActiveSessions(count int64) { m.sessionsActive.Set(float64(count)) }

// MetricsSnapshot is the JSON shape returned when callers hit /metrics with
// `?format=json` or `Accept: application/json`. Useful for ad-hoc dashboards
// that don't want to parse OpenMetrics text.
type MetricsSnapshot struct {
	HTTPRequestsTotal      int64   `json:"http_requests_total"`
	HTTPResponsesTotal     int64   `json:"http_responses_total"`
	ActiveConnections      int64   `json:"http_active_connections"`
	PodsCreatedTotal       int64   `json:"pods_created_total"`
	PodsDestroyedTotal     int64   `json:"pods_destroyed_total"`
	ActivePods             int64   `json:"pods_active"`
	SessionsCreatedTotal   int64   `json:"sessions_created_total"`
	SessionsEndedTotal     int64   `json:"sessions_ended_total"`
	ActiveSessions         int64   `json:"sessions_active"`
	CheckpointsPassedTotal int64   `json:"checkpoints_passed_total"`
	CheckpointsFailedTotal int64   `json:"checkpoints_failed_total"`
	RedisOperationsTotal   int64   `json:"redis_operations_total"`
	RedisCacheHitsTotal    int64   `json:"redis_cache_hits_total"`
	RedisCacheMissTotal    int64   `json:"redis_cache_miss_total"`
	DBQueriesTotal         int64   `json:"db_queries_total"`
	UptimeSeconds          float64 `json:"uptime_seconds"`
	// ReconcileSuccessTimestamp is the unix time of the last fully successful
	// gauge reconcile, or 0 if none has completed.
	ReconcileSuccessTimestamp int64 `json:"metrics_reconcile_success_timestamp_seconds"`
}

// Snapshot reads the current values out of the registry. It's intended for
// tests and the JSON variant of the scrape endpoint — operators should prefer
// the Prometheus text format which exposes histograms, runtime collectors,
// and all the standard `go_*`/`process_*` series.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		HTTPRequestsTotal:         counterValue(m.httpRequestsTotal),
		HTTPResponsesTotal:        counterValue(m.httpResponsesTotal),
		ActiveConnections:         gaugeValue(m.activeConnections),
		PodsCreatedTotal:          counterValue(m.podsCreated),
		PodsDestroyedTotal:        counterValue(m.podsDestroyed),
		ActivePods:                gaugeValue(m.activePods),
		SessionsCreatedTotal:      counterValue(m.sessionsCreated),
		SessionsEndedTotal:        counterValue(m.sessionsEnded),
		ActiveSessions:            gaugeValue(m.sessionsActive),
		CheckpointsPassedTotal:    counterValue(m.checkpointsPassed),
		CheckpointsFailedTotal:    counterValue(m.checkpointsFailed),
		RedisOperationsTotal:      counterValue(m.redisOperations),
		RedisCacheHitsTotal:       counterValue(m.redisCacheHits),
		RedisCacheMissTotal:       counterValue(m.redisCacheMiss),
		DBQueriesTotal:            counterValue(m.dbQueriesTotal),
		UptimeSeconds:             time.Since(m.startTime).Seconds(),
		ReconcileSuccessTimestamp: gaugeValue(m.reconcileSuccessTime),
	}
}

// counterValue pulls the current cumulative count out of a Counter via the
// prometheus.Collector contract. Returns 0 on any unexpected shape.
func counterValue(c prometheus.Counter) int64 {
	var m dto.Metric
	if err := c.Write(&m); err != nil || m.Counter == nil {
		return 0
	}
	return int64(m.Counter.GetValue())
}

// gaugeValue pulls the current value out of a Gauge.
func gaugeValue(g prometheus.Gauge) int64 {
	var m dto.Metric
	if err := g.Write(&m); err != nil || m.Gauge == nil {
		return 0
	}
	return int64(m.Gauge.GetValue())
}

// Handler returns the /metrics endpoint. The Prometheus text format is the
// default; `?format=json` or `Accept: application/json` returns a JSON
// Snapshot so curl-without-jq users can still introspect.
func (m *Metrics) Handler() http.HandlerFunc {
	promHandler := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          m.registry,
	})
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(m.Snapshot()); err != nil {
				http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
			}
			return
		}
		promHandler.ServeHTTP(w, r)
	}
}

// responseWriter wraps http.ResponseWriter to capture the final status code
// so the middleware can record http_responses_by_code_total.
type responseWriter struct {
	http.ResponseWriter
	statusCode int32 // atomic; writes from main goroutine, reads from finishing path
	written    bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		atomic.StoreInt32(&rw.statusCode, int32(code))
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker for WebSocket upgrades.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

// Middleware records request lifecycle metrics for every HTTP request: the
// total counter increments on entry, response counter + duration histogram
// fire on exit, and the active-connections gauge tracks in-flight.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.HTTPRequestStarted()

		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)

		m.HTTPRequestFinished(time.Since(start))
		m.HTTPResponseByCode(int(atomic.LoadInt32(&rw.statusCode)))
	})
}
