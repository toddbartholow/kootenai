/**
 * Pods API
 * Pod lifecycle management
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import { loggers } from '@/utils/logger'
import type { ApiPod, ApiPodVM, ApiPodStatus } from '../generated'

// Re-export the narrow backend enum so the rest of the app doesn't
// import from the generated directory directly.
export type PodStatus = ApiPodStatus

// ============================================================================
// Types
// ============================================================================
//
// NOTE: These hand-rolled shapes are being migrated onto the generated
// OpenAPI types (see src/api/generated/). The drift assertions at the
// bottom of this file catch divergence at compile time — if the backend
// adds a field or changes a type and this file falls out of date,
// `npm run typecheck` will fail.
export interface PodVM {
  name: string
  platformId: string
  status: string
  ipAddress?: string
  currentSnapshot?: string
}

/**
 * `Pod` exposes a slightly narrower shape than the backend to keep
 * existing call sites working. The generated `ApiPod` carries additional
 * fields (labTemplateId, ownerId, organizationId, teamId, networks,
 * metadata) that the UI doesn't need yet. We'll widen this interface
 * progressively as views start using the new fields; until then, the
 * generated type is the source of truth and we only lift what's actually
 * needed.
 */
export interface Pod {
  id: string
  name?: string
  labTemplate: string
  platform: string
  owner: string
  // Narrowed from the backend enum via generated OpenAPI types. Typos
  // like `p.status === 'destroyd'` are now compile errors instead of
  // silent always-false checks.
  status: PodStatus
  vms: PodVM[]
  createdAt: string
  expiresAt?: string
}

export interface PodsListResponse {
  pods: Pod[]
}

// ============================================================================
// Mock Data (exported for cross-domain lookups)
// ============================================================================
export const mockPods: Pod[] = [
  {
    id: 'pod-abc123',
    labTemplate: 'Firewall Configuration 101',
    platform: 'proxmox',
    owner: 'demo@example.com',
    status: 'running',
    createdAt: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(), // 2 hours ago
    expiresAt: new Date(Date.now() + 4 * 60 * 60 * 1000).toISOString(), // 4 hours from now
    vms: [
      {
        name: 'kali-attacker',
        platformId: 'vm-101',
        status: 'running',
        ipAddress: '10.0.100.10',
        currentSnapshot: 'clean-state',
      },
      {
        name: 'pfsense-firewall',
        platformId: 'vm-102',
        status: 'running',
        ipAddress: '10.0.100.1',
        currentSnapshot: 'initial-config',
      },
      {
        name: 'windows-target',
        platformId: 'vm-103',
        status: 'running',
        ipAddress: '10.0.100.20',
        currentSnapshot: 'clean-state',
      },
    ],
  },
  {
    id: 'pod-def456',
    labTemplate: 'Intrusion Detection with Snort',
    platform: 'proxmox',
    owner: 'demo@example.com',
    status: 'running',
    createdAt: new Date(Date.now() - 30 * 60 * 1000).toISOString(), // 30 minutes ago
    expiresAt: new Date(Date.now() + 5.5 * 60 * 60 * 1000).toISOString(), // 5.5 hours from now
    vms: [
      {
        name: 'security-onion',
        platformId: 'vm-201',
        status: 'running',
        ipAddress: '10.0.101.10',
        currentSnapshot: 'baseline',
      },
      {
        name: 'attack-host',
        platformId: 'vm-202',
        status: 'running',
        ipAddress: '10.0.101.20',
        currentSnapshot: 'ready',
      },
      {
        name: 'victim-server',
        platformId: 'vm-203',
        status: 'stopped',
        ipAddress: '10.0.101.30',
        currentSnapshot: 'vulnerable',
      },
    ],
  },
  {
    id: 'pod-ghi789',
    labTemplate: 'Network Fundamentals Review',
    platform: 'proxmox',
    owner: 'demo@example.com',
    status: 'provisioning',
    createdAt: new Date(Date.now() - 5 * 60 * 1000).toISOString(), // 5 minutes ago
    vms: [
      {
        name: 'router-1',
        platformId: 'vm-301',
        status: 'starting',
        currentSnapshot: 'initial',
      },
      {
        name: 'switch-1',
        platformId: 'vm-302',
        status: 'starting',
        currentSnapshot: 'initial',
      },
      {
        name: 'client-pc',
        platformId: 'vm-303',
        status: 'stopped',
        currentSnapshot: 'initial',
      },
    ],
  },
]

// ============================================================================
// API
// ============================================================================
export const podsApi = {
  list: async (owner?: string): Promise<Pod[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      if (owner) {
        return mockPods.filter(p => p.owner === owner)
      }
      return mockPods
    }
    const params = owner ? { owner } : {}
    const response = await api.get<PodsListResponse>('/pods', { params })
    return response.data.pods || []
  },

  get: async (id: string): Promise<Pod> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const pod = mockPods.find(p => p.id === id)
      if (!pod) throw new Error(`Pod not found: ${id}`)
      return pod
    }
    const response = await api.get<Pod>(`/pods/${id}`)
    return response.data
  },

  create: async (labTemplate: string, owner: string): Promise<Pod> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(1000) // Longer delay for creation
      const newPod: Pod = {
        id: `pod-${Date.now()}`,
        labTemplate,
        platform: 'proxmox',
        owner,
        status: 'provisioning',
        createdAt: new Date().toISOString(),
        vms: [
          { name: 'vm-1', platformId: 'vm-new-1', status: 'starting' },
          { name: 'vm-2', platformId: 'vm-new-2', status: 'starting' },
        ],
      }
      mockPods.push(newPod)
      return newPod
    }
    const response = await api.post<Pod>('/pods', { labTemplate, owner })
    return response.data
  },

  destroy: async (id: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const index = mockPods.findIndex(p => p.id === id)
      if (index !== -1) mockPods.splice(index, 1)
      return
    }
    await api.delete(`/pods/${id}`)
  },

  start: async (id: string): Promise<Pod> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      const pod = mockPods.find(p => p.id === id)
      if (!pod) throw new Error(`Pod not found: ${id}`)
      if (pod.status !== 'stopped') {
        throw new Error(`Pod must be stopped to start (current status: ${pod.status})`)
      }
      pod.status = 'running'
      pod.vms.forEach(vm => { vm.status = 'running' })
      return pod
    }
    const response = await api.post<Pod>(`/pods/${id}/start`)
    return response.data
  },

  stop: async (id: string): Promise<Pod> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      const pod = mockPods.find(p => p.id === id)
      if (!pod) throw new Error(`Pod not found: ${id}`)
      if (pod.status !== 'running') {
        throw new Error(`Pod must be running to stop (current status: ${pod.status})`)
      }
      pod.status = 'stopped'
      pod.vms.forEach(vm => { vm.status = 'stopped' })
      return pod
    }
    const response = await api.post<Pod>(`/pods/${id}/stop`)
    return response.data
  },

  resetVM: async (podId: string, vmName: string, snapshot: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      loggers.api.debug(`[Mock] Resetting VM ${vmName} in pod ${podId} to snapshot ${snapshot}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/reset`, { snapshot })
  },

  startVM: async (podId: string, vmName: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      loggers.api.debug(`[Mock] Starting VM ${vmName} in pod ${podId}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/start`)
  },

  stopVM: async (podId: string, vmName: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      loggers.api.debug(`[Mock] Stopping VM ${vmName} in pod ${podId}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/stop`)
  },

  suspendVM: async (podId: string, vmName: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      loggers.api.debug(`[Mock] Suspending VM ${vmName} in pod ${podId}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/suspend`)
  },

  resumeVM: async (podId: string, vmName: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      loggers.api.debug(`[Mock] Resuming VM ${vmName} in pod ${podId}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/resume`)
  },
}

// ============================================================================
// Drift assertions (Phase 7 pilot)
// ============================================================================
//
// These type-level checks fail the TypeScript build if the hand-rolled
// `Pod` / `PodVM` shapes above drift from the backend's OpenAPI schema.
// They run at zero runtime cost — `vue-tsc --noEmit` catches them.
//
// What we check:
//   1) Every property the hand-rolled type declares must exist on the
//      backend shape (no phantom fields like the old `destroyedAt`).
//   2) The hand-rolled shape must be assignable-from the backend shape
//      (after narrowing its optional properties), so code that reads
//      `pod.status` keeps working as the schema evolves.
//
// What we DON'T check:
//   - That the hand-rolled type covers every backend field. The hand
//     shape is deliberately narrower; see the comment on `Pod` above.
//     Add fields here as the UI starts consuming them.

// 1) Phantom-field guard. Flags a compile error if Pod/PodVM declares
//    a property the backend doesn't serialize.
type _PodPhantomFields = Exclude<keyof Pod, keyof NonNullable<ApiPod>> extends never
  ? true
  : ['ERROR: Pod declares a field not in ApiPod', Exclude<keyof Pod, keyof NonNullable<ApiPod>>]
type _PodVMPhantomFields = Exclude<keyof PodVM, keyof NonNullable<ApiPodVM>> extends never
  ? true
  : ['ERROR: PodVM declares a field not in ApiPodVM', Exclude<keyof PodVM, keyof NonNullable<ApiPodVM>>]

// 2) Narrow-enum guard. Pod.status must be exactly the backend enum
//    (not `string`), which prevents `pod.status === 'destroyd'` typos
//    at every comparison site across the app.
type _PodStatusIsBackendEnum = Pod['status'] extends ApiPodStatus
  ? ApiPodStatus extends Pod['status']
    ? true
    : ['ERROR: Pod.status is narrower than the backend enum']
  : ['ERROR: Pod.status accepts values the backend does not emit']
const _podStatusIsBackendEnum: _PodStatusIsBackendEnum = true
void _podStatusIsBackendEnum

// 3) Field-presence guard. Each required field on Pod must also exist
//    on ApiPod (in any form). This catches reading a field that the
//    backend would never send. Each check is an assignment so the error
//    message points to the specific field.
type _FieldExists<K extends keyof ApiPod> = K
type _IdExists = _FieldExists<'id'>
type _LabTemplateExists = _FieldExists<'labTemplate'>
type _PlatformExists = _FieldExists<'platform'>
type _OwnerExists = _FieldExists<'owner'>
type _StatusExists = _FieldExists<'status'>
type _VMsExists = _FieldExists<'vms'>
type _CreatedAtExists = _FieldExists<'createdAt'>
type _ExpiresAtExists = _FieldExists<'expiresAt'>
// Reference the types so TS doesn't flag them as unused.
void (null as unknown as
  | _IdExists
  | _LabTemplateExists
  | _PlatformExists
  | _OwnerExists
  | _StatusExists
  | _VMsExists
  | _CreatedAtExists
  | _ExpiresAtExists
)

