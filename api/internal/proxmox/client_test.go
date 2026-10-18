package proxmox

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
		baseURL:    serverURL + "/api2/json",
		httpClient: http.DefaultClient,
		tokenID:    "user@pam!api",
		token:      "secret",
		logger:     logger,
	}
	// Initialize resilient client with minimal retry for tests
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "proxmox-test",
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

// mockProxmoxServer creates a test server that simulates Proxmox VE API responses
func mockProxmoxServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		// Get nodes
		case r.URL.Path == "/api2/json/nodes" && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[
					{"node":"pve1","status":"online","cpu":0.15,"maxcpu":8,"mem":4294967296,"maxmem":17179869184},
					{"node":"pve2","status":"online","cpu":0.25,"maxcpu":16,"mem":8589934592,"maxmem":34359738368}
				]`),
			}
			json.NewEncoder(w).Encode(resp)

		// Get VMs on a node
		case strings.HasSuffix(r.URL.Path, "/qemu") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[
					{"vmid":100,"name":"test-vm-1","status":"running","cpus":2,"maxmem":2147483648,"template":false},
					{"vmid":101,"name":"test-vm-2","status":"stopped","cpus":4,"maxmem":4294967296,"template":true}
				]`),
			}
			json.NewEncoder(w).Encode(resp)

		// Get VM status
		case strings.Contains(r.URL.Path, "/status/current") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`{"status":"running","cpus":2,"maxmem":2147483648,"cpu":0.05,"mem":1073741824}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Get snapshots
		case strings.HasSuffix(r.URL.Path, "/snapshot") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[
					{"name":"current","description":"current state","parent":"baseline"},
					{"name":"baseline","description":"Initial snapshot","snaptime":1704067200,"vmstate":0}
				]`),
			}
			json.NewEncoder(w).Encode(resp)

		// Create snapshot
		case strings.HasSuffix(r.URL.Path, "/snapshot") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001234:ABCD"`)})

		// Revert snapshot
		case strings.Contains(r.URL.Path, "/rollback") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001235:ABCE"`)})

		// Delete snapshot
		case strings.Contains(r.URL.Path, "/snapshot/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001236:ABCF"`)})

		// Clone VM
		case strings.HasSuffix(r.URL.Path, "/clone") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001237:ABD0"`)})

		// Start VM
		case strings.Contains(r.URL.Path, "/status/start") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001238:ABD1"`)})

		// Stop VM
		case strings.Contains(r.URL.Path, "/status/stop") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001239:ABD2"`)})

		// Delete VM
		case strings.HasPrefix(r.URL.Path, "/api2/json/nodes/") && r.Method == http.MethodDelete && !strings.Contains(r.URL.Path, "/snapshot"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:0000123A:ABD3"`)})

		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
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
				Host:    "proxmox.example.com",
				Port:    8006,
				TokenID: "user@pam!api",
				Token:   "secret-token",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			cfg: Config{
				TokenID: "user@pam!api",
				Token:   "secret-token",
			},
			wantErr: true,
			errMsg:  "proxmox host is required",
		},
		{
			name: "missing token ID",
			cfg: Config{
				Host:  "proxmox.example.com",
				Token: "secret-token",
			},
			wantErr: true,
			errMsg:  "token credentials are required",
		},
		{
			name: "missing token",
			cfg: Config{
				Host:    "proxmox.example.com",
				TokenID: "user@pam!api",
			},
			wantErr: true,
			errMsg:  "token credentials are required",
		},
		{
			name: "default port",
			cfg: Config{
				Host:    "proxmox.example.com",
				TokenID: "user@pam!api",
				Token:   "secret-token",
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

func TestClient_GetNodes(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Node != "pve1" {
		t.Errorf("expected node pve1, got %s", nodes[0].Node)
	}
	if nodes[0].Status != "online" {
		t.Errorf("expected status online, got %s", nodes[0].Status)
	}
}

func TestClient_GetVMs(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	vms, err := client.GetVMs(ctx, "pve1")
	if err != nil {
		t.Fatalf("GetVMs failed: %v", err)
	}

	if len(vms) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(vms))
	}

	// Check that node is set on each VM
	for _, vm := range vms {
		if vm.Node != "pve1" {
			t.Errorf("expected node pve1 on VM, got %s", vm.Node)
		}
	}

	if vms[0].VMID != 100 {
		t.Errorf("expected VMID 100, got %d", vms[0].VMID)
	}
	if vms[0].Name != "test-vm-1" {
		t.Errorf("expected name test-vm-1, got %s", vms[0].Name)
	}
}

func TestClient_GetVM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	vm, err := client.GetVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("GetVM failed: %v", err)
	}

	if vm.VMID != 100 {
		t.Errorf("expected VMID 100, got %d", vm.VMID)
	}
	if vm.Node != "pve1" {
		t.Errorf("expected node pve1, got %s", vm.Node)
	}
	if vm.Status != "running" {
		t.Errorf("expected status running, got %s", vm.Status)
	}
}

func TestClient_GetSnapshots(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	snapshots, err := client.GetSnapshots(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("GetSnapshots failed: %v", err)
	}

	if len(snapshots) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snapshots))
	}

	// Find the baseline snapshot
	var baseline *Snapshot
	for i := range snapshots {
		if snapshots[i].Name == "baseline" {
			baseline = &snapshots[i]
			break
		}
	}

	if baseline == nil {
		t.Fatal("expected to find baseline snapshot")
	}
	if baseline.Description != "Initial snapshot" {
		t.Errorf("expected description 'Initial snapshot', got %s", baseline.Description)
	}
}

func TestClient_CreateSnapshot(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.CreateSnapshot(ctx, "pve1", 100, "test-snap", "Test snapshot", false)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
}

func TestClient_CreateSnapshot_WithRAM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.CreateSnapshot(ctx, "pve1", 100, "test-snap-ram", "Snapshot with RAM", true)
	if err != nil {
		t.Fatalf("CreateSnapshot with RAM failed: %v", err)
	}
}

func TestClient_RevertSnapshot(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.RevertSnapshot(ctx, "pve1", 100, "baseline")
	if err != nil {
		t.Fatalf("RevertSnapshot failed: %v", err)
	}
}

func TestClient_DeleteSnapshot(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.DeleteSnapshot(ctx, "pve1", 100, "old-snapshot")
	if err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
}

func TestClient_CloneVM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()

	// Test linked clone
	err := client.CloneVM(ctx, "pve1", 100, 200, "cloned-vm", true)
	if err != nil {
		t.Fatalf("CloneVM (linked) failed: %v", err)
	}

	// Test full clone
	err = client.CloneVM(ctx, "pve1", 100, 201, "cloned-vm-full", false)
	if err != nil {
		t.Fatalf("CloneVM (full) failed: %v", err)
	}
}

func TestClient_StartVM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.StartVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("StartVM failed: %v", err)
	}
}

func TestClient_StopVM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.StopVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("StopVM failed: %v", err)
	}
}

func TestClient_DeleteVM(t *testing.T) {
	server := mockProxmoxServer(t)
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.DeleteVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("DeleteVM failed: %v", err)
	}
}

func TestClient_AuthorizationHeader(t *testing.T) {
	// Create a server that validates the auth header format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "PVEAPIToken=user@pam!mytoken=secret123"
		if auth != expected {
			t.Errorf("expected auth header %q, got %q", expected, auth)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`[]`)})
	}))
	defer server.Close()

	// Create custom test client with specific credentials
	logger := slog.Default()
	client := &Client{
		baseURL:    server.URL + "/api2/json",
		httpClient: http.DefaultClient,
		tokenID:    "user@pam!mytoken",
		token:      "secret123",
		logger:     logger,
	}
	client.resilient = resilience.NewResilientClient(resilience.ResilientClientConfig{
		Name: "proxmox-test",
		CircuitBreaker: resilience.CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 2,
			Timeout:          30 * time.Second,
		},
		Retry: resilience.RetryConfig{
			MaxAttempts:     1,
			InitialDelay:    10 * time.Millisecond,
			MaxDelay:        100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: resilience.IsRetryableError,
		},
		Logger: logger,
	})

	ctx := context.Background()
	_, err := client.GetNodes(ctx)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	// Test server that returns errors
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error: VM not found"))
	}))
	defer errorServer.Close()

	client := newTestClient(errorServer.URL)

	ctx := context.Background()

	// Test various operations return errors
	err := client.CreateSnapshot(ctx, "pve1", 999, "snap", "desc", false)
	if err == nil {
		t.Error("expected error for CreateSnapshot")
	}

	err = client.RevertSnapshot(ctx, "pve1", 999, "snap")
	if err == nil {
		t.Error("expected error for RevertSnapshot")
	}

	err = client.DeleteSnapshot(ctx, "pve1", 999, "snap")
	if err == nil {
		t.Error("expected error for DeleteSnapshot")
	}

	err = client.CloneVM(ctx, "pve1", 999, 1000, "clone", true)
	if err == nil {
		t.Error("expected error for CloneVM")
	}

	err = client.StartVM(ctx, "pve1", 999)
	if err == nil {
		t.Error("expected error for StartVM")
	}

	err = client.StopVM(ctx, "pve1", 999)
	if err == nil {
		t.Error("expected error for StopVM")
	}

	err = client.DeleteVM(ctx, "pve1", 999)
	if err == nil {
		t.Error("expected error for DeleteVM")
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	// Server that delays response
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context was canceled
		select {
		case <-r.Context().Done():
			return
		default:
		}
		json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`[]`)})
	}))
	defer slowServer.Close()

	client := newTestClient(slowServer.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetNodes(ctx)
	if err == nil {
		t.Error("expected error due to canceled context")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected default host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 8006 {
		t.Errorf("expected default port 8006, got %d", cfg.Port)
	}
	if cfg.DefaultNode != "pve" {
		t.Errorf("expected default node pve, got %s", cfg.DefaultNode)
	}
}

// -----------------------------------------------------------------------------
// Additional VM State Operations Tests
// -----------------------------------------------------------------------------

func TestClient_ShutdownVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/status/shutdown") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001240:ABD4"`)})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.ShutdownVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("ShutdownVM failed: %v", err)
	}
}

func TestClient_RebootVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/status/reboot") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001241:ABD5"`)})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.RebootVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("RebootVM failed: %v", err)
	}
}

func TestClient_SuspendVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/status/suspend") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001242:ABD6"`)})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.SuspendVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("SuspendVM failed: %v", err)
	}
}

func TestClient_ResumeVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/status/resume") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(apiResponse{Data: json.RawMessage(`"UPID:pve:00001243:ABD7"`)})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	err := client.ResumeVM(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("ResumeVM failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Console Proxy Tests
// -----------------------------------------------------------------------------

func TestClient_GetSpiceProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/spiceproxy") && r.Method == http.MethodPost {
			resp := apiResponse{
				Data: json.RawMessage(`{
					"host": "<PROXMOX_IP>",
					"port": 61000,
					"tls-port": 61001,
					"ticket": "spice-ticket-abc123",
					"password": "spice-password-xyz",
					"type": "spice",
					"proxy": "https://<PROXMOX_IP>:3128"
				}`),
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	ticket, err := client.GetSpiceProxy(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("GetSpiceProxy failed: %v", err)
	}

	if ticket.Host != "<PROXMOX_IP>" {
		t.Errorf("expected host <PROXMOX_IP>, got %s", ticket.Host)
	}
	if ticket.Port != 61000 {
		t.Errorf("expected port 61000, got %d", ticket.Port)
	}
	if ticket.TLSPort != 61001 {
		t.Errorf("expected TLS port 61001, got %d", ticket.TLSPort)
	}
	if ticket.Ticket != "spice-ticket-abc123" {
		t.Errorf("expected ticket spice-ticket-abc123, got %s", ticket.Ticket)
	}
	if ticket.Password != "spice-password-xyz" {
		t.Errorf("expected password spice-password-xyz, got %s", ticket.Password)
	}
}

func TestClient_GetVNCProxy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "/vncproxy") && r.Method == http.MethodPost {
			resp := apiResponse{
				Data: json.RawMessage(`{
					"ticket": "vnc-ticket-def456",
					"port": "5901",
					"user": "root@pam",
					"upid": "UPID:pve:00001244:ABD8"
				}`),
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	ticket, err := client.GetVNCProxy(ctx, "pve1", 100)
	if err != nil {
		t.Fatalf("GetVNCProxy failed: %v", err)
	}

	if ticket.Ticket != "vnc-ticket-def456" {
		t.Errorf("expected ticket vnc-ticket-def456, got %s", ticket.Ticket)
	}
	if ticket.PortInt() != 5901 {
		t.Errorf("expected port 5901, got %d", ticket.PortInt())
	}
	if ticket.User != "root@pam" {
		t.Errorf("expected user root@pam, got %s", ticket.User)
	}
}

func TestClient_GetSpiceProxy_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("VM 999 not found"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	_, err := client.GetSpiceProxy(ctx, "pve1", 999)
	if err == nil {
		t.Error("expected error for non-existent VM")
	}
}

func TestClient_GetVNCProxy_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("VM 999 not found"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	_, err := client.GetVNCProxy(ctx, "pve1", 999)
	if err == nil {
		t.Error("expected error for non-existent VM")
	}
}

// -----------------------------------------------------------------------------
// GetVMIP Tests
// -----------------------------------------------------------------------------

func TestClient_GetVMIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/api2/json/nodes" && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[{"node":"pve1","status":"online"}]`),
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/agent/network-get-interfaces") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`{
					"result": [
						{
							"name": "lo",
							"ip-addresses": [
								{"ip-address": "127.0.0.1", "ip-address-type": "ipv4"}
							]
						},
						{
							"name": "eth0",
							"ip-addresses": [
								{"ip-address": "192.168.1.100", "ip-address-type": "ipv4"},
								{"ip-address": "fe80::1", "ip-address-type": "ipv6"}
							]
						}
					]
				}`),
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	ip, err := client.GetVMIP(ctx, "100")
	if err != nil {
		t.Fatalf("GetVMIP failed: %v", err)
	}

	if ip != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", ip)
	}
}

func TestClient_GetVMIP_NoIPv4(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/api2/json/nodes" && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[{"node":"pve1","status":"online"}]`),
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/agent/network-get-interfaces") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`{
					"result": [
						{
							"name": "lo",
							"ip-addresses": [
								{"ip-address": "127.0.0.1", "ip-address-type": "ipv4"}
							]
						}
					]
				}`),
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	_, err := client.GetVMIP(ctx, "100")
	if err == nil {
		t.Error("expected error when no non-loopback IPv4 found")
	}
}

func TestClient_GetVMIP_GuestAgentNotRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "PVEAPIToken=") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/api2/json/nodes" && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`[{"node":"pve1","status":"online"}]`),
			}
			json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/agent/network-get-interfaces") && r.Method == http.MethodGet:
			// Guest agent not running - Proxmox returns 500
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("QEMU guest agent is not running"))

		default:
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	ctx := context.Background()
	_, err := client.GetVMIP(ctx, "100")
	if err == nil {
		t.Error("expected error when guest agent is not running")
	}
}

// -----------------------------------------------------------------------------
// Utility Method Tests
// -----------------------------------------------------------------------------

func TestClient_Host(t *testing.T) {
	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	host := client.Host()
	if host != "proxmox.example.com:8006" {
		t.Errorf("expected host proxmox.example.com:8006, got %s", host)
	}
}

func TestClient_Hostname(t *testing.T) {
	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	hostname := client.Hostname()
	if hostname != "proxmox.example.com" {
		t.Errorf("expected hostname proxmox.example.com, got %s", hostname)
	}
}

func TestClient_AuthHeader(t *testing.T) {
	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret-token-123",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	authHeader := client.AuthHeader()
	expected := "PVEAPIToken=user@pam!api=secret-token-123"
	if authHeader != expected {
		t.Errorf("expected auth header %s, got %s", expected, authHeader)
	}
}

func TestClient_CircuitState(t *testing.T) {
	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	state := client.CircuitState()
	if state != resilience.StateClosed {
		t.Errorf("expected initial circuit state to be Closed, got %v", state)
	}
}

func TestClient_CircuitStats(t *testing.T) {
	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret",
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	stats := client.CircuitStats()
	// Initial stats should have zero failures
	if stats.Failures != 0 {
		t.Errorf("expected initial Failures to be 0, got %d", stats.Failures)
	}
}

func TestClient_WithLogger(t *testing.T) {
	customLogger := slog.New(slog.NewTextHandler(nil, nil))

	cfg := Config{
		Host:    "proxmox.example.com",
		Port:    8006,
		TokenID: "user@pam!api",
		Token:   "secret",
	}
	client, err := NewClient(cfg, WithLogger(customLogger))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Verify logger was set (indirect check via client field)
	if client.logger != customLogger {
		t.Error("expected custom logger to be set")
	}
}

func TestVNCTicket_PortInt(t *testing.T) {
	tests := []struct {
		name     string
		port     json.Number
		expected int
	}{
		{"valid port", json.Number("5901"), 5901},
		{"zero port", json.Number("0"), 0},
		{"high port", json.Number("59999"), 59999},
		{"invalid port", json.Number("invalid"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &VNCTicket{Port: tt.port}
			if got := ticket.PortInt(); got != tt.expected {
				t.Errorf("PortInt() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestNewClientBaseURL(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{"hostname", "proxmox.example.com", 0, "https://proxmox.example.com:8006/api2/json"},
		{"ipv4 with port", "192.0.2.10", 8443, "https://192.0.2.10:8443/api2/json"},
		{"url form", "https://proxmox.example.com:8443", 0, "https://proxmox.example.com:8443/api2/json"},
		{"bare ipv6", "2001:db8::10", 0, "https://[2001:db8::10]:8006/api2/json"},
		{"bracketed ipv6", "[2001:db8::10]", 0, "https://[2001:db8::10]:8006/api2/json"},
		{"ipv6 url form", "https://[2001:db8::10]:8443", 0, "https://[2001:db8::10]:8443/api2/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(Config{Host: tt.host, Port: tt.port, TokenID: "user@pam!api", Token: "secret-token"})
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if client.baseURL != tt.want {
				t.Errorf("baseURL = %q, want %q", client.baseURL, tt.want)
			}
			if _, err := url.Parse(client.baseURL); err != nil {
				t.Errorf("baseURL does not parse: %v", err)
			}
		})
	}
}
