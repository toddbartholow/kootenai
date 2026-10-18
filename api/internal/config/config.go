// Package config provides configuration loading from environment variables and .env files
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	authldap "github.com/toddbartholow/kootenai/api/internal/auth/ldap"
	"github.com/toddbartholow/kootenai/api/internal/cli"
	"github.com/toddbartholow/kootenai/api/internal/cloudstack"
	"github.com/toddbartholow/kootenai/api/internal/database"
	"github.com/toddbartholow/kootenai/api/internal/logging"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
	"github.com/toddbartholow/kootenai/api/internal/proxmox"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server"
	"github.com/toddbartholow/kootenai/api/internal/wazuh"
)

// Config holds the full application configuration
type Config struct {
	Server       server.Config      `yaml:"server"`
	Database     database.Config    `yaml:"database"`
	NATS         natsclient.Config  `yaml:"nats"`
	Redis        redisclient.Config `yaml:"redis"`
	Proxmox      proxmox.Config     `yaml:"proxmox"`
	CloudStack   cloudstack.Config  `yaml:"cloudstack"`
	Wazuh        wazuh.Config       `yaml:"wazuh"`
	CLI          cli.Config         `yaml:"cli"`
	Auth         auth.Config        `yaml:"auth"`
	LDAP         authldap.Config    `yaml:"ldap"`
	Logging      logging.Config     `yaml:"logging"`
	TemplatesDir string             `yaml:"templates_dir"`

	// TemplateVMIDs maps lab template names to Proxmox VM IDs
	TemplateVMIDs map[string]int `yaml:"template_vmids"`

	// Development settings
	DevMode        bool `yaml:"dev_mode"`
	MockProxmox    bool `yaml:"mock_proxmox"`
	MockCloudStack bool `yaml:"mock_cloudstack"`
}

// LoadEnv loads environment variables from .env files
// It looks for .env and .env.local in the current directory and parent directories
// .env.local takes precedence over .env
func LoadEnv() error {
	// Try to find .env file in current or parent directories
	// .env.local files are loaded last to override base .env settings
	envPaths := []string{
		".env",
		"../.env",
		"../../.env",
		filepath.Join(os.Getenv("HOME"), ".labctl", ".env"),
		".env.local",
		"../.env.local",
		"../../.env.local",
	}

	// Load all found .env files in order (later files override earlier ones)
	for _, path := range envPaths {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				slog.Warn("Failed to parse .env file", "path", path, "error", err)
			}
		}
	}

	// No .env file found - that's okay, we'll use defaults and explicit env vars
	return nil
}

// LoadDefaults creates a Config with default values only (no env overrides)
func LoadDefaults() *Config {
	return &Config{
		Server:       server.DefaultConfig(),
		Database:     database.DefaultConfig(),
		NATS:         natsclient.DefaultConfig(),
		Redis:        redisclient.DefaultConfig(),
		Proxmox:      proxmox.DefaultConfig(),
		CloudStack:   cloudstack.DefaultConfig(),
		Wazuh:        wazuh.DefaultConfig(),
		CLI:          cli.DefaultConfig(),
		Auth:         auth.DefaultConfig(),
		Logging:      logging.DefaultConfig(),
		TemplatesDir: "templates",
	}
}

// Load creates a Config populated from defaults, config files, and environment variables
func Load() *Config {
	// Load .env file if present (ignore errors - it's optional)
	_ = LoadEnv()

	cfg := LoadDefaults()

	// Override with environment variables
	cfg.LoadFromEnv()

	return cfg
}

// LoadFromEnv overrides config values from environment variables
func (c *Config) LoadFromEnv() {
	// Database
	if v := os.Getenv("DATABASE_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DATABASE_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := os.Getenv("DATABASE_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DATABASE_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("DATABASE_NAME"); v != "" {
		c.Database.Database = v
	}
	if v := os.Getenv("DATABASE_SSL_MODE"); v != "" {
		c.Database.SSLMode = v
	}

	// Server
	if v := os.Getenv("API_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("API_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.Port = port
		}
	}
	// CORS configuration (comma-separated list of allowed origins)
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		origins := strings.Split(v, ",")
		c.Server.CORSOrigins = make([]string, 0, len(origins))
		for _, origin := range origins {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				c.Server.CORSOrigins = append(c.Server.CORSOrigins, origin)
			}
		}
	}

	// NATS
	if v := os.Getenv("NATS_URL"); v != "" {
		c.NATS.URL = v
	}

	// Redis
	if v := os.Getenv("REDIS_HOST"); v != "" {
		c.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.Redis.Password = v
	}
	if v := os.Getenv("REDIS_DATABASE"); v != "" {
		if db, err := strconv.Atoi(v); err == nil {
			c.Redis.Database = db
		}
	}
	if v := os.Getenv("REDIS_ENABLED"); v != "" {
		c.Redis.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("REDIS_TLS"); v != "" {
		c.Redis.TLS = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("REDIS_KEY_PREFIX"); v != "" {
		c.Redis.KeyPrefix = v
	}
	if v := os.Getenv("REDIS_POOL_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			c.Redis.MaxPoolSize = size
		}
	}

	// Proxmox
	if v := os.Getenv("PROXMOX_HOST"); v != "" {
		c.Proxmox.Host = v
	}
	if v := os.Getenv("PROXMOX_TOKEN_ID"); v != "" {
		c.Proxmox.TokenID = v
	}
	if v := os.Getenv("PROXMOX_TOKEN"); v != "" {
		c.Proxmox.Token = v
	}
	if v := os.Getenv("PROXMOX_NODE"); v != "" {
		c.Proxmox.DefaultNode = v
	}
	if v := os.Getenv("PROXMOX_INSECURE"); v != "" {
		c.Proxmox.Insecure = strings.ToLower(v) == "true"
	}

	// CloudStack
	if v := os.Getenv("CLOUDSTACK_URL"); v != "" {
		c.CloudStack.Host = v
	}
	if v := os.Getenv("CLOUDSTACK_API_KEY"); v != "" {
		c.CloudStack.APIKey = v
	}
	if v := os.Getenv("CLOUDSTACK_SECRET_KEY"); v != "" {
		c.CloudStack.SecretKey = v
	}
	if v := os.Getenv("CLOUDSTACK_INSECURE"); v != "" {
		c.CloudStack.Insecure = strings.ToLower(v) == "true"
	}

	// Wazuh
	if v := os.Getenv("WAZUH_MANAGER_HOST"); v != "" {
		c.Wazuh.ManagerURL = "https://" + v + ":55000"
	}
	if v := os.Getenv("WAZUH_MANAGER_URL"); v != "" {
		c.Wazuh.ManagerURL = v
	}
	if v := os.Getenv("WAZUH_API_USER"); v != "" {
		c.Wazuh.ManagerUsername = v
	}
	if v := os.Getenv("WAZUH_API_PASSWORD"); v != "" {
		c.Wazuh.ManagerPassword = v
	}
	if v := os.Getenv("WAZUH_WEBHOOK_SECRET"); v != "" {
		c.Wazuh.WebhookSecret = v
	}
	if v := os.Getenv("WAZUH_REQUIRE_SIGNATURE"); v != "" {
		c.Wazuh.RequireSignature = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("WAZUH_ALLOWED_IPS"); v != "" {
		ips := strings.Split(v, ",")
		c.Wazuh.AllowedIPs = make([]string, 0, len(ips))
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				c.Wazuh.AllowedIPs = append(c.Wazuh.AllowedIPs, ip)
			}
		}
	}

	// Auth / JWT
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.Auth.JWTSecret = v
	}
	if v := os.Getenv("JWT_EXPIRATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Auth.JWTExpiration = d
		}
	}
	// SECURITY: Demo mode allows unauthenticated access - only for development
	if v := os.Getenv("AUTH_DEMO_MODE"); v != "" {
		c.Auth.DemoMode = strings.ToLower(v) == "true"
	}
	// Demo role determines permissions for unauthenticated demo users
	// Default is "student" for safety, but can be set to "admin" for full access
	if v := os.Getenv("AUTH_DEMO_ROLE"); v != "" {
		c.Auth.DemoRole = v
	}
	// HttpOnly cookie auth (ADR-0002 Phase B)
	if v := os.Getenv("AUTH_COOKIE_MODE"); v != "" {
		c.Auth.CookieMode = strings.ToLower(v) == "true"
	}

	// LDAP / FreeIPA
	c.LDAP = authldap.DefaultConfig()
	if v := os.Getenv("LDAP_HOST"); v != "" {
		c.LDAP.Host = v
	}
	if v := os.Getenv("LDAP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.LDAP.Port = port
		}
	}
	if v := os.Getenv("LDAP_BIND_DN"); v != "" {
		c.LDAP.BindDN = v
	}
	if v := os.Getenv("LDAP_BIND_PASSWORD"); v != "" {
		c.LDAP.BindPassword = v
	}
	if v := os.Getenv("LDAP_BASE_DN"); v != "" {
		c.LDAP.BaseDN = v
	}
	if v := os.Getenv("LDAP_USE_TLS"); v != "" {
		c.LDAP.UseTLS = strings.ToLower(v) != "false"
	}
	if v := os.Getenv("LDAP_START_TLS"); v != "" {
		c.LDAP.StartTLS = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("LDAP_TLS_SKIP_VERIFY"); v != "" {
		c.LDAP.TLSSkipVerify = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("LDAP_TLS_CA_CERT"); v != "" {
		c.LDAP.TLSCACertPath = v
	}
	if v := os.Getenv("LDAP_USER_SEARCH_FILTER"); v != "" {
		c.LDAP.UserSearchFilter = v
	}
	if v := os.Getenv("LDAP_DEFAULT_DOMAIN"); v != "" {
		c.LDAP.DefaultDomain = v
	}
	if v := os.Getenv("LDAP_AUTH_FIRST"); v != "" {
		c.LDAP.AuthFirst = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("LDAP_DEFAULT_ROLE"); v != "" {
		c.LDAP.DefaultRole = v
	}
	if v := os.Getenv("LDAP_GROUP_ROLE_MAPPING"); v != "" {
		c.LDAP.GroupRoleMapping = parseGroupRoleMapping(v)
	}

	// Canvas LTI
	if v := os.Getenv("CANVAS_URL"); v != "" {
		c.Server.LTI.CanvasURL = v
	}
	if v := os.Getenv("CANVAS_CLIENT_ID"); v != "" {
		c.Server.LTI.ClientID = v
	}
	if v := os.Getenv("CANVAS_DEPLOYMENT_ID"); v != "" {
		c.Server.LTI.DeploymentID = v
	}
	if v := os.Getenv("CANVAS_AUTHORIZATION_URL"); v != "" {
		c.Server.LTI.AuthorizationURL = v
	}
	if v := os.Getenv("CANVAS_TOKEN_URL"); v != "" {
		c.Server.LTI.TokenURL = v
	}
	if v := os.Getenv("CANVAS_JWKS_URL"); v != "" {
		c.Server.LTI.JWKsURL = v
	}
	if v := os.Getenv("LTI_TOOL_ISSUER"); v != "" {
		c.Server.LTI.ToolIssuer = v
	}
	// LTI keys can be provided as file paths or inline PEM content
	// #nosec G304 -- Paths are from environment variables (admin configuration).
	if v := os.Getenv("LTI_PRIVATE_KEY_PATH"); v != "" {
		if keyData, err := os.ReadFile(v); err == nil {
			c.Server.LTI.ToolPrivateKey = string(keyData)
		} else {
			// Log to stderr since logger may not be initialized yet
			fmt.Fprintf(os.Stderr, "WARNING: Failed to read LTI private key from %s: %v\n", v, err)
		}
	}
	if v := os.Getenv("LTI_PRIVATE_KEY"); v != "" {
		c.Server.LTI.ToolPrivateKey = v
	}
	// #nosec G304 -- Paths are from environment variables (admin configuration).
	if v := os.Getenv("LTI_PUBLIC_KEY_PATH"); v != "" {
		if keyData, err := os.ReadFile(v); err == nil {
			c.Server.LTI.ToolPublicKey = string(keyData)
		} else {
			// Log to stderr since logger may not be initialized yet
			fmt.Fprintf(os.Stderr, "WARNING: Failed to read LTI public key from %s: %v\n", v, err)
		}
	}
	if v := os.Getenv("LTI_PUBLIC_KEY"); v != "" {
		c.Server.LTI.ToolPublicKey = v
	}
	if v := os.Getenv("CANVAS_API_ACCESS_TOKEN"); v != "" {
		c.Server.LTI.APIAccessToken = v
	}

	// Logging
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		c.Logging.Format = v
	}
	if v := os.Getenv("LOG_OUTPUT"); v != "" {
		c.Logging.Output = v
	}
	if v := os.Getenv("LOG_FILE"); v != "" {
		c.Logging.FilePath = v
	}

	// Development settings
	if v := os.Getenv("DEV_MODE"); v != "" {
		c.DevMode = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("MOCK_PROXMOX"); v != "" {
		c.MockProxmox = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("MOCK_CLOUDSTACK"); v != "" {
		c.MockCloudStack = strings.ToLower(v) == "true"
	}

	// Templates directory
	if v := os.Getenv("TEMPLATES_DIR"); v != "" {
		c.TemplatesDir = v
	}

	// CLI
	if v := os.Getenv("CLI_API_URL"); v != "" {
		c.CLI.APIBaseURL = v
	}

	// Template VMID mappings (format: "name1:vmid1,name2:vmid2")
	// Example: TEMPLATE_VMIDS=ubuntu-lab-wazuh-template:9003,ubuntu-22.04-server:9002
	if v := os.Getenv("TEMPLATE_VMIDS"); v != "" {
		c.TemplateVMIDs = make(map[string]int)
		pairs := strings.Split(v, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				if vmid, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					c.TemplateVMIDs[name] = vmid
				}
			}
		}
	}
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.DevMode
}

// HasProxmox returns true if Proxmox is configured
func (c *Config) HasProxmox() bool {
	return c.Proxmox.Host != "" && !c.MockProxmox
}

// HasCloudStack returns true if CloudStack is configured
func (c *Config) HasCloudStack() bool {
	return c.CloudStack.Host != "" && !c.MockCloudStack
}

// HasNATS returns true if NATS is configured
func (c *Config) HasNATS() bool {
	return c.NATS.URL != ""
}

// HasRedis returns true if Redis is enabled
func (c *Config) HasRedis() bool {
	return c.Redis.Enabled
}

// HasWazuh returns true if Wazuh is configured
func (c *Config) HasWazuh() bool {
	return c.Wazuh.ManagerURL != ""
}

// HasLTI returns true if Canvas LTI is configured
func (c *Config) HasLTI() bool {
	return c.Server.LTI.ClientID != ""
}

// HasLDAP returns true if LDAP authentication is configured
func (c *Config) HasLDAP() bool {
	return c.LDAP.Host != ""
}

// parseGroupRoleMapping parses "dn1:role1,dn2:role2" format into a map.
func parseGroupRoleMapping(s string) map[string]string {
	m := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}
