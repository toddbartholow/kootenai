package consoleaccess

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHandleDirectSPICEProxy_InvalidVMID(t *testing.T) {
	mgr := newTestManager(t)

	tests := []struct {
		name       string
		vmid       string
		wantStatus int
	}{
		{
			name:       "non-numeric vmid",
			vmid:       "abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty vmid",
			vmid:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "float vmid",
			vmid:       "100.5",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/proxmox/vms/"+tt.vmid+"/spice", nil)

			// Set up chi context with URL parameter
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("vmid", tt.vmid)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler := mgr.handleDirectSPICEProxy()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
