package consoleaccess

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var (
	allowedWebSocketOrigins []string
	initOriginsOnce         sync.Once
	reNodeName              = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// initWebSocketOrigins initializes the allowed origins for WebSocket connections
func initWebSocketOrigins() {
	origins := os.Getenv("CORS_ORIGINS")
	if origins == "" {
		// Default to localhost origins for development
		allowedWebSocketOrigins = []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
		}
		return
	}

	// Parse comma-separated origins
	for _, raw := range strings.Split(origins, ",") {
		origin := strings.TrimSpace(raw)
		if origin != "" && origin != "*" {
			allowedWebSocketOrigins = append(allowedWebSocketOrigins, origin)
		}
	}

	// If wildcard was the only origin, use permissive defaults for dev
	if len(allowedWebSocketOrigins) == 0 {
		allowedWebSocketOrigins = []string{
			"http://localhost:3000",
			"http://localhost:5173",
		}
	}
}

// checkWebSocketOrigin validates the request origin against allowed origins
func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No origin header - allow (same-origin requests don't send Origin)
		return true
	}

	initOriginsOnce.Do(initWebSocketOrigins)

	// Check against allowed origins
	for _, allowed := range allowedWebSocketOrigins {
		if origin == allowed {
			return true
		}
	}

	return false
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	Subprotocols:    []string{"binary"},
	CheckOrigin:     checkWebSocketOrigin,
}

// getProxmoxTLSConfig returns TLS configuration for Proxmox connections.
// TLS verification can be skipped via PROXMOX_SKIP_TLS_VERIFY=true for self-signed certs.
// In production with proper certificates, this should be false.
func getProxmoxTLSConfig() *tls.Config {
	skipVerify := os.Getenv("PROXMOX_SKIP_TLS_VERIFY") == "true" ||
		os.Getenv("PROXMOX_INSECURE") == "true" // Legacy env var support

	if skipVerify {
		// #nosec G402 -- Intentionally skipping TLS verification for self-signed Proxmox certs.
		// This is controlled by explicit environment variable and logged at startup.
		return &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Default: verify TLS certificates
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
}

// handleVNCProxy handles WebSocket connections and proxies them to Proxmox VNC
// GET /api/v1/pods/{podID}/vms/{vmName}/vnc
func (m *Manager) handleVNCProxy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		m.logger.Info("VNC proxy request", "podId", podID, "vmName", vmName)

		// Get the console ticket from orchestrator
		ticket, err := m.orchestrator.GetVMConsole(r.Context(), podID, vmName, "vnc")
		if err != nil {
			m.logger.Error("Failed to get VNC ticket", "error", err, "podId", podID, "vmName", vmName)
			http.Error(w, "Failed to get VNC access", http.StatusInternalServerError)
			return
		}

		if ticket.Type != "vnc" {
			m.logger.Error("VM does not support VNC", "type", ticket.Type, "podId", podID, "vmName", vmName)
			http.Error(w, "VM does not support VNC console", http.StatusBadRequest)
			return
		}

		// Build the Proxmox VNC WebSocket URL
		// Format: wss://{host}/api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={ticket}
		proxmoxHost := ticket.Host
		node := ticket.Node
		vmid := ticket.VMID
		port := ticket.Port
		vncTicket := url.QueryEscape(ticket.Ticket)

		proxmoxWsURL := fmt.Sprintf("wss://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%d&vncticket=%s",
			proxmoxHost, node, vmid, port, vncTicket)

		m.logger.Debug("Connecting to Proxmox VNC WebSocket", "url", proxmoxWsURL)

		// Connect to Proxmox VNC WebSocket
		dialer := websocket.Dialer{
			TLSClientConfig:  getProxmoxTLSConfig(),
			HandshakeTimeout: 10 * time.Second,
			Subprotocols:     []string{"binary"},
		}

		// Add authorization header for API token auth
		headers := http.Header{}
		authHeader := m.orchestrator.ProxmoxAuthHeader()
		if authHeader != "" {
			headers.Set("Authorization", authHeader)
			m.logger.Debug("Added Proxmox API token auth to WebSocket connection")
		}

		proxmoxConn, resp, err := dialer.Dial(proxmoxWsURL, headers)
		if err != nil {
			if resp != nil {
				m.logger.Error("Failed to connect to Proxmox VNC",
					"error", err,
					"statusCode", resp.StatusCode,
					"status", resp.Status,
				)
			} else {
				m.logger.Error("Failed to connect to Proxmox VNC", "error", err)
			}
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}
		defer proxmoxConn.Close()

		m.logger.Info("Connected to Proxmox VNC WebSocket", "podId", podID, "vmName", vmName)

		// Upgrade client connection to WebSocket
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.logger.Error("Failed to upgrade client connection", "error", err)
			return
		}
		defer clientConn.Close()

		m.logger.Info("Client WebSocket upgraded, starting proxy", "podId", podID, "vmName", vmName)

		// Proxy data between client and Proxmox
		var wg sync.WaitGroup
		wg.Add(2)

		// Error channels for both directions
		clientToProxmox := make(chan error, 1)
		proxmoxToClient := make(chan error, 1)

		// Client -> Proxmox
		go func() {
			defer wg.Done()
			err := proxyWebSocket(clientConn, proxmoxConn, "client->proxmox")
			clientToProxmox <- err
		}()

		// Proxmox -> Client
		go func() {
			defer wg.Done()
			err := proxyWebSocket(proxmoxConn, clientConn, "proxmox->client")
			proxmoxToClient <- err
		}()

		// Wait for either direction to close
		select {
		case err := <-clientToProxmox:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Client to Proxmox closed", "error", err)
			}
			proxmoxConn.Close()
		case err := <-proxmoxToClient:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Proxmox to client closed", "error", err)
			}
			clientConn.Close()
		}

		wg.Wait()
		m.logger.Info("VNC proxy session ended", "podId", podID, "vmName", vmName)
	}
}

// proxyWebSocket copies WebSocket messages from src to dst
func proxyWebSocket(src, dst *websocket.Conn, direction string) error {
	for {
		messageType, message, err := src.ReadMessage()
		if err != nil {
			return err
		}

		err = dst.WriteMessage(messageType, message)
		if err != nil {
			return err
		}
	}
}

// handleDirectVNCProxy handles VNC proxy for direct VM access (by vmid)
// GET /api/v1/proxmox/vms/{vmid}/vnc
func (m *Manager) handleDirectVNCProxy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vmidStr := chi.URLParam(r, "vmid")
		vmid, err := strconv.Atoi(vmidStr)
		if err != nil {
			http.Error(w, "Invalid vmid", http.StatusBadRequest)
			return
		}

		// Optional node parameter (defaults to "pve")
		node := r.URL.Query().Get("node")
		if node == "" {
			node = "pve"
		}
		if !reNodeName.MatchString(node) {
			http.Error(w, "Invalid node name", http.StatusBadRequest)
			return
		}

		m.logger.Info("Direct VNC proxy request", "vmid", vmid, "node", node)

		// Get VNC ticket directly from orchestrator
		ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmid, "vnc")
		if err != nil {
			m.logger.Error("Failed to get VNC ticket", "error", err, "vmid", vmid, "node", node)
			http.Error(w, "Failed to get VNC access", http.StatusInternalServerError)
			return
		}

		if ticket.Type != "vnc" {
			m.logger.Error("VM does not support VNC", "type", ticket.Type, "vmid", vmid)
			http.Error(w, "VM does not support VNC console", http.StatusBadRequest)
			return
		}

		// Build the Proxmox VNC WebSocket URL
		proxmoxHost := ticket.Host
		vncTicket := url.QueryEscape(ticket.Ticket)

		proxmoxWsURL := fmt.Sprintf("wss://%s/api2/json/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
			proxmoxHost, ticket.Node, vmid, ticket.Port, vncTicket)

		m.logger.Debug("Connecting to Proxmox VNC WebSocket", "url", proxmoxWsURL)

		// Connect to Proxmox VNC WebSocket
		dialer := websocket.Dialer{
			TLSClientConfig:  getProxmoxTLSConfig(),
			HandshakeTimeout: 10 * time.Second,
			Subprotocols:     []string{"binary"},
		}

		// Add authorization header for API token auth
		headers := http.Header{}
		authHeader := m.orchestrator.ProxmoxAuthHeader()
		if authHeader != "" {
			headers.Set("Authorization", authHeader)
			m.logger.Debug("Added Proxmox API token auth to WebSocket connection")
		}

		proxmoxConn, resp, err := dialer.Dial(proxmoxWsURL, headers)
		if err != nil {
			if resp != nil {
				m.logger.Error("Failed to connect to Proxmox VNC",
					"error", err,
					"statusCode", resp.StatusCode,
					"status", resp.Status,
				)
				if resp.Body != nil {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					resp.Body.Close()
					m.logger.Error("Proxmox response", "body", string(body))
				}
			} else {
				m.logger.Error("Failed to connect to Proxmox VNC", "error", err)
			}
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}
		defer proxmoxConn.Close()

		m.logger.Info("Connected to Proxmox VNC WebSocket", "vmid", vmid, "node", node)

		// Upgrade client connection to WebSocket
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.logger.Error("Failed to upgrade client connection", "error", err)
			return
		}
		defer clientConn.Close()

		m.logger.Info("Client WebSocket upgraded, starting proxy", "vmid", vmid)

		// Proxy data between client and Proxmox
		var wg sync.WaitGroup
		wg.Add(2)

		clientToProxmox := make(chan error, 1)
		proxmoxToClient := make(chan error, 1)

		go func() {
			defer wg.Done()
			err := proxyWebSocket(clientConn, proxmoxConn, "client->proxmox")
			clientToProxmox <- err
		}()

		go func() {
			defer wg.Done()
			err := proxyWebSocket(proxmoxConn, clientConn, "proxmox->client")
			proxmoxToClient <- err
		}()

		select {
		case err := <-clientToProxmox:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Client to Proxmox closed", "error", err)
			}
			proxmoxConn.Close()
		case err := <-proxmoxToClient:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Proxmox to client closed", "error", err)
			}
			clientConn.Close()
		}

		wg.Wait()
		m.logger.Info("Direct VNC proxy session ended", "vmid", vmid)
	}
}

// handleVNCProxyWithTicket handles VNC proxy using a ticket from query params
// This is an alternative endpoint that accepts the ticket directly
// GET /api/v1/vnc/proxy?host={host}&port={port}&node={node}&vmid={vmid}&ticket={ticket}
func (m *Manager) handleVNCProxyWithTicket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.URL.Query().Get("host")
		portStr := r.URL.Query().Get("port")
		node := r.URL.Query().Get("node")
		vmid := r.URL.Query().Get("vmid")
		ticket := r.URL.Query().Get("ticket")

		if host == "" || portStr == "" || node == "" || vmid == "" || ticket == "" {
			http.Error(w, "Missing required parameters: host, port, node, vmid, ticket", http.StatusBadRequest)
			return
		}
		if !reNodeName.MatchString(node) {
			http.Error(w, "Invalid node name", http.StatusBadRequest)
			return
		}

		// Validate host against configured Proxmox host to prevent SSRF
		allowedHost := m.orchestrator.ProxmoxHostname()
		if allowedHost == "" {
			http.Error(w, "Proxmox host not configured", http.StatusInternalServerError)
			return
		}
		if extractHostname(host) != extractHostname(allowedHost) {
			m.logger.Warn("SSRF attempt: host mismatch", "requested", host, "allowed", allowedHost)
			http.Error(w, "Forbidden: invalid host", http.StatusForbidden)
			return
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			http.Error(w, "Invalid port number", http.StatusBadRequest)
			return
		}

		m.logger.Info("VNC proxy with ticket request", "host", host, "vmid", vmid, "node", node)

		// Build the Proxmox VNC WebSocket URL
		vncTicket := url.QueryEscape(ticket)
		proxmoxWsURL := fmt.Sprintf("wss://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%d&vncticket=%s",
			host, node, vmid, port, vncTicket)

		// Connect to Proxmox VNC WebSocket
		dialer := websocket.Dialer{
			TLSClientConfig:  getProxmoxTLSConfig(),
			HandshakeTimeout: 10 * time.Second,
			Subprotocols:     []string{"binary"},
		}

		// Add authorization header for API token auth
		headers := http.Header{}
		authHeader := m.orchestrator.ProxmoxAuthHeader()
		if authHeader != "" {
			headers.Set("Authorization", authHeader)
			m.logger.Debug("Added Proxmox API token auth to WebSocket connection")
		}

		proxmoxConn, resp, err := dialer.Dial(proxmoxWsURL, headers)
		if err != nil {
			if resp != nil {
				m.logger.Error("Failed to connect to Proxmox VNC",
					"error", err,
					"statusCode", resp.StatusCode,
				)
				if resp.Body != nil {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
					resp.Body.Close()
					m.logger.Error("Proxmox response", "body", string(body))
				}
			} else {
				m.logger.Error("Failed to connect to Proxmox VNC", "error", err)
			}
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}
		defer proxmoxConn.Close()

		// Upgrade client connection to WebSocket
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.logger.Error("Failed to upgrade client connection", "error", err)
			return
		}
		defer clientConn.Close()

		m.logger.Info("VNC proxy session started", "vmid", vmid)

		// Proxy data between client and Proxmox
		var wg sync.WaitGroup
		wg.Add(2)

		clientToProxmox := make(chan error, 1)
		proxmoxToClient := make(chan error, 1)

		go func() {
			defer wg.Done()
			err := proxyWebSocket(clientConn, proxmoxConn, "client->proxmox")
			clientToProxmox <- err
		}()

		go func() {
			defer wg.Done()
			err := proxyWebSocket(proxmoxConn, clientConn, "proxmox->client")
			proxmoxToClient <- err
		}()

		select {
		case err := <-clientToProxmox:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Client to Proxmox closed", "error", err)
			}
			proxmoxConn.Close()
		case err := <-proxmoxToClient:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Proxmox to client closed", "error", err)
			}
			clientConn.Close()
		}

		wg.Wait()
		m.logger.Info("VNC proxy session ended", "vmid", vmid)
	}
}

// extractHostname strips the port from a host:port string, returning just the hostname.
func extractHostname(hostport string) string {
	if idx := strings.LastIndex(hostport, ":"); idx != -1 {
		// Check it's not part of an IPv6 address
		if !strings.Contains(hostport, "]") || strings.HasSuffix(hostport[:idx], "]") {
			return hostport[:idx]
		}
	}
	return hostport
}
