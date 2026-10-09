/**
 * Console API
 * VNC/SPICE console access for VMs
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface ConsoleTicket {
  type: 'spice' | 'vnc'
  host: string
  port: number
  tlsPort?: number
  ticket: string
  password?: string
  node: string
  vmid: string
}

// ============================================================================
// API
// ============================================================================
export const consoleApi = {
  getTicket: async (
    podId: string,
    vmName: string,
    type?: 'spice' | 'vnc',
  ): Promise<ConsoleTicket> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      // Return mock console ticket for development (RFC 5737 documentation IP)
      return {
        type: 'vnc',
        host: '192.0.2.20',
        port: 5900,
        ticket: 'mock-ticket-' + Date.now(),
        node: 'pve',
        vmid: '9001',
      }
    }
    // Only pass type if explicitly specified; otherwise let server auto-detect (VNC first, then SPICE)
    const params = type ? { type } : {}
    const response = await api.get<ConsoleTicket>(`/pods/${podId}/vms/${vmName}/console`, {
      params,
    })
    return response.data
  },
}
