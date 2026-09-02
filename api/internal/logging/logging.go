// Package logging provides configurable logging with support for file and console output.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Config holds logging configuration options.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error)
	Level string `yaml:"level"`

	// Output specifies where logs go: "stdout", "file", or "both"
	Output string `yaml:"output"`

	// FilePath is the path to the log file (used when Output is "file" or "both")
	// Default: /var/log/labctl/labctl.log
	FilePath string `yaml:"file_path"`

	// Format specifies the log format: "json" or "text"
	// JSON is better for log aggregation, text is better for tail -f
	Format string `yaml:"format"`

	// MaxSize is the maximum size in MB before rotation (0 = no rotation)
	MaxSize int `yaml:"max_size"`
}

// DefaultConfig returns a sensible default logging configuration.
func DefaultConfig() Config {
	return Config{
		Level:    "info",
		Output:   "stdout",
		FilePath: "/var/log/labctl/labctl.log",
		Format:   "json",
		MaxSize:  100,
	}
}

// Logger wraps slog.Logger and manages file handles.
type Logger struct {
	*slog.Logger
	file *os.File
}

// Close closes any open file handles.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Setup creates a new logger based on the configuration.
func Setup(cfg Config) (*Logger, error) {
	level := parseLevel(cfg.Level)

	var writers []io.Writer
	var logFile *os.File

	// Determine output destinations
	switch cfg.Output {
	case "file":
		f, err := openLogFile(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("opening log file: %w", err)
		}
		logFile = f
		writers = append(writers, f)

	case "both":
		f, err := openLogFile(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("opening log file: %w", err)
		}
		logFile = f
		writers = append(writers, os.Stdout, f)

	default: // "stdout" or anything else
		writers = append(writers, os.Stdout)
	}

	// Create multi-writer if needed
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// Create handler based on format
	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = NewTextHandler(writer, opts)
	default: // "json"
		handler = slog.NewJSONHandler(writer, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
		file:   logFile,
	}, nil
}

// openLogFile opens or creates the log file, creating parent directories if needed.
func openLogFile(path string) (*os.File, error) {
	// Ensure parent directory exists
	// Log directory permissions 0755 allow monitoring tools to access logs.
	// Path is from server configuration, not user input.
	dir := filepath.Dir(path)
	/* #nosec G301 */
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", dir, err)
	}

	// Open file for appending, create if doesn't exist
	// #nosec G302,G304 -- Log file permissions 0644 allow monitoring tools to read logs.
	// Path is from server configuration, not user input.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", path, err)
	}

	return f, nil
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// TextHandler is a human-readable slog handler for easy tail -f reading.
type TextHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
}

// NewTextHandler creates a handler that outputs human-readable text logs.
func NewTextHandler(w io.Writer, opts *slog.HandlerOptions) *TextHandler {
	level := slog.LevelInfo
	if opts != nil && opts.Level != nil {
		level = opts.Level.Level()
	}
	return &TextHandler{
		w:     w,
		level: level,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *TextHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes the log record.
func (h *TextHandler) Handle(_ context.Context, r slog.Record) error {
	// Format: 2024-01-15 10:30:00 [INFO] Starting server host=0.0.0.0 port=8080
	timestamp := r.Time.Format("2006-01-02 15:04:05")
	levelStr := levelString(r.Level)

	// Build the log line
	line := fmt.Sprintf("%s [%s] %s", timestamp, levelStr, r.Message)

	// Add pre-set attrs
	for _, attr := range h.attrs {
		line += " " + formatAttr(h.group, attr)
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		line += " " + formatAttr(h.group, a)
		return true
	})

	line += "\n"

	_, err := h.w.Write([]byte(line))
	return err
}

// WithAttrs returns a new handler with the given attributes.
func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &TextHandler{
		w:     h.w,
		level: h.level,
		attrs: newAttrs,
		group: h.group,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *TextHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}
	return &TextHandler{
		w:     h.w,
		level: h.level,
		attrs: h.attrs,
		group: newGroup,
	}
}

// levelString returns a fixed-width level string for alignment.
func levelString(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO "
	case slog.LevelWarn:
		return "WARN "
	case slog.LevelError:
		return "ERROR"
	default:
		return "INFO "
	}
}

// formatAttr formats a single attribute as key=value.
func formatAttr(group string, a slog.Attr) string {
	key := a.Key
	if group != "" {
		key = group + "." + key
	}

	// Handle different value types
	switch v := a.Value.Any().(type) {
	case string:
		// Quote strings with spaces
		if containsSpace(v) {
			return fmt.Sprintf("%s=%q", key, v)
		}
		return fmt.Sprintf("%s=%s", key, v)
	case time.Time:
		return fmt.Sprintf("%s=%s", key, v.Format(time.RFC3339))
	case time.Duration:
		return fmt.Sprintf("%s=%s", key, v.String())
	case error:
		return fmt.Sprintf("%s=%q", key, v.Error())
	default:
		return fmt.Sprintf("%s=%v", key, v)
	}
}

// containsSpace checks if a string contains whitespace.
func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return true
		}
	}
	return false
}
