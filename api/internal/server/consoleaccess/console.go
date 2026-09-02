// Package consoleaccess provides HTTP handlers for VM console operations (VNC, SPICE).
package consoleaccess

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// handleGetVMConsole returns console access information for a VM
// GET /api/v1/pods/{podID}/vms/{vmName}/console
func (m *Manager) handleGetVMConsole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		// Optional console type parameter (empty = auto-detect, tries VNC first then SPICE)
		consoleType := r.URL.Query().Get("type")

		ticket, err := m.orchestrator.GetVMConsole(r.Context(), podID, vmName, consoleType)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "get VM console")
			return
		}

		m.logger.Info("Console ticket generated", "podId", podID, "vmName", vmName, "type", consoleType)
		m.responder.JSONResponse(w, http.StatusOK, ticket)
	}
}

// handleGetDirectVMConsole returns console access for a VM by VMID directly
// This bypasses pod lookup and is useful for development/testing
// GET /api/v1/proxmox/vms/{vmid}/console
func (m *Manager) handleGetDirectVMConsole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		// Optional node parameter (defaults to "pve" as a reasonable default)
		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		// Optional console type parameter (vnc or spice, defaults to vnc)
		consoleType := r.URL.Query().Get("type")
		if consoleType == "" {
			consoleType = "vnc"
		}

		ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmid, consoleType)
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "get direct VM console")
			return
		}

		m.logger.Info("Direct console ticket generated", "vmid", vmid, "node", node, "type", consoleType)
		m.responder.JSONResponse(w, http.StatusOK, ticket)
	}
}

// handleDownloadSpiceFile generates a .vv file for SPICE clients (virt-viewer)
// GET /api/v1/proxmox/vms/{vmid}/spice.vv
func (m *Manager) handleDownloadSpiceFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		// Get SPICE ticket from orchestrator
		ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmid, "spice")
		if err != nil {
			m.responder.SafeErrorResponse(w, err, "get SPICE ticket")
			return
		}

		if ticket.Type != "spice" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.spiceNotSupported", nil)
			return
		}

		// Get real Proxmox host
		proxmoxHost := m.orchestrator.ProxmoxHostname()
		if proxmoxHost == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "console.errors.proxmoxNotConfigured", nil)
			return
		}

		// Generate .vv file content (virt-viewer format)
		// Format documented at: https://gitlab.com/virt-viewer/virt-viewer/-/blob/main/data/virt-viewer.schemas
		vvContent := fmt.Sprintf(`[virt-viewer]
type=spice
host=%s
port=
tls-port=%d
password=%s
delete-this-file=1
fullscreen=0
title=VM %d - %%d
toggle-fullscreen=shift+f11
release-cursor=shift+f12
secure-attention=ctrl+alt+end
enable-smartcard=0
enable-usbredir=1
color-depth=0
`, proxmoxHost, ticket.TLSPort, ticket.Password, vmid)

		// Set headers for file download
		filename := fmt.Sprintf("vm-%d.vv", vmid)
		w.Header().Set("Content-Type", "application/x-virt-viewer")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(vvContent)))

		m.logger.Info("SPICE .vv file generated", "vmid", vmid, "node", node)
		_, _ = w.Write([]byte(vvContent))
	}
}

// handleDirectVMStart starts a VM by VMID directly
// POST /api/v1/proxmox/vms/{vmid}/start
func (m *Manager) handleDirectVMStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		if err := m.orchestrator.StartVMDirect(r.Context(), node, vmid); err != nil {
			m.responder.SafeErrorResponse(w, err, "start VM")
			return
		}

		m.logger.Info("VM started", "vmid", vmid, "node", node)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "started", "vmid": vmidStr})
	}
}

// handleDirectVMStop force-stops a VM by VMID directly
// POST /api/v1/proxmox/vms/{vmid}/stop
func (m *Manager) handleDirectVMStop() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		if err := m.orchestrator.StopVMDirect(r.Context(), node, vmid); err != nil {
			m.responder.SafeErrorResponse(w, err, "stop VM")
			return
		}

		m.logger.Info("VM stopped", "vmid", vmid, "node", node)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "stopped", "vmid": vmidStr})
	}
}

// handleDirectVMShutdown gracefully shuts down a VM by VMID directly
// POST /api/v1/proxmox/vms/{vmid}/shutdown
func (m *Manager) handleDirectVMShutdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		if err := m.orchestrator.ShutdownVMDirect(r.Context(), node, vmid); err != nil {
			m.responder.SafeErrorResponse(w, err, "shutdown VM")
			return
		}

		m.logger.Info("VM shutdown initiated", "vmid", vmid, "node", node)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "shutting_down", "vmid": vmidStr})
	}
}

// handleDirectVMReboot reboots a VM by VMID directly
// POST /api/v1/proxmox/vms/{vmid}/reboot
func (m *Manager) handleDirectVMReboot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !serverutil.IsAdminRequest(r) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "console.errors.adminRequired", nil)
			return
		}
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "console.errors.invalidVmid", map[string]any{"Vmid": vmidStr})
			return
		}

		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}

		if err := m.orchestrator.RebootVMDirect(r.Context(), node, vmid); err != nil {
			m.responder.SafeErrorResponse(w, err, "reboot VM")
			return
		}

		m.logger.Info("VM reboot initiated", "vmid", vmid, "node", node)
		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "rebooting", "vmid": vmidStr})
	}
}
