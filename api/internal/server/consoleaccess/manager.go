package consoleaccess

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// Manager manages VM console operations (VNC, SPICE)
// Uses the concrete Orchestrator type because it requires Proxmox-specific
// methods (GetDirectVMConsole, ProxmoxHostname, etc.) not in the Client interface.
type Manager struct {
	orchestrator    *orchestrator.Orchestrator
	logger          *slog.Logger
	responder       *httputil.Responder
	adminMiddleware func(http.Handler) http.Handler
}

// Config configures Manager
type Config struct {
	Orchestrator    *orchestrator.Orchestrator
	Logger          *slog.Logger
	Responder       *httputil.Responder
	AdminMiddleware func(http.Handler) http.Handler
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		orchestrator:    cfg.Orchestrator,
		logger:          cfg.Logger,
		responder:       cfg.Responder,
		adminMiddleware: cfg.AdminMiddleware,
	}
}

// SetupProxmoxRoutes registers console routes on the router
// These are the direct Proxmox VM access routes under /api/v1/proxmox
func (m *Manager) SetupProxmoxRoutes(r chi.Router) {
	r.Route("/proxmox", func(r chi.Router) {
		if m.adminMiddleware != nil {
			r.Use(m.adminMiddleware)
		}
		r.Route("/vms/{vmid}", func(r chi.Router) {
			r.Get("/console", m.handleGetDirectVMConsole())
			r.Get("/spice.vv", m.handleDownloadSpiceFile())
			r.Get("/vnc", m.handleDirectVNCProxy())
			r.Get("/spice", m.handleDirectSPICEProxy())
			r.Post("/start", m.handleDirectVMStart())
			r.Post("/stop", m.handleDirectVMStop())
			r.Post("/shutdown", m.handleDirectVMShutdown())
			r.Post("/reboot", m.handleDirectVMReboot())
		})
	})
}

// SetupPodConsoleRoutes registers console routes nested under pods
func (m *Manager) SetupPodConsoleRoutes(r chi.Router) {
	r.Get("/vms/{vmName}/console", m.handleGetVMConsole())
	r.Get("/vms/{vmName}/vnc", m.handleVNCProxy())
	r.Get("/vms/{vmName}/spice", m.handleSPICEProxy())
}

// SetupVNCProxyRoutes registers the VNC proxy with ticket route
func (m *Manager) SetupVNCProxyRoutes(r chi.Router) {
	r.Get("/vnc/proxy", m.handleVNCProxyWithTicket())
}
