package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("Level = %s, want info", cfg.Level)
	}
	if cfg.Output != "stdout" {
		t.Errorf("Output = %s, want stdout", cfg.Output)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %s, want json", cfg.Format)
	}
	if cfg.MaxSize != 100 {
		t.Errorf("MaxSize = %d, want 100", cfg.MaxSize)
	}
}

func TestSetup_Stdout(t *testing.T) {
	cfg := Config{
		Level:  "info",
		Output: "stdout",
		Format: "json",
	}

	logger, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer logger.Close()

	if logger.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestSetup_TextFormat(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Output: "stdout",
		Format: "text",
	}

	logger, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer logger.Close()

	if logger.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestSetup_File(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cfg := Config{
		Level:    "info",
		Output:   "file",
		FilePath: logPath,
		Format:   "json",
	}

	logger, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer logger.Close()

	// Log something
	logger.Info("test message", "key", "value")

	// Close to flush
	logger.Close()

	// Check file exists and has content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Error("Log file should not be empty")
	}
	if !strings.Contains(string(content), "test message") {
		t.Error("Log file should contain test message")
	}
}

func TestSetup_Both(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "both.log")

	cfg := Config{
		Level:    "info",
		Output:   "both",
		FilePath: logPath,
		Format:   "json",
	}

	logger, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer logger.Close()

	if logger.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestSetup_InvalidFilePath(t *testing.T) {
	cfg := Config{
		Level:    "info",
		Output:   "file",
		FilePath: "/nonexistent/path/that/cannot/be/created/\x00invalid",
		Format:   "json",
	}

	_, err := Setup(cfg)
	if err == nil {
		t.Error("Setup() should fail with invalid file path")
	}
}

func TestSetup_BothInvalidFilePath(t *testing.T) {
	cfg := Config{
		Level:    "info",
		Output:   "both",
		FilePath: "/nonexistent/path/that/cannot/be/created/\x00invalid",
		Format:   "json",
	}

	_, err := Setup(cfg)
	if err == nil {
		t.Error("Setup() should fail with invalid file path")
	}
}

func TestLogger_Close(t *testing.T) {
	// Test Close with no file
	logger := &Logger{
		Logger: slog.Default(),
		file:   nil,
	}

	err := logger.Close()
	if err != nil {
		t.Errorf("Close() with nil file should not error: %v", err)
	}

	// Test Close with file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "close.log")

	cfg := Config{
		Level:    "info",
		Output:   "file",
		FilePath: logPath,
		Format:   "json",
	}

	loggerWithFile, err := Setup(cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	err = loggerWithFile.Close()
	if err != nil {
		t.Errorf("Close() should not error: %v", err)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTextHandler_Enabled(t *testing.T) {
	handler := NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Debug should be disabled when level is Warn")
	}
	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Info should be disabled when level is Warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("Warn should be enabled when level is Warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("Error should be enabled when level is Warn")
	}
}

func TestTextHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})

	record := slog.NewRecord(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), slog.LevelInfo, "Test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	err := handler.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "2024-01-15 10:30:00") {
		t.Error("Output should contain timestamp")
	}
	if !strings.Contains(output, "[INFO ]") {
		t.Error("Output should contain level")
	}
	if !strings.Contains(output, "Test message") {
		t.Error("Output should contain message")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Output should contain attributes")
	}
}

func TestTextHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, nil)

	// Add pre-set attrs
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("preset", "value"),
	})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "Test", 0)
	err := handlerWithAttrs.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "preset=value") {
		t.Error("Output should contain preset attribute")
	}
}

func TestTextHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, nil)

	// Add group
	handlerWithGroup := handler.WithGroup("mygroup")

	// Cast to access group field for verification
	h := handlerWithGroup.(*TextHandler)
	if h.group != "mygroup" {
		t.Errorf("group = %s, want mygroup", h.group)
	}

	// Test nested groups
	handlerNested := h.WithGroup("nested")
	hn := handlerNested.(*TextHandler)
	if hn.group != "mygroup.nested" {
		t.Errorf("nested group = %s, want mygroup.nested", hn.group)
	}
}

func TestTextHandler_NilOpts(t *testing.T) {
	handler := NewTextHandler(os.Stdout, nil)
	if handler.level != slog.LevelInfo {
		t.Errorf("Default level should be Info, got %v", handler.level)
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO "},
		{slog.LevelWarn, "WARN "},
		{slog.LevelError, "ERROR"},
		{slog.Level(100), "INFO "}, // unknown level
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := levelString(tt.level)
			if result != tt.expected {
				t.Errorf("levelString(%v) = %s, want %s", tt.level, result, tt.expected)
			}
		})
	}
}

func TestFormatAttr(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		attr     slog.Attr
		expected string
	}{
		{
			name:     "simple string",
			group:    "",
			attr:     slog.String("key", "value"),
			expected: "key=value",
		},
		{
			name:     "string with spaces",
			group:    "",
			attr:     slog.String("key", "value with spaces"),
			expected: `key="value with spaces"`,
		},
		{
			name:     "with group",
			group:    "grp",
			attr:     slog.String("key", "value"),
			expected: "grp.key=value",
		},
		{
			name:     "integer",
			group:    "",
			attr:     slog.Int("count", 42),
			expected: "count=42",
		},
		{
			name:     "duration",
			group:    "",
			attr:     slog.Duration("elapsed", 5*time.Second),
			expected: "elapsed=5s",
		},
		{
			name:     "time",
			group:    "",
			attr:     slog.Time("timestamp", time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			expected: "timestamp=2024-01-15T10:00:00Z",
		},
		{
			name:     "error",
			group:    "",
			attr:     slog.Any("err", errors.New("test error")),
			expected: `err="test error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAttr(tt.group, tt.attr)
			if result != tt.expected {
				t.Errorf("formatAttr() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"hello", false},
		{"hello world", true},
		{"hello\tworld", true},
		{"hello\nworld", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSpace(tt.input)
			if result != tt.expected {
				t.Errorf("containsSpace(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestOpenLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "subdir", "test.log")

	f, err := openLogFile(logPath)
	if err != nil {
		t.Fatalf("openLogFile() error = %v", err)
	}
	defer f.Close()

	// Check that directory and file were created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}
}

func TestTextHandler_HandleWithPresetAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, nil)

	// Add multiple pre-set attrs
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("service", "test"),
		slog.String("version", "1.0"),
	})

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "Test", 0)
	record.AddAttrs(slog.String("request_id", "123"))

	err := handlerWithAttrs.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "service=test") {
		t.Error("Output should contain preset service attr")
	}
	if !strings.Contains(output, "version=1.0") {
		t.Error("Output should contain preset version attr")
	}
	if !strings.Contains(output, "request_id=123") {
		t.Error("Output should contain request_id attr")
	}
}

func TestTextHandler_HandleWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewTextHandler(&buf, nil)

	// Add group and attrs
	handlerWithGroup := handler.WithGroup("http").(*TextHandler)
	handlerWithGroup.attrs = []slog.Attr{
		slog.String("method", "GET"),
	}

	record := slog.NewRecord(time.Now(), slog.LevelInfo, "Request", 0)
	err := handlerWithGroup.Handle(context.Background(), record)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "http.method=GET") {
		t.Errorf("Output should contain grouped attr, got: %s", output)
	}
}
