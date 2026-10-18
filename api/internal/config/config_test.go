package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test that LoadDefaults returns pure defaults without .env or environment overrides
	cfg := LoadDefaults()

	// Check defaults
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected default host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("expected default database host 'localhost', got '%s'", cfg.Database.Host)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got '%s'", cfg.Logging.Level)
	}
	if cfg.TemplatesDir != "templates" {
		t.Errorf("expected default templates dir 'templates', got '%s'", cfg.TemplatesDir)
	}
}

func TestLoadFromEnv_Database(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("DATABASE_HOST", "db.example.com")
	os.Setenv("DATABASE_PORT", "5433")
	os.Setenv("DATABASE_USER", "testuser")
	os.Setenv("DATABASE_PASSWORD", "testpass")
	os.Setenv("DATABASE_NAME", "testdb")
	os.Setenv("DATABASE_SSL_MODE", "require")

	cfg := Load()

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("expected database host 'db.example.com', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("expected database port 5433, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("expected database user 'testuser', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("expected database password 'testpass', got '%s'", cfg.Database.Password)
	}
	if cfg.Database.Database != "testdb" {
		t.Errorf("expected database name 'testdb', got '%s'", cfg.Database.Database)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected ssl mode 'require', got '%s'", cfg.Database.SSLMode)
	}
}

func TestLoadFromEnv_Server(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("API_HOST", "0.0.0.0")
	os.Setenv("API_PORT", "9090")

	cfg := Load()

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected server host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected server port 9090, got %d", cfg.Server.Port)
	}
}

func TestLoadFromEnv_CORSOrigins(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "single origin",
			envValue: "http://localhost:3000",
			expected: []string{"http://localhost:3000"},
		},
		{
			name:     "multiple origins",
			envValue: "http://localhost:3000,https://app.example.com",
			expected: []string{"http://localhost:3000", "https://app.example.com"},
		},
		{
			name:     "origins with spaces",
			envValue: " http://localhost:3000 , https://app.example.com ",
			expected: []string{"http://localhost:3000", "https://app.example.com"},
		},
		{
			name:     "empty entries filtered",
			envValue: "http://localhost:3000,,https://app.example.com",
			expected: []string{"http://localhost:3000", "https://app.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalEnv := clearEnv()
			defer restoreEnv(originalEnv)

			os.Setenv("CORS_ORIGINS", tt.envValue)

			cfg := Load()

			if len(cfg.Server.CORSOrigins) != len(tt.expected) {
				t.Errorf("expected %d CORS origins, got %d", len(tt.expected), len(cfg.Server.CORSOrigins))
				return
			}
			for i, origin := range cfg.Server.CORSOrigins {
				if origin != tt.expected[i] {
					t.Errorf("expected CORS origin %d to be '%s', got '%s'", i, tt.expected[i], origin)
				}
			}
		})
	}
}

func TestLoadFromEnv_Redis(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("REDIS_HOST", "redis.example.com")
	os.Setenv("REDIS_PORT", "6380")
	os.Setenv("REDIS_PASSWORD", "redispass")
	os.Setenv("REDIS_DATABASE", "1")
	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_TLS", "true")
	os.Setenv("REDIS_KEY_PREFIX", "myapp:")
	os.Setenv("REDIS_POOL_SIZE", "20")

	cfg := Load()

	if cfg.Redis.Host != "redis.example.com" {
		t.Errorf("expected redis host 'redis.example.com', got '%s'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("expected redis port 6380, got %d", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "redispass" {
		t.Errorf("expected redis password 'redispass', got '%s'", cfg.Redis.Password)
	}
	if cfg.Redis.Database != 1 {
		t.Errorf("expected redis database 1, got %d", cfg.Redis.Database)
	}
	if !cfg.Redis.Enabled {
		t.Error("expected redis enabled to be true")
	}
	if !cfg.Redis.TLS {
		t.Error("expected redis TLS to be true")
	}
	if cfg.Redis.KeyPrefix != "myapp:" {
		t.Errorf("expected redis key prefix 'myapp:', got '%s'", cfg.Redis.KeyPrefix)
	}
	if cfg.Redis.MaxPoolSize != 20 {
		t.Errorf("expected redis pool size 20, got %d", cfg.Redis.MaxPoolSize)
	}
}

func TestLoadFromEnv_Proxmox(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("PROXMOX_HOST", "https://pve.example.com:8006")
	os.Setenv("PROXMOX_TOKEN_ID", "root@pam!mytoken")
	os.Setenv("PROXMOX_TOKEN", "secret-token-value")
	os.Setenv("PROXMOX_NODE", "pve1")
	os.Setenv("PROXMOX_INSECURE", "true")

	cfg := Load()

	if cfg.Proxmox.Host != "https://pve.example.com:8006" {
		t.Errorf("expected proxmox host, got '%s'", cfg.Proxmox.Host)
	}
	if cfg.Proxmox.TokenID != "root@pam!mytoken" {
		t.Errorf("expected proxmox token ID, got '%s'", cfg.Proxmox.TokenID)
	}
	if cfg.Proxmox.Token != "secret-token-value" {
		t.Errorf("expected proxmox token, got '%s'", cfg.Proxmox.Token)
	}
	if cfg.Proxmox.DefaultNode != "pve1" {
		t.Errorf("expected proxmox node 'pve1', got '%s'", cfg.Proxmox.DefaultNode)
	}
	if !cfg.Proxmox.Insecure {
		t.Error("expected proxmox insecure to be true")
	}
}

func TestLoadFromEnv_CloudStack(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("CLOUDSTACK_URL", "https://cloudstack.example.com")
	os.Setenv("CLOUDSTACK_API_KEY", "api-key-value")
	os.Setenv("CLOUDSTACK_SECRET_KEY", "secret-key-value")
	os.Setenv("CLOUDSTACK_INSECURE", "true")

	cfg := Load()

	if cfg.CloudStack.Host != "https://cloudstack.example.com" {
		t.Errorf("expected cloudstack host, got '%s'", cfg.CloudStack.Host)
	}
	if cfg.CloudStack.APIKey != "api-key-value" {
		t.Errorf("expected cloudstack API key, got '%s'", cfg.CloudStack.APIKey)
	}
	if cfg.CloudStack.SecretKey != "secret-key-value" {
		t.Errorf("expected cloudstack secret key, got '%s'", cfg.CloudStack.SecretKey)
	}
	if !cfg.CloudStack.Insecure {
		t.Error("expected cloudstack insecure to be true")
	}
}

func TestLoadFromEnv_Wazuh(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("WAZUH_MANAGER_URL", "https://wazuh.example.com:55000")
	os.Setenv("WAZUH_API_USER", "wazuh-api")
	os.Setenv("WAZUH_API_PASSWORD", "wazuh-pass")
	os.Setenv("WAZUH_WEBHOOK_SECRET", "webhook-secret")

	cfg := Load()

	if cfg.Wazuh.ManagerURL != "https://wazuh.example.com:55000" {
		t.Errorf("expected wazuh manager URL, got '%s'", cfg.Wazuh.ManagerURL)
	}
	if cfg.Wazuh.ManagerUsername != "wazuh-api" {
		t.Errorf("expected wazuh username, got '%s'", cfg.Wazuh.ManagerUsername)
	}
	if cfg.Wazuh.ManagerPassword != "wazuh-pass" {
		t.Errorf("expected wazuh password, got '%s'", cfg.Wazuh.ManagerPassword)
	}
	if cfg.Wazuh.WebhookSecret != "webhook-secret" {
		t.Errorf("expected wazuh webhook secret, got '%s'", cfg.Wazuh.WebhookSecret)
	}
}

func TestLoadFromEnv_WazuhManagerHost(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	// Test the legacy WAZUH_MANAGER_HOST env var
	os.Setenv("WAZUH_MANAGER_HOST", "wazuh-host.example.com")

	cfg := Load()

	expected := "https://wazuh-host.example.com:55000"
	if cfg.Wazuh.ManagerURL != expected {
		t.Errorf("expected wazuh manager URL '%s', got '%s'", expected, cfg.Wazuh.ManagerURL)
	}
}

func TestLoadFromEnv_Auth(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("JWT_SECRET", "my-super-secret-jwt-key-32chars")
	os.Setenv("JWT_EXPIRATION", "2h")
	os.Setenv("AUTH_DEMO_MODE", "true")

	cfg := Load()

	if cfg.Auth.JWTSecret != "my-super-secret-jwt-key-32chars" {
		t.Errorf("expected JWT secret, got '%s'", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.JWTExpiration != 2*time.Hour {
		t.Errorf("expected JWT expiration 2h, got %v", cfg.Auth.JWTExpiration)
	}
	if !cfg.Auth.DemoMode {
		t.Error("expected demo mode to be true")
	}
}

func TestLoadFromEnv_Logging(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")
	os.Setenv("LOG_OUTPUT", "file")
	os.Setenv("LOG_FILE", "/var/log/app.log")

	cfg := Load()

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected log format 'json', got '%s'", cfg.Logging.Format)
	}
	if cfg.Logging.Output != "file" {
		t.Errorf("expected log output 'file', got '%s'", cfg.Logging.Output)
	}
	if cfg.Logging.FilePath != "/var/log/app.log" {
		t.Errorf("expected log file path, got '%s'", cfg.Logging.FilePath)
	}
}

func TestLoadFromEnv_DevMode(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("DEV_MODE", "true")
	os.Setenv("MOCK_PROXMOX", "true")
	os.Setenv("MOCK_CLOUDSTACK", "true")

	cfg := Load()

	if !cfg.DevMode {
		t.Error("expected dev mode to be true")
	}
	if !cfg.MockProxmox {
		t.Error("expected mock proxmox to be true")
	}
	if !cfg.MockCloudStack {
		t.Error("expected mock cloudstack to be true")
	}
}

func TestLoadFromEnv_TemplatesDir(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("TEMPLATES_DIR", "/custom/templates")

	cfg := Load()

	if cfg.TemplatesDir != "/custom/templates" {
		t.Errorf("expected templates dir '/custom/templates', got '%s'", cfg.TemplatesDir)
	}
}

func TestLoadFromEnv_CLI(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("CLI_API_URL", "http://localhost:8080")

	cfg := Load()

	if cfg.CLI.APIBaseURL != "http://localhost:8080" {
		t.Errorf("expected CLI API URL 'http://localhost:8080', got '%s'", cfg.CLI.APIBaseURL)
	}
}

func TestLoadFromEnv_CanvasLTI(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	os.Setenv("CANVAS_URL", "https://canvas.example.com")
	os.Setenv("CANVAS_CLIENT_ID", "canvas-client-123")
	os.Setenv("CANVAS_DEPLOYMENT_ID", "deploy-456")

	cfg := Load()

	if cfg.Server.LTI.CanvasURL != "https://canvas.example.com" {
		t.Errorf("expected canvas URL, got '%s'", cfg.Server.LTI.CanvasURL)
	}
	if cfg.Server.LTI.ClientID != "canvas-client-123" {
		t.Errorf("expected canvas client ID, got '%s'", cfg.Server.LTI.ClientID)
	}
	if cfg.Server.LTI.DeploymentID != "deploy-456" {
		t.Errorf("expected canvas deployment ID, got '%s'", cfg.Server.LTI.DeploymentID)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	originalEnv := clearEnv()
	defer restoreEnv(originalEnv)

	// Set invalid port - should be ignored and use default
	os.Setenv("API_PORT", "not-a-number")

	cfg := Load()

	// Should fall back to default
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080 when invalid port provided, got %d", cfg.Server.Port)
	}
}

func TestIsDevelopment(t *testing.T) {
	cfg := &Config{DevMode: true}
	if !cfg.IsDevelopment() {
		t.Error("expected IsDevelopment to return true")
	}

	cfg.DevMode = false
	if cfg.IsDevelopment() {
		t.Error("expected IsDevelopment to return false")
	}
}

func TestHasProxmox(t *testing.T) {
	cfg := &Config{}

	// No host
	if cfg.HasProxmox() {
		t.Error("expected HasProxmox to return false when host is empty")
	}

	// Has host
	cfg.Proxmox.Host = "https://pve.example.com"
	if !cfg.HasProxmox() {
		t.Error("expected HasProxmox to return true when host is set")
	}

	// Has host but mocked
	cfg.MockProxmox = true
	if cfg.HasProxmox() {
		t.Error("expected HasProxmox to return false when mocked")
	}
}

func TestHasCloudStack(t *testing.T) {
	cfg := &Config{}

	// No host
	if cfg.HasCloudStack() {
		t.Error("expected HasCloudStack to return false when host is empty")
	}

	// Has host
	cfg.CloudStack.Host = "https://cloudstack.example.com"
	if !cfg.HasCloudStack() {
		t.Error("expected HasCloudStack to return true when host is set")
	}

	// Has host but mocked
	cfg.MockCloudStack = true
	if cfg.HasCloudStack() {
		t.Error("expected HasCloudStack to return false when mocked")
	}
}

func TestHasNATS(t *testing.T) {
	cfg := &Config{}

	if cfg.HasNATS() {
		t.Error("expected HasNATS to return false when URL is empty")
	}

	cfg.NATS.URL = "nats://localhost:4222"
	if !cfg.HasNATS() {
		t.Error("expected HasNATS to return true when URL is set")
	}
}

func TestHasRedis(t *testing.T) {
	cfg := &Config{}

	if cfg.HasRedis() {
		t.Error("expected HasRedis to return false when disabled")
	}

	cfg.Redis.Enabled = true
	if !cfg.HasRedis() {
		t.Error("expected HasRedis to return true when enabled")
	}
}

func TestHasWazuh(t *testing.T) {
	cfg := &Config{}

	if cfg.HasWazuh() {
		t.Error("expected HasWazuh to return false when URL is empty")
	}

	cfg.Wazuh.ManagerURL = "https://wazuh.example.com:55000"
	if !cfg.HasWazuh() {
		t.Error("expected HasWazuh to return true when URL is set")
	}
}

func TestHasLTI(t *testing.T) {
	cfg := &Config{}

	if cfg.HasLTI() {
		t.Error("expected HasLTI to return false when client ID is empty")
	}

	cfg.Server.LTI.ClientID = "canvas-client-123"
	if !cfg.HasLTI() {
		t.Error("expected HasLTI to return true when client ID is set")
	}
}

// Helper functions for test isolation

var envVarsToPreserve = []string{
	"DATABASE_HOST", "DATABASE_PORT", "DATABASE_USER", "DATABASE_PASSWORD", "DATABASE_NAME", "DATABASE_SSL_MODE",
	"API_HOST", "API_PORT",
	"NATS_URL",
	"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DATABASE", "REDIS_ENABLED", "REDIS_TLS", "REDIS_KEY_PREFIX", "REDIS_POOL_SIZE",
	"PROXMOX_HOST", "PROXMOX_TOKEN_ID", "PROXMOX_TOKEN", "PROXMOX_NODE", "PROXMOX_INSECURE",
	"CLOUDSTACK_URL", "CLOUDSTACK_API_KEY", "CLOUDSTACK_SECRET_KEY", "CLOUDSTACK_INSECURE",
	"WAZUH_MANAGER_HOST", "WAZUH_MANAGER_URL", "WAZUH_API_USER", "WAZUH_API_PASSWORD", "WAZUH_WEBHOOK_SECRET",
	"JWT_SECRET", "JWT_EXPIRATION", "AUTH_DEMO_MODE",
	"CANVAS_URL", "CANVAS_CLIENT_ID", "CANVAS_DEPLOYMENT_ID",
	"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUT", "LOG_FILE",
	"DEV_MODE", "MOCK_PROXMOX", "MOCK_CLOUDSTACK",
	"TEMPLATES_DIR",
	"CLI_API_URL",
}

func clearEnv() map[string]string {
	original := make(map[string]string)
	for _, key := range envVarsToPreserve {
		if val, exists := os.LookupEnv(key); exists {
			original[key] = val
		}
		os.Unsetenv(key)
	}
	return original
}

func restoreEnv(original map[string]string) {
	for _, key := range envVarsToPreserve {
		os.Unsetenv(key)
	}
	for key, val := range original {
		os.Setenv(key, val)
	}
}
