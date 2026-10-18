# Web UI for Pod and Pathway Creation - Implementation Plan

## Goal
Create a web UI that allows users to create new pods and pathways in the Kootenai platform.

## Design Decisions
- **Pathway Access:** Instructors and Admins only (role-based)
- **Pathway UI:** Multi-step wizard (Basic Info → Modules → Labs → Review)
- **Mock Data:** Yes - support development/demo mode without backend

## Tech Stack (Existing)
- **Framework:** Vue.js 3 with TypeScript, Composition API (`<script setup>`)
- **State Management:** Pinia stores
- **UI Components:** PrimeVue (unstyled) + Tailwind CSS v4
- **API Client:** Axios with namespaced pattern (`podsApi`, `pathwaysApi`)
- **Router:** Vue Router with lazy loading

---

## Implementation Plan

### Part 1: Pod Creation UI

#### 1.1 Add API Endpoint Support
**File:** `web/src/api/client.ts`

The `podsApi.create()` method already exists. Verify and add mock data support if needed.

```typescript
// Existing - verify this works:
create: async (labTemplate: string, owner: string): Promise<Pod>
```

#### 1.2 Create Pod Creation View
**File:** `web/src/views/CreatePodView.vue` (NEW)

Form fields:
- Lab Template dropdown (required) - fetched from `labsApi.list()`
- Owner field (auto-filled from current user, editable for admins)
- Submit button with loading state

Features:
- Form validation (template required)
- Error handling with toast notifications
- Redirect to pod detail after creation
- Loading skeleton while fetching templates

#### 1.3 Add Route
**File:** `web/src/router.ts`

```typescript
{
  path: '/pods/create',
  name: 'create-pod',
  component: () => import('@/views/CreatePodView.vue'),
  meta: { requiresAuth: true }
}
```

#### 1.4 Add Navigation
**File:** `web/src/views/PodsView.vue`

Add "Create Pod" button linking to the new view.

---

### Part 2: Pathway Creation UI

#### 2.1 Add API Endpoint Support
**File:** `web/src/api/client.ts`

Add missing methods to `pathwaysApi`:
```typescript
create: async (data: CreatePathwayRequest): Promise<Pathway>
createModule: async (pathwayId: string, data: CreateModuleRequest): Promise<PathwayModule>
addLabToModule: async (moduleId: string, data: AddLabRequest): Promise<ModuleLab>
```

Add TypeScript interfaces:
```typescript
interface CreatePathwayRequest {
  name: string
  slug?: string
  description?: string
  difficulty?: 'beginner' | 'intermediate' | 'advanced' | 'mixed'
  visibility?: 'global' | 'organization' | 'private'
  estimatedHours?: number
  tags?: string[]
}

interface CreateModuleRequest {
  name: string
  slug?: string
  description?: string
  unlockType?: 'sequential' | 'all_previous' | 'manual' | 'always'
  estimatedMinutes?: number
}

interface AddLabRequest {
  labTemplateId: string
  displayOrder?: number
  isRequired?: boolean
  passThresholdOverride?: number
}
```

#### 2.2 Create Pathway Creation Wizard
**File:** `web/src/views/CreatePathwayView.vue` (NEW)

Multi-step wizard with 3 steps:

**Step 1: Basic Info**
- Name (required)
- Description (textarea)
- Difficulty (dropdown: beginner/intermediate/advanced/mixed)
- Estimated Hours (number)
- Tags (chip input)
- Visibility (radio: global/organization/private)

**Step 2: Add Modules**
- List of added modules with reorder capability
- "Add Module" button opens inline form:
  - Name (required)
  - Description
  - Unlock Type (dropdown with descriptions)
- Edit/delete existing modules

**Step 3: Add Labs to Modules**
- Accordion for each module
- Lab template selector dropdown
- Required checkbox
- Pass threshold override (optional)
- Reorder labs within module

**Final Step: Review & Create**
- Summary of pathway structure
- Create button
- Redirect to pathway detail on success

#### 2.3 Create Supporting Components

**File:** `web/src/components/pathway/PathwayWizardStep1.vue` (NEW)
- Basic info form fields

**File:** `web/src/components/pathway/PathwayWizardStep2.vue` (NEW)
- Module list with CRUD operations

**File:** `web/src/components/pathway/PathwayWizardStep3.vue` (NEW)
- Labs assignment per module

**File:** `web/src/components/pathway/ModuleForm.vue` (NEW)
- Reusable module creation/edit form

**File:** `web/src/components/pathway/LabSelector.vue` (NEW)
- Lab template search/select component

#### 2.4 Add Route
**File:** `web/src/router.ts`

```typescript
{
  path: '/pathways/create',
  name: 'create-pathway',
  component: () => import('@/views/CreatePathwayView.vue'),
  meta: { requiresAuth: true, requiresRole: 'instructor' }
}
```

#### 2.5 Add Navigation
**File:** `web/src/views/PathwaysListView.vue`

Add "Create Pathway" button (visible to instructors/admins).

---

### Part 3: Shared Components

#### 3.1 Labs API Integration
**File:** `web/src/api/client.ts`

Add/verify `labsApi` namespace:
```typescript
export const labsApi = {
  list: async (options?: LabListOptions): Promise<LabTemplate[]>,
  get: async (id: string): Promise<LabTemplate>
}
```

#### 3.2 Form Components (if not existing)
- Form validation display component
- Multi-select/tags input
- Step indicator for wizard

---

## File Summary

### New Files
| File | Purpose |
|------|---------|
| `web/src/views/CreatePodView.vue` | Pod creation form |
| `web/src/views/CreatePathwayView.vue` | Pathway creation wizard |
| `web/src/components/pathway/PathwayWizardStep1.vue` | Basic info step |
| `web/src/components/pathway/PathwayWizardStep2.vue` | Modules step |
| `web/src/components/pathway/PathwayWizardStep3.vue` | Labs assignment step |
| `web/src/components/pathway/ModuleForm.vue` | Module form component |
| `web/src/components/pathway/LabSelector.vue` | Lab selection component |

### Modified Files
| File | Changes |
|------|---------|
| `web/src/api/client.ts` | Add pathway/module/lab creation methods |
| `web/src/router.ts` | Add routes for create views |
| `web/src/views/PodsView.vue` | Add "Create Pod" button |
| `web/src/views/PathwaysListView.vue` | Add "Create Pathway" button |

---

## UI/UX Design Notes

### Pod Creation
- Simple single-page form
- Template dropdown with search
- Immediate feedback on creation status
- Link to view pod after creation

### Pathway Creation Wizard
- Horizontal stepper showing progress
- Back/Next navigation
- Form validation per step
- Ability to go back and edit
- Draft auto-save (optional enhancement)

### Common Patterns
- Loading skeletons while fetching data
- Toast notifications for success/error
- Confirmation dialogs for destructive actions
- Responsive design (mobile-friendly)

---

## Verification

### Manual Testing
1. **Pod Creation:**
   - Navigate to /pods/create
   - Select a template from dropdown
   - Click create
   - Verify pod appears in list
   - Verify redirect to pod detail

2. **Pathway Creation:**
   - Navigate to /pathways/create
   - Complete step 1 (basic info)
   - Add 2 modules in step 2
   - Add labs to each module in step 3
   - Review and create
   - Verify pathway appears in list
   - Verify modules and labs are correct

### API Verification
```bash
# Test pod creation
curl -X POST http://localhost:8080/api/v1/pods \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"labTemplate": "test-lab", "owner": "testuser"}'

# Test pathway creation
curl -X POST http://localhost:8080/api/v1/pathways \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Pathway", "difficulty": "beginner"}'
```

### Browser Testing
- Test in Chrome, Firefox, Safari
- Test responsive design at different breakpoints
- Test with mock data mode enabled
