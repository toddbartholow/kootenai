# Canvas Module Integration with Kootenai Pods

## Overview

This document describes the implementation of LTI Deep Linking to allow instructors to create Canvas assignments linked to Kootenai templates. When students complete checkpoints, grades sync automatically to Canvas using percentage-based scoring.

## Architecture

```
Instructor Flow:
Canvas → Add Assignment → External Tool → Kootenai Template Selector → Deep Link Response → Canvas Assignment Created

Student Flow:
Canvas Assignment → LTI Launch → Kootenai (with correct template) → Complete Checkpoints → Grade Sync → Canvas Gradebook
```

## Implementation Steps

### Phase 1: Database Schema

**File:** `api/internal/database/migrations/025_lti_assignments.sql`

```sql
CREATE TABLE lti_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    canvas_course_id VARCHAR(128) NOT NULL,
    resource_link_id VARCHAR(255) NOT NULL,
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id),
    custom_title VARCHAR(255),
    max_points DECIMAL(10,2) NOT NULL DEFAULT 100,
    organization_id UUID REFERENCES organizations(id),
    created_by UUID REFERENCES users(id),
    deployment_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(canvas_course_id, resource_link_id)
);

ALTER TABLE lab_sessions ADD COLUMN lti_assignment_id UUID REFERENCES lti_assignments(id);
```

### Phase 2: Backend - Deep Linking Types

**File:** `api/internal/canvas/deep_linking.go` (new)

Add types for:
- `DeepLinkingSettings` - extracted from LTI claims
- `ContentItem` - resource link to return to Canvas
- `LineItemClaim` - for automatic gradebook column creation
- `DeepLinkingService` - builds signed JWT response

Key method:
```go
func (s *DeepLinkingService) BuildDeepLinkingResponse(
    ctx context.Context,
    settings *DeepLinkingSettings,
    items []ContentItem,
    deploymentID string,
) (string, error)
```

### Phase 3: Backend - LTI Handler Updates

**File:** `api/internal/server/lti_handlers.go`

1. **Update `handleLTICallback`** - After `ValidateLaunch`, check message type:
   ```go
   messageType := launch.GetMessageType()
   if messageType == "LtiDeepLinkingRequest" {
       s.handleDeepLinkingRequest(w, r, launch)
       return
   }
   ```

2. **Add `handleDeepLinkingRequest`** - Extract deep linking settings, redirect to template selection UI

3. **Update `processLTILaunch`** - Look up `lti_assignments` by `resource_link_id` to get correct template

### Phase 4: Backend - New API Endpoints

**File:** `api/internal/server/routes.go`

Add routes:
- `GET /api/v1/lti/templates` - List published templates for selection
- `POST /api/v1/lti/deep-link/submit` - Process template selection, return JWT

**File:** `api/internal/server/lti_deeplink_handlers.go` (new)

```go
// handleListLTITemplates returns published templates for instructor selection
func (s *Server) handleListLTITemplates(w http.ResponseWriter, r *http.Request)

// handleDeepLinkSubmit processes template selection and returns auto-submitting form
func (s *Server) handleDeepLinkSubmit(w http.ResponseWriter, r *http.Request)
```

### Phase 5: Backend - Repository

**File:** `api/internal/database/repositories/lti_assignment_repo.go` (new)

```go
type LTIAssignmentRepository interface {
    Create(ctx context.Context, assignment *models.LTIAssignment) error
    GetByResourceLink(ctx context.Context, courseID, resourceLinkID string) (*models.LTIAssignment, error)
    GetByID(ctx context.Context, id string) (*models.LTIAssignment, error)
}
```

### Phase 6: Frontend - Template Selection UI

**File:** `web/src/views/LTISelectView.vue` (new)

Minimal iframe-friendly page for template selection:
- Grid of lab template cards
- Shows: name, description, difficulty, checkpoint count, points
- Click to select and submit

**File:** `web/src/router/index.ts`

Add route:
```typescript
{
  path: '/lti/select',
  name: 'lti-select',
  component: () => import('../views/LTISelectView.vue'),
  meta: { layout: 'minimal', requiresAuth: false }
}
```

### Phase 7: Grade Sync Integration

The existing grade sync infrastructure handles this automatically:
- `checkpoint_progress` updates trigger session recalculation
- `GradeUpdate` published to NATS
- `GradeSyncHandler` submits to Canvas AGS

**Formula:** `grade = (earned_points / max_points) * 100`

No changes needed - existing flow in:
- `api/internal/checkpoint/evaluator.go`
- `api/internal/server/gradesync_handler.go`
- `api/internal/canvas/grades.go`

## Critical Files to Modify

| File | Change |
|------|--------|
| `api/internal/server/lti_handlers.go` | Add message type detection, deep linking routing |
| `api/internal/canvas/lti.go` | Add `GetMessageType()` helper, parse deep linking claims |
| `api/internal/server/routes.go` | Register new endpoints |
| `config/lti/tool-config.json` | Already configured (no changes needed) |

## New Files to Create

| File | Purpose |
|------|---------|
| `api/internal/database/migrations/025_lti_assignments.sql` | Schema for assignment mapping |
| `api/internal/canvas/deep_linking.go` | Deep linking types and JWT builder |
| `api/internal/server/lti_deeplink_handlers.go` | Template list and submission handlers |
| `api/internal/database/repositories/lti_assignment_repo.go` | CRUD for lti_assignments |
| `api/internal/models/lti_assignment.go` | Model definition |
| `web/src/views/LTISelectView.vue` | Template selection UI |

## Canvas Configuration

Already configured in `config/lti/tool-config.json`:
- `assignment_selection` placement with `LtiDeepLinkingRequest` message type
- Custom fields for `canvas_course_id`, `canvas_assignment_id`
- AGS scopes for grade passback

## Testing Strategy

### Unit Tests
- `api/internal/canvas/deep_linking_test.go` - JWT building, claim parsing
- `api/internal/server/lti_deeplink_handlers_test.go` - Handler logic

### Integration Test Flow
1. Simulate `LtiDeepLinkingRequest` launch
2. Verify redirect to `/lti/select`
3. Submit template selection
4. Verify JWT response format
5. Simulate student launch with `resource_link_id`
6. Verify correct template loaded
7. Complete checkpoint, verify grade in Canvas

### Manual Testing
```bash
# 1. In Canvas: Create assignment → External Tool → Kootenai
# 2. Select a lab template in the UI
# 3. Verify assignment created with correct title/points
# 4. As student: Click assignment link
# 5. Verify correct lab loads
# 6. Complete checkpoints
# 7. Check Canvas gradebook for grade
```

## Environment Variables

Existing (no new ones required):
- `CANVAS_URL` - Canvas instance URL
- `CANVAS_API_ACCESS_TOKEN` - For optional REST API calls
- LTI config in `config/lti/canvas-config.yaml`

## Verification Commands

```bash
# Run migrations
mage db:migrate

# Run tests
cd api && go test ./internal/canvas/... ./internal/server/...

# Deploy
mage deploy:api

# Check logs
ssh labadmin@<INFRA_VM_IP> 'docker logs kootenai-api -f'
```
