# VNC Console Implementation Guide

This document describes the implementation of the embedded VNC console for Proxmox VMs in the Kootenai web interface, including the challenges encountered and their solutions.

## Overview

The VNC console allows users to access Proxmox VM consoles directly from the web UI using the noVNC library. The implementation involves:

1. **Frontend**: Vue.js component using `@novnc/novnc` library
2. **Backend**: Go WebSocket proxy that bridges the browser to Proxmox's VNC WebSocket API
3. **Vite Config**: Dev server proxy configuration for WebSocket support

## Architecture

```
Browser (noVNC)
    ↓ WebSocket
Vite Dev Server (localhost:3000)
    ↓ Proxy
Go API Server (localhost:8080)
    ↓ WebSocket
Proxmox VNC WebSocket API
    ↓
VM Console
```

## Issues and Solutions

### Issue 1: "Error 401: No Ticket"

**Symptom**: Proxmox returned 401 authentication errors when trying to connect to VNC.

**Root Cause**: Proxmox's noVNC web console uses session-based authentication. When accessing `/api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket`, Proxmox expects either:
- A valid session cookie (from web UI login)
- API token authentication via header

**Solution**: Created a WebSocket proxy in the Go backend that:
1. Obtains a VNC ticket from Proxmox API using the API token
2. Connects to Proxmox's VNC WebSocket with proper authentication
3. Proxies data between the browser and Proxmox

**Files Changed**:
- `api/internal/server/vnc_proxy_handler.go` - WebSocket proxy handler
- `api/internal/server/server.go` - Added route `/api/v1/proxmox/vms/{vmid}/vnc`

### Issue 2: "Security Negotiation Failed - Authentication Failed"

**Symptom**: noVNC connected to the WebSocket but failed during VNC security handshake.

**Root Cause**: The VNC ticket returned by Proxmox IS the RFB password. The code was incorrectly looking for a separate `password` field.

**Solution**: Use `ticket.ticket` as the RFB credentials password:

```typescript
// WRONG - ticket.password is empty for VNC
const vncPassword = props.ticket.password || ''

// CORRECT - The VNC ticket IS the password
const vncPassword = props.ticket.ticket || props.ticket.password || ''
```

**Key Insight**: Proxmox VNC tickets serve dual purpose:
1. URL parameter for WebSocket connection (`vncticket=...`)
2. RFB authentication password for noVNC client

### Issue 3: Slow Connection (10+ seconds delay)

**Symptom**: Console took 10+ seconds to become interactive after clicking "VNC Console".

**Root Cause**: The noVNC RFB module was being dynamically imported inside the `connect()` function, adding latency every time.

**Solution**: Pre-load the noVNC module at component load time:

```typescript
// Pre-load noVNC module on component load for faster connection
let RFBClass: any = null
import('@novnc/novnc/lib/rfb.js').then(m => {
  RFBClass = m.default
  console.log('noVNC RFB class pre-loaded')
}).catch(() => {
  console.warn('Failed to pre-load noVNC')
})

async function connect() {
  // Use pre-loaded RFB class if available
  let RFB = RFBClass
  if (!RFB) {
    RFB = await import('@novnc/novnc/lib/rfb.js').then(m => m.default).catch(() => null)
  }
  // ...
}
```

### Issue 4: Display Dimensions/Scaling

**Symptom**: Console display had incorrect dimensions - too much black space, content at wrong position.

**Root Cause**:
1. Fixed height container didn't match VM aspect ratios
2. noVNC creates its own canvas, redundant canvas element caused confusion
3. No CSS to properly scale the noVNC canvas

**Solution**:

1. Use aspect-ratio CSS for responsive container:
```html
<div
  ref="containerRef"
  class="relative bg-black vnc-container"
  style="aspect-ratio: 16/10; width: 100%; max-height: 70vh;"
>
```

2. Remove redundant canvas element (noVNC creates its own)

3. Add scoped CSS to style noVNC's internal canvas:
```css
<style scoped>
.vnc-container :deep(canvas) {
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
}

.vnc-container {
  overflow: hidden;
}
</style>
```

4. Configure RFB scaling options:
```typescript
rfb.scaleViewport = true
rfb.resizeSession = true
rfb.clipViewport = false
rfb.dragViewport = false
```

**Note**: The warning "Server did not accept the resize request: Invalid screen layout" is expected when the VM doesn't support dynamic resolution changes (common for text-mode terminals).

### Issue 5: WebSocket Proxy Not Working in Development

**Symptom**: WebSocket connections failed through Vite dev server.

**Root Cause**: Vite's proxy config didn't have WebSocket support enabled.

**Solution**: Enable WebSocket proxying in `vite.config.ts`:

```typescript
server: {
  port: 3000,
  proxy: {
    '/api': {
      target: apiBaseUrl,
      changeOrigin: true,
      ws: true, // Enable WebSocket for VNC proxy endpoints
    },
  },
}
```

## Final Implementation

### Backend (Go)

The WebSocket proxy handler (`vnc_proxy_handler.go`):
1. Receives WebSocket upgrade request from browser
2. Gets VNC ticket from Proxmox API (includes port, ticket, node info)
3. Connects to Proxmox VNC WebSocket with API token auth
4. Upgrades browser connection to WebSocket
5. Bidirectionally proxies data between browser and Proxmox

### Frontend (Vue.js)

The VncConsole component (`VncConsole.vue`):
1. Pre-loads noVNC RFB module at component creation
2. Builds WebSocket URL pointing to our proxy endpoint
3. Creates RFB instance with ticket as password
4. Handles connect/disconnect/error events
5. Provides a **draggable floating window** with Proxmox-style toolbar
6. Shows fallback to Proxmox native console if noVNC fails

#### Draggable Window

The console appears as a floating window that can be:
- **Dragged** by the title bar to reposition on screen
- **Centered** automatically on open
- **Constrained** to stay within viewport bounds

```typescript
// Window position and dragging
const windowPosition = ref({ x: 50, y: 50 })
const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })

function startDrag(e: MouseEvent) {
  if (!windowRef.value) return
  isDragging.value = true
  const rect = windowRef.value.getBoundingClientRect()
  dragOffset.value = {
    x: e.clientX - rect.left,
    y: e.clientY - rect.top
  }
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
}
```

#### Proxmox-Style Left Sidebar Toolbar

The toolbar provides all essential VM console controls:

| Button | Function | X11 Keysym |
|--------|----------|------------|
| **Ctrl** | Sticky Ctrl modifier (toggles on/off) | `0xFFE3` (XK_Control_L) |
| **Alt** | Sticky Alt modifier (toggles on/off) | `0xFFE9` (XK_Alt_L) |
| **Windows** | Send Windows/Super key | `0xFFEB` (XK_Super_L) |
| **Tab** | Send Tab key | `0xFF09` (XK_Tab) |
| **Esc** | Send Escape key | `0xFF1B` (XK_Escape) |
| **Ctrl+Alt+Del** | Send Ctrl+Alt+Delete sequence | Built-in RFB method |
| **Fullscreen** | Toggle fullscreen mode | N/A |
| **Settings** | Show connection info panel | N/A |
| **Disconnect** | Close VNC connection | N/A |

**Sticky Keys**: Ctrl and Alt are "sticky" - clicking them toggles the modifier state, allowing you to click Ctrl, then type a letter to send Ctrl+letter combinations. The buttons highlight blue when active.

```typescript
// Send Ctrl key (sticky toggle)
function toggleCtrl() {
  ctrlActive.value = !ctrlActive.value
  if (rfb) {
    rfb.sendKey(0xFFE3, ctrlActive.value) // XK_Control_L
  }
}
```

The toolbar can be collapsed to a thin strip to maximize console viewing area.

### Key Files

| File | Purpose |
|------|---------|
| `web/src/components/console/VncConsole.vue` | VNC console Vue component |
| `api/internal/server/vnc_proxy_handler.go` | WebSocket proxy handlers |
| `api/internal/server/server.go` | Route registration |
| `api/internal/orchestrator/orchestrator.go` | `GetDirectVMConsole()` method |
| `web/vite.config.ts` | Dev server WebSocket proxy config |
| `web/src/api/client.ts` | `proxmoxApi.getDirectConsole()` API client |

## Dependencies

- `@novnc/novnc@1.5.0` - noVNC library for VNC-over-WebSocket
- `github.com/gorilla/websocket` - Go WebSocket library

## Testing

1. Start the API server with Proxmox credentials configured
2. Start the Vite dev server
3. Navigate to `/proxmox-vms`
4. Click "VNC Console" on a running VM
5. Verify:
   - Console shows "connected" status quickly (< 2 seconds)
   - VM display is visible and properly scaled
   - Window can be dragged by title bar
   - Window stays within viewport bounds
   - Keyboard/mouse input works
   - **Toolbar features**:
     - Ctrl button toggles sticky (highlights blue when active)
     - Alt button toggles sticky (highlights blue when active)
     - Windows key sends Super key to VM
     - Tab/Esc buttons send correct keys
     - Ctrl+Alt+Del button sends the key sequence
     - Settings button shows connection info panel
     - Toolbar collapses/expands correctly
   - Fullscreen toggle works
   - Disconnect properly closes connection

## Troubleshooting

### Console stays on "Connecting..."
- Check browser console for WebSocket errors
- Verify API server is running and accessible
- Check Proxmox API token permissions

### "Security error" or authentication failures
- Verify the ticket is being passed as RFB password
- Check Proxmox API token has VNC access permissions
- Ensure ticket hasn't expired (they're short-lived)

### Black screen after connecting
- VM may be powered off or at boot screen
- Try sending Ctrl+Alt+Del to wake the VM
- Check if VM has graphical output configured

### Display scaling issues
- Verify `scaleViewport: true` is set on RFB instance
- Check container has proper dimensions
- Some VMs don't support dynamic resize - this is normal
