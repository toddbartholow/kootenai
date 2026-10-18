# LTI VNC Console Troubleshooting Guide

This document covers the problems encountered and solutions implemented when integrating noVNC console access through Canvas LTI.

## Overview

The LTI launch flow creates a lab pod for students and provides VNC console access to VMs directly within Canvas. Several issues were encountered during implementation.

---

## Problem 1: noVNC CDN CORS Errors

### Symptoms
```
Loading module from "https://cdn.jsdelivr.net/npm/@novnc/novnc@1.4.0/..."
was blocked because of a disallowed MIME type ("text/plain")
```

Canvas's Content Security Policy (CSP) blocks loading JavaScript modules from external CDNs.

### Solution
Self-host noVNC files within the API server using Go's `embed` directive.

**Files Created:**
- `api/internal/server/static.go` - Embedded filesystem handler
- `api/internal/server/static/novnc/` - noVNC v1.4.0 files

**Key Code (`static.go`):**
```go
//go:embed static/*
var staticFiles embed.FS

// Custom response writer to set correct MIME types
type mimeResponseWriter struct {
    http.ResponseWriter
    mimeType    string
    wroteHeader bool
}

func (w *mimeResponseWriter) WriteHeader(code int) {
    if !w.wroteHeader && w.mimeType != "" {
        w.Header().Set("Content-Type", w.mimeType)
    }
    w.wroteHeader = true
    w.ResponseWriter.WriteHeader(code)
}
```

**Route added to `server.go`:**
```go
r.Mount("/static/", http.StripPrefix("/static/", s.StaticHandler()))
```

---

## Problem 2: JavaScript MIME Type Errors

### Symptoms
```
Loading module was blocked because of a disallowed MIME type ("text/plain")
```

Go's `http.FileServer` was serving `.js` files as `text/plain` instead of `application/javascript`.

### Solution
Wrap the file server response writer to override Content-Type based on file extension.

**MIME type mapping in `static.go`:**
```go
var mimeTypes = map[string]string{
    ".js":    "application/javascript",
    ".mjs":   "application/javascript",
    ".css":   "text/css",
    ".html":  "text/html",
    // ... other types
}
```

---

## Problem 3: Missing Vendor Files (404 Errors)

### Symptoms
```
GET /static/novnc/vendor/pako/lib/zlib/inflate.js 404 Not Found
```

noVNC requires the pako library for compression, located in the `vendor/` subdirectory.

### Solution
Ensure the complete noVNC distribution is synced, including the `vendor/` folder:
```
api/internal/server/static/novnc/
├── core/
│   ├── rfb.js
│   ├── input/
│   ├── decoders/
│   └── ...
└── vendor/
    └── pako/
        └── lib/
            └── zlib/
                ├── inflate.js
                ├── deflate.js
                └── ...
```

---

## Problem 4: VMs Not Starting Automatically

### Symptoms
After LTI launch, pods were created but VMs remained stopped.

### Root Cause
The `StartPod` method only starts pods with status "stopped", but `CreatePod` sets the initial status to "running" (optimistic status).

### Solution
Added `StartVMDirect` method to orchestrator that bypasses pod status check:

**`orchestrator.go`:**
```go
func (o *Orchestrator) StartVMDirect(ctx context.Context, node string, vmid int) error {
    if o.proxmox == nil {
        return fmt.Errorf("proxmox client not configured")
    }
    o.logger.Info("Starting VM directly", "node", node, "vmid", vmid)
    return o.proxmox.StartVM(ctx, node, vmid)
}
```

**`lti_handlers.go` - after pod creation:**
```go
for _, vm := range pod.VMs {
    if vm.Platform == "proxmox" && vm.PlatformID != "" {
        vmid := 0
        if _, err := fmt.Sscanf(vm.PlatformID, "%d", &vmid); err == nil && vmid > 0 {
            if err := s.orchestrator.StartVMDirect(ctx, vm.Node, vmid); err != nil {
                s.logger.Error("Failed to start VM", "error", err, "vmid", vmid)
            }
        }
    }
}
```

---

## Problem 5: VNC Authentication Failed

### Symptoms
```
Failed when connecting: Security negotiation failed on security result
(reason: Authentication failed)
```

VNC WebSocket connected to Proxmox but RFB authentication failed.

### Root Cause
The LTI console was using a different VNC proxy endpoint than the working web UI:

| Component | Endpoint | Behavior |
|-----------|----------|----------|
| Web UI (works) | `/api/v1/proxmox/vms/:vmid/vnc` | `handleDirectVNCProxy` gets fresh ticket |
| LTI (broken) | `/api/v1/vnc/proxy?ticket=...` | `handleVNCProxyWithTicket` uses passed ticket |

The issue: When using `handleVNCProxyWithTicket`, the ticket passed in the URL was different from the ticket the proxy used to connect to Proxmox. Proxmox expects the RFB password to match the ticket in the WebSocket URL.

### Solution
Changed LTI console to use the same endpoint as the working web UI:

**Before (broken):**
```go
vncWSURL := fmt.Sprintf("%s://%s/api/v1/vnc/proxy?host=%s&port=%d&node=%s&vmid=%s&ticket=%s",
    wsProtocol, host, ticket.Host, ticket.Port, ticket.Node, vmID, url.QueryEscape(ticket.Ticket))
```

**After (working):**
```go
vncWSURL := fmt.Sprintf("%s://%s/api/v1/proxmox/vms/%s/vnc?node=%s",
    wsProtocol, host, vmID, node)
```

The `handleDirectVNCProxy` handler gets its own fresh ticket from Proxmox, ensuring the WebSocket URL ticket and RFB password match.

---

## Problem 6: VNC Password Not Rendered in JavaScript

### Symptoms
The `vncPassword` variable was empty in the rendered HTML, causing authentication to fail with an empty password.

### Root Cause
The Docker container builds from source on the infra VM, not from the pre-built binary. Changes to source files on the development machine weren't reflected until synced.

### Solution
Sync source files AND rebuild the Docker container:

```bash
# 1. Sync the updated source file
scp api/internal/server/lti_handlers.go labadmin@<INFRA_VM_IP>:~/kootenai/api/internal/server/

# 2. Rebuild the container (--no-cache ensures fresh build)
ssh labadmin@<INFRA_VM_IP> 'cd ~/kootenai/deploy && docker compose build --no-cache api'

# 3. Restart the container
ssh labadmin@<INFRA_VM_IP> 'cd ~/kootenai/deploy && docker compose up -d api'
```

**Note:** Simply running `docker compose restart api` does NOT rebuild - it only restarts the existing container with the old code.

---

## Problem 7: Missing Proxmox Authorization Header

### Symptoms
VNC proxy connections to Proxmox failed with 401 Unauthorized.

### Root Cause
The `handleVNCProxyWithTicket` function was missing the Proxmox API authorization header.

### Solution
Added authorization header to WebSocket dial:

```go
headers := http.Header{}
authHeader := s.orchestrator.ProxmoxAuthHeader()
if authHeader != "" {
    headers.Set("Authorization", authHeader)
}
proxmoxConn, resp, err := dialer.Dial(proxmoxWsURL, headers)
```

---

## Deployment Checklist

When making changes to LTI console code:

1. **Edit source files** on development machine
2. **Sync to infra VM:**
   ```bash
   scp api/internal/server/lti_handlers.go labadmin@<INFRA_VM_IP>:~/kootenai/api/internal/server/
   ```
3. **Rebuild Docker container:**
   ```bash
   ssh labadmin@<INFRA_VM_IP> 'cd ~/kootenai/deploy && docker compose build --no-cache api && docker compose up -d api'
   ```
4. **Verify changes:**
   ```bash
   curl -s "http://<INFRA_VM_IP>:8080/lti/console?vmid=173835&node=pve" | grep "const vncPassword"
   ```
5. **Test in browser:**
   ```
   http://<INFRA_VM_IP>:8080/lti/console?vmid=173835&node=pve
   ```

---

## Architecture Summary

```
Canvas LTI Launch
       │
       ▼
┌──────────────────┐
│ handleLTICallback │ ─── Validates LTI token, creates pod
└────────┬─────────┘
         │
         ▼
┌────────────────────┐
│ renderVNCConsolePage│ ─── Shows "Open Console" button
└────────┬───────────┘
         │ (user clicks)
         ▼
┌─────────────────┐
│ handleLTIConsole │ ─── Serves noVNC page with embedded ticket
└────────┬────────┘
         │
         ▼
┌─────────────────────┐
│ noVNC JavaScript    │ ─── Connects WebSocket, sends password
└────────┬────────────┘
         │
         ▼
┌──────────────────────┐
│ handleDirectVNCProxy │ ─── Gets fresh ticket, proxies to Proxmox
└────────┬─────────────┘
         │
         ▼
┌─────────────────┐
│ Proxmox VNC     │ ─── VM console display
└─────────────────┘
```

---

## Related Files

| File | Purpose |
|------|---------|
| `api/internal/server/lti_handlers.go` | LTI launch and console handlers |
| `api/internal/server/vnc_proxy_handler.go` | WebSocket VNC proxy |
| `api/internal/server/static.go` | Embedded noVNC file server |
| `api/internal/server/static/novnc/` | Self-hosted noVNC files |
| `api/internal/orchestrator/orchestrator.go` | Pod/VM management |
