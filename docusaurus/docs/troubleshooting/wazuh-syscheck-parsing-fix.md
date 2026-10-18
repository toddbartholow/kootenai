# Wazuh Syscheck Alert Parsing Fix

## Issue Summary

**Date Fixed:** 2026-01-06
**Affected Component:** `api/internal/wazuh/alert.go`
**Symptom:** Wazuh syscheck (file integrity monitoring) events failed to parse, causing checkpoint detection to fail for file-based triggers.

## Problem Description

Syscheck events from Wazuh were being logged as "Parsed raw alert format" with empty `alertId`, `agentName`, and `podId` fields, even though the JSON payload contained valid data. This prevented the checkpoint evaluator from matching file creation/modification events to lab checkpoints.

### Error Message

```
json: cannot unmarshal string into Go struct field Syscheck.alert.syscheck.size_before of type int64
```

### Root Cause

The Wazuh API sends `size_before` and `size_after` fields in syscheck alerts as **JSON strings**, not numbers:

```json
{
  "syscheck": {
    "path": "/home/student/notes.txt",
    "size_before": "237",
    "size_after": "278",
    ...
  }
}
```

However, the Go struct defined these fields as `int64`:

```go
// INCORRECT - caused unmarshal failure
type Syscheck struct {
    Size         int64  `json:"size_after,omitempty"`
    SizeBefore   int64  `json:"size_before,omitempty"`
    ...
}
```

When `json.Unmarshal` encountered a string value for an `int64` field, it returned an error, causing the entire alert parsing to fail silently (the error was returned but the code fell through to raw parsing which also couldn't extract the data properly).

## Solution

### 1. Changed Field Types

Updated `api/internal/wazuh/alert.go` to accept strings:

```go
// Syscheck contains file integrity monitoring data
type Syscheck struct {
    Path         string            `json:"path"`
    Mode         string            `json:"mode,omitempty"`
    Event        string            `json:"event"`
    Size         string            `json:"size_after,omitempty"`   // Wazuh sends as string
    SizeBefore   string            `json:"size_before,omitempty"`  // Wazuh sends as string
    // ... other fields
}
```

### 2. Added String-to-Int Conversion

Since `models.SyscheckData.Size` expects `int64`, added conversion in `buildEventData()`:

```go
case events.EventTypeSyscheck:
    if a.Syscheck != nil {
        // Parse size from string (Wazuh sends as string)
        var size int64
        if a.Syscheck.Size != "" {
            if parsed, err := strconv.ParseInt(a.Syscheck.Size, 10, 64); err == nil {
                size = parsed
            }
        }
        data = models.SyscheckData{
            Path:         a.Syscheck.Path,
            Event:        a.Syscheck.Event,
            Size:         size,
            // ... other fields
        }
    }
```

## Verification

After the fix, syscheck events parse correctly:

```json
{
  "level": "INFO",
  "msg": "Wazuh webhook parse attempt",
  "unmarshalErr": null,
  "payloadAlertID": "1767344731.4835558",
  "payloadAlertAgentName": "38ce2e2e-c97f-49f2-9131-9d7b40be6975-linux-vm",
  "payloadAlertRuleID": "550"
}
```

And checkpoints are now detected:

```json
{
  "level": "INFO",
  "msg": "Checkpoint passed",
  "sessionId": "38ce2e2e-0001-0001-0001-000000000001",
  "checkpointId": "vim-file",
  "points": 20
}
```

## Testing

To verify syscheck events are working:

1. SSH to a monitored VM as the lab user
2. Create or modify a file in a monitored directory (e.g., `/home/student/`)
3. Check API logs for "Evaluating event against checkpoints" with `eventType: syscheck`
4. Verify checkpoint_progress table updates in the database

```bash
# Trigger a syscheck event
ssh student@<VM_IP>
echo "test" >> ~/notes.txt

# Check API logs
docker logs kootenai-api 2>&1 | grep -E 'syscheck|Checkpoint'

# Check database
docker exec kootenai-postgres psql -U labadmin -d virtuallab \
  -c "SELECT checkpoint_id, status, earned_points FROM checkpoint_progress WHERE status = 'passed';"
```

## Related Files

- `api/internal/wazuh/alert.go` - Alert struct definitions and parsing
- `api/internal/wazuh/models.go` - WebhookPayload wrapper struct
- `api/internal/server/event_handlers.go` - Webhook handler
- `api/internal/checkpoint/evaluator.go` - Checkpoint evaluation logic

## Lessons Learned

1. **Always verify JSON field types** against actual API responses, not just documentation
2. **Add debug logging** that shows unmarshal errors explicitly during development
3. **Wazuh sends numeric values as strings** in several places - be cautious with type assumptions
