// Package server provides LTI console handlers for rendering VNC/SPICE console pages
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/canvas"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// renderVNCConsolePage renders a launcher page that opens the VNC console in a new window
// This avoids Canvas CSP restrictions that block external scripts in iframes
func (m *CanvasManager) renderVNCConsolePage(w http.ResponseWriter, r *http.Request, pod *models.Pod, launch *canvas.LTILaunchRequest) {
	// Course/user info for display. html/template auto-escapes these inside
	// HTML contexts, so we pass raw values (no manual html.EscapeString).
	courseName := launch.Context.Title
	if courseName == "" {
		courseName = launch.Context.Label
	}
	userName := ""
	if launch.CanvasUser != nil {
		userName = launch.CanvasUser.Name
	}

	// Build the direct console URL (absolute URL to API server, bypasses Canvas CSP).
	// Use ToolIssuer which is the API's external URL (e.g., https://lab.example.com).
	apiBaseURL := m.config.ToolIssuer
	if apiBaseURL == "" {
		m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.toolIssuerNotConfigured", nil)
		return
	}
	// Generate HMAC-signed console token to prevent unauthenticated access
	consoleToken := m.generateConsoleToken(pod.ID)

	consoleURL := fmt.Sprintf("%s/lti/console?podId=%s&token=%s", apiBaseURL, pod.ID, consoleToken)

	data := struct {
		CourseName string
		UserName   string
		ConsoleURL string
		VMCount    int
	}{
		CourseName: courseName,
		UserName:   userName,
		ConsoleURL: consoleURL,
		VMCount:    len(pod.VMs),
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if err := vncLauncherTemplate.Execute(w, data); err != nil {
		m.logger.Error("Failed to render VNC launcher page", "error", err)
	}
}

// handleLTIConsole serves the actual VNC console page (opened in new window)
// Supports multi-VM pods with tab switching
func (m *CanvasManager) handleLTIConsole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := r.URL.Query().Get("podId")

		// Validate console token to prevent unauthenticated access
		token := r.URL.Query().Get("token")

		if podID == "" {
			// Single VM mode also requires a valid token (vmid used as token subject)
			vmID := r.URL.Query().Get("vmid")
			node := r.URL.Query().Get("node")
			if node == "" {
				node = "pve"
			}
			if vmID == "" {
				m.responder.LocalizedHTTPError(r.Context(), w, http.StatusBadRequest, "lti.console.errors.missingPodOrVmid", nil)
				return
			}
			if !m.validateConsoleToken(vmID, token) {
				m.responder.LocalizedHTTPError(r.Context(), w, http.StatusForbidden, "lti.console.errors.invalidToken", nil)
				return
			}
			// Single VM mode - create a simple VM list
			m.renderSingleVMConsole(w, r, vmID, node)
			return
		}

		if !m.validateConsoleToken(podID, token) {
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusForbidden, "lti.console.errors.invalidToken", nil)
			return
		}

		// Multi-VM mode: load pod from database
		if m.podRepo == nil {
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.podRepoNotConfigured", nil)
			return
		}

		pod, err := m.podRepo.GetByID(r.Context(), podID)
		if err != nil {
			m.logger.Error("Failed to load pod for console", "error", err, "podId", podID)
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.loadPodFailed", nil)
			return
		}

		if pod == nil || len(pod.VMs) == 0 {
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusNotFound, "lti.console.errors.podNotFoundOrEmpty", nil)
			return
		}

		// Build VM list for JavaScript
		type vmInfo struct {
			Name   string `json:"name"`
			VMID   string `json:"vmid"`
			Node   string `json:"node"`
			Status string `json:"status"`
		}

		vmList := make([]vmInfo, 0, len(pod.VMs))
		for _, vm := range pod.VMs {
			node := vm.Node
			if node == "" {
				node = "pve"
			}
			status := vm.Status
			if status == "" || status == "created" {
				status = "running" // Assume running if not specified
			}
			vmList = append(vmList, vmInfo{
				Name:   vm.Name,
				VMID:   vm.PlatformID,
				Node:   node,
				Status: status,
			})
		}

		vmListJSON, err := json.Marshal(vmList)
		if err != nil {
			m.logger.Error("Failed to marshal VM list", "error", err)
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.internal", nil)
			return
		}

		// Build WebSocket base URL
		// Check for HTTPS - either direct TLS or behind a reverse proxy
		wsProtocol := "ws"
		if r.TLS != nil {
			wsProtocol = "wss"
		} else if r.Header.Get("X-Forwarded-Proto") == "https" {
			wsProtocol = "wss"
		} else if strings.HasPrefix(r.Header.Get("Origin"), "https://") {
			wsProtocol = "wss"
		} else if strings.HasPrefix(r.Header.Get("Referer"), "https://") {
			wsProtocol = "wss"
		}
		host := r.Host
		if host == "" {
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusBadRequest, "lti.console.errors.missingHostHeader", nil)
			return
		}
		wsBaseURL := fmt.Sprintf("%s://%s/api/v1/proxmox/vms", wsProtocol, host)

		// Get initial VNC ticket for first VM
		firstVM := pod.VMs[0]
		node := firstVM.Node
		if node == "" {
			node = "pve"
		}
		vmidInt := 0
		fmt.Sscanf(firstVM.PlatformID, "%d", &vmidInt)
		ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmidInt, "vnc")
		if err != nil {
			m.logger.Error("Failed to get initial VNC ticket", "error", err, "vmid", firstVM.PlatformID)
			m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.vncAccessFailed", nil)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		// Render the multi-VM console page
		m.renderMultiVMConsoleHTML(w, pod.LabTemplate, string(vmListJSON), wsBaseURL, ticket.Ticket)
	}
}

// renderSingleVMConsole renders console for a single VM (backwards compatibility)
func (m *CanvasManager) renderSingleVMConsole(w http.ResponseWriter, r *http.Request, vmID, node string) {
	vmidInt := 0
	fmt.Sscanf(vmID, "%d", &vmidInt)
	ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmidInt, "vnc")
	if err != nil {
		m.logger.Error("Failed to get VNC ticket for console", "error", err, "vmid", vmID)
		m.responder.LocalizedHTTPError(r.Context(), w, http.StatusInternalServerError, "lti.console.errors.vncAccessFailed", nil)
		return
	}

	// Check for HTTPS - either direct TLS or behind a reverse proxy
	wsProtocol := "ws"
	if r.TLS != nil {
		wsProtocol = "wss"
	} else if r.Header.Get("X-Forwarded-Proto") == "https" {
		wsProtocol = "wss"
	} else if strings.HasPrefix(r.Header.Get("Origin"), "https://") {
		wsProtocol = "wss"
	} else if strings.HasPrefix(r.Header.Get("Referer"), "https://") {
		wsProtocol = "wss"
	}
	host := r.Host
	if host == "" {
		m.responder.LocalizedHTTPError(r.Context(), w, http.StatusBadRequest, "lti.console.errors.missingHostHeader", nil)
		return
	}
	wsBaseURL := fmt.Sprintf("%s://%s/api/v1/proxmox/vms", wsProtocol, host)

	// Create single-VM list using json.Marshal to prevent injection
	singleVMList := []struct {
		Name   string `json:"name"`
		VMID   string `json:"vmid"`
		Node   string `json:"node"`
		Status string `json:"status"`
	}{{Name: "VM", VMID: vmID, Node: node, Status: "running"}}
	vmListBytes, _ := json.Marshal(singleVMList)
	vmListJSON := string(vmListBytes)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	m.renderMultiVMConsoleHTML(w, "Kootenai", vmListJSON, wsBaseURL, ticket.Ticket)
}

// renderMultiVMConsoleHTML renders the multi-VM console page. vmListJSON is
// pre-marshalled JSON (already validated server-side) and is injected as a JS
// expression via template.JS — html/template would otherwise HTML-escape the
// JSON characters and break the resulting JavaScript. wsBaseURL and
// initialPassword are auto-quoted as JS string literals by html/template.
func (m *CanvasManager) renderMultiVMConsoleHTML(w http.ResponseWriter, labName, vmListJSON, wsBaseURL, initialPassword string) {
	data := struct {
		LabName         string
		VMList          template.JS
		WSBaseURL       string
		InitialPassword string
	}{
		LabName:         labName,
		VMList:          template.JS(vmListJSON),
		WSBaseURL:       wsBaseURL,
		InitialPassword: initialPassword,
	}

	if err := ltiConsoleTemplate.Execute(w, data); err != nil {
		m.logger.Error("Failed to render multi-VM console", "error", err)
	}
}

// generateConsoleToken creates an HMAC-signed token for console page access.
// Token format: timestamp.hmac where hmac = HMAC-SHA256(podID+timestamp, secret)
func (m *CanvasManager) generateConsoleToken(podID string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(m.consoleSecret))
	mac.Write([]byte(podID + ts))
	sig := hex.EncodeToString(mac.Sum(nil))
	return ts + "." + sig
}

// validateConsoleToken validates an HMAC-signed console token.
// Tokens expire after 5 minutes.
func (m *CanvasManager) validateConsoleToken(podID, token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	// Check expiry (5 minutes)
	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return false
	}
	// Recompute HMAC
	mac := hmac.New(sha256.New, []byte(m.consoleSecret))
	mac.Write([]byte(podID + parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}
