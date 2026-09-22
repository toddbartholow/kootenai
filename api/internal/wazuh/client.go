// Package wazuh provides integration with Wazuh SIEM for the Kootenai platform
package wazuh

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/resilience"
)

// APIClient provides methods to interact with the Wazuh Manager REST API
type APIClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	resilient  *resilience.ResilientClient
	logger     *slog.Logger

	// JWT token management
	token    string
	tokenMu  sync.RWMutex
	tokenExp time.Time
}

// APIClientConfig holds configuration for the Wazuh API client
type APIClientConfig struct {
	// ManagerURL is the base URL for the Wazuh Manager API (e.g., "https://wazuh.example.com:55000")
	ManagerURL string

	// Username for API authentication
	Username string

	// Password for API authentication
	Password string

	// InsecureSkipVerify disables TLS certificate verification (not recommended for production)
	InsecureSkipVerify bool

	// Timeout for HTTP requests
	Timeout time.Duration

	// CircuitThreshold is the number of failures before the circuit breaker opens
	CircuitThreshold int

	// CircuitTimeout is how long the circuit stays open before trying again
	CircuitTimeout time.Duration

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// RetryDelay is the initial delay between retries
	RetryDelay time.Duration

	// Logger for logging (optional)
	Logger *slog.Logger
}

// NewAPIClient creates a new Wazuh API client
func NewAPIClient(cfg APIClientConfig) *APIClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.CircuitThreshold == 0 {
		cfg.CircuitThreshold = 5
	}
	if cfg.CircuitTimeout == 0 {
		cfg.CircuitTimeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 100 * time.Millisecond
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// #nosec G402 -- InsecureSkipVerify is configurable via APIClientConfig.InsecureSkipVerify
	// for internal/lab network environments using self-signed certificates.
	// Production deployments should use proper certificates and set InsecureSkipVerify=false.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	client := &APIClient{
		baseURL:  cfg.ManagerURL,
		username: cfg.Username,
		password: cfg.Password,
		logger:   cfg.Logger,
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
	}

	// Initialize resilient client with circuit breaker and retry logic
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "wazuh",
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold: cfg.CircuitThreshold,
			SuccessThreshold: 2,
			Timeout:          cfg.CircuitTimeout,
			OnStateChange: func(from, to resilience.CircuitState) {
				client.logger.Warn("wazuh circuit breaker state changed",
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
		Logger: cfg.Logger,
	})

	return client
}

// apiResponse is the standard Wazuh API response wrapper
type apiResponse struct {
	Data   json.RawMessage `json:"data"`
	Error  int             `json:"error"`
	Detail string          `json:"detail,omitempty"`
}

// apiDataWrapper wraps the data field which contains affected_items
type apiDataWrapper struct {
	AffectedItems []json.RawMessage `json:"affected_items"`
	TotalItems    int               `json:"total_affected_items"`
	FailedItems   []json.RawMessage `json:"failed_items,omitempty"`
}

// authenticate obtains a JWT token from the Wazuh API
func (c *APIClient) authenticate(ctx context.Context) error {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Check if token is still valid (with 1 minute buffer)
	if c.token != "" && time.Now().Add(time.Minute).Before(c.tokenExp) {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/security/user/authenticate?raw=true", nil)
	if err != nil {
		return fmt.Errorf("creating auth request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading auth response: %w", err)
	}

	c.token = string(token)
	// Wazuh tokens typically expire in 15 minutes
	c.tokenExp = time.Now().Add(14 * time.Minute)

	return nil
}

// doRequest performs an authenticated API request with circuit breaker and retry
func (c *APIClient) doRequest(ctx context.Context, method, path string, body any) (*apiResponse, error) {
	var apiResp *apiResponse

	err := c.resilient.Execute(ctx, func() error {
		// Ensure we have a valid token
		if err := c.authenticate(ctx); err != nil {
			return fmt.Errorf("authenticating: %w", err)
		}

		var reqBody io.Reader
		if body != nil {
			jsonBody, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}
			reqBody = bytes.NewReader(jsonBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		c.tokenMu.RLock()
		req.Header.Set("Authorization", "Bearer "+c.token)
		c.tokenMu.RUnlock()

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		// Handle non-2xx status codes
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
		}

		var parsedResp apiResponse
		if err := json.Unmarshal(respBody, &parsedResp); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}

		apiResp = &parsedResp
		return nil
	})

	if err != nil {
		return nil, err
	}

	if apiResp.Error != 0 {
		return nil, fmt.Errorf("API error %d: %s", apiResp.Error, apiResp.Detail)
	}

	return apiResp, nil
}

// RegisterAgentRequest represents the request body for agent registration
type RegisterAgentRequest struct {
	Name string `json:"name"`
	IP   string `json:"ip,omitempty"`
}

// RegisterAgentResponse represents the response from agent registration
type RegisterAgentResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// RegisterAgent registers a new agent with the Wazuh Manager
func (c *APIClient) RegisterAgent(ctx context.Context, name, ip string) (*RegisterAgentResponse, error) {
	reqBody := RegisterAgentRequest{
		Name: name,
		IP:   ip,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/agents", reqBody)
	if err != nil {
		return nil, fmt.Errorf("registering agent: %w", err)
	}

	var data apiDataWrapper
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing registration response: %w", err)
	}

	if len(data.AffectedItems) == 0 {
		return nil, fmt.Errorf("no agent data in response")
	}

	var agent RegisterAgentResponse
	if err := json.Unmarshal(data.AffectedItems[0], &agent); err != nil {
		return nil, fmt.Errorf("parsing agent data: %w", err)
	}

	return &agent, nil
}

// GetAgentKey retrieves the key for an existing agent
func (c *APIClient) GetAgentKey(ctx context.Context, agentID string) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/agents/"+agentID+"/key", nil)
	if err != nil {
		return "", fmt.Errorf("getting agent key: %w", err)
	}

	var data apiDataWrapper
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return "", fmt.Errorf("parsing key response: %w", err)
	}

	if len(data.AffectedItems) == 0 {
		return "", fmt.Errorf("no key data in response")
	}

	var keyData struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data.AffectedItems[0], &keyData); err != nil {
		return "", fmt.Errorf("parsing key data: %w", err)
	}

	return keyData.Key, nil
}

// APIAgentInfo represents agent information from the API
type APIAgentInfo struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IP            string  `json:"ip"`
	Status        string  `json:"status"`
	Group         string  `json:"group,omitempty"`
	Version       string  `json:"version,omitempty"`
	OS            AgentOS `json:"os,omitempty"`
	DateAdd       string  `json:"dateAdd,omitempty"`
	LastKeepAlive string  `json:"lastKeepAlive,omitempty"`
	NodeName      string  `json:"node_name,omitempty"`
}

// GetAgent retrieves information about an agent
func (c *APIClient) GetAgent(ctx context.Context, agentID string) (*APIAgentInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/agents?agents_list="+agentID, nil)
	if err != nil {
		return nil, fmt.Errorf("getting agent: %w", err)
	}

	var data apiDataWrapper
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing agent response: %w", err)
	}

	if len(data.AffectedItems) == 0 {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	var agent APIAgentInfo
	if err := json.Unmarshal(data.AffectedItems[0], &agent); err != nil {
		return nil, fmt.Errorf("parsing agent data: %w", err)
	}

	return &agent, nil
}

// GetAgentByName retrieves agent information by name
func (c *APIClient) GetAgentByName(ctx context.Context, name string) (*APIAgentInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/agents?name="+name, nil)
	if err != nil {
		return nil, fmt.Errorf("getting agent by name: %w", err)
	}

	var data apiDataWrapper
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing agent response: %w", err)
	}

	if len(data.AffectedItems) == 0 {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	var agent APIAgentInfo
	if err := json.Unmarshal(data.AffectedItems[0], &agent); err != nil {
		return nil, fmt.Errorf("parsing agent data: %w", err)
	}

	return &agent, nil
}

// DeleteAgent removes an agent from the Wazuh Manager
func (c *APIClient) DeleteAgent(ctx context.Context, agentID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/agents?agents_list="+agentID+"&status=all&older_than=0s", nil)
	if err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	return nil
}

// AddAgentToGroup adds an agent to a group
func (c *APIClient) AddAgentToGroup(ctx context.Context, agentID, groupID string) error {
	_, err := c.doRequest(ctx, http.MethodPut, "/agents/"+agentID+"/group/"+groupID, nil)
	if err != nil {
		return fmt.Errorf("adding agent to group: %w", err)
	}
	return nil
}

// CreateGroup creates a new agent group
func (c *APIClient) CreateGroup(ctx context.Context, groupID string) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/groups", map[string]string{"group_id": groupID})
	if err != nil {
		return fmt.Errorf("creating group: %w", err)
	}
	return nil
}

// DeleteGroup removes an agent group
func (c *APIClient) DeleteGroup(ctx context.Context, groupID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/groups?groups_list="+groupID, nil)
	if err != nil {
		return fmt.Errorf("deleting group: %w", err)
	}
	return nil
}

// RestartAgent requests an agent restart
func (c *APIClient) RestartAgent(ctx context.Context, agentID string) error {
	_, err := c.doRequest(ctx, http.MethodPut, "/agents/"+agentID+"/restart", nil)
	if err != nil {
		return fmt.Errorf("restarting agent: %w", err)
	}
	return nil
}

// ManagerInfo represents Wazuh Manager information
type ManagerInfo struct {
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Type     string `json:"type"`
	NodeName string `json:"node_name"`
	NodeType string `json:"node_type"`
}

// GetManagerInfo retrieves information about the Wazuh Manager
func (c *APIClient) GetManagerInfo(ctx context.Context) (*ManagerInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/manager/info", nil)
	if err != nil {
		return nil, fmt.Errorf("getting manager info: %w", err)
	}

	var data apiDataWrapper
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("parsing manager response: %w", err)
	}

	if len(data.AffectedItems) == 0 {
		return nil, fmt.Errorf("no manager data in response")
	}

	var info ManagerInfo
	if err := json.Unmarshal(data.AffectedItems[0], &info); err != nil {
		return nil, fmt.Errorf("parsing manager data: %w", err)
	}

	return &info, nil
}

// Ping checks if the Wazuh Manager API is reachable and authentication works
func (c *APIClient) Ping(ctx context.Context) error {
	_, err := c.GetManagerInfo(ctx)
	return err
}

// CircuitState returns the current state of the circuit breaker
func (c *APIClient) CircuitState() resilience.CircuitState {
	return c.resilient.CircuitState()
}

// CircuitStats returns circuit breaker statistics
func (c *APIClient) CircuitStats() resilience.CircuitStats {
	return c.resilient.Stats()
}
