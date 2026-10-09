---
title: WebSocket Events
description: Real-time WebSocket API for pod status, session progress, and achievements
tags:
  - api
  - websocket
  - real-time
  - events
---

# WebSocket Events

Kootenai uses WebSocket connections for real-time updates. This document covers connection
setup, event types, and message formats.

## Overview

WebSocket provides real-time updates for checkpoint evaluation, session and grade changes,
pod provisioning progress, assessment updates, hint nudges, and the admin monitoring feed.

## Connecting

The routes are registered **inside** `r.Route("/api/v1", ...)`
(`api/internal/server/websocket_handlers.go:66-69`, mounted at
`api/internal/server/server.go:1611-1613`), so both paths carry the `/api/v1` prefix.

### General WebSocket

For user-wide notifications:

```
wss://api.example.com/api/v1/ws
```

### Pod-Specific WebSocket

For pod-specific events:

```
wss://api.example.com/api/v1/ws/{podID}
```

Both accept an optional `?sessionId=` query parameter to scope the stream to one session.

### Authentication

`extractWebSocketToken` (`websocket_handlers.go:73-100`) reads exactly three sources, in
this order. **`?token=` is not one of them** — a query-parameter token is ignored entirely.

**1. `Authorization` header.** A `Bearer ` prefix is stripped if present; otherwise the whole
header value is taken as the token. Browsers cannot set this on an upgrade, so it is mainly
for non-browser clients.

```
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

**2. `Sec-WebSocket-Protocol`** — the browser-friendly route. The token is joined to the
prefix `access_token.` with a **dot**, as a **single** subprotocol string:

```javascript
const ws = new WebSocket(
  'wss://api.example.com/api/v1/ws',
  [`access_token.${token}`]        // ONE string, dot-joined
);
```

:::caution Two-element arrays do not work
`['access_token', token]` — the form this page used to show — sends two separate
subprotocols, neither of which has the `access_token.` prefix, so the token is never found.
The prefix includes the dot.
:::

**3. The `auth_token` cookie.** Browsers do send cookies on a WebSocket upgrade, so if the
API set an HttpOnly session cookie at login, the connection authenticates with no extra work.

If no auth service is configured, or demo mode is on, the handler skips token extraction
entirely and treats the connection as a fixed demo user.

### Connection Lifecycle

```mermaid
sequenceDiagram
    participant Client
    participant Server

    Client->>Server: Connect (with token)
    Server->>Server: Validate token
    Server-->>Client: Connection accepted

    loop While connected
        Server-->>Client: Event message
        Client-->>Server: Ping (keepalive)
        Server-->>Client: Pong
    end

    Client->>Server: Close
    Server-->>Client: Close acknowledged
```

## Message Format

Every frame is one `events.WebSocketMessage` (`api/internal/events/messages.go:182-186`):

```json
{
  "type": "checkpoint",
  "subject": "kootenai.checkpoint.updated",
  "payload": { }
}
```

:::caution The envelope is `{type, subject, payload}`
Not `{type, timestamp, data}`. There is no top-level `timestamp` — the event's own timestamp
lives inside `payload`, and `subject` is the NATS subject the message arrived on. Client code
written against `message.data` will read `undefined` on every frame.
:::

## Event Types

The complete set, from the `Broadcast*` methods on the hub
(`api/internal/websocket/hub.go:327-415`). Type names are **flat lowercase with
underscores** — there are no dotted names such as `pod.status` or `achievement.earned`, and
no achievement or system-alert events are broadcast at all.

| `type` | Payload | Broadcast to |
|--------|---------|--------------|
| `checkpoint` | `events.CheckpointUpdate` | Clients on that pod / session |
| `session` | `events.SessionEvent` | Clients on that pod / session |
| `grade` | `events.GradeUpdate` | Clients on that session |
| `assessment` | `models.AssessmentUpdate` | Clients on that session |
| `pod_provisioning` | `PodProvisioningEvent` | Clients on that pod |
| `hint_nudge` | Free-form | Clients on that session |
| `monitoring_event` | `events.MonitoringEvent` | All clients (admin monitoring) |

The first four payloads embed a `MessageHeader` — `id`, `timestamp`, `source`, `version`
(`messages.go:15-20`).

### checkpoint

A checkpoint was evaluated (`messages.go:91-115`):

```json
{
  "type": "checkpoint",
  "subject": "kootenai.checkpoint.updated",
  "payload": {
    "id": "evt-uuid",
    "timestamp": "2025-01-15T10:35:00Z",
    "source": "checkpoint-evaluator",
    "version": "1",
    "podId": "pod-abc123",
    "sessionId": "sess-xyz789",
    "checkpointId": "create-user",
    "action": "passed",
    "status": "passed",
    "points": 10,
    "earnedPoints": 10,
    "triggerType": "file",
    "sessionEarnedPoints": 35,
    "sessionMaxPoints": 100,
    "sessionPercentage": 35.0,
    "feedback": "Optional message"
  }
}
```

`action` is one of `passed`, `failed`, `reset`, `skipped`.

### session

A session lifecycle change (`messages.go:122-144`):

```json
{
  "type": "session",
  "subject": "kootenai.session.submitted",
  "payload": {
    "id": "evt-uuid",
    "timestamp": "2025-01-15T11:00:00Z",
    "source": "session-manager",
    "version": "1",
    "sessionId": "sess-xyz789",
    "podId": "pod-abc123",
    "userId": "user-uuid",
    "action": "submitted",
    "labTemplate": "linux-basics-101",
    "earnedPoints": 85,
    "maxPoints": 100,
    "percentage": 85.0,
    "passed": true,
    "startedAt": "2025-01-15T10:00:00Z",
    "endedAt": "2025-01-15T11:00:00Z",
    "canvasAssignmentId": "12345"
  }
}
```

`action` is one of `started`, `ended`, `paused`, `resumed`, `reset`, `submitted`.

### grade

A grade change that may need syncing to Canvas (`messages.go:151-167`):

```json
{
  "type": "grade",
  "subject": "kootenai.grade.updated",
  "payload": {
    "id": "evt-uuid",
    "timestamp": "2025-01-15T11:00:02Z",
    "source": "grade-service",
    "version": "1",
    "sessionId": "sess-xyz789",
    "userId": "user-uuid",
    "earnedPoints": 85,
    "maxPoints": 100,
    "percentage": 85.0,
    "passed": true,
    "canvasAssignmentId": "12345",
    "canvasCourseId": "678",
    "canvasUserId": "91011"
  }
}
```

Canvas fields are populated only for sessions bound to an assignment — and since LTI is
gated off in this build, grade sync does not reach Canvas. The event still fires.

### pod_provisioning

Progress while a pod is being built (`api/internal/websocket/hub.go:369-381`). This is the
event to drive a progress bar from:

```json
{
  "type": "pod_provisioning",
  "subject": "",
  "payload": {
    "podId": "pod-abc123",
    "ownerId": "user-uuid",
    "status": "provisioning",
    "phase": "cloning",
    "message": "Cloning student-vm from template",
    "progress": 40,
    "vmName": "student-vm",
    "vmStatus": "running",
    "error": "",
    "timestamp": "2025-01-15T10:30:00Z",
    "requestId": "req-uuid"
  }
}
```

Pod `status` values are the `models.Pod` statuses: `provisioning`, `running`, `stopped`,
`error`, `destroying`, `destroyed`.

### monitoring_event

Wazuh-observed activity, broadcast to every connected client for the admin monitoring
dashboard (`messages.go:189-203`):

```json
{
  "type": "monitoring_event",
  "subject": "kootenai.monitoring.event",
  "payload": {
    "id": 4821,
    "timestamp": "2025-01-15T10:30:00Z",
    "podId": "pod-abc123",
    "sessionId": "sess-xyz789",
    "vmName": "student-vm",
    "agentId": "003",
    "eventType": "file_added",
    "ruleId": "554",
    "ruleLevel": 7,
    "description": "File added to the system",
    "processed": true,
    "matchedCheckpoints": ["create-user"]
  }
}
```

### assessment and hint_nudge

`assessment` carries a `models.AssessmentUpdate` for Packet-Tracer-style grading;
`hint_nudge` carries whatever the caller passed and has no fixed schema
(`hub.go:398-405`).

---

## Client Implementation

### JavaScript Example

```javascript
class KootenaiWebSocket {
  // token is optional: if the API set the HttpOnly auth_token cookie at login,
  // the browser sends it on the upgrade and no subprotocol is needed.
  constructor(apiUrl, token) {
    this.url = `${apiUrl.replace(/^http/, 'ws')}/api/v1/ws`;
    // ONE dot-joined subprotocol string, not a two-element array.
    this.protocols = token ? [`access_token.${token}`] : undefined;
    this.handlers = new Map();
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url, this.protocols);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };

    this.ws.onclose = () => {
      console.log('WebSocket disconnected');
      setTimeout(() => this.connect(), 5000);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  handleMessage(message) {
    // The envelope is { type, subject, payload } — not { type, data }.
    const handler = this.handlers.get(message.type);
    if (handler) {
      handler(message.payload, message.subject);
    }
  }

  on(eventType, callback) {
    this.handlers.set(eventType, callback);
  }

  close() {
    this.ws.close();
  }
}

// Usage — note the flat, underscored event names.
const ws = new KootenaiWebSocket('https://api.example.com', token);

ws.on('pod_provisioning', (payload) => {
  console.log(`Pod ${payload.podId}: ${payload.phase} (${payload.progress}%)`);
});

ws.on('checkpoint', (payload) => {
  console.log(`Checkpoint ${payload.checkpointId} ${payload.action} — ` +
              `${payload.sessionPercentage}% of the lab complete`);
});

ws.on('session', (payload) => {
  if (payload.action === 'submitted') {
    console.log(`Scored ${payload.earnedPoints}/${payload.maxPoints}`);
  }
});
```

### Vue Composable

The frontend's own composable is `web/src/composables/useWebSocket.ts`; read that for the
production version, which adds exponential backoff and visibility handling. A minimal
equivalent:

```typescript
// useKootenaiSocket.ts
import { ref, onMounted, onUnmounted } from 'vue';

export function useKootenaiSocket(token?: string) {
  const connected = ref(false);
  const events = ref<any[]>([]);
  let ws: WebSocket | null = null;

  const connect = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Cookie auth needs no subprotocol; pass a token only if you hold one in JS.
    ws = new WebSocket(
      `${protocol}//${window.location.host}/api/v1/ws`,
      token ? [`access_token.${token}`] : undefined,
    );

    ws.onopen = () => {
      connected.value = true;
    };

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data);   // { type, subject, payload }
      events.value.push(message);
    };

    ws.onclose = () => {
      connected.value = false;
      setTimeout(connect, 5000);
    };
  };

  onMounted(connect);
  onUnmounted(() => ws?.close());

  return { connected, events };
}
```

---

## VNC Console WebSocket

For VM console access, a separate WebSocket proxy is available.

### Endpoint

```
wss://api.example.com/api/v1/pods/{podID}/vms/{vmName}/vnc
```

Registered by `consoleaccess.Manager.SetupPodConsoleRoutes`
(`api/internal/server/consoleaccess/manager.go:62-66`), nested inside the pod routes at
`api/internal/server/pods/manager.go:143`. Sibling endpoints on the same prefix:
`/console` (connection details) and `/spice`.

### Authentication

This route sits on the **authenticated** router, so it uses the standard chain — the
`Authorization` header or the `auth_token` cookie — and applies pod-ownership checks. It
does **not** read `?token=`; the `token` values inside `vnc_proxy.go` are the Proxmox API
credentials used on the outbound leg, not client auth.

### Protocol

Binary WebSocket frames carrying RFB, compatible with noVNC:

1. Connect to the WebSocket endpoint
2. The API proxies the connection to Proxmox
3. Binary frames contain RFB protocol data
4. Use the noVNC client library to render

### Example with noVNC

```javascript
import RFB from '@novnc/novnc';

// No ?token= — the browser's auth_token cookie authenticates the upgrade.
const rfb = new RFB(
  document.getElementById('screen'),
  `wss://api.example.com/api/v1/pods/${podId}/vms/${vmName}/vnc`
);

rfb.scaleViewport = true;
rfb.resizeSession = true;
```

---

## Error Handling

### Connection Errors

The upgrade is refused with an ordinary HTTP status before the WebSocket handshake
completes, so these arrive as a failed connection rather than as a message:

| Status | Cause | Recovery |
|--------|-------|----------|
| 401 | No usable token in any of the three sources, or the token is invalid or expired | Re-authenticate and reconnect |
| 401 | Pod not found on `/api/v1/ws/{podID}` — an unknown pod is reported as 401, not 404 | Confirm the pod still exists |
| 403 | Authenticated, but not the pod's owner and not an admin | Check permissions |

:::note There is no `error` event frame
The server never broadcasts a message of type `error`, and there is no `AUTH_EXPIRED` code
anywhere in the tree. Authentication failures happen at upgrade time. Once a connection is
open, the only signal of trouble is the socket closing.
:::

---

## Best Practices

### Connection Management

1. **Single connection per client** - Don't create multiple connections
2. **Reconnection** - Implement exponential backoff
3. **Cleanup** - Close connection when leaving page

The server drives its own keepalive using protocol-level ping/pong frames. There is no
application-level `{"type":"ping"}` message to send, and the server has no handler for
one.

### Event Handling

1. **Idempotent handlers** - Events may be delivered more than once
2. **Order independence** - Don't assume event order
3. **Stale events** - Check timestamps for relevance
4. **Error boundaries** - Don't let handler errors kill connection
