# Wazuh Integration

This document describes how the Kootenai platform integrates with Wazuh for security monitoring and checkpoint evaluation.

## Overview

Wazuh agents installed on lab VMs send security events (file integrity monitoring, command execution, service changes, etc.) to the Wazuh Manager. The Kootenai API receives these events via webhook and uses them to:

1. **Track student progress** by evaluating events against checkpoint conditions
2. **Provide real-time monitoring** for instructors via the Events Dashboard
3. **Store event history** for audit and troubleshooting

## Architecture

```
┌─────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Lab VMs    │───►│  Wazuh Manager  │───►│  Kootenai    │
│  (Agents)   │    │  (Webhook)      │    │  API            │
└─────────────┘    └─────────────────┘    └────────┬────────┘
                                                   │
                   ┌─────────────────┐    ┌────────▼────────┐
                   │  WebSocket      │◄───│  PostgreSQL     │
                   │  Clients        │    │  (Events)       │
                   └─────────────────┘    └─────────────────┘
```

## Components

### Wazuh Agent Configuration

Each lab VM must have a Wazuh agent installed and configured to connect to the Wazuh Manager. The agent name follows the pattern: `{podID}-{vmName}` (e.g., `ad78a3b9-235c-4213-8a91-b40c35953707-linux-vm`).

Agent registration should include these labels for session correlation:
- `session_id` - Session ID for grade tracking
- `pod_id` - Pod ID for event routing
- `vm_name` - VM name within the pod

These are the keys the API unmarshals from `agent.labels` (`api/internal/wazuh/alert.go`);
they carry no prefix.

### Wazuh Manager Integration Script

The Wazuh Manager runs an integration script that forwards alerts to the Kootenai API. The script is configured in `/var/ossec/etc/ossec.conf`:

```xml
<integration>
  <name>custom-kootenai</name>
  <hook_url>http://API_HOST:8080/api/v1/events/wazuh</hook_url>
  <alert_format>json</alert_format>
</integration>
```

The integration script (`/var/ossec/integrations/custom-kootenai`) forwards alerts in this format:

```json
{
  "alert": {
    "id": "1234567890.123456",
    "timestamp": "2024-01-15T10:30:00.000+0000",
    "rule": {
      "id": "550",
      "level": 7,
      "description": "File integrity checksum changed"
    },
    "agent": {
      "id": "001",
      "name": "pod-uuid-linux-vm"
    },
    "syscheck": {
      "path": "/etc/hosts",
      "event": "modified"
    }
  }
}
```

### API Webhook Handler

The API receives Wazuh alerts at `POST /api/v1/events/wazuh` and:

1. Parses the alert (supports both wrapped `{"alert": ...}` and raw formats)
2. Extracts pod ID from agent name
3. Looks up active session for the pod
4. Evaluates the event against checkpoint conditions
5. Stores the event in PostgreSQL
6. Broadcasts to WebSocket clients for real-time monitoring

### Event Storage

Events are stored in the `events` table with these key fields:

| Field | Description |
|-------|-------------|
| `id` | Auto-incrementing event ID |
| `timestamp` | When the event occurred |
| `pod_id` | Which pod generated the event |
| `session_id` | Associated lab session (if found) |
| `vm_name` | Source VM name |
| `agent_id` | Wazuh agent ID |
| `event_type` | Type (syscheck, audit, package, etc.) |
| `wazuh_rule_id` | Wazuh rule that triggered |
| `wazuh_level` | Alert severity level |
| `data` | Full event payload (JSONB) |
| `processed` | Whether checkpoint matched |
| `matched_checkpoints` | Array of triggered checkpoints |

### Checkpoint Evaluation

The checkpoint evaluator (`internal/checkpoint/evaluator.go`) processes events and checks them against checkpoint conditions defined in lab templates.

Supported trigger types:
- `file_exists` - File created at path
- `file_content` - File contains expected content
- `file_deleted` - File was removed
- `package` - Package installed/removed
- `service` - Service started/stopped
- `command_executed` - Specific command run
- `user_created` - User account created
- `permission_changed` - File permissions modified

### Real-time Monitoring

The Events Dashboard (`/admin/events`) shows:
- Live event stream via WebSocket
- Event filtering by type, rule level, time range
- Checkpoint match highlighting
- Detailed event view

WebSocket message type: `monitoring_event`

```typescript
interface MonitoringEventPayload {
  id: number
  timestamp: string
  podId: string
  sessionId?: string
  vmName: string
  agentId?: string
  eventType: string
  ruleId?: string
  ruleLevel?: number
  description?: string
  data?: Record<string, unknown>
  processed: boolean
  matchedCheckpoints?: string[]
}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `WAZUH_WEBHOOK_SECRET` | Shared secret for webhook auth | (none) |
| `WAZUH_MANAGER_HOST` | Wazuh Manager hostname | `wazuh-manager` |
| `WAZUH_MANAGER_PORT` | Wazuh Manager API port | `55000` |

### Docker Compose

The Wazuh Manager runs as a separate Docker Compose stack. See `deploy/wazuh/docker-compose.yml`.

## Troubleshooting

### Events Not Appearing

1. Check Wazuh Manager logs: `docker compose logs wazuh-manager`
2. Verify integration script: `cat /var/ossec/integrations/custom-kootenai`
3. Test webhook manually:
   ```bash
   curl -X POST http://localhost:8080/api/v1/events/wazuh \
     -H "Content-Type: application/json" \
     -d '{"alert":{"id":"test","timestamp":"2024-01-15T10:30:00Z","rule":{"id":"550","level":7,"description":"Test"},"agent":{"id":"001","name":"test-agent"}}}'
   ```
4. Check API logs for parsing errors

### Session Not Found

If events arrive but don't trigger checkpoints:
1. Verify agent name format: `{podID}-{vmName}`
2. Check for active session: `SELECT * FROM lab_sessions WHERE pod_id = '...' AND ended_at IS NULL`
3. Ensure session is registered with evaluator (API logs show "Started session checkpoint tracking")

### WebSocket Not Receiving Events

1. Check WebSocket connection in browser DevTools
2. Verify `/api/ws` endpoint is accessible
3. Look for "BroadcastMonitoringEvent" in API logs

## Related Files

- `api/internal/server/event_handlers.go` - Webhook handler
- `api/internal/wazuh/alert.go` - Alert parsing
- `api/internal/events/messages.go` - Event types
- `api/internal/checkpoint/evaluator.go` - Checkpoint evaluation
- `api/internal/websocket/hub.go` - WebSocket broadcasting
- `web/src/views/EventsMonitoringView.vue` - Monitoring dashboard
- `web/src/composables/useWebSocket.ts` - WebSocket client
