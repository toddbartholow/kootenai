// Package tracing provides OpenTelemetry distributed tracing for the application
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config holds tracing configuration
type Config struct {
	// ServiceName is the name of the service
	ServiceName string
	// ServiceVersion is the version of the service
	ServiceVersion string
	// Environment is the deployment environment (e.g., "production", "staging")
	Environment string
	// Enabled controls whether tracing is active
	Enabled bool
	// Exporter specifies the exporter type ("otlp", "stdout", "none")
	Exporter string
	// OTLPEndpoint is the OTLP collector endpoint (for "otlp" exporter)
	OTLPEndpoint string
	// SampleRate is the sampling rate (0.0 to 1.0)
	SampleRate float64
	// Logger is used for tracing-related logging
	Logger *slog.Logger
}

// DefaultConfig returns sensible defaults for tracing configuration
func DefaultConfig() Config {
	return Config{
		ServiceName:    "kootenai-api",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		Enabled:        false,
		Exporter:       "none",
		SampleRate:     1.0,
	}
}

// Provider wraps the OpenTelemetry tracer provider
type Provider struct {
	tp     *sdktrace.TracerProvider
	tracer trace.Tracer
	config Config
}

// NewProvider creates a new tracing provider
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	if !config.Enabled {
		return &Provider{
			tracer: noop.NewTracerProvider().Tracer(config.ServiceName),
			config: config,
		}, nil
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch config.Exporter {
	case "otlp":
		opts := []otlptracehttp.Option{}
		if config.OTLPEndpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(config.OTLPEndpoint))
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	default:
		// No exporter - use no-op provider
		return &Provider{
			tracer: noop.NewTracerProvider().Tracer(config.ServiceName),
			config: config,
		}, nil
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if config.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SampleRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := tp.Tracer(config.ServiceName)

	if config.Logger != nil {
		config.Logger.Info("tracing initialized",
			slog.String("service", config.ServiceName),
			slog.String("exporter", config.Exporter),
			slog.Float64("sampleRate", config.SampleRate),
		)
	}

	return &Provider{
		tp:     tp,
		tracer: tracer,
		config: config,
	}, nil
}

// Shutdown shuts down the tracer provider
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.tp == nil {
		return nil
	}
	return p.tp.Shutdown(ctx)
}

// Tracer returns the configured tracer
func (p *Provider) Tracer() trace.Tracer {
	return p.tracer
}

// StartSpan starts a new span
func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}

// HTTPMiddleware returns an HTTP middleware that adds tracing to requests
func (p *Provider) HTTPMiddleware(next http.Handler) http.Handler {
	if !p.config.Enabled {
		return next
	}
	return otelhttp.NewHandler(next, p.config.ServiceName,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
	)
}

// HTTPTransport returns an HTTP transport that adds tracing to outgoing requests
func (p *Provider) HTTPTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if !p.config.Enabled {
		return base
	}
	return otelhttp.NewTransport(base)
}

// SpanFromContext returns the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// SpanContextFromContext returns the span context from context
func SpanContextFromContext(ctx context.Context) trace.SpanContext {
	return trace.SpanContextFromContext(ctx)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetError marks the span as having an error
func SetError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetAttributes sets attributes on the current span
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// TraceID returns the trace ID from context as a string
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanID returns the span ID from context as a string
func SpanID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasSpanID() {
		return sc.SpanID().String()
	}
	return ""
}

// Common attribute helpers

// UserIDAttr creates a user ID attribute
func UserIDAttr(userID string) attribute.KeyValue {
	return attribute.String("user.id", userID)
}

// SessionIDAttr creates a session ID attribute
func SessionIDAttr(sessionID string) attribute.KeyValue {
	return attribute.String("session.id", sessionID)
}

// PodIDAttr creates a pod ID attribute
func PodIDAttr(podID string) attribute.KeyValue {
	return attribute.String("pod.id", podID)
}

// LabTemplateAttr creates a lab template attribute
func LabTemplateAttr(template string) attribute.KeyValue {
	return attribute.String("lab.template", template)
}

// VMNameAttr creates a VM name attribute
func VMNameAttr(vmName string) attribute.KeyValue {
	return attribute.String("vm.name", vmName)
}

// OperationAttr creates an operation name attribute
func OperationAttr(op string) attribute.KeyValue {
	return attribute.String("operation", op)
}

// ExternalServiceAttr creates an external service name attribute
func ExternalServiceAttr(service string) attribute.KeyValue {
	return attribute.String("external.service", service)
}

// DatabaseOperationAttr creates a database operation attribute
func DatabaseOperationAttr(op string) attribute.KeyValue {
	return semconv.DBOperationName(op)
}

// HTTPMethodAttr creates an HTTP method attribute
func HTTPMethodAttr(method string) attribute.KeyValue {
	return semconv.HTTPRequestMethodKey.String(method)
}

// HTTPStatusCodeAttr creates an HTTP status code attribute
func HTTPStatusCodeAttr(code int) attribute.KeyValue {
	return semconv.HTTPResponseStatusCode(code)
}

// ErrorTypeAttr creates an error type attribute
func ErrorTypeAttr(errType string) attribute.KeyValue {
	return attribute.String("error.type", errType)
}
