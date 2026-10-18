package wazuh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockWazuhServer creates a test server that simulates Wazuh Manager API responses
func mockWazuhServer(t *testing.T) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle authentication
		if r.URL.Path == "/security/user/authenticate" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "wazuh" || pass != "wazuh" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Write([]byte("mock-jwt-token"))
			return
		}

		// Check authorization header for all other requests
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		// Manager info
		case r.URL.Path == "/manager/info" && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":[{"version":"v4.8.0","hostname":"wazuh-manager","type":"manager","node_name":"node01","node_type":"master"}],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Register agent
		case r.URL.Path == "/agents" && r.Method == http.MethodPost:
			var req RegisterAgentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":[{"id":"001","key":"MDAxIHRlc3QtYWdlbnQgYW55IGtleQ=="}],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Get agent
		case strings.HasPrefix(r.URL.Path, "/agents") && r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "agents_list"):
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":[{"id":"001","name":"test-agent","ip":"192.168.1.100","status":"active","version":"v4.8.0","dateAdd":"2024-01-15T10:30:00Z","lastKeepAlive":"2024-01-15T12:00:00Z"}],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Get agent key
		case strings.HasSuffix(r.URL.Path, "/key") && r.Method == http.MethodGet:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":[{"key":"MDAxIHRlc3QtYWdlbnQgYW55IGtleQ=="}],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Delete agent
		case strings.HasPrefix(r.URL.Path, "/agents") && r.Method == http.MethodDelete:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":["001"],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Create group
		case r.URL.Path == "/groups" && r.Method == http.MethodPost:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":["test-group"],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Delete group
		case strings.HasPrefix(r.URL.Path, "/groups") && r.Method == http.MethodDelete:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":["test-group"],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Add agent to group
		case strings.Contains(r.URL.Path, "/group/") && r.Method == http.MethodPut:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":["001"],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		// Restart agent
		case strings.HasSuffix(r.URL.Path, "/restart") && r.Method == http.MethodPut:
			resp := apiResponse{
				Data: json.RawMessage(`{"affected_items":["001"],"total_affected_items":1}`),
			}
			json.NewEncoder(w).Encode(resp)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
}

func TestAPIClient_Authenticate(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	err := client.authenticate(ctx)
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}

	if client.token == "" {
		t.Error("expected token to be set after authentication")
	}
}

func TestAPIClient_AuthenticateFailure(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wrong",
		Password:           "credentials",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	err := client.authenticate(ctx)
	if err == nil {
		t.Fatal("expected authentication to fail with wrong credentials")
	}
}

func TestAPIClient_GetManagerInfo(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	info, err := client.GetManagerInfo(ctx)
	if err != nil {
		t.Fatalf("GetManagerInfo failed: %v", err)
	}

	if info.Version != "v4.8.0" {
		t.Errorf("expected version v4.8.0, got %s", info.Version)
	}
	if info.Hostname != "wazuh-manager" {
		t.Errorf("expected hostname wazuh-manager, got %s", info.Hostname)
	}
}

func TestAPIClient_RegisterAgent(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	resp, err := client.RegisterAgent(ctx, "test-agent", "192.168.1.100")
	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	if resp.ID != "001" {
		t.Errorf("expected agent ID 001, got %s", resp.ID)
	}
	if resp.Key == "" {
		t.Error("expected agent key to be non-empty")
	}
}

func TestAPIClient_GetAgent(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	agent, err := client.GetAgent(ctx, "001")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if agent.ID != "001" {
		t.Errorf("expected agent ID 001, got %s", agent.ID)
	}
	if agent.Name != "test-agent" {
		t.Errorf("expected agent name test-agent, got %s", agent.Name)
	}
	if agent.Status != "active" {
		t.Errorf("expected agent status active, got %s", agent.Status)
	}
}

func TestAPIClient_GetAgentKey(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	key, err := client.GetAgentKey(ctx, "001")
	if err != nil {
		t.Fatalf("GetAgentKey failed: %v", err)
	}

	if key == "" {
		t.Error("expected key to be non-empty")
	}
}

func TestAPIClient_DeleteAgent(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	err := client.DeleteAgent(ctx, "001")
	if err != nil {
		t.Fatalf("DeleteAgent failed: %v", err)
	}
}

func TestAPIClient_GroupOperations(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()

	// Create group
	err := client.CreateGroup(ctx, "test-group")
	if err != nil {
		t.Fatalf("CreateGroup failed: %v", err)
	}

	// Add agent to group
	err = client.AddAgentToGroup(ctx, "001", "test-group")
	if err != nil {
		t.Fatalf("AddAgentToGroup failed: %v", err)
	}

	// Delete group
	err = client.DeleteGroup(ctx, "test-group")
	if err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
}

func TestAPIClient_RestartAgent(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	err := client.RestartAgent(ctx, "001")
	if err != nil {
		t.Fatalf("RestartAgent failed: %v", err)
	}
}

func TestAPIClient_Ping(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()
	err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestAPIClient_TokenReuse(t *testing.T) {
	server := mockWazuhServer(t)
	defer server.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         server.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
	})

	ctx := context.Background()

	// First call should authenticate
	_, err := client.GetManagerInfo(ctx)
	if err != nil {
		t.Fatalf("first GetManagerInfo failed: %v", err)
	}

	firstToken := client.token

	// Second call should reuse the token
	_, err = client.GetManagerInfo(ctx)
	if err != nil {
		t.Fatalf("second GetManagerInfo failed: %v", err)
	}

	if client.token != firstToken {
		t.Error("expected token to be reused, but a new token was generated")
	}
}

func TestAPIClient_Timeout(t *testing.T) {
	// Create a server that delays response
	slowServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("mock-token"))
	}))
	defer slowServer.Close()

	client := NewAPIClient(APIClientConfig{
		ManagerURL:         slowServer.URL,
		Username:           "wazuh",
		Password:           "wazuh",
		InsecureSkipVerify: true,
		Timeout:            50 * time.Millisecond, // Very short timeout
	})

	ctx := context.Background()
	err := client.authenticate(ctx)
	if err == nil {
		t.Error("expected timeout error, but got none")
	}
}
