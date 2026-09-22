package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("expected non-nil metrics")
	}

	// Verify all counters are initialized to 0
	snapshot := m.Snapshot()
	if snapshot.HTTPRequestsTotal != 0 {
		t.Errorf("expected HTTPRequestsTotal to be 0, got %d", snapshot.HTTPRequestsTotal)
	}
	if snapshot.ActivePods != 0 {
		t.Errorf("expected ActivePods to be 0, got %d", snapshot.ActivePods)
	}
}

func TestHTTPRequestMetrics(t *testing.T) {
	m := New()

	m.HTTPRequestStarted()
	m.HTTPRequestStarted()
	m.HTTPRequestFinished(100 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.HTTPRequestsTotal != 2 {
		t.Errorf("expected HTTPRequestsTotal to be 2, got %d", snapshot.HTTPRequestsTotal)
	}
	if snapshot.ActiveConnections != 1 {
		t.Errorf("expected ActiveConnections to be 1, got %d", snapshot.ActiveConnections)
	}
	if snapshot.HTTPResponsesTotal != 1 {
		t.Errorf("expected HTTPResponsesTotal to be 1, got %d", snapshot.HTTPResponsesTotal)
	}
}

func TestPodMetrics(t *testing.T) {
	m := New()

	m.PodCreated()
	m.PodCreated()
	m.PodDestroyed()

	snapshot := m.Snapshot()
	if snapshot.PodsCreatedTotal != 2 {
		t.Errorf("expected PodsCreatedTotal to be 2, got %d", snapshot.PodsCreatedTotal)
	}
	if snapshot.PodsDestroyedTotal != 1 {
		t.Errorf("expected PodsDestroyedTotal to be 1, got %d", snapshot.PodsDestroyedTotal)
	}
	// The counters must not touch the gauge — it is owned by the reconcile.
	if snapshot.ActivePods != 0 {
		t.Errorf("expected ActivePods to be untouched at 0, got %d", snapshot.ActivePods)
	}
}

func TestSessionMetrics(t *testing.T) {
	m := New()

	m.SessionCreated()
	m.SessionCreated()
	m.SessionEnded()

	snapshot := m.Snapshot()
	if snapshot.SessionsCreatedTotal != 2 {
		t.Errorf("expected SessionsCreatedTotal to be 2, got %d", snapshot.SessionsCreatedTotal)
	}
	if snapshot.SessionsEndedTotal != 1 {
		t.Errorf("expected SessionsEndedTotal to be 1, got %d", snapshot.SessionsEndedTotal)
	}
	// The counters must not touch the gauge — it is owned by the reconcile.
	if snapshot.ActiveSessions != 0 {
		t.Errorf("expected ActiveSessions to be untouched at 0, got %d", snapshot.ActiveSessions)
	}
}

// The bulk sweeps end an unbounded number of sessions in one statement.
func TestSessionsEndedBulk(t *testing.T) {
	m := New()

	m.SessionsEnded(7)
	m.SessionsEnded(0)
	m.SessionsEnded(-1)

	if got := m.Snapshot().SessionsEndedTotal; got != 7 {
		t.Errorf("SessionsEndedTotal = %d, want 7 (zero and negative counts ignored)", got)
	}
}

func TestCheckpointMetrics(t *testing.T) {
	m := New()

	m.CheckpointPassed()
	m.CheckpointPassed()
	m.CheckpointFailed()

	snapshot := m.Snapshot()
	if snapshot.CheckpointsPassedTotal != 2 {
		t.Errorf("expected CheckpointsPassedTotal to be 2, got %d", snapshot.CheckpointsPassedTotal)
	}
	if snapshot.CheckpointsFailedTotal != 1 {
		t.Errorf("expected CheckpointsFailedTotal to be 1, got %d", snapshot.CheckpointsFailedTotal)
	}
}

func TestRedisMetrics(t *testing.T) {
	m := New()

	m.RedisOperation()
	m.RedisCacheHit()
	m.RedisCacheHit()
	m.RedisCacheMiss()

	snapshot := m.Snapshot()
	if snapshot.RedisOperationsTotal != 1 {
		t.Errorf("expected RedisOperationsTotal to be 1, got %d", snapshot.RedisOperationsTotal)
	}
	if snapshot.RedisCacheHitsTotal != 2 {
		t.Errorf("expected RedisCacheHitsTotal to be 2, got %d", snapshot.RedisCacheHitsTotal)
	}
	if snapshot.RedisCacheMissTotal != 1 {
		t.Errorf("expected RedisCacheMissTotal to be 1, got %d", snapshot.RedisCacheMissTotal)
	}
}

func TestDBQueryMetrics(t *testing.T) {
	m := New()

	m.DBQuery(10 * time.Millisecond)
	m.DBQuery(20 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.DBQueriesTotal != 2 {
		t.Errorf("expected DBQueriesTotal to be 2, got %d", snapshot.DBQueriesTotal)
	}
}

func TestSetGauges(t *testing.T) {
	m := New()

	m.SetActivePods(5)
	m.SetActiveSessions(10)

	snapshot := m.Snapshot()
	if snapshot.ActivePods != 5 {
		t.Errorf("expected ActivePods to be 5, got %d", snapshot.ActivePods)
	}
	if snapshot.ActiveSessions != 10 {
		t.Errorf("expected ActiveSessions to be 10, got %d", snapshot.ActiveSessions)
	}
}

func TestUptimeSeconds(t *testing.T) {
	m := New()

	// Wait a bit to ensure uptime is measurable
	time.Sleep(10 * time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.UptimeSeconds <= 0 {
		t.Errorf("expected UptimeSeconds to be positive, got %f", snapshot.UptimeSeconds)
	}
}

func TestPrometheusOutput(t *testing.T) {
	m := New()
	m.HTTPRequestStarted()
	m.PodCreated()

	// Scrape via the handler — that's the public surface for OpenMetrics
	// output. The handler also exposes the go_* / process_* runtime
	// collectors that the prometheus/client_golang library registers.
	handler := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	output := rr.Body.String()

	expectedMetrics := []string{
		"labctl_http_requests_total",
		"labctl_pods_created_total",
		"labctl_uptime_seconds",
		"go_goroutines",                 // from collectors.NewGoCollector
		"process_resident_memory_bytes", // from collectors.NewProcessCollector
	}
	for _, metric := range expectedMetrics {
		if !strings.Contains(output, metric) {
			t.Errorf("expected output to contain %s", metric)
		}
	}
	if !strings.Contains(output, "# HELP") {
		t.Error("expected output to contain HELP comments")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("expected output to contain TYPE comments")
	}
}

func TestHandlerJSON(t *testing.T) {
	m := New()
	m.HTTPRequestStarted()

	handler := m.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics?format=json", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Check that response contains JSON fields
	body := rr.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Error("expected JSON response to contain http_requests_total")
	}
}

func TestHandlerPrometheus(t *testing.T) {
	m := New()
	m.HTTPRequestStarted()

	handler := m.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected Content-Type text/plain, got %s", contentType)
	}

	// Check that response contains Prometheus format
	body := rr.Body.String()
	if !strings.Contains(body, "labctl_http_requests_total") {
		t.Error("expected Prometheus response to contain labctl_http_requests_total")
	}
}

func TestMiddleware(t *testing.T) {
	m := New()

	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond) // Simulate some work
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	snapshot := m.Snapshot()
	if snapshot.HTTPRequestsTotal != 1 {
		t.Errorf("expected HTTPRequestsTotal to be 1, got %d", snapshot.HTTPRequestsTotal)
	}
	if snapshot.HTTPResponsesTotal != 1 {
		t.Errorf("expected HTTPResponsesTotal to be 1, got %d", snapshot.HTTPResponsesTotal)
	}
	if snapshot.ActiveConnections != 0 {
		t.Errorf("expected ActiveConnections to be 0 after request completes, got %d", snapshot.ActiveConnections)
	}
}

// Counter/Gauge/Histogram primitives moved to prometheus/client_golang; their
// behaviour is covered by that library's own tests. The application-level
// behaviour (which counters fire when) is verified by the higher-level
// TestHTTPRequestMetrics / TestPodMetrics / etc. tests above.
