package consoleaccess

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// handleSPICEProxy handles WebSocket connections and proxies them to Proxmox SPICE
// GET /api/v1/pods/{podID}/vms/{vmName}/spice
func (m *Manager) handleSPICEProxy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		podID := chi.URLParam(r, "podID")
		vmName := chi.URLParam(r, "vmName")

		m.logger.Info("SPICE proxy request", "podId", podID, "vmName", vmName)

		// Get the console ticket from orchestrator (request SPICE type)
		ticket, err := m.orchestrator.GetVMConsole(r.Context(), podID, vmName, "spice")
		if err != nil {
			m.logger.Error("Failed to get SPICE ticket", "error", err, "podId", podID, "vmName", vmName)
			http.Error(w, "Failed to get SPICE access", http.StatusInternalServerError)
			return
		}

		if ticket.Type != "spice" {
			m.logger.Error("VM does not support SPICE", "type", ticket.Type, "podId", podID, "vmName", vmName)
			http.Error(w, "VM does not support SPICE console", http.StatusBadRequest)
			return
		}

		// SPICE connects to the Proxmox host on the dynamically assigned TLS port
		// NOTE: ticket.Host contains a special SPICE proxy authentication string (e.g., "pvespiceproxy:...")
		// NOT the actual Proxmox hostname. We must use the real Proxmox IP/hostname from config.
		spiceHost := m.orchestrator.ProxmoxHostname()
		if spiceHost == "" {
			m.logger.Error("Proxmox hostname not configured")
			http.Error(w, "Proxmox not configured", http.StatusInternalServerError)
			return
		}

		// Port 3128 is the Proxmox SPICE proxy port (plain HTTP CONNECT proxy)
		spicePort := "3128"
		spiceAddr := net.JoinHostPort(spiceHost, spicePort)
		m.logger.Debug("Connecting to Proxmox SPICE proxy", "addr", spiceAddr, "ticketHost", ticket.Host, "tlsPort", ticket.TLSPort)

		// Connect to Proxmox SPICE proxy with plain TCP
		conn, err := net.DialTimeout("tcp", spiceAddr, 10*time.Second)
		if err != nil {
			m.logger.Error("Failed to connect to Proxmox SPICE proxy",
				"error", err,
				"addr", spiceAddr,
			)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}
		defer conn.Close()

		// Send HTTP CONNECT request to the SPICE proxy
		// The ticket.Host contains the authentication string (e.g., "pvespiceproxy:...")
		// that the proxy uses to authenticate and route to the correct VM
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", ticket.Host, ticket.Host)
		m.logger.Debug("Sending CONNECT request", "host", ticket.Host)

		_, err = conn.Write([]byte(connectReq))
		if err != nil {
			m.logger.Error("Failed to send CONNECT request", "error", err)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}

		// Read the response from the proxy
		reader := bufio.NewReader(conn)
		resp, err := reader.ReadString('\n')
		if err != nil {
			m.logger.Error("Failed to read CONNECT response", "error", err)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}

		m.logger.Debug("CONNECT response", "response", strings.TrimSpace(resp))

		// Check if the CONNECT was successful (should be "HTTP/1.1 200 OK" or similar)
		if !strings.Contains(resp, "200") {
			m.logger.Error("CONNECT request failed", "response", resp)
			http.Error(w, "Failed to connect to VM console: proxy rejected connection", http.StatusBadGateway)
			return
		}

		// Read the rest of the HTTP headers (empty line terminates)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				m.logger.Error("Failed to read CONNECT headers", "error", err)
				http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
				return
			}
			if strings.TrimSpace(line) == "" {
				break
			}
		}

		m.logger.Info("Connected to Proxmox SPICE via proxy", "addr", spiceAddr)

		// Upgrade client connection to WebSocket
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.logger.Error("Failed to upgrade client connection", "error", err)
			return
		}
		defer clientConn.Close()

		m.logger.Info("Client WebSocket upgraded, starting SPICE proxy", "podId", podID, "vmName", vmName)

		// NOTE: The SPICE password is passed to the client via the console API ticket endpoint.
		// The spice-html5 library handles SPICE authentication directly using the password.
		// This proxy just passes binary SPICE protocol data through.

		// Proxy data between client WebSocket and SPICE TCP connection
		var wg sync.WaitGroup
		wg.Add(2)

		// Error channels for both directions
		clientToSpice := make(chan error, 1)
		spiceToClient := make(chan error, 1)

		// Client WebSocket -> SPICE TCP
		go func() {
			defer wg.Done()
			for {
				_, message, err := clientConn.ReadMessage()
				if err != nil {
					clientToSpice <- err
					return
				}
				_, err = conn.Write(message)
				if err != nil {
					clientToSpice <- err
					return
				}
			}
		}()

		// SPICE TCP -> Client WebSocket
		go func() {
			defer wg.Done()
			buf := make([]byte, 4096)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					spiceToClient <- err
					return
				}
				err = clientConn.WriteMessage(websocket.BinaryMessage, buf[:n])
				if err != nil {
					spiceToClient <- err
					return
				}
			}
		}()

		// Wait for either direction to close
		select {
		case err := <-clientToSpice:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Client to SPICE closed", "error", err)
			}
			conn.Close()
		case err := <-spiceToClient:
			if err != nil {
				m.logger.Debug("SPICE to client closed", "error", err)
			}
			clientConn.Close()
		}

		wg.Wait()
		m.logger.Info("SPICE proxy session ended", "podId", podID, "vmName", vmName)
	}
}

// handleDirectSPICEProxy handles SPICE proxy for direct VM access (by vmid)
// GET /api/v1/proxmox/vms/{vmid}/spice
func (m *Manager) handleDirectSPICEProxy() http.HandlerFunc {
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

		m.logger.Info("Direct SPICE proxy request", "vmid", vmid, "node", node)

		// Get SPICE ticket directly from orchestrator
		ticket, err := m.orchestrator.GetDirectVMConsole(r.Context(), node, vmid, "spice")
		if err != nil {
			m.logger.Error("Failed to get SPICE ticket", "error", err, "vmid", vmid, "node", node)
			http.Error(w, "Failed to get SPICE access", http.StatusInternalServerError)
			return
		}

		if ticket.Type != "spice" {
			m.logger.Error("VM does not support SPICE", "type", ticket.Type, "vmid", vmid)
			http.Error(w, "VM does not support SPICE console", http.StatusBadRequest)
			return
		}

		// SPICE connects to the Proxmox host on the dynamically assigned TLS port
		// NOTE: ticket.Host contains a special SPICE proxy authentication string (e.g., "pvespiceproxy:...")
		// NOT the actual Proxmox hostname. We must use the real Proxmox IP/hostname from config.
		spiceHost := m.orchestrator.ProxmoxHostname()
		if spiceHost == "" {
			m.logger.Error("Proxmox hostname not configured")
			http.Error(w, "Proxmox not configured", http.StatusInternalServerError)
			return
		}

		// Port 3128 is the Proxmox SPICE proxy port (plain HTTP CONNECT proxy)
		spicePort := "3128"
		spiceAddr := net.JoinHostPort(spiceHost, spicePort)
		m.logger.Debug("Connecting to Proxmox SPICE proxy", "addr", spiceAddr, "ticketHost", ticket.Host, "tlsPort", ticket.TLSPort)

		// Connect to Proxmox SPICE proxy with plain TCP
		conn, err := net.DialTimeout("tcp", spiceAddr, 10*time.Second)
		if err != nil {
			m.logger.Error("Failed to connect to Proxmox SPICE proxy",
				"error", err,
				"addr", spiceAddr,
			)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}
		defer conn.Close()

		// Send HTTP CONNECT request to the SPICE proxy
		// The ticket.Host contains the authentication string (e.g., "pvespiceproxy:...")
		// that the proxy uses to authenticate and route to the correct VM
		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", ticket.Host, ticket.Host)
		m.logger.Debug("Sending CONNECT request", "host", ticket.Host)

		_, err = conn.Write([]byte(connectReq))
		if err != nil {
			m.logger.Error("Failed to send CONNECT request", "error", err)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}

		// Read the response from the proxy
		reader := bufio.NewReader(conn)
		resp, err := reader.ReadString('\n')
		if err != nil {
			m.logger.Error("Failed to read CONNECT response", "error", err)
			http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
			return
		}

		m.logger.Debug("CONNECT response", "response", strings.TrimSpace(resp))

		// Check if the CONNECT was successful (should be "HTTP/1.1 200 OK" or similar)
		if !strings.Contains(resp, "200") {
			m.logger.Error("CONNECT request failed", "response", resp)
			http.Error(w, "Failed to connect to VM console: proxy rejected connection", http.StatusBadGateway)
			return
		}

		// Read the rest of the HTTP headers (empty line terminates)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				m.logger.Error("Failed to read CONNECT headers", "error", err)
				http.Error(w, "Failed to connect to VM console", http.StatusBadGateway)
				return
			}
			if strings.TrimSpace(line) == "" {
				break
			}
		}

		m.logger.Info("Connected to Proxmox SPICE via proxy", "vmid", vmid, "node", node)

		// Upgrade client connection to WebSocket
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			m.logger.Error("Failed to upgrade client connection", "error", err)
			return
		}
		defer clientConn.Close()

		m.logger.Info("Client WebSocket upgraded, starting SPICE proxy", "vmid", vmid)

		// Proxy data between client WebSocket and SPICE TCP connection
		var wg sync.WaitGroup
		wg.Add(2)

		clientToSpice := make(chan error, 1)
		spiceToClient := make(chan error, 1)

		// Client WebSocket -> SPICE TCP
		go func() {
			defer wg.Done()
			for {
				_, message, err := clientConn.ReadMessage()
				if err != nil {
					clientToSpice <- err
					return
				}
				_, err = conn.Write(message)
				if err != nil {
					clientToSpice <- err
					return
				}
			}
		}()

		// SPICE TCP -> Client WebSocket
		go func() {
			defer wg.Done()
			buf := make([]byte, 4096)
			for {
				n, err := conn.Read(buf)
				if err != nil {
					spiceToClient <- err
					return
				}
				err = clientConn.WriteMessage(websocket.BinaryMessage, buf[:n])
				if err != nil {
					spiceToClient <- err
					return
				}
			}
		}()

		select {
		case err := <-clientToSpice:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				m.logger.Debug("Client to SPICE closed", "error", err)
			}
			conn.Close()
		case err := <-spiceToClient:
			if err != nil {
				m.logger.Debug("SPICE to client closed", "error", err)
			}
			clientConn.Close()
		}

		wg.Wait()
		m.logger.Info("Direct SPICE proxy session ended", "vmid", vmid)
	}
}
