// Package cloudstack provides a client for interacting with Apache CloudStack API
package cloudstack

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA1 is required by CloudStack API signature protocol
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/resilience"
)

// Sentinel errors for network operations
var (
	ErrNetworkNotFound = errors.New("network not found")
	ErrNetworkInUse    = errors.New("network has active VMs")
)

// Client represents a CloudStack API client
type Client struct {
	baseURL    string
	apiKey     string
	secretKey  string
	httpClient *http.Client
	resilient  *resilience.ResilientClient
	logger     *slog.Logger
}

// Config holds configuration for the CloudStack client
type Config struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
	Insecure  bool   `yaml:"insecure"`

	// Resilience settings
	MaxRetries       int           `yaml:"max_retries"`       // Maximum retry attempts (default: 3)
	RetryDelay       time.Duration `yaml:"retry_delay"`       // Initial retry delay (default: 100ms)
	CircuitThreshold int           `yaml:"circuit_threshold"` // Failures before circuit opens (default: 5)
	CircuitTimeout   time.Duration `yaml:"circuit_timeout"`   // Time before circuit half-opens (default: 30s)
}

// DefaultConfig returns a default CloudStack configuration
func DefaultConfig() Config {
	return Config{
		Host:             "localhost",
		Port:             8080,
		MaxRetries:       3,
		RetryDelay:       100 * time.Millisecond,
		CircuitThreshold: 5,
		CircuitTimeout:   30 * time.Second,
	}
}

// ClientOption configures a CloudStack client
type ClientOption func(*Client)

// WithLogger sets the logger for the client
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewClient creates a new CloudStack API client
func NewClient(cfg Config, opts ...ClientOption) (*Client, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("cloudstack host is required")
	}
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("cloudstack API credentials are required")
	}

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	baseURL := fmt.Sprintf("http://%s:%d/client/api", cfg.Host, port)

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

	client := &Client{
		baseURL:   baseURL,
		apiKey:    cfg.APIKey,
		secretKey: cfg.SecretKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: slog.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Initialize resilient client
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "cloudstack",
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold: cfg.CircuitThreshold,
			SuccessThreshold: 2,
			Timeout:          cfg.CircuitTimeout,
			OnStateChange: func(from, to resilience.CircuitState) {
				client.logger.Warn("cloudstack circuit breaker state changed",
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

// VM represents a CloudStack virtual machine
type VM struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DisplayName     string `json:"displayname"`
	State           string `json:"state"`
	ZoneID          string `json:"zoneid"`
	ZoneName        string `json:"zonename"`
	TemplateID      string `json:"templateid"`
	TemplateName    string `json:"templatename"`
	ServiceOffering string `json:"serviceofferingname"`
	CPUNumber       int    `json:"cpunumber"`
	Memory          int    `json:"memory"`
	Created         string `json:"created"`
	IPAddress       string `json:"ipaddress,omitempty"`
}

// VMSnapshot represents a CloudStack VM snapshot
type VMSnapshot struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DisplayName      string `json:"displayname"`
	Description      string `json:"description"`
	State            string `json:"state"`
	Current          bool   `json:"current"`
	Type             string `json:"type"`
	Created          string `json:"created"`
	VirtualMachineID string `json:"virtualmachineid"`
}

// Zone represents a CloudStack zone
type Zone struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NetworkType     string `json:"networktype"`
	AllocationState string `json:"allocationstate"`
}

// Template represents a CloudStack template
type Template struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DisplayText     string `json:"displaytext"`
	IsPublic        bool   `json:"ispublic"`
	IsReady         bool   `json:"isready"`
	PasswordEnabled bool   `json:"passwordenabled"`
	Format          string `json:"format"`
	Hypervisor      string `json:"hypervisor"`
}

// signRequest signs a CloudStack API request
func (c *Client) signRequest(params url.Values) string {
	params.Set("apiKey", c.apiKey)
	params.Set("response", "json")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var signParts []string
	for _, k := range keys {
		v := params.Get(k)
		signParts = append(signParts, fmt.Sprintf("%s=%s", strings.ToLower(k), url.QueryEscape(strings.ToLower(v))))
	}
	signString := strings.Join(signParts, "&")

	mac := hmac.New(sha1.New, []byte(c.secretKey))
	mac.Write([]byte(signString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	params.Set("signature", signature)

	return params.Encode()
}

// do performs a CloudStack API request with retry and circuit breaker
func (c *Client) do(ctx context.Context, command string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("command", command)

	var body []byte

	err := c.resilient.Execute(ctx, func() error {
		// Re-sign request for each attempt (in case params were modified)
		queryString := c.signRequest(params)
		reqURL := c.baseURL + "?" + queryString

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB max response
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		// Check for retryable HTTP status codes
		if resilience.IsRetryableHTTPStatus(resp.StatusCode) {
			return fmt.Errorf("retryable HTTP status: %d", resp.StatusCode)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return body, nil
}

// CircuitState returns the current state of the circuit breaker
func (c *Client) CircuitState() resilience.CircuitState {
	return c.resilient.CircuitState()
}

// CircuitStats returns the circuit breaker statistics
func (c *Client) CircuitStats() resilience.CircuitStats {
	return c.resilient.Stats()
}

// ListZones returns all zones
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	resp, err := c.do(ctx, "listZones", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListZonesResponse struct {
			Zone []Zone `json:"zone"`
		} `json:"listzonesresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling zones: %w", err)
	}

	return result.ListZonesResponse.Zone, nil
}

// ListVirtualMachines returns VMs with optional filters
func (c *Client) ListVirtualMachines(ctx context.Context, zoneID, state string) ([]VM, error) {
	params := url.Values{}
	if zoneID != "" {
		params.Set("zoneid", zoneID)
	}
	if state != "" {
		params.Set("state", state)
	}

	resp, err := c.do(ctx, "listVirtualMachines", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListVirtualMachinesResponse struct {
			VirtualMachine []VM `json:"virtualmachine"`
		} `json:"listvirtualmachinesresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling VMs: %w", err)
	}

	return result.ListVirtualMachinesResponse.VirtualMachine, nil
}

// GetVirtualMachine returns a specific VM by ID
func (c *Client) GetVirtualMachine(ctx context.Context, vmID string) (*VM, error) {
	params := url.Values{}
	params.Set("id", vmID)

	resp, err := c.do(ctx, "listVirtualMachines", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListVirtualMachinesResponse struct {
			VirtualMachine []VM `json:"virtualmachine"`
		} `json:"listvirtualmachinesresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling VM: %w", err)
	}

	if len(result.ListVirtualMachinesResponse.VirtualMachine) == 0 {
		return nil, fmt.Errorf("VM not found: %s", vmID)
	}

	return &result.ListVirtualMachinesResponse.VirtualMachine[0], nil
}

// ListVMSnapshots returns all snapshots for a VM
func (c *Client) ListVMSnapshots(ctx context.Context, vmID string) ([]VMSnapshot, error) {
	params := url.Values{}
	if vmID != "" {
		params.Set("virtualmachineid", vmID)
	}

	resp, err := c.do(ctx, "listVMSnapshot", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListVMSnapshotResponse struct {
			VMSnapshot []VMSnapshot `json:"vmSnapshot"`
		} `json:"listvmsnapshotresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling snapshots: %w", err)
	}

	return result.ListVMSnapshotResponse.VMSnapshot, nil
}

// RevertToVMSnapshot reverts a VM to a snapshot
func (c *Client) RevertToVMSnapshot(ctx context.Context, snapshotID string) error {
	params := url.Values{}
	params.Set("vmsnapshotid", snapshotID)

	_, err := c.do(ctx, "revertToVMSnapshot", params)
	return err
}

// StartVirtualMachine starts a VM
func (c *Client) StartVirtualMachine(ctx context.Context, vmID string) error {
	params := url.Values{}
	params.Set("id", vmID)

	_, err := c.do(ctx, "startVirtualMachine", params)
	return err
}

// StopVirtualMachine stops a VM
func (c *Client) StopVirtualMachine(ctx context.Context, vmID string, forced bool) error {
	params := url.Values{}
	params.Set("id", vmID)
	params.Set("forced", fmt.Sprintf("%t", forced))

	_, err := c.do(ctx, "stopVirtualMachine", params)
	return err
}

// DestroyVirtualMachine destroys a VM
func (c *Client) DestroyVirtualMachine(ctx context.Context, vmID string, expunge bool) error {
	params := url.Values{}
	params.Set("id", vmID)
	params.Set("expunge", fmt.Sprintf("%t", expunge))

	_, err := c.do(ctx, "destroyVirtualMachine", params)
	return err
}

// ServiceOffering represents a CloudStack service offering (compute profile)
type ServiceOffering struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayText string `json:"displaytext"`
	CPUNumber   int    `json:"cpunumber"`
	Memory      int    `json:"memory"`
}

// Network represents a CloudStack network
type Network struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DisplayText       string `json:"displaytext"`
	Type              string `json:"type"`
	ZoneID            string `json:"zoneid"`
	State             string `json:"state,omitempty"`
	CIDR              string `json:"cidr,omitempty"`
	Gateway           string `json:"gateway,omitempty"`
	Netmask           string `json:"netmask,omitempty"`
	NetworkOfferingID string `json:"networkofferingid,omitempty"`
	VLAN              string `json:"vlan,omitempty"`
	Account           string `json:"account,omitempty"`
	Domain            string `json:"domain,omitempty"`
}

// ListTemplates returns templates with optional filters
func (c *Client) ListTemplates(ctx context.Context, zoneID, templateFilter string) ([]Template, error) {
	params := url.Values{}
	if zoneID != "" {
		params.Set("zoneid", zoneID)
	}
	// templateFilter: featured, self, selfexecutable, sharedexecutable, executable, community, all
	if templateFilter == "" {
		templateFilter = "featured"
	}
	params.Set("templatefilter", templateFilter)

	resp, err := c.do(ctx, "listTemplates", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListTemplatesResponse struct {
			Template []Template `json:"template"`
		} `json:"listtemplatesresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling templates: %w", err)
	}

	return result.ListTemplatesResponse.Template, nil
}

// GetTemplateByName finds a template by name in a zone
func (c *Client) GetTemplateByName(ctx context.Context, zoneID, name string) (*Template, error) {
	templates, err := c.ListTemplates(ctx, zoneID, "all")
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.Name == name {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("template not found: %s", name)
}

// ListServiceOfferings returns available service offerings
func (c *Client) ListServiceOfferings(ctx context.Context) ([]ServiceOffering, error) {
	resp, err := c.do(ctx, "listServiceOfferings", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListServiceOfferingsResponse struct {
			ServiceOffering []ServiceOffering `json:"serviceoffering"`
		} `json:"listserviceofferingsresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling service offerings: %w", err)
	}

	return result.ListServiceOfferingsResponse.ServiceOffering, nil
}

// ListNetworks returns networks with optional zone filter
func (c *Client) ListNetworks(ctx context.Context, zoneID string) ([]Network, error) {
	params := url.Values{}
	if zoneID != "" {
		params.Set("zoneid", zoneID)
	}

	resp, err := c.do(ctx, "listNetworks", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListNetworksResponse struct {
			Network []Network `json:"network"`
		} `json:"listnetworksresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling networks: %w", err)
	}

	return result.ListNetworksResponse.Network, nil
}

// DeployVirtualMachineParams holds parameters for VM deployment
type DeployVirtualMachineParams struct {
	Name              string
	DisplayName       string
	ZoneID            string
	TemplateID        string
	ServiceOfferingID string
	NetworkIDs        []string // Optional: network IDs to attach
	StartVM           bool
}

// DeployVirtualMachine creates a new VM from a template
func (c *Client) DeployVirtualMachine(ctx context.Context, params DeployVirtualMachineParams) (*VM, error) {
	values := url.Values{}
	values.Set("zoneid", params.ZoneID)
	values.Set("templateid", params.TemplateID)
	values.Set("serviceofferingid", params.ServiceOfferingID)

	if params.Name != "" {
		values.Set("name", params.Name)
	}
	if params.DisplayName != "" {
		values.Set("displayname", params.DisplayName)
	}
	if len(params.NetworkIDs) > 0 {
		values.Set("networkids", strings.Join(params.NetworkIDs, ","))
	}
	values.Set("startvm", fmt.Sprintf("%t", params.StartVM))

	resp, err := c.do(ctx, "deployVirtualMachine", values)
	if err != nil {
		return nil, err
	}

	// deployVirtualMachine returns an async job
	var result struct {
		DeployVirtualMachineResponse struct {
			ID    string `json:"id"`
			JobID string `json:"jobid"`
		} `json:"deployvirtualmachineresponse"`
	}

	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling deploy response: %w", unmarshalErr)
	}

	// Wait for the async job to complete
	vm, err := c.waitForVMJob(ctx, result.DeployVirtualMachineResponse.JobID)
	if err != nil {
		return nil, fmt.Errorf("waiting for VM deployment: %w", err)
	}

	return vm, nil
}

// waitForVMJob waits for an async job to complete and returns the VM
func (c *Client) waitForVMJob(ctx context.Context, jobID string) (*VM, error) {
	for {
		params := url.Values{}
		params.Set("jobid", jobID)

		resp, err := c.do(ctx, "queryAsyncJobResult", params)
		if err != nil {
			return nil, err
		}

		var result struct {
			QueryAsyncJobResultResponse struct {
				JobStatus     int             `json:"jobstatus"`
				JobResultCode int             `json:"jobresultcode"`
				JobResult     json.RawMessage `json:"jobresult"`
			} `json:"queryasyncjobresultresponse"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("unmarshaling job result: %w", err)
		}

		switch result.QueryAsyncJobResultResponse.JobStatus {
		case 0: // In progress
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		case 1: // Success
			var vmResult struct {
				VirtualMachine VM `json:"virtualmachine"`
			}
			if err := json.Unmarshal(result.QueryAsyncJobResultResponse.JobResult, &vmResult); err != nil {
				return nil, fmt.Errorf("unmarshaling VM result: %w", err)
			}
			return &vmResult.VirtualMachine, nil
		case 2: // Error
			return nil, fmt.Errorf("async job failed with code %d", result.QueryAsyncJobResultResponse.JobResultCode)
		}
	}
}

// CreateVMSnapshot creates a snapshot of a VM
func (c *Client) CreateVMSnapshot(ctx context.Context, vmID, name, description string, snapshotMemory bool) (*VMSnapshot, error) {
	params := url.Values{}
	params.Set("virtualmachineid", vmID)
	if name != "" {
		params.Set("name", name)
	}
	if description != "" {
		params.Set("description", description)
	}
	params.Set("snapshotmemory", fmt.Sprintf("%t", snapshotMemory))

	resp, err := c.do(ctx, "createVMSnapshot", params)
	if err != nil {
		return nil, err
	}

	// createVMSnapshot returns an async job
	var result struct {
		CreateVMSnapshotResponse struct {
			ID    string `json:"id"`
			JobID string `json:"jobid"`
		} `json:"createvmsnapshotresponse"`
	}

	if unmarshalErr := json.Unmarshal(resp, &result); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling snapshot response: %w", unmarshalErr)
	}

	// Wait for the async job to complete
	snapshot, err := c.waitForSnapshotJob(ctx, result.CreateVMSnapshotResponse.JobID)
	if err != nil {
		return nil, fmt.Errorf("waiting for snapshot creation: %w", err)
	}

	return snapshot, nil
}

// waitForSnapshotJob waits for a snapshot async job to complete
func (c *Client) waitForSnapshotJob(ctx context.Context, jobID string) (*VMSnapshot, error) {
	for {
		params := url.Values{}
		params.Set("jobid", jobID)

		resp, err := c.do(ctx, "queryAsyncJobResult", params)
		if err != nil {
			return nil, err
		}

		var result struct {
			QueryAsyncJobResultResponse struct {
				JobStatus     int             `json:"jobstatus"`
				JobResultCode int             `json:"jobresultcode"`
				JobResult     json.RawMessage `json:"jobresult"`
			} `json:"queryasyncjobresultresponse"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			return nil, fmt.Errorf("unmarshaling job result: %w", err)
		}

		switch result.QueryAsyncJobResultResponse.JobStatus {
		case 0: // In progress
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		case 1: // Success
			var snapResult struct {
				VMSnapshot VMSnapshot `json:"vmsnapshot"`
			}
			if err := json.Unmarshal(result.QueryAsyncJobResultResponse.JobResult, &snapResult); err != nil {
				return nil, fmt.Errorf("unmarshaling snapshot result: %w", err)
			}
			return &snapResult.VMSnapshot, nil
		case 2: // Error
			return nil, fmt.Errorf("async job failed with code %d", result.QueryAsyncJobResultResponse.JobResultCode)
		}
	}
}

// DeleteVMSnapshot deletes a VM snapshot
func (c *Client) DeleteVMSnapshot(ctx context.Context, snapshotID string) error {
	params := url.Values{}
	params.Set("vmsnapshotid", snapshotID)

	_, err := c.do(ctx, "deleteVMSnapshot", params)
	return err
}

// waitForAsyncJob waits for an async job to complete and unmarshals the result
func (c *Client) waitForAsyncJob(ctx context.Context, jobID, resultKey string, result any) error {
	for {
		params := url.Values{}
		params.Set("jobid", jobID)

		resp, err := c.do(ctx, "queryAsyncJobResult", params)
		if err != nil {
			return err
		}

		var jobResult struct {
			QueryAsyncJobResultResponse struct {
				JobStatus     int             `json:"jobstatus"`
				JobResultCode int             `json:"jobresultcode"`
				JobResult     json.RawMessage `json:"jobresult"`
			} `json:"queryasyncjobresultresponse"`
		}

		if err := json.Unmarshal(resp, &jobResult); err != nil {
			return fmt.Errorf("unmarshaling job result: %w", err)
		}

		switch jobResult.QueryAsyncJobResultResponse.JobStatus {
		case 0: // In progress
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		case 1: // Success
			if result == nil {
				return nil
			}
			// Wrap result in the expected key
			wrapper := make(map[string]json.RawMessage)
			if err := json.Unmarshal(jobResult.QueryAsyncJobResultResponse.JobResult, &wrapper); err != nil {
				return fmt.Errorf("unmarshaling job result wrapper: %w", err)
			}
			if data, ok := wrapper[resultKey]; ok {
				if err := json.Unmarshal(data, result); err != nil {
					return fmt.Errorf("unmarshaling %s result: %w", resultKey, err)
				}
			}
			return nil
		case 2: // Error
			return fmt.Errorf("async job failed with code %d", jobResult.QueryAsyncJobResultResponse.JobResultCode)
		}
	}
}

// withRetry executes an operation with exponential backoff retry
func (c *Client) withRetry(ctx context.Context, operation func() error, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := operation(); err != nil {
			lastErr = err
			// Don't retry on context cancellation
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// Calculate backoff: 100ms, 200ms, 400ms, 800ms, 1600ms...
			backoff := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		return nil
	}
	return lastErr
}

// CreateNetworkParams holds parameters for network creation
type CreateNetworkParams struct {
	Name              string
	DisplayText       string
	NetworkOfferingID string
	ZoneID            string
	Gateway           string // Optional
	Netmask           string // Optional
	StartIP           string // Optional
	EndIP             string // Optional
}

// Validate validates the CreateNetworkParams
func (p *CreateNetworkParams) Validate() error {
	if p.Name == "" {
		return errors.New("network name is required")
	}
	if p.NetworkOfferingID == "" {
		return errors.New("network offering ID is required")
	}
	if p.ZoneID == "" {
		return errors.New("zone ID is required")
	}
	return nil
}

// CreateNetwork creates an isolated network in CloudStack
func (c *Client) CreateNetwork(ctx context.Context, params CreateNetworkParams) (*Network, error) {
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	values := url.Values{}
	values.Set("name", params.Name)
	values.Set("networkofferingid", params.NetworkOfferingID)
	values.Set("zoneid", params.ZoneID)

	if params.DisplayText != "" {
		values.Set("displaytext", params.DisplayText)
	} else {
		values.Set("displaytext", params.Name)
	}
	if params.Gateway != "" {
		values.Set("gateway", params.Gateway)
	}
	if params.Netmask != "" {
		values.Set("netmask", params.Netmask)
	}
	if params.StartIP != "" {
		values.Set("startip", params.StartIP)
	}
	if params.EndIP != "" {
		values.Set("endip", params.EndIP)
	}

	resp, err := c.do(ctx, "createNetwork", values)
	if err != nil {
		return nil, err
	}

	var result struct {
		CreateNetworkResponse struct {
			Network Network `json:"network"`
		} `json:"createnetworkresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling create network response: %w", err)
	}

	return &result.CreateNetworkResponse.Network, nil
}

// GetNetwork retrieves a network by ID
func (c *Client) GetNetwork(ctx context.Context, networkID string) (*Network, error) {
	params := url.Values{}
	params.Set("id", networkID)

	resp, err := c.do(ctx, "listNetworks", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		ListNetworksResponse struct {
			Network []Network `json:"network"`
		} `json:"listnetworksresponse"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling network: %w", err)
	}

	if len(result.ListNetworksResponse.Network) == 0 {
		return nil, ErrNetworkNotFound
	}

	return &result.ListNetworksResponse.Network[0], nil
}

// DeleteNetwork deletes a network by ID with retry logic
func (c *Client) DeleteNetwork(ctx context.Context, networkID string) error {
	return c.withRetry(ctx, func() error {
		params := url.Values{}
		params.Set("id", networkID)
		params.Set("forced", "true")

		resp, err := c.do(ctx, "deleteNetwork", params)
		if err != nil {
			// Check if network doesn't exist (already deleted)
			if strings.Contains(err.Error(), "Unable to find network") {
				return nil
			}
			// Check if network is in use
			if strings.Contains(err.Error(), "in use") {
				return ErrNetworkInUse
			}
			return err
		}

		// deleteNetwork returns an async job
		var result struct {
			DeleteNetworkResponse struct {
				JobID string `json:"jobid"`
			} `json:"deletenetworkresponse"`
		}

		if err := json.Unmarshal(resp, &result); err != nil {
			return fmt.Errorf("unmarshaling delete network response: %w", err)
		}

		// Wait for the async job to complete
		return c.waitForAsyncJob(ctx, result.DeleteNetworkResponse.JobID, "", nil)
	}, 5)
}
