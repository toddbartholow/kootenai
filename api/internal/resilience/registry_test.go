package resilience

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	cb := NewCircuitBreaker("test-service", DefaultCircuitBreakerConfig(), nil)

	registry.Register(cb)

	got, ok := registry.Get("test-service")
	if !ok {
		t.Fatal("expected circuit breaker to be registered")
	}
	if got != cb {
		t.Error("retrieved circuit breaker doesn't match registered one")
	}

	// Test getting non-existent breaker
	_, ok = registry.Get("non-existent")
	if ok {
		t.Error("expected non-existent breaker to not be found")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()
	cb := NewCircuitBreaker("test-service", DefaultCircuitBreakerConfig(), nil)

	registry.Register(cb)
	registry.Unregister("test-service")

	_, ok := registry.Get("test-service")
	if ok {
		t.Error("expected circuit breaker to be unregistered")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewCircuitBreaker("service-a", DefaultCircuitBreakerConfig(), nil))
	registry.Register(NewCircuitBreaker("service-b", DefaultCircuitBreakerConfig(), nil))
	registry.Register(NewCircuitBreaker("service-c", DefaultCircuitBreakerConfig(), nil))

	names := registry.List()
	if len(names) != 3 {
		t.Errorf("expected 3 breakers, got %d", len(names))
	}

	nameMap := make(map[string]bool)
	for _, n := range names {
		nameMap[n] = true
	}

	for _, expected := range []string{"service-a", "service-b", "service-c"} {
		if !nameMap[expected] {
			t.Errorf("expected %s in list", expected)
		}
	}
}

func TestRegistry_Stats(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewCircuitBreaker("service-a", DefaultCircuitBreakerConfig(), nil))
	registry.Register(NewCircuitBreaker("service-b", DefaultCircuitBreakerConfig(), nil))

	stats := registry.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 stats entries, got %d", len(stats))
	}

	if _, ok := stats["service-a"]; !ok {
		t.Error("expected stats for service-a")
	}
	if _, ok := stats["service-b"]; !ok {
		t.Error("expected stats for service-b")
	}
}

func TestRegistry_Health(t *testing.T) {
	registry := NewRegistry()

	cb1 := NewCircuitBreaker("healthy-service", DefaultCircuitBreakerConfig(), nil)
	cb2 := NewCircuitBreaker("unhealthy-service", CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1,
	}, nil)

	registry.Register(cb1)
	registry.Register(cb2)

	// Initially all should be healthy
	health := registry.Health()
	if !health.Healthy {
		t.Error("expected healthy status initially")
	}
	if health.OpenCount != 0 {
		t.Errorf("expected 0 open circuits, got %d", health.OpenCount)
	}
	if health.Total != 2 {
		t.Errorf("expected 2 total circuits, got %d", health.Total)
	}

	// Trigger failure to open cb2
	cb2.Execute(context.TODO(), func() error {
		return http.ErrServerClosed
	})

	health = registry.Health()
	if health.Healthy {
		t.Error("expected unhealthy status after opening circuit")
	}
	if health.OpenCount != 1 {
		t.Errorf("expected 1 open circuit, got %d", health.OpenCount)
	}
}

func TestRegistry_HTTPHandler(t *testing.T) {
	registry := NewRegistry()
	registry.Register(NewCircuitBreaker("test-service", DefaultCircuitBreakerConfig(), nil))

	handler := registry.HTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/health/circuits", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var health HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !health.Healthy {
		t.Error("expected healthy status")
	}
	if _, ok := health.Circuits["test-service"]; !ok {
		t.Error("expected test-service in circuits")
	}
}

func TestRegistry_HTTPHandler_Unhealthy(t *testing.T) {
	registry := NewRegistry()
	cb := NewCircuitBreaker("failing-service", CircuitBreakerConfig{
		FailureThreshold: 1,
		Timeout:          1,
	}, nil)
	registry.Register(cb)

	// Trigger failure
	cb.Execute(context.TODO(), func() error {
		return http.ErrServerClosed
	})

	handler := registry.HTTPHandler()
	req := httptest.NewRequest(http.MethodGet, "/health/circuits", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestNewCircuitBreakerWithRegistry(t *testing.T) {
	registry := NewRegistry()
	cb := NewCircuitBreakerWithRegistry("auto-registered", DefaultCircuitBreakerConfig(), registry)

	got, ok := registry.Get("auto-registered")
	if !ok {
		t.Fatal("expected circuit breaker to be auto-registered")
	}
	if got != cb {
		t.Error("retrieved circuit breaker doesn't match")
	}
}

func TestNewResilientClientWithRegistry(t *testing.T) {
	registry := NewRegistry()
	rc := NewResilientClientWithRegistry(ResilientClientConfig{
		Name:           "resilient-service",
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		Retry:          DefaultRetryConfig(),
	}, registry)

	_, ok := registry.Get("resilient-service")
	if !ok {
		t.Fatal("expected resilient client's circuit breaker to be registered")
	}

	// Verify the client works
	if rc.CircuitState() != StateClosed {
		t.Errorf("expected closed state, got %v", rc.CircuitState())
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Verify global registry exists and works
	if GlobalRegistry == nil {
		t.Fatal("GlobalRegistry should not be nil")
	}

	cb := NewCircuitBreaker("global-test", DefaultCircuitBreakerConfig(), nil)
	GlobalRegistry.Register(cb)

	_, ok := GlobalRegistry.Get("global-test")
	if !ok {
		t.Error("expected global registry to work")
	}

	// Clean up
	GlobalRegistry.Unregister("global-test")
}
