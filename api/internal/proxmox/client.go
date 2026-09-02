// Package proxmox provides a client for interacting with Proxmox VE API
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/resilience"
)

// Client represents a Proxmox VE API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	tokenID    string
	resilient  *resilience.ResilientClient
	logger     *slog.Logger
}

// Config holds configuration for the Proxmox client
type Config struct {
	URL         string `yaml:"url"` // Full URL (alternative to Host/Port)
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	TokenID     string `yaml:"token_id"`     // Format: user@realm!tokenname
	Token       string `yaml:"token"`        // The secret token value
	Insecure    bool   `yaml:"insecure"`     // Skip TLS verification
	DefaultNode string `yaml:"default_node"` // Default Proxmox node for VM operations

	// Resilience settings
	MaxRetries       int           `yaml:"max_retries"`       // Maximum retry attempts (default: 3)
	RetryDelay       time.Duration `yaml:"retry_delay"`       // Initial retry delay (default: 100ms)
	CircuitThreshold int           `yaml:"circuit_threshold"` // Failures before circuit opens (default: 5)
	CircuitTimeout   time.Duration `yaml:"circuit_timeout"`   // Time before circuit half-opens (default: 30s)
}

// DefaultConfig returns a default Proxmox configuration
func DefaultConfig() Config {
	return Config{
		Host:             "localhost",
		Port:             8006,
		DefaultNode:      "pve",
		MaxRetries:       3,
		RetryDelay:       100 * time.Millisecond,
		CircuitThreshold: 5,
		CircuitTimeout:   30 * time.Second,
	}
}

// ClientOption configures a Proxmox client
type ClientOption func(*Client)

// WithLogger sets the logger for the client
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewClient creates a new Proxmox API client
func NewClient(cfg Config, opts ...ClientOption) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("proxmox host is required")
	}
	if cfg.TokenID == "" || cfg.Token == "" {
		return nil, fmt.Errorf("proxmox API token credentials are required")
	}

	// Normalize host - handle both URL format (https://host:port) and plain hostname
	host := cfg.Host
	port := cfg.Port

	// If host looks like a URL, parse it
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		u, err := url.Parse(host)
		if err == nil {
			host = u.Hostname()
			if p := u.Port(); p != "" {
				if parsedPort, err := strconv.Atoi(p); err == nil {
					port = parsedPort
				}
			}
		}
	}

	if port == 0 {
		port = 8006
	}

	baseURL := fmt.Sprintf("https://%s:%d/api2/json", host, port)

	// Apply defaults for resilience settings
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 100 * time.Millisecond
	}
	if cfg.CircuitThreshold <= 0 {
		cfg.CircuitThreshold = 5
	}
	if cfg.CircuitTimeout <= 0 {
		cfg.CircuitTimeout = 30 * time.Second
	}

	// Create HTTP client with optional TLS skip verification
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	if cfg.Insecure {
		// #nosec G402 -- InsecureSkipVerify is explicitly enabled via Config.Insecure
		// for internal/lab network environments using self-signed certificates.
		// Production deployments should use proper certificates and set Insecure=false.
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	client := &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		tokenID:    cfg.TokenID,
		token:      cfg.Token,
		logger:     slog.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Initialize resilient client
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "proxmox",
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold: cfg.CircuitThreshold,
			SuccessThreshold: 2,
			Timeout:          cfg.CircuitTimeout,
			OnStateChange: func(from, to resilience.CircuitState) {
				client.logger.Warn("proxmox circuit breaker state changed",
					slog.String("from", from.String()),
					slog.String("to", to.String()),
				)
			},
		},
		Retry: resilience.RetryConfig{
			MaxAttempts:     cfg.MaxRetries,
			InitialDelay:    cfg.RetryDelay,
			MaxDelay:        5 * time.Second,
			Multiplier:      2.0,
			RetryableErrors: resilience.IsRetryableError,
		},
		Logger: client.logger,
	})

	return client, nil
}

// Host returns the Proxmox host from the base URL (hostname:port format)
func (c *Client) Host() string {
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return ""
	}
	return parsedURL.Host
}

// AuthHeader returns the Authorization header value for API token authentication
func (c *Client) AuthHeader() string {
	return fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.token)
}

// VM represents a Proxmox virtual machine
type VM struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Node     string  `json:"node"`
	CPUs     int     `json:"cpus"`
	MaxMem   int64   `json:"maxmem"`
	MaxDisk  int64   `json:"maxdisk"`
	Uptime   int64   `json:"uptime"`
	Template bool    `json:"template"`
	CPU      float64 `json:"cpu"`
	Mem      int64   `json:"mem"`
	Lock     string  `json:"lock,omitempty"`
}

// Snapshot represents a VM snapshot
type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parent      string `json:"parent"`
	Snaptime    int64  `json:"snaptime"`
	VMState     int    `json:"vmstate"` // 0 = no RAM state, 1 = RAM state included
}

// Node represents a Proxmox cluster node
type Node struct {
	Node    string  `json:"node"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	MaxCPU  int     `json:"maxcpu"`
	Mem     int64   `json:"mem"`
	MaxMem  int64   `json:"maxmem"`
	Disk    int64   `json:"disk"`
	MaxDisk int64   `json:"maxdisk"`
	Uptime  int64   `json:"uptime"`
}

// apiResponse wraps the Proxmox API response format
type apiResponse struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error,omitempty"`
}

// do performs an HTTP request with authentication, retry, and circuit breaker
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	reqURL := c.baseURL + path

	var resp *http.Response
	var bodyBytes []byte

	// If body is provided, read it so we can retry
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	err := c.resilient.Execute(ctx, func() error {
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = strings.NewReader(string(bodyBytes))
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.token))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return err
		}

		// Check for retryable HTTP status codes
		if resilience.IsRetryableHTTPStatus(resp.StatusCode) {
			resp.Body.Close()
			return fmt.Errorf("retryable HTTP status: %d", resp.StatusCode)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// CircuitState returns the current state of the circuit breaker
func (c *Client) CircuitState() resilience.CircuitState {
	return c.resilient.CircuitState()
}

// CircuitStats returns the circuit breaker statistics
func (c *Client) CircuitStats() resilience.CircuitStats {
	return c.resilient.Stats()
}

// Hostname returns the Proxmox server hostname (extracted from baseURL)
func (c *Client) Hostname() string {
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return ""
	}
	return parsedURL.Hostname()
}

// maxErrorBodyLength is the maximum length for error body content in logs
const maxErrorBodyLength = 1000

// readErrorBody safely reads the response body for error messages.
// Returns a descriptive message even if reading fails.
// Note: This function returns the body as a string for inclusion in error messages.
// The caller is responsible for logging the error with appropriate context.
func readErrorBody(resp *http.Response) string {
	// Check Content-Type to determine if body is text-based
	contentType := resp.Header.Get("Content-Type")
	isTextContent := contentType == "" || // Assume text if not specified
		strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/problem+json")

	if !isTextContent {
		// Don't try to read binary content
		return fmt.Sprintf("(status %d, binary response: %s)", resp.StatusCode, contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		// Include the read error in the returned string so callers can log it
		return fmt.Sprintf("(status %d, failed to read response body: %v)", resp.StatusCode, err)
	}
	if len(body) == 0 {
		return fmt.Sprintf("(status %d, empty response body)", resp.StatusCode)
	}

	// Truncate very long responses to avoid log spam
	bodyStr := string(body)
	if len(bodyStr) > maxErrorBodyLength {
		bodyStr = bodyStr[:maxErrorBodyLength] + "... (truncated)"
	}
	return fmt.Sprintf("(status %d) %s", resp.StatusCode, bodyStr)
}

// GetNodes returns all nodes in the cluster
func (c *Client) GetNodes(ctx context.Context) ([]Node, error) {
	resp, err := c.do(ctx, http.MethodGet, "/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("getting nodes: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var nodes []Node
	if err := json.Unmarshal(apiResp.Data, &nodes); err != nil {
		return nil, fmt.Errorf("unmarshaling nodes: %w", err)
	}

	return nodes, nil
}

// GetVMs returns all VMs on a specific node
func (c *Client) GetVMs(ctx context.Context, node string) ([]VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", node)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting VMs: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var vms []VM
	if err := json.Unmarshal(apiResp.Data, &vms); err != nil {
		return nil, fmt.Errorf("unmarshaling VMs: %w", err)
	}

	for i := range vms {
		vms[i].Node = node
	}

	return vms, nil
}

// GetVM returns a specific VM
func (c *Client) GetVM(ctx context.Context, node string, vmid int) (*VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting VM: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var vm VM
	if err := json.Unmarshal(apiResp.Data, &vm); err != nil {
		return nil, fmt.Errorf("unmarshaling VM: %w", err)
	}
	vm.Node = node
	vm.VMID = vmid

	return &vm, nil
}

// CreateSnapshot creates a snapshot of a VM
func (c *Client) CreateSnapshot(ctx context.Context, node string, vmid int, name, description string, includeRAM bool) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)

	data := url.Values{}
	data.Set("snapname", name)
	if description != "" {
		data.Set("description", description)
	}
	if includeRAM {
		data.Set("vmstate", "1")
	}

	resp, err := c.do(ctx, http.MethodPost, path, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create snapshot: %s", readErrorBody(resp))
	}

	return nil
}

// GetSnapshots returns all snapshots for a VM
func (c *Client) GetSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting snapshots: %w", err)
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var snapshots []Snapshot
	if err := json.Unmarshal(apiResp.Data, &snapshots); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshots: %w", err)
	}

	return snapshots, nil
}

// RevertSnapshot reverts a VM to a specific snapshot
func (c *Client) RevertSnapshot(ctx context.Context, node string, vmid int, snapname string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/rollback", node, vmid, snapname)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("reverting snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to revert snapshot: %s", readErrorBody(resp))
	}

	return nil
}

// DeleteSnapshot deletes a VM snapshot
func (c *Client) DeleteSnapshot(ctx context.Context, node string, vmid int, snapname string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s", node, vmid, snapname)

	resp, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting snapshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete snapshot: %s", readErrorBody(resp))
	}

	return nil
}

// CloneVM creates a linked or full clone of a VM
func (c *Client) CloneVM(ctx context.Context, node string, vmid, newVMID int, name string, linked bool) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, vmid)

	data := url.Values{}
	data.Set("newid", fmt.Sprintf("%d", newVMID))
	data.Set("name", name)
	if linked {
		data.Set("full", "0")
	} else {
		data.Set("full", "1")
	}

	resp, err := c.do(ctx, http.MethodPost, path, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("cloning VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to clone VM: %s", readErrorBody(resp))
	}

	return nil
}

// StartVM starts a VM
func (c *Client) StartVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("starting VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to start VM: %s", readErrorBody(resp))
	}

	return nil
}

// StopVM stops a VM (force stop, like pulling power)
func (c *Client) StopVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("stopping VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to stop VM: %s", readErrorBody(resp))
	}

	return nil
}

// ShutdownVM gracefully shuts down a VM (sends ACPI shutdown signal)
func (c *Client) ShutdownVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("shutting down VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to shutdown VM: %s", readErrorBody(resp))
	}

	return nil
}

// RebootVM reboots a VM (sends ACPI reboot signal)
func (c *Client) RebootVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/reboot", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("rebooting VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to reboot VM: %s", readErrorBody(resp))
	}

	return nil
}

// SuspendVM suspends a running VM
func (c *Client) SuspendVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/suspend", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("suspending VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to suspend VM: %s", readErrorBody(resp))
	}

	return nil
}

// ResumeVM resumes a suspended VM
func (c *Client) ResumeVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/resume", node, vmid)

	resp, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("resuming VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to resume VM: %s", readErrorBody(resp))
	}

	return nil
}

// WaitForVMStopped waits for a VM to reach the "stopped" status.
// It polls the VM status every second until stopped or context is cancelled.
func (c *Client) WaitForVMStopped(ctx context.Context, node string, vmid int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for VM %d to stop", vmid)
		case <-ticker.C:
			vm, err := c.GetVM(ctx, node, vmid)
			if err != nil {
				// VM might already be deleted, which is fine
				return nil
			}
			if vm.Status == "stopped" {
				return nil
			}
			c.logger.Debug("Waiting for VM to stop", "vmid", vmid, "status", vm.Status)
		}
	}
}

// DeleteVM deletes a VM
func (c *Client) DeleteVM(ctx context.Context, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid)

	resp, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("deleting VM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete VM: %s", readErrorBody(resp))
	}

	return nil
}

// SpiceTicket represents the response from Proxmox SPICE proxy API
type SpiceTicket struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	TLSPort          int    `json:"tls-port"`
	Ticket           string `json:"ticket"`
	Password         string `json:"password"`
	Type             string `json:"type"`
	CA               string `json:"ca,omitempty"`
	HostSubject      string `json:"host-subject,omitempty"`
	ToggleFullscreen string `json:"toggle-fullscreen,omitempty"`
	SecureAttention  string `json:"secure-attention,omitempty"`
	DeleteThisFile   int    `json:"delete-this-file,omitempty"`
	Proxy            string `json:"proxy,omitempty"`
	ReleaseAllKeys   string `json:"release-cursor,omitempty"`
}

// GetSpiceProxy requests a SPICE proxy ticket for a VM
// This allows connecting to the VM console via SPICE protocol
func (c *Client) GetSpiceProxy(ctx context.Context, node string, vmid int) (*SpiceTicket, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/spiceproxy", node, vmid)

	// POST request to get SPICE ticket - proxy parameter is just the hostname without scheme or port
	data := url.Values{}
	// Parse the baseURL to extract just the hostname (Proxmox expects just the hostname)
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}
	data.Set("proxy", parsedURL.Hostname())

	resp, err := c.do(ctx, http.MethodPost, path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("getting SPICE proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get SPICE proxy: %s", readErrorBody(resp))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var ticket SpiceTicket
	if err := json.Unmarshal(apiResp.Data, &ticket); err != nil {
		return nil, fmt.Errorf("unmarshaling SPICE ticket: %w", err)
	}

	return &ticket, nil
}

// GetVNCProxy requests a VNC proxy ticket for a VM (alternative to SPICE)
type VNCTicket struct {
	Ticket string      `json:"ticket"`
	Port   json.Number `json:"port"` // Proxmox returns port as string
	User   string      `json:"user"`
	Cert   string      `json:"cert,omitempty"`
	UPID   string      `json:"upid,omitempty"`
}

// PortInt returns the port as an integer
func (v *VNCTicket) PortInt() int {
	port, _ := v.Port.Int64()
	return int(port)
}

func (c *Client) GetVNCProxy(ctx context.Context, node string, vmid int) (*VNCTicket, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", node, vmid)

	// POST request to get VNC ticket
	data := url.Values{}
	data.Set("websocket", "1")

	resp, err := c.do(ctx, http.MethodPost, path, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("getting VNC proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get VNC proxy: %s", readErrorBody(resp))
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var ticket VNCTicket
	if err := json.Unmarshal(apiResp.Data, &ticket); err != nil {
		return nil, fmt.Errorf("unmarshaling VNC ticket: %w", err)
	}

	return &ticket, nil
}

// NetworkInterface represents a network interface from the QEMU guest agent
type NetworkInterface struct {
	Name         string `json:"name"`
	HardwareAddr string `json:"hardware-address"`
	IPAddresses  []struct {
		IPAddress     string `json:"ip-address"`
		IPAddressType string `json:"ip-address-type"`
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses"`
}

// GetVMIP returns the primary IPv4 address for a VM using the QEMU guest agent
// This implements the checkpoint.VMIPLookup interface
func (c *Client) GetVMIP(ctx context.Context, vmPlatformID string) (string, error) {
	// Parse VMID
	vmid, err := strconv.Atoi(vmPlatformID)
	if err != nil {
		return "", fmt.Errorf("invalid VMID: %w", err)
	}

	// First find the node this VM is on
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("getting nodes: %w", err)
	}

	if c.logger != nil {
		c.logger.Debug("Looking up VM IP", "vmid", vmid, "nodes", len(nodes))
	}

	for _, node := range nodes {
		// Try to get network interfaces from this node
		path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node.Node, vmid)
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			if c.logger != nil {
				c.logger.Debug("Failed to get interfaces from node", "node", node.Node, "vmid", vmid, "error", err)
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body := readErrorBody(resp)
			resp.Body.Close()
			if c.logger != nil {
				c.logger.Debug("Non-OK response from node", "node", node.Node, "vmid", vmid, "status", resp.StatusCode, "body", body)
			}
			continue
		}

		var apiResp struct {
			Data struct {
				Result []NetworkInterface `json:"result"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			if c.logger != nil {
				c.logger.Debug("Failed to decode response", "node", node.Node, "vmid", vmid, "error", err)
			}
			continue
		}

		if c.logger != nil {
			c.logger.Debug("Got interfaces", "node", node.Node, "vmid", vmid, "count", len(apiResp.Data.Result))
		}

		// Find the first non-loopback IPv4 address
		for _, iface := range apiResp.Data.Result {
			if iface.Name == "lo" {
				continue
			}
			for _, addr := range iface.IPAddresses {
				if addr.IPAddressType == "ipv4" && addr.IPAddress != "127.0.0.1" {
					return addr.IPAddress, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no IP address found for VM %s", vmPlatformID)
}

// GuestExecResult represents the result of a guest agent command execution
type GuestExecResult struct {
	PID      int    `json:"pid"`
	Exited   bool   `json:"exited"`
	ExitCode int    `json:"exitcode"`
	OutData  string `json:"out-data"` // Base64 encoded stdout
	ErrData  string `json:"err-data"` // Base64 encoded stderr
	Signal   int    `json:"signal"`
}

// ExecuteGuestCommand executes a command on a VM using the QEMU guest agent.
// This requires the qemu-guest-agent to be installed and running in the VM.
// The command is executed synchronously and returns stdout/stderr.
func (c *Client) ExecuteGuestCommand(ctx context.Context, node string, vmid int, command string) (string, error) {
	// Step 1: Execute the command (async)
	execPath := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec", node, vmid)

	data := url.Values{}
	data.Set("command", command)

	resp, err := c.do(ctx, http.MethodPost, execPath, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("executing guest command: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to execute guest command: %s", readErrorBody(resp))
	}

	var execResp struct {
		Data struct {
			PID int `json:"pid"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return "", fmt.Errorf("decoding exec response: %w", err)
	}

	pid := execResp.Data.PID
	if pid == 0 {
		return "", fmt.Errorf("guest agent returned invalid PID")
	}

	// Step 2: Poll for result with timeout
	statusPath := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec-status?pid=%d", node, vmid, pid)

	// Create a timeout context if not already set
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}

	pollInterval := 500 * time.Millisecond
	for {
		// Check if we're close to deadline
		if time.Until(deadline) < pollInterval {
			return "", fmt.Errorf("timeout waiting for command result")
		}

		resp, err := c.do(ctx, http.MethodGet, statusPath, nil)
		if err != nil {
			return "", fmt.Errorf("getting exec status: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body := readErrorBody(resp)
			resp.Body.Close()
			return "", fmt.Errorf("failed to get exec status: %s", body)
		}

		var statusResp struct {
			Data GuestExecResult `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("decoding status response: %w", err)
		}
		resp.Body.Close()

		result := statusResp.Data
		if result.Exited {
			// Decode base64 output
			output := ""
			if result.OutData != "" {
				decoded, err := decodeBase64(result.OutData)
				if err != nil {
					c.logger.Warn("Failed to decode stdout", "error", err)
				} else {
					output = decoded
				}
			}

			// Check for errors
			if result.ExitCode != 0 {
				errOutput := ""
				if result.ErrData != "" {
					decoded, _ := decodeBase64(result.ErrData)
					errOutput = decoded
				}
				return output, fmt.Errorf("command exited with code %d: %s", result.ExitCode, errOutput)
			}

			return output, nil
		}

		// Not finished yet, wait and poll again (context-aware)
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for command result: %w", ctx.Err())
		case <-time.After(pollInterval):
		}
	}
}

// decodeBase64 decodes a base64 string, handling both standard and URL-safe encodings
func decodeBase64(s string) (string, error) {
	// Try standard encoding first
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Try URL-safe encoding
		decoded, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			// Try raw (no padding) variants
			decoded, err = base64.RawStdEncoding.DecodeString(s)
			if err != nil {
				return "", err
			}
		}
	}
	return string(decoded), nil
}
