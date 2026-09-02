# Kootenai Database Schema (ASCII)

```
┌─────────────────────────────────────────────────────────────────────────────────────────────┐
│                                  KOOTENAI DATABASE SCHEMA                                   │
└─────────────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────┐         ┌─────────────────────────┐
│     lab_templates       │         │         users           │
├─────────────────────────┤         ├─────────────────────────┤
│ id (PK, UUID)           │         │ id (PK, UUID)           │
│ name (UNIQUE)           │         │ external_id (UNIQUE)    │
│ description             │         │ username (UNIQUE)       │
│ version                 │         │ email                   │
│ platform (ENUM)         │         │ display_name            │
│ duration_minutes        │         │ role                    │
│ difficulty              │         │ created_at              │
│ max_points              │         │ updated_at              │
│ pass_threshold          │         │ last_login_at           │
│ spec (JSONB)            │         │ is_active               │
│ checkpoints (JSONB)     │         │ metadata (JSONB)        │
│ created_at              │         └───────────┬─────────────┘
│ updated_at              │                     │
│ is_active               │                     │
└───────────┬─────────────┘                     │
            │                                   │
            │ 1                              1  │
            │                                   │
            ▼ *                              *  ▼
┌─────────────────────────────────────────────────────────────┐
│                          pods                               │
├─────────────────────────────────────────────────────────────┤
│ id (PK, UUID)                                               │
│ lab_template_id (FK) ────────────────────────────────────►  │
│ owner_id (FK) ───────────────────────────────────────────►  │
│ platform (ENUM: proxmox, cloudstack, any)                   │
│ status (ENUM: provisioning, running, stopped, error, ...)   │
│ vms (JSONB)                                                 │
│ networks (JSONB)                                            │
│ created_at, updated_at, expires_at, destroyed_at            │
│ metadata (JSONB)                                            │
└────────────────────────────┬────────────────────────────────┘
                             │
                             │ 1
                             │
                             ▼ *
┌─────────────────────────────────────────────────────────────┐
│                      lab_sessions                           │
├─────────────────────────────────────────────────────────────┤
│ id (PK, UUID)                                               │
│ pod_id (FK) ─────────────────────────────────────────────►  │
│ user_id (FK) ────────────────────────────────────────────►  │
│ lab_template_id (FK) ────────────────────────────────────►  │
│ canvas_course_id, canvas_assignment_id, canvas_user_id      │
│ started_at, ended_at, due_at                                │
│ max_points, earned_points, percentage, passed               │
│ grade_synced_at, grade_sync_error                           │
│ metadata (JSONB)                                            │
└──────┬─────────────────────────────────┬────────────────────┘
       │                                 │
       │ 1                               │ 1
       │                                 │
       ▼ *                               ▼ 1
┌──────────────────────────┐    ┌────────────────────────────┐
│  checkpoint_progress     │    │   assessment_results       │
├──────────────────────────┤    ├────────────────────────────┤
│ id (PK, UUID)            │    │ id (PK, UUID)              │
│ session_id (FK, UNIQUE   │    │ session_id (FK, UNIQUE)    │
│   with checkpoint_id)    │    │ score, max_score           │
│ checkpoint_id            │    │ percentage, item_count     │
│ status (ENUM)            │    │ passed_count, status       │
│ points, earned_points    │    │ started_at, completed_at   │
│ passed_at                │    │ components (JSONB)         │
│ triggered_by_event_id    │    │ devices (JSONB)            │
│ attempt_count, feedback  │    │ created_at, updated_at     │
│ created_at, updated_at   │    └────────────────────────────┘
└──────────────────────────┘
                                        ┌────────────────────────────┐
       │ 1                              │   grade_sync_queue         │
       │                                ├────────────────────────────┤
       ▼ *                              │ id (PK, UUID)              │
┌──────────────────────────┐            │ session_id (FK)            │
│     grade_history        │            │ status, earned_points      │
├──────────────────────────┤            │ max_points, percentage     │
│ id (PK, UUID)            │            │ canvas_course_id           │
│ session_id (FK)          │            │ canvas_assignment_id       │
│ earned_points, max_points│            │ canvas_user_id             │
│ percentage, passed       │            │ attempts, last_attempt_at  │
│ checkpoint_snapshot      │            │ completed_at, error_message│
│ synced_to_canvas         │            │ created_at                 │
│ canvas_response          │            └────────────────────────────┘
│ created_at, created_by   │
└──────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                         events                              │
├─────────────────────────────────────────────────────────────┤
│ timestamp (PK, first in composite)                          │
│ id (BIGSERIAL, PK second in composite)                      │
│ pod_id (NOT NULL)                                           │
│ session_id (FK, nullable)                                   │
│ vm_name, agent_id                                           │
│ event_type, rule_id, rule_level                             │
│ description, location                                       │
│ data (JSONB)                                                │
│ processed, matched_checkpoints[]                            │
└─────────────────────────────────────────────────────────────┘

┌──────────────────────────┐    ┌────────────────────────────┐
│     wazuh_agents         │    │       audit_log            │
├──────────────────────────┤    ├────────────────────────────┤
│ id (PK, UUID)            │    │ id (PK, BIGSERIAL)         │
│ agent_id (UNIQUE)        │    │ timestamp                  │
│ agent_name               │    │ actor_id (FK users)        │
│ pod_id (FK, nullable)    │    │ actor_type, action         │
│ vm_name, ip_address      │    │ resource_type, resource_id │
│ os_name, os_version      │    │ details (JSONB)            │
│ status, registered_at    │    │ ip_address, user_agent     │
│ last_seen_at             │    └────────────────────────────┘
│ config (JSONB)           │
└──────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                       ENUM TYPES                            │
├─────────────────────────────────────────────────────────────┤
│ platform_type:    proxmox | cloudstack | any                │
│ pod_status:       provisioning | running | stopped |        │
│                   error | destroying | destroyed            │
│ checkpoint_status: pending | passed | failed | skipped |    │
│                    partial                                  │
│ trigger_type:     file_exists | file_content | file_deleted │
│                   package | service | command_executed |    │
│                   user_created | permission_changed |       │
│                   network_connection | active_check | custom│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                    RELATIONSHIPS                            │
├─────────────────────────────────────────────────────────────┤
│  lab_templates ◄──┐                                         │
│                   ├── pods (lab_template_id, owner_id)      │
│  users ◄──────────┘                                         │
│                                                             │
│  pods ◄───────────┬── lab_sessions (pod_id, user_id,        │
│  users ◄──────────┤       lab_template_id)                  │
│  lab_templates ◄──┘                                         │
│                                                             │
│  lab_sessions ◄───┬── checkpoint_progress                   │
│                   ├── assessment_results (1:1)              │
│                   ├── grade_history                         │
│                   └── grade_sync_queue                      │
│                                                             │
│  pods ◄─────────────── wazuh_agents                         │
│  users ◄────────────── audit_log                            │
│  pods ◄─────────────── events (pod_id, session_id)          │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                   REQUIRED EXTENSIONS                       │
├─────────────────────────────────────────────────────────────┤
│ uuid-ossp    - UUID generation (uuid_generate_v4)           │
│ pgcrypto     - Cryptographic functions                      │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                      FUNCTIONS                              │
├─────────────────────────────────────────────────────────────┤
│ update_session_progress()                                   │
│   - Recalculates earned_points, percentage, passed          │
│   - Triggered on checkpoint_progress INSERT/UPDATE          │
│                                                             │
│ update_updated_at()                                         │
│   - Sets updated_at = NOW() on row update                   │
│   - Used by multiple tables                                 │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                       TRIGGERS                              │
├─────────────────────────────────────────────────────────────┤
│ trigger_update_session_progress                             │
│   ON checkpoint_progress AFTER INSERT OR UPDATE             │
│   EXECUTE update_session_progress()                         │
│                                                             │
│ trigger_lab_templates_updated                               │
│   ON lab_templates BEFORE UPDATE                            │
│   EXECUTE update_updated_at()                               │
│                                                             │
│ trigger_users_updated                                       │
│   ON users BEFORE UPDATE                                    │
│   EXECUTE update_updated_at()                               │
│                                                             │
│ trigger_checkpoint_progress_updated                         │
│   ON checkpoint_progress BEFORE UPDATE                      │
│   EXECUTE update_updated_at()                               │
│                                                             │
│ trigger_pods_updated                                        │
│   ON pods BEFORE UPDATE                                     │
│   EXECUTE update_updated_at()                               │
│                                                             │
│ trigger_assessment_results_updated                          │
│   ON assessment_results BEFORE UPDATE                       │
│   EXECUTE update_updated_at()                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                        INDEXES                              │
├─────────────────────────────────────────────────────────────┤
│ lab_templates:                                              │
│   idx_lab_templates_platform (platform)                     │
│   idx_lab_templates_active (is_active) WHERE is_active=true │
│                                                             │
│ users:                                                      │
│   (unique constraints auto-create indexes)                  │
│                                                             │
│ pods:                                                       │
│   idx_pods_owner (owner_id)                                 │
│   idx_pods_template (lab_template_id)                       │
│   idx_pods_status (status)                                  │
│   idx_pods_expires (expires_at) WHERE expires_at NOT NULL   │
│   idx_pods_owner_active (owner_id, status)                  │
│       WHERE status NOT IN ('destroyed', 'destroying')       │
│                                                             │
│ lab_sessions:                                               │
│   idx_sessions_pod (pod_id)                                 │
│   idx_sessions_user (user_id)                               │
│   idx_sessions_template (lab_template_id)                   │
│   idx_sessions_canvas (canvas_assignment_id)                │
│       WHERE canvas_assignment_id IS NOT NULL                │
│   idx_sessions_active (ended_at) WHERE ended_at IS NULL     │
│   idx_sessions_canvas_assignment_user                       │
│       (canvas_assignment_id, canvas_user_id)                │
│       WHERE canvas_assignment_id IS NOT NULL                │
│                                                             │
│ checkpoint_progress:                                        │
│   idx_checkpoint_progress_session (session_id)              │
│   idx_checkpoint_progress_status (status)                   │
│                                                             │
│ assessment_results:                                         │
│   idx_assessment_results_session (session_id)               │
│   idx_assessment_results_status (status)                    │
│   idx_assessment_results_updated (updated_at)               │
│                                                             │
│ events:                                                     │
│   idx_events_pod (pod_id)                                   │
│   idx_events_session (session_id)                           │
│       WHERE session_id IS NOT NULL                          │
│   idx_events_agent (agent_id)                               │
│   idx_events_type (event_type)                              │
│   idx_events_unprocessed (processed)                        │
│       WHERE processed = false                               │
│   idx_events_data (data) USING GIN                          │
│                                                             │
│ wazuh_agents:                                               │
│   idx_wazuh_agents_pod (pod_id)                             │
│   idx_wazuh_agents_status (status)                          │
│                                                             │
│ grade_history:                                              │
│   idx_grade_history_session (session_id)                    │
│   idx_grade_history_created (created_at)                    │
│                                                             │
│ grade_sync_queue:                                           │
│   idx_grade_sync_queue_status (status)                      │
│   idx_grade_sync_queue_session (session_id)                 │
│   idx_grade_sync_queue_pending (created_at)                 │
│       WHERE status = 'pending'                              │
│   idx_grade_sync_queue_retry (status, attempts, created_at) │
│       WHERE status IN ('failed', 'pending')                 │
│                                                             │
│ audit_log:                                                  │
│   idx_audit_log_actor (actor_id)                            │
│   idx_audit_log_action (action)                             │
│   idx_audit_log_resource (resource_type, resource_id)       │
│   idx_audit_log_timestamp (timestamp)                       │
└─────────────────────────────────────────────────────────────┘
```

## Key Relationships Summary

| Parent Table | Child Table | Relationship |
|-------------|-------------|--------------|
| `lab_templates` | `pods` | 1:many |
| `users` | `pods` | 1:many (owner) |
| `pods` | `lab_sessions` | 1:many |
| `users` | `lab_sessions` | 1:many |
| `lab_sessions` | `checkpoint_progress` | 1:many |
| `lab_sessions` | `assessment_results` | 1:1 |
| `lab_sessions` | `grade_history` | 1:many |
| `lab_sessions` | `grade_sync_queue` | 1:many |
| `pods` | `wazuh_agents` | 1:many |
| `pods` | `events` | 1:many |
| `users` | `audit_log` | 1:many |
