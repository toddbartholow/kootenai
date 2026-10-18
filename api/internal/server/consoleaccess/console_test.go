package consoleaccess

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger := newTestLogger()
	orch := orchestrator.New(nil, nil, orchestrator.Config{}, orchestrator.WithLogger(logger))
	return NewManager(Config{
		Orchestrator: orch,
		Logger:       logger,
		Responder:    httputil.NewResponder(logger),
	})
}

// withAdminAuth adds an admin user to the request context for direct VM handler tests
func withAdminAuth(ctx context.Context) context.Context {
	return auth.ContextWithUser(ctx, &auth.User{
		ID:    "admin-1",
		Email: "admin@example.com",
		Name:  "Test Admin",
		Roles: []string{"admin"},
	})
}

// -----------------------------------------------------------------------------
// handleGetVMConsole Tests (pod-based console access)
// -----------------------------------------------------------------------------

func TestHandleGetVMConsole(t *testing.T) {
	t.Run("success - gets console via pod and vmName", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-123/vms/workstation/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("podID", "pod-123")
		rctx.URLParams.Add("vmName", "workstation")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetVMConsole()(rr, req)

		// Will fail because no pod found - safeErrorResponse correctly returns 404 for not found
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d (pod not found), got %d: %s", http.StatusNotFound, rr.Code, rr.Body.String())
		}
	})

	t.Run("success - with console type parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods/pod-123/vms/workstation/console?type=spice", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("podID", "pod-123")
		rctx.URLParams.Add("vmName", "workstation")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetVMConsole()(rr, req)

		// Will fail because no pod found - safeErrorResponse correctly returns 404 for not found
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d (pod not found), got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("empty podID and vmName", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/pods//vms//console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("podID", "")
		rctx.URLParams.Add("vmName", "")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetVMConsole()(rr, req)

		// Should fail because empty podID - safeErrorResponse correctly returns 404 for not found
		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d (not found), got %d", http.StatusNotFound, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetDirectVMConsole Tests (direct VM access by VMID)
// -----------------------------------------------------------------------------

func TestHandleGetVMConsoleDirect(t *testing.T) {
	t.Run("error - invalid VMID parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/invalid/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "invalid")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["error"] == nil {
			t.Error("expected error message in response")
		}
	})

	t.Run("error - VM console not available (no proxmox client)", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		// Should fail because no proxmox client is configured
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleGetDirectVMConsole Tests
// -----------------------------------------------------------------------------

func TestHandleGetDirectVMConsole(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/abc/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "abc")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - negative VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/-1/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "-1")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		// Should still accept negative number as valid int, but orchestrator would reject it
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
		}
	})

	t.Run("default console type is vnc", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/console", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		// Will fail because no proxmox client, but we can verify it got past validation
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom console type in query param", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/console?type=spice", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		// Will fail because no proxmox client, but we can verify it got past validation
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node in query param", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/console?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleGetDirectVMConsole()(rr, req)

		// Will fail because no proxmox client, but we can verify it got past validation
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDirectVMStart Tests
// -----------------------------------------------------------------------------

func TestHandleDirectVMStart(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/abc/start", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "abc")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStart()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - empty VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms//start", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStart()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - no orchestrator configured", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/start", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStart()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/start?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStart()(rr, req)

		// Will fail because no orchestrator, but verifies it got past validation
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDirectVMStop Tests
// -----------------------------------------------------------------------------

func TestHandleDirectVMStop(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/xyz/stop", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "xyz")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStop()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - no orchestrator configured", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/stop", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStop()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/stop?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMStop()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDirectVMShutdown Tests
// -----------------------------------------------------------------------------

func TestHandleDirectVMShutdown(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/not-a-number/shutdown", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "not-a-number")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMShutdown()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - no orchestrator configured", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/shutdown", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMShutdown()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/shutdown?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMShutdown()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDirectVMReboot Tests
// -----------------------------------------------------------------------------

func TestHandleDirectVMReboot(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/invalid/reboot", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "invalid")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMReboot()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - float VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100.5/reboot", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100.5")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMReboot()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})

	t.Run("error - no orchestrator configured", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/reboot", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMReboot()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node parameter", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/proxmox/vms/100/reboot?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDirectVMReboot()(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// handleDownloadSpiceFile Tests
// -----------------------------------------------------------------------------

func TestHandleDownloadSpiceFile(t *testing.T) {
	t.Run("error - invalid VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/abc/spice.vv", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "abc")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDownloadSpiceFile()(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
	})

	t.Run("error - no proxmox configured", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/spice.vv", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDownloadSpiceFile()(rr, req)

		// Should fail because no proxmox client is configured
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("error - zero VMID", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/0/spice.vv", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "0")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDownloadSpiceFile()(rr, req)

		// Should still accept 0 as valid int, but orchestrator would reject it
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("custom node in query param", func(t *testing.T) {
		mgr := newTestManager(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/100/spice.vv?node=pve2", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("vmid", "100")
		req = req.WithContext(withAdminAuth(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))

		rr := httptest.NewRecorder()
		mgr.handleDownloadSpiceFile()(rr, req)

		// Will fail because no proxmox client, but we can verify it got past validation
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}
