/**
 * Snapshots API
 * VM snapshot management within pods
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import { loggers } from '@/utils/logger'

// ============================================================================
// Types
// ============================================================================
export interface Snapshot {
  name: string
  description?: string
  parent?: string
}

export interface SnapshotsListResponse {
  podId: string
  vmName: string
  snapshots: Snapshot[]
  count: number
}

// ============================================================================
// Mock Data
// ============================================================================
export const mockSnapshots: Record<string, Record<string, Snapshot[]>> = {
  'pod-abc123': {
    'kali-attacker': [
      { name: 'clean-state', description: 'Fresh Kali installation' },
      { name: 'tools-installed', description: 'With additional pentest tools', parent: 'clean-state' },
    ],
    'pfsense-firewall': [
      { name: 'initial-config', description: 'Basic pfSense configuration' },
      { name: 'rules-configured', description: 'With firewall rules', parent: 'initial-config' },
    ],
    'windows-target': [
      { name: 'clean-state', description: 'Fresh Windows Server installation' },
      { name: 'services-running', description: 'With web and SQL services', parent: 'clean-state' },
    ],
  },
  'pod-def456': {
    'security-onion': [
      { name: 'baseline', description: 'Security Onion baseline' },
      { name: 'snort-configured', description: 'With Snort rules', parent: 'baseline' },
    ],
    'attack-host': [
      { name: 'ready', description: 'Attack tools ready' },
    ],
    'victim-server': [
      { name: 'vulnerable', description: 'Vulnerable server configuration' },
    ],
  },
}

// ============================================================================
// API
// ============================================================================
export const snapshotsApi = {
  list: async (podId: string, vmName: string): Promise<Snapshot[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const podSnapshots = mockSnapshots[podId]
      if (podSnapshots && podSnapshots[vmName]) {
        return podSnapshots[vmName]
      }
      return []
    }
    const response = await api.get<SnapshotsListResponse>(`/pods/${podId}/vms/${vmName}/snapshots`)
    return response.data.snapshots || []
  },

  create: async (podId: string, vmName: string, name: string, description?: string, includeRam?: boolean): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      loggers.api.debug(`[Mock] Creating snapshot ${name} for VM ${vmName} in pod ${podId}`)
      return
    }
    await api.post(`/pods/${podId}/vms/${vmName}/snapshots`, { name, description, includeRam })
  },

  delete: async (podId: string, vmName: string, snapshotName: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      loggers.api.debug(`[Mock] Deleting snapshot ${snapshotName} from VM ${vmName} in pod ${podId}`)
      return
    }
    await api.delete(`/pods/${podId}/vms/${vmName}/snapshots/${snapshotName}`)
  },
}
