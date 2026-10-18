package consoleaccess

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandleDirectVNCProxy_InvalidVMID(t *testing.T) {
	mgr := newTestManager(t)

	tests := []struct {
		name       string
		vmid       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "non-numeric vmid",
			vmid:       "abc",
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid vmid",
		},
		{
			name:       "empty vmid",
			vmid:       "",
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid vmid",
		},
		{
			name:       "float vmid",
			vmid:       "100.5",
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid vmid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/"+tt.vmid+"/vnc", nil)

			// Set up chi context with URL parameter
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("vmid", tt.vmid)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler := mgr.handleDirectVNCProxy()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleVNCProxyWithTicket_MissingParams(t *testing.T) {
	mgr := newTestManager(t)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing all params",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing host",
			query:      "port=5900&node=pve&vmid=100&ticket=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing port",
			query:      "host=localhost&node=pve&vmid=100&ticket=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing node",
			query:      "host=localhost&port=5900&vmid=100&ticket=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing vmid",
			query:      "host=localhost&port=5900&node=pve&ticket=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing ticket",
			query:      "host=localhost&port=5900&node=pve&vmid=100",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid port number",
			query:      "host=localhost&port=abc&node=pve&vmid=100&ticket=abc",
			wantStatus: http.StatusInternalServerError, // SSRF host validation fires first (no Proxmox configured in test)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/v1/vnc/proxy"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rr := httptest.NewRecorder()

			handler := mgr.handleVNCProxyWithTicket()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}
