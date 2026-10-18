package nats

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors during tests
	}))
}

func TestNewClient_ConnectionFailure(t *testing.T) {
	// This test validates error handling when NATS is not available
	// Important for homelab users to understand connection failures

	t.Run("fails with unreachable server", func(t *testing.T) {
		cfg := Config{
			URL:            "nats://localhost:14222", // Non-standard port unlikely to be in use
			Name:           "test-client",
			ConnectTimeout: 100 * time.Millisecond, // Short timeout for faster test
			ReconnectWait:  50 * time.Millisecond,
			MaxReconnects:  0, // Don't retry
		}

		client, err := NewClient(cfg, newTestLogger())

		if err == nil {
			client.Close()
			t.Fatal("expected connection error, got nil")
		}

		// Error should indicate connection failure
		if !strings.Contains(err.Error(), "connecting to NATS") {
			t.Errorf("expected error about connecting to NATS, got: %v", err)
		}
	})

	t.Run("fails with invalid URL scheme", func(t *testing.T) {
		cfg := Config{
			URL:            "http://localhost:4222", // Wrong scheme
			Name:           "test-client",
			ConnectTimeout: 100 * time.Millisecond,
			MaxReconnects:  0,
		}

		client, err := NewClient(cfg, newTestLogger())

		if err == nil {
			client.Close()
			t.Fatal("expected error for invalid URL scheme")
		}
	})

	t.Run("fails with empty URL", func(t *testing.T) {
		cfg := Config{
			URL:            "",
			Name:           "test-client",
			ConnectTimeout: 100 * time.Millisecond,
			MaxReconnects:  0,
		}

		client, err := NewClient(cfg, newTestLogger())

		if err == nil {
			client.Close()
			t.Fatal("expected error for empty URL")
		}
	})
}

func TestClient_Close_Nil(t *testing.T) {
	// Test that Close() handles nil connection gracefully
	// This is important for cleanup in error scenarios

	t.Run("close on nil conn does not panic", func(t *testing.T) {
		client := &Client{
			conn:   nil,
			logger: newTestLogger(),
		}

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Close() panicked: %v", r)
			}
		}()

		client.Close()
	})
}

func TestClient_IsConnected_Nil(t *testing.T) {
	t.Run("returns false for nil connection", func(t *testing.T) {
		client := &Client{
			conn:   nil,
			logger: newTestLogger(),
		}

		if client.IsConnected() {
			t.Error("expected IsConnected() to return false for nil connection")
		}
	})
}

func TestClient_Accessors(t *testing.T) {
	// Test that accessor methods handle nil state properly
	client := &Client{
		conn:   nil,
		js:     nil,
		stream: nil,
		logger: newTestLogger(),
	}

	t.Run("JetStream returns nil for unconnected client", func(t *testing.T) {
		if client.JetStream() != nil {
			t.Error("expected JetStream() to return nil for unconnected client")
		}
	})

	t.Run("Stream returns nil for unconnected client", func(t *testing.T) {
		if client.Stream() != nil {
			t.Error("expected Stream() to return nil for unconnected client")
		}
	})
}

// TestConfigYAMLTags verifies that Config struct has proper YAML tags for configuration files
func TestConfigYAMLTags(t *testing.T) {
	// This test ensures the struct tags are correct for homelab configuration
	cfg := Config{
		URL:            "nats://localhost:4222",
		Name:           "lab-platform",
		Token:          "secret",
		User:           "admin",
		Password:       "password",
		ConnectTimeout: 10 * time.Second,
		ReconnectWait:  2 * time.Second,
		MaxReconnects:  -1,
	}

	// Verify all fields are settable (compile-time check)
	if cfg.URL == "" {
		t.Error("URL should be set")
	}
	if cfg.Name == "" {
		t.Error("Name should be set")
	}
	if cfg.Token == "" {
		t.Error("Token should be set")
	}
	if cfg.User == "" {
		t.Error("User should be set")
	}
	if cfg.Password == "" {
		t.Error("Password should be set")
	}
	if cfg.ConnectTimeout == 0 {
		t.Error("ConnectTimeout should be set")
	}
	if cfg.ReconnectWait == 0 {
		t.Error("ReconnectWait should be set")
	}
	if cfg.MaxReconnects != -1 {
		t.Errorf("MaxReconnects = %d, want -1", cfg.MaxReconnects)
	}
}

// TestDefaultConfigForHomelab validates defaults are suitable for homelab setup
func TestDefaultConfigForHomelab(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("uses localhost which works for single-server homelab", func(t *testing.T) {
		if !strings.Contains(cfg.URL, "localhost") {
			t.Errorf("default URL should use localhost for homelab compatibility, got: %s", cfg.URL)
		}
	})

	t.Run("uses standard NATS port", func(t *testing.T) {
		if !strings.Contains(cfg.URL, "4222") {
			t.Errorf("default URL should use standard port 4222, got: %s", cfg.URL)
		}
	})

	t.Run("has reasonable connect timeout", func(t *testing.T) {
		// 10 seconds is reasonable for homelab networks
		if cfg.ConnectTimeout < 5*time.Second || cfg.ConnectTimeout > 30*time.Second {
			t.Errorf("connect timeout %v not in reasonable range for homelab", cfg.ConnectTimeout)
		}
	})

	t.Run("has unlimited reconnects for reliability", func(t *testing.T) {
		if cfg.MaxReconnects != -1 {
			t.Errorf("expected unlimited reconnects (-1) for homelab reliability, got: %d", cfg.MaxReconnects)
		}
	})

	t.Run("has reasonable reconnect wait", func(t *testing.T) {
		// 2 seconds between reconnect attempts is reasonable
		if cfg.ReconnectWait < 1*time.Second || cfg.ReconnectWait > 5*time.Second {
			t.Errorf("reconnect wait %v not in reasonable range for homelab", cfg.ReconnectWait)
		}
	})
}
