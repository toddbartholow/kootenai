package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	goldap "github.com/go-ldap/ldap/v3"
)

// Sentinel errors for LDAP operations.
var (
	ErrUserNotFound       = errors.New("ldap: user not found")
	ErrInvalidCredentials = errors.New("ldap: invalid credentials")
	ErrConnectionFailed   = errors.New("ldap: connection failed")
	ErrTLSRequired        = errors.New("ldap: TLS is required")
	ErrSearchFailed       = errors.New("ldap: search failed")
)

// UserInfo holds user attributes extracted from an LDAP search result.
type UserInfo struct {
	UID         string              // LDAP uid attribute
	DN          string              // Full distinguished name
	Email       string              // mail attribute
	DisplayName string              // displayName or cn
	FirstName   string              // givenName
	LastName    string              // sn
	Groups      []string            // memberOf DNs
	Role        string              // Mapped platform role
	RawAttrs    map[string][]string // All returned attributes
}

// Client provides LDAP search+bind authentication with connection pooling.
type Client struct {
	config Config
	pool   chan *goldap.Conn
	logger *slog.Logger
	mu     sync.Mutex
	closed bool
}

// NewClient creates a new LDAP client. Validates the config and optionally
// tests the connection with a service-account bind.
func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	poolSize := cfg.MaxIdleConns
	if poolSize <= 0 {
		poolSize = 5
	}

	c := &Client{
		config: cfg,
		pool:   make(chan *goldap.Conn, poolSize),
		logger: logger.With("component", "ldap"),
	}

	return c, nil
}

// Config returns the client's configuration (read-only).
func (c *Client) Config() Config {
	return c.config
}

// Close drains the connection pool and marks the client as closed.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.pool)
	for conn := range c.pool {
		conn.Close()
	}
}

// HealthCheck tests the LDAP connection by performing a service-account bind.
func (c *Client) HealthCheck(ctx context.Context) error {
	conn, err := c.dial()
	if err != nil {
		return fmt.Errorf("ldap health check: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind(c.config.BindDN, c.config.BindPassword); err != nil {
		return fmt.Errorf("ldap health check bind: %w", err)
	}
	return nil
}

// Authenticate performs search+bind authentication:
// 1. Bind with service account
// 2. Search for user by username
// 3. Extract attributes and groups
// 4. Bind as user to verify password
// 5. Map groups to platform role
func (c *Client) Authenticate(ctx context.Context, username, password string) (*UserInfo, error) {
	if password == "" {
		return nil, ErrInvalidCredentials
	}

	// Step 1: Get a service-account-bound connection for the search
	conn, err := c.getConn()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer c.putConn(conn)

	// Step 2: Search for the user
	searchFilter := c.config.SearchFilter(username)
	timeout := c.config.SearchTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	searchReq := goldap.NewSearchRequest(
		c.config.BaseDN,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		0, // size limit
		int(timeout.Seconds()),
		false, // types only
		searchFilter,
		c.config.SearchAttributes(),
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		c.logger.Error("LDAP search failed", "filter", searchFilter, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}

	if len(result.Entries) == 0 {
		return nil, ErrUserNotFound
	}
	if len(result.Entries) > 1 {
		c.logger.Warn("LDAP search returned multiple entries, using first", "filter", searchFilter, "count", len(result.Entries))
	}

	entry := result.Entries[0]

	// Step 3: Extract user info
	info := c.extractUserInfo(entry)

	// Step 4: Bind as user to verify password (separate connection to avoid
	// rebinding the pooled service-account connection)
	userConn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer userConn.Close()

	if err := userConn.Bind(info.DN, password); err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultInvalidCredentials) {
			return nil, ErrInvalidCredentials
		}
		c.logger.Error("LDAP user bind failed", "dn", info.DN, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	// Step 5: Map groups to platform role
	info.Role = c.config.MapGroupsToRoles(info.Groups)

	c.logger.Info("LDAP authentication successful", "uid", info.UID, "email", info.Email, "role", info.Role)
	return info, nil
}

// extractUserInfo maps LDAP entry attributes to a UserInfo struct.
func (c *Client) extractUserInfo(entry *goldap.Entry) *UserInfo {
	m := c.config.UserAttrMap

	info := &UserInfo{
		DN:        entry.DN,
		UID:       entry.GetAttributeValue(m.UID),
		Email:     entry.GetAttributeValue(m.Email),
		FirstName: entry.GetAttributeValue(m.FirstName),
		LastName:  entry.GetAttributeValue(m.LastName),
		Groups:    entry.GetAttributeValues(m.MemberOf),
		RawAttrs:  make(map[string][]string),
	}

	// DisplayName: prefer explicit attribute, fall back to "FirstName LastName"
	info.DisplayName = entry.GetAttributeValue(m.DisplayName)
	if info.DisplayName == "" {
		info.DisplayName = strings.TrimSpace(info.FirstName + " " + info.LastName)
	}

	// Preserve all raw attributes
	for _, attr := range entry.Attributes {
		info.RawAttrs[attr.Name] = attr.Values
	}

	return info
}

// dial creates a new LDAP connection with TLS.
func (c *Client) dial() (*goldap.Conn, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}

	addr := c.config.Addr()

	if c.config.StartTLS {
		// Connect plain, then upgrade with StartTLS
		conn, err := goldap.DialURL("ldap://"+addr, goldap.DialWithTLSConfig(tlsConfig))
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", addr, err)
		}
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("StartTLS %s: %w", addr, err)
		}
		return conn, nil
	}

	// Direct LDAPS connection
	conn, err := goldap.DialURL("ldaps://"+addr, goldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, fmt.Errorf("dialTLS %s: %w", addr, err)
	}
	return conn, nil
}

// tlsConfig builds the TLS configuration from the LDAP config.
func (c *Client) tlsConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: c.config.TLSSkipVerify, // #nosec G402 -- TLS verification is configurable (TLSSkipVerify) for dev environments
		ServerName:         c.config.Host,
	}

	if c.config.TLSCACertPath != "" {
		caCert, err := os.ReadFile(c.config.TLSCACertPath)
		if err != nil {
			return nil, fmt.Errorf("read CA cert %s: %w", c.config.TLSCACertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA cert %s", c.config.TLSCACertPath)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

// getConn gets a connection from the pool or dials a new one, bound with
// the service account.
func (c *Client) getConn() (*goldap.Conn, error) {
	// Try to get an idle connection from the pool
	select {
	case conn := <-c.pool:
		if conn != nil && !conn.IsClosing() {
			return conn, nil
		}
		// Connection was stale, fall through to dial
	default:
		// Pool empty, dial new
	}

	conn, err := c.dial()
	if err != nil {
		return nil, err
	}

	// Bind as service account
	if err := conn.Bind(c.config.BindDN, c.config.BindPassword); err != nil {
		conn.Close()
		return nil, fmt.Errorf("service account bind: %w", err)
	}

	return conn, nil
}

// putConn returns a connection to the pool or closes it if the pool is full.
func (c *Client) putConn(conn *goldap.Conn) {
	if conn == nil || conn.IsClosing() {
		return
	}
	select {
	case c.pool <- conn:
		// Returned to pool
	default:
		// Pool full, close
		conn.Close()
	}
}
