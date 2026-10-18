package nats

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("URL", func(t *testing.T) {
		expected := "nats://localhost:4222"
		if cfg.URL != expected {
			t.Errorf("expected URL %q, got %q", expected, cfg.URL)
		}
	})

	t.Run("Name", func(t *testing.T) {
		expected := "lab-platform"
		if cfg.Name != expected {
			t.Errorf("expected Name %q, got %q", expected, cfg.Name)
		}
	})

	t.Run("ConnectTimeout", func(t *testing.T) {
		expected := 10 * time.Second
		if cfg.ConnectTimeout != expected {
			t.Errorf("expected ConnectTimeout %v, got %v", expected, cfg.ConnectTimeout)
		}
	})

	t.Run("ReconnectWait", func(t *testing.T) {
		expected := 2 * time.Second
		if cfg.ReconnectWait != expected {
			t.Errorf("expected ReconnectWait %v, got %v", expected, cfg.ReconnectWait)
		}
	})

	t.Run("MaxReconnects unlimited", func(t *testing.T) {
		expected := -1 // Unlimited
		if cfg.MaxReconnects != expected {
			t.Errorf("expected MaxReconnects %d, got %d", expected, cfg.MaxReconnects)
		}
	})

	t.Run("credentials empty by default", func(t *testing.T) {
		if cfg.Token != "" {
			t.Errorf("expected Token to be empty, got %q", cfg.Token)
		}
		if cfg.User != "" {
			t.Errorf("expected User to be empty, got %q", cfg.User)
		}
		if cfg.Password != "" {
			t.Errorf("expected Password to be empty, got %q", cfg.Password)
		}
	})
}

func TestConfigStruct(t *testing.T) {
	t.Run("custom config", func(t *testing.T) {
		cfg := Config{
			URL:            "nats://myserver:4222",
			Name:           "test-client",
			Token:          "secret-token",
			ConnectTimeout: 5 * time.Second,
			ReconnectWait:  1 * time.Second,
			MaxReconnects:  10,
		}

		if cfg.URL != "nats://myserver:4222" {
			t.Errorf("unexpected URL: %s", cfg.URL)
		}
		if cfg.Name != "test-client" {
			t.Errorf("unexpected Name: %s", cfg.Name)
		}
		if cfg.Token != "secret-token" {
			t.Errorf("unexpected Token: %s", cfg.Token)
		}
		if cfg.ConnectTimeout != 5*time.Second {
			t.Errorf("unexpected ConnectTimeout: %v", cfg.ConnectTimeout)
		}
		if cfg.ReconnectWait != 1*time.Second {
			t.Errorf("unexpected ReconnectWait: %v", cfg.ReconnectWait)
		}
		if cfg.MaxReconnects != 10 {
			t.Errorf("unexpected MaxReconnects: %d", cfg.MaxReconnects)
		}
	})

	t.Run("user/password auth", func(t *testing.T) {
		cfg := Config{
			URL:      "nats://myserver:4222",
			Name:     "test-client",
			User:     "admin",
			Password: "admin-pass",
		}

		if cfg.URL != "nats://myserver:4222" {
			t.Errorf("unexpected URL: %s", cfg.URL)
		}
		if cfg.Name != "test-client" {
			t.Errorf("unexpected Name: %s", cfg.Name)
		}
		if cfg.User != "admin" {
			t.Errorf("unexpected User: %s", cfg.User)
		}
		if cfg.Password != "admin-pass" {
			t.Errorf("unexpected Password: %s", cfg.Password)
		}
		// Token should be empty when using user/password
		if cfg.Token != "" {
			t.Errorf("Token should be empty when using user/password auth")
		}
	})

	t.Run("cluster URLs", func(t *testing.T) {
		// Test that URLs can contain cluster addresses
		cfg := Config{
			URL:  "nats://node1:4222,nats://node2:4222,nats://node3:4222",
			Name: "cluster-client",
		}

		if cfg.URL != "nats://node1:4222,nats://node2:4222,nats://node3:4222" {
			t.Errorf("unexpected cluster URL: %s", cfg.URL)
		}
		if cfg.Name != "cluster-client" {
			t.Errorf("unexpected Name: %s", cfg.Name)
		}
	})
}

func TestConsumerConfigStruct(t *testing.T) {
	t.Run("durable consumer", func(t *testing.T) {
		cfg := ConsumerConfig{
			Name:          "event-store",
			Durable:       true,
			FilterSubject: "labs.events.>",
			MaxDeliver:    5,
			AckWait:       30,
		}

		if cfg.Name != "event-store" {
			t.Errorf("unexpected Name: %s", cfg.Name)
		}
		if !cfg.Durable {
			t.Error("expected Durable to be true")
		}
		if cfg.FilterSubject != "labs.events.>" {
			t.Errorf("unexpected FilterSubject: %s", cfg.FilterSubject)
		}
		if cfg.MaxDeliver != 5 {
			t.Errorf("unexpected MaxDeliver: %d", cfg.MaxDeliver)
		}
		if cfg.AckWait != 30 {
			t.Errorf("unexpected AckWait: %d", cfg.AckWait)
		}
	})

	t.Run("ephemeral consumer", func(t *testing.T) {
		cfg := ConsumerConfig{
			Name:          "websocket-broadcast",
			Durable:       false,
			FilterSubject: "labs.checkpoints.>",
			MaxDeliver:    1,
		}

		if cfg.Name != "websocket-broadcast" {
			t.Errorf("unexpected Name: %s", cfg.Name)
		}
		if cfg.Durable {
			t.Error("expected Durable to be false for ephemeral consumer")
		}
		if cfg.FilterSubject != "labs.checkpoints.>" {
			t.Errorf("unexpected FilterSubject: %s", cfg.FilterSubject)
		}
		if cfg.MaxDeliver != 1 {
			t.Errorf("expected MaxDeliver 1 for ephemeral consumer, got %d", cfg.MaxDeliver)
		}
	})
}
