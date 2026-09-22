package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ServiceName != "kootenai-api" {
		t.Errorf("expected service name 'kootenai-api', got %s", config.ServiceName)
	}
	if config.Enabled {
		t.Error("expected tracing to be disabled by default")
	}
	if config.SampleRate != 1.0 {
		t.Errorf("expected sample rate 1.0, got %f", config.SampleRate)
	}
}

func TestNewProvider_Disabled(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = false

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	if provider.Tracer() == nil {
		t.Error("expected tracer to be non-nil even when disabled")
	}
}

func TestNewProvider_NoopExporter(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = true
	config.Exporter = "none"

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	if provider.Tracer() == nil {
		t.Error("expected tracer to be non-nil")
	}
}

func TestNewProvider_StdoutExporter(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = true
	config.Exporter = "stdout"

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	if provider.Tracer() == nil {
		t.Error("expected tracer to be non-nil")
	}
	if provider.tp == nil {
		t.Error("expected tracer provider to be non-nil")
	}
}

func TestProvider_StartSpan(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = true
	config.Exporter = "stdout"

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	spanCtx, span := provider.StartSpan(ctx, "test-operation")
	defer span.End()

	if span == nil {
		t.Error("expected span to be non-nil")
	}
	if spanCtx == nil {
		t.Error("expected context to be non-nil")
	}
}

func TestProvider_HTTPMiddleware_Disabled(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = false

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := provider.HTTPMiddleware(handler)

	// When disabled, middleware should return the same handler
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestProvider_HTTPTransport_Disabled(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = false

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer provider.Shutdown(ctx)

	baseTransport := &http.Transport{}
	transport := provider.HTTPTransport(baseTransport)

	// When disabled, should return the base transport unchanged
	if transport != baseTransport {
		t.Error("expected base transport to be returned unchanged when disabled")
	}

	// Test with nil base transport
	nilTransport := provider.HTTPTransport(nil)
	if nilTransport != http.DefaultTransport {
		t.Error("expected default transport when nil base and disabled")
	}
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()
	span := SpanFromContext(ctx)

	// Should return a no-op span when no span in context
	if span == nil {
		t.Error("expected span to be non-nil")
	}
}

func TestSpanContextFromContext(t *testing.T) {
	ctx := context.Background()
	sc := SpanContextFromContext(ctx)

	// Should return invalid span context when no span in context
	if sc.IsValid() {
		t.Error("expected invalid span context when no span in context")
	}
}

func TestTraceID(t *testing.T) {
	ctx := context.Background()
	traceID := TraceID(ctx)

	// Should return empty string when no span in context
	if traceID != "" {
		t.Errorf("expected empty trace ID, got %s", traceID)
	}
}

func TestSpanID(t *testing.T) {
	ctx := context.Background()
	spanID := SpanID(ctx)

	// Should return empty string when no span in context
	if spanID != "" {
		t.Errorf("expected empty span ID, got %s", spanID)
	}
}

func TestAddEvent(t *testing.T) {
	// This test just ensures the function doesn't panic
	ctx := context.Background()
	AddEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestSetError(t *testing.T) {
	// This test just ensures the function doesn't panic
	ctx := context.Background()
	SetError(ctx, errors.New("test error"))
}

func TestSetAttributes(t *testing.T) {
	// This test just ensures the function doesn't panic
	ctx := context.Background()
	SetAttributes(ctx, attribute.String("key", "value"))
}

func TestAttributeHelpers(t *testing.T) {
	tests := []struct {
		name     string
		attr     attribute.KeyValue
		key      string
		expected string
	}{
		{
			name:     "UserIDAttr",
			attr:     UserIDAttr("user-123"),
			key:      "user.id",
			expected: "user-123",
		},
		{
			name:     "SessionIDAttr",
			attr:     SessionIDAttr("session-456"),
			key:      "session.id",
			expected: "session-456",
		},
		{
			name:     "PodIDAttr",
			attr:     PodIDAttr("pod-789"),
			key:      "pod.id",
			expected: "pod-789",
		},
		{
			name:     "LabTemplateAttr",
			attr:     LabTemplateAttr("linux-basics"),
			key:      "lab.template",
			expected: "linux-basics",
		},
		{
			name:     "VMNameAttr",
			attr:     VMNameAttr("ubuntu-server"),
			key:      "vm.name",
			expected: "ubuntu-server",
		},
		{
			name:     "OperationAttr",
			attr:     OperationAttr("create"),
			key:      "operation",
			expected: "create",
		},
		{
			name:     "ExternalServiceAttr",
			attr:     ExternalServiceAttr("proxmox"),
			key:      "external.service",
			expected: "proxmox",
		},
		{
			name:     "ErrorTypeAttr",
			attr:     ErrorTypeAttr("validation"),
			key:      "error.type",
			expected: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.attr.Key) != tt.key {
				t.Errorf("expected key %s, got %s", tt.key, tt.attr.Key)
			}
			if tt.attr.Value.AsString() != tt.expected {
				t.Errorf("expected value %s, got %s", tt.expected, tt.attr.Value.AsString())
			}
		})
	}
}

func TestHTTPMethodAttr(t *testing.T) {
	attr := HTTPMethodAttr("GET")
	if attr.Value.AsString() != "GET" {
		t.Errorf("expected GET, got %s", attr.Value.AsString())
	}
}

func TestHTTPStatusCodeAttr(t *testing.T) {
	attr := HTTPStatusCodeAttr(200)
	if attr.Value.AsInt64() != 200 {
		t.Errorf("expected 200, got %d", attr.Value.AsInt64())
	}
}

func TestProvider_Shutdown(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = true
	config.Exporter = "stdout"

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestProvider_Shutdown_Disabled(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()
	config.Enabled = false

	provider, err := NewProvider(ctx, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("unexpected shutdown error: %v", err)
	}
}

func TestSamplerConfigurations(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"always sample", 1.0},
		{"never sample", 0.0},
		{"50% sample", 0.5},
		{"above 1.0", 1.5},
		{"negative", -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			config := DefaultConfig()
			config.Enabled = true
			config.Exporter = "stdout"
			config.SampleRate = tt.sampleRate

			provider, err := NewProvider(ctx, config)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer provider.Shutdown(ctx)

			// Just verify it doesn't panic
			if provider.Tracer() == nil {
				t.Error("expected tracer to be non-nil")
			}
		})
	}
}
