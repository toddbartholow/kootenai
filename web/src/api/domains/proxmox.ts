/**
 * Proxmox API
 * Direct VM access (bypasses pod lookup, useful for development)
 */
import { api } from '../config'
import type { ConsoleTicket } from './console'

// ============================================================================
// Types
// ============================================================================
export interface ProxmoxVM {
  vmid: number
  name: string
  status: string
  node: string
}

// ============================================================================
// API
// ============================================================================
export const proxmoxApi = {
  // Get console ticket for a VM by VMID directly (bypasses pod lookup)
  getDirectConsole: async (vmid: number, type?: 'spice' | 'vnc', node?: string): Promise<ConsoleTicket> => {
    const params: Record<string, string> = {}
    if (type) params['type'] = type
    if (node) params['node'] = node
    const response = await api.get<ConsoleTicket>(`/proxmox/vms/${vmid}/console`, { params })
    return response.data
  },
}
