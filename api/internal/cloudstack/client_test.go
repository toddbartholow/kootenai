package cloudstack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/resilience"
)

// newTestClient creates a properly initialized test client for the given test server
func newTestClient(serverURL string) *Client {
	logger := slog.Default()
	client := &Client{
		baseURL:    serverURL,
		apiKey:     "test-key",
		secretKey:  "test-secret",
		httpClient: http.DefaultClient,
		logger:     logger,
	}
	// Initialize resilient client with minimal retry for tests
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "cloudstack-test",
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		},
		Retry: resilience.RetryConfig{
			MaxAttempts:     1, // No retries in tests for faster execution
			InitialDelay:    10 * time.Millisecond,
			MaxDelay:        100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: resilience.IsRetryableError,
		},
		Logger: logger,
	})
	return client
}

// mockCloudStackServer creates a test server that simulates CloudStack API responses
func mockCloudStackServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse command from query
		query := r.URL.Query()
		command := query.Get("command")

		// Verify signature is present
		if query.Get("signature") == "" {
			http.Error(w, "missing signature", http.StatusUnauthorized)
			return
		}

		switch command {
		case "listZones":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"listzonesresponse": map[string]interface{}{
					"zone": []Zone{
						{ID: "zone-1", Name: "zone1", NetworkType: "Advanced", AllocationState: "Enabled"},
						{ID: "zone-2", Name: "zone2", NetworkType: "Basic", AllocationState: "Enabled"},
					},
				},
			})

		case "listVirtualMachines":
			vmID := query.Get("id")
			if vmID != "" {
				// Single VM lookup
				if vmID == "not-found" {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"listvirtualmachinesresponse": map[string]interface{}{},
					})
				} else {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"listvirtualmachinesresponse": map[string]interface{}{
							"virtualmachine": []VM{
								{ID: vmID, Name: "test-vm", State: "Running", ZoneID: "zone-1", CPUNumber: 2, Memory: 2048},
							},
						},
					})
				}
			} else {
				// List all VMs
				json.NewEncoder(w).Encode(map[string]interface{}{
					"listvirtualmachinesresponse": map[string]interface{}{
						"virtualmachine": []VM{
							{ID: "vm-1", Name: "vm1", State: "Running", ZoneID: "zone-1"},
							{ID: "vm-2", Name: "vm2", State: "Stopped", ZoneID: "zone-1"},
						},
					},
				})
			}

		case "listVMSnapshot":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"listvmsnapshotresponse": map[string]interface{}{
					"vmSnapshot": []VMSnapshot{
						{ID: "snap-1", Name: "baseline", State: "Ready", Current: true, VirtualMachineID: "vm-1"},
						{ID: "snap-2", Name: "checkpoint1", State: "Ready", Current: false, VirtualMachineID: "vm-1"},
					},
				},
			})

		case "listTemplates":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"listtemplatesresponse": map[string]interface{}{
					"template": []Template{
						{ID: "tmpl-1", Name: "Ubuntu 22.04", IsReady: true, Format: "QCOW2"},
						{ID: "tmpl-2", Name: "CentOS 8", IsReady: true, Format: "QCOW2"},
					},
				},
			})

		case "listServiceOfferings":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"listserviceofferingsresponse": map[string]interface{}{
					"serviceoffering": []ServiceOffering{
						{ID: "so-1", Name: "Small", CPUNumber: 1, Memory: 1024},
						{ID: "so-2", Name: "Medium", CPUNumber: 2, Memory: 2048},
					},
				},
			})

		case "listNetworks":
			networkID := query.Get("id")
			if networkID == "not-found" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"listnetworksresponse": map[string]interface{}{},
				})
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"listnetworksresponse": map[string]interface{}{
						"network": []Network{
							{ID: "net-1", Name: "default", Type: "Isolated", ZoneID: "zone-1", State: "Allocated"},
						},
					},
				})
			}

		case "createNetwork":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"createnetworkresponse": map[string]interface{}{
					"network": Network{
						ID:      "net-new",
						Name:    query.Get("name"),
						Type:    "Isolated",
						ZoneID:  query.Get("zoneid"),
						State:   "Allocated",
						Gateway: query.Get("gateway"),
						Netmask: query.Get("netmask"),
					},
				},
			})

		case "deleteNetwork":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"deletenetworkresponse": map[string]interface{}{
					"jobid": "job-delete-net",
				},
			})

		case "deployVirtualMachine":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"deployvirtualmachineresponse": map[string]interface{}{
					"id":    "vm-new",
					"jobid": "job-deploy",
				},
			})

		case "createVMSnapshot":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"createvmsnapshotresponse": map[string]interface{}{
					"id":    "snap-new",
					"jobid": "job-snapshot",
				},
			})

		case "revertToVMSnapshot", "deleteVMSnapshot":
			json.NewEncoder(w).Encode(map[string]interface{}{
				command + "response": map[string]interface{}{
					"success": true,
				},
			})

		case "startVirtualMachine", "stopVirtualMachine", "destroyVirtualMachine":
			json.NewEncoder(w).Encode(map[string]interface{}{
				command + "response": map[string]interface{}{
					"jobid": "job-" + command,
				},
			})

		case "queryAsyncJobResult":
			jobID := query.Get("jobid")
			switch jobID {
			case "job-deploy":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus": 1,
						"jobresult": map[string]interface{}{
							"virtualmachine": VM{ID: "vm-new", Name: "deployed-vm", State: "Running"},
						},
					},
				})
			case "job-snapshot":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus": 1,
						"jobresult": map[string]interface{}{
							"vmsnapshot": VMSnapshot{ID: "snap-new", Name: "new-snapshot", State: "Ready"},
						},
					},
				})
			case "job-delete-net":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus": 1,
						"jobresult": map[string]interface{}{
							"success": true,
						},
					},
				})
			case "job-failed":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus":     2,
						"jobresultcode": 530,
					},
				})
			case "job-pending":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus": 0,
					},
				})
			default:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"queryasyncjobresultresponse": map[string]interface{}{
						"jobstatus": 1,
						"jobresult": map[string]interface{}{
							"success": true,
						},
					},
				})
			}

		default:
			http.Error(w, "unknown command: "+command, http.StatusBadRequest)
		}
	}))
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Host:      "cloudstack.example.com",
				Port:      8080,
				APIKey:    "api-key",
				SecretKey: "secret-key",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: Config{
				APIKey:    "api-key",
				SecretKey: "secret-key",
			},
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name: "missing API key",
			cfg: Config{
				Host:      "cloudstack.example.com",
				SecretKey: "secret-key",
			},
			wantErr: true,
			errMsg:  "API credentials are required",
		},
		{
			name: "missing secret key",
			cfg: Config{
				Host:   "cloudstack.example.com",
				APIKey: "api-key",
			},
			wantErr: true,
			errMsg:  "API credentials are required",
		},
		{
			name: "default port",
			cfg: Config{
				Host:      "cloudstack.example.com",
				APIKey:    "api-key",
				SecretKey: "secret-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
			}
		})
	}
}

func TestClient_SignRequest(t *testing.T) {
	client := &Client{
		apiKey:    "test-api-key",
		secretKey: "test-secret-key",
	}

	params := url.Values{}
	params.Set("command", "listZones")

	signed := client.signRequest(params)

	// Verify required params are added
	parsedParams, err := url.ParseQuery(signed)
	if err != nil {
		t.Fatalf("failed to parse signed query: %v", err)
	}

	if parsedParams.Get("apiKey") != "test-api-key" {
		t.Error("apiKey not set in signed request")
	}
	if parsedParams.Get("response") != "json" {
		t.Error("response format not set in signed request")
	}
	if parsedParams.Get("signature") == "" {
		t.Error("signature not set in signed request")
	}
}

func TestClient_ListZones(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	zones, err := client.ListZones(ctx)
	if err != nil {
		t.Fatalf("ListZones failed: %v", err)
	}

	if len(zones) != 2 {
		t.Errorf("expected 2 zones, got %d", len(zones))
	}

	if zones[0].ID != "zone-1" {
		t.Errorf("expected zone ID zone-1, got %s", zones[0].ID)
	}
}

func TestClient_ListVirtualMachines(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	vms, err := client.ListVirtualMachines(ctx, "zone-1", "")
	if err != nil {
		t.Fatalf("ListVirtualMachines failed: %v", err)
	}

	if len(vms) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(vms))
	}
}

func TestClient_GetVirtualMachine(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()

	// Test existing VM
	vm, err := client.GetVirtualMachine(ctx, "vm-1")
	if err != nil {
		t.Fatalf("GetVirtualMachine failed: %v", err)
	}
	if vm.ID != "vm-1" {
		t.Errorf("expected VM ID vm-1, got %s", vm.ID)
	}

	// Test non-existing VM
	_, err = client.GetVirtualMachine(ctx, "not-found")
	if err == nil {
		t.Error("expected error for non-existing VM")
	}
}

func TestClient_ListVMSnapshots(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	snapshots, err := client.ListVMSnapshots(ctx, "vm-1")
	if err != nil {
		t.Fatalf("ListVMSnapshots failed: %v", err)
	}

	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}

	// Find current snapshot
	var current *VMSnapshot
	for i := range snapshots {
		if snapshots[i].Current {
			current = &snapshots[i]
			break
		}
	}
	if current == nil {
		t.Error("expected to find current snapshot")
	}
}

func TestClient_ListTemplates(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	templates, err := client.ListTemplates(ctx, "zone-1", "featured")
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	if len(templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(templates))
	}
}

func TestClient_GetTemplateByName(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()

	// Test existing template
	tmpl, err := client.GetTemplateByName(ctx, "zone-1", "Ubuntu 22.04")
	if err != nil {
		t.Fatalf("GetTemplateByName failed: %v", err)
	}
	if tmpl.ID != "tmpl-1" {
		t.Errorf("expected template ID tmpl-1, got %s", tmpl.ID)
	}

	// Test non-existing template
	_, err = client.GetTemplateByName(ctx, "zone-1", "NonExistent")
	if err == nil {
		t.Error("expected error for non-existing template")
	}
}

func TestClient_ListServiceOfferings(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	offerings, err := client.ListServiceOfferings(ctx)
	if err != nil {
		t.Fatalf("ListServiceOfferings failed: %v", err)
	}

	if len(offerings) != 2 {
		t.Errorf("expected 2 offerings, got %d", len(offerings))
	}
}

func TestClient_ListNetworks(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	networks, err := client.ListNetworks(ctx, "zone-1")
	if err != nil {
		t.Fatalf("ListNetworks failed: %v", err)
	}

	if len(networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(networks))
	}
}

func TestClient_GetNetwork(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()

	// Test existing network
	network, err := client.GetNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("GetNetwork failed: %v", err)
	}
	if network.ID != "net-1" {
		t.Errorf("expected network ID net-1, got %s", network.ID)
	}

	// Test non-existing network
	_, err = client.GetNetwork(ctx, "not-found")
	if err != ErrNetworkNotFound {
		t.Errorf("expected ErrNetworkNotFound, got %v", err)
	}
}

func TestClient_CreateNetwork(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	network, err := client.CreateNetwork(ctx, CreateNetworkParams{
		Name:              "test-network",
		NetworkOfferingID: "offering-1",
		ZoneID:            "zone-1",
		Gateway:           "10.0.0.1",
		Netmask:           "255.255.255.0",
	})
	if err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}

	if network.ID != "net-new" {
		t.Errorf("expected network ID net-new, got %s", network.ID)
	}
	if network.Name != "test-network" {
		t.Errorf("expected network name test-network, got %s", network.Name)
	}
}

func TestClient_CreateNetwork_Validation(t *testing.T) {
	client := newTestClient("http://localhost")

	ctx := context.Background()

	// Missing name
	_, err := client.CreateNetwork(ctx, CreateNetworkParams{
		NetworkOfferingID: "offering-1",
		ZoneID:            "zone-1",
	})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Error("expected validation error for missing name")
	}

	// Missing network offering
	_, err = client.CreateNetwork(ctx, CreateNetworkParams{
		Name:   "test",
		ZoneID: "zone-1",
	})
	if err == nil || !strings.Contains(err.Error(), "network offering ID is required") {
		t.Error("expected validation error for missing network offering")
	}

	// Missing zone
	_, err = client.CreateNetwork(ctx, CreateNetworkParams{
		Name:              "test",
		NetworkOfferingID: "offering-1",
	})
	if err == nil || !strings.Contains(err.Error(), "zone ID is required") {
		t.Error("expected validation error for missing zone")
	}
}

func TestClient_DeleteNetwork(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.DeleteNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("DeleteNetwork failed: %v", err)
	}
}

func TestClient_DeployVirtualMachine(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	vm, err := client.DeployVirtualMachine(ctx, DeployVirtualMachineParams{
		Name:              "new-vm",
		ZoneID:            "zone-1",
		TemplateID:        "tmpl-1",
		ServiceOfferingID: "so-1",
		StartVM:           true,
	})
	if err != nil {
		t.Fatalf("DeployVirtualMachine failed: %v", err)
	}

	if vm.ID != "vm-new" {
		t.Errorf("expected VM ID vm-new, got %s", vm.ID)
	}
}

func TestClient_CreateVMSnapshot(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	snapshot, err := client.CreateVMSnapshot(ctx, "vm-1", "test-snapshot", "Test description", false)
	if err != nil {
		t.Fatalf("CreateVMSnapshot failed: %v", err)
	}

	if snapshot.ID != "snap-new" {
		t.Errorf("expected snapshot ID snap-new, got %s", snapshot.ID)
	}
}

func TestClient_RevertToVMSnapshot(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.RevertToVMSnapshot(ctx, "snap-1")
	if err != nil {
		t.Fatalf("RevertToVMSnapshot failed: %v", err)
	}
}

func TestClient_DeleteVMSnapshot(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.DeleteVMSnapshot(ctx, "snap-1")
	if err != nil {
		t.Fatalf("DeleteVMSnapshot failed: %v", err)
	}
}

func TestClient_VMOperations(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()

	// Start VM
	err := client.StartVirtualMachine(ctx, "vm-1")
	if err != nil {
		t.Fatalf("StartVirtualMachine failed: %v", err)
	}

	// Stop VM
	err = client.StopVirtualMachine(ctx, "vm-1", false)
	if err != nil {
		t.Fatalf("StopVirtualMachine failed: %v", err)
	}

	// Destroy VM
	err = client.DestroyVirtualMachine(ctx, "vm-1", true)
	if err != nil {
		t.Fatalf("DestroyVirtualMachine failed: %v", err)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	server := mockCloudStackServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.ListZones(ctx)
	if err == nil {
		t.Error("expected error due to canceled context")
	}
}

func TestClient_WithRetry(t *testing.T) {
	attempts := 0
	client := &Client{}

	ctx := context.Background()
	err := client.withRetry(ctx, func() error {
		attempts++
		if attempts < 3 {
			return http.ErrServerClosed // Simulated error
		}
		return nil
	}, 5)

	if err != nil {
		t.Fatalf("withRetry failed: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_WithRetry_MaxRetries(t *testing.T) {
	attempts := 0
	client := &Client{}

	ctx := context.Background()
	err := client.withRetry(ctx, func() error {
		attempts++
		return http.ErrServerClosed // Always fail
	}, 3)

	if err == nil {
		t.Error("expected error after max retries")
	}
	if attempts != 4 { // 1 initial + 3 retries
		t.Errorf("expected 4 attempts, got %d", attempts)
	}
}

func TestClient_WithRetry_ContextCancelled(t *testing.T) {
	client := &Client{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.withRetry(ctx, func() error {
		return context.Canceled
	}, 5)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestClient_AsyncJobPolling_Timeout(t *testing.T) {
	// Create a server that always returns pending
	pendingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"queryasyncjobresultresponse": map[string]interface{}{
				"jobstatus": 0, // Always pending
			},
		})
	}))
	defer pendingServer.Close()

	client := newTestClient(pendingServer.URL)

	// Use a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.waitForAsyncJob(ctx, "job-1", "", nil)
	if err != context.DeadlineExceeded {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are defined correctly
	if ErrNetworkNotFound == nil {
		t.Error("ErrNetworkNotFound should not be nil")
	}
	if ErrNetworkInUse == nil {
		t.Error("ErrNetworkInUse should not be nil")
	}
}
