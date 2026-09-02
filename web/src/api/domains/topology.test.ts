/**
 * Tests for Topology API module
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { topologyApi } from './topology'
import { api } from '../config'
import * as mockModule from '../shared/mock'

// Mock the api module
vi.mock('../config', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

// Store original mock settings
const originalUseMockData = mockModule.USE_MOCK_DATA

describe('topologyApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    // Restore original mock settings
    vi.stubGlobal('USE_MOCK_DATA', originalUseMockData)
  })

  describe('getTopology', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        // Force mock mode
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock topology data for known pod', async () => {
        const result = await topologyApi.getTopology('pod-abc123')

        expect(result).toBeDefined()
        expect(result.podId).toBe('pod-abc123')
        expect(result.labTemplate).toBe('Firewall Configuration 101')
        expect(result.segments).toHaveLength(2)
        expect(result.vms).toHaveLength(3)
      })

      it('returns mock topology with external and internal segments', async () => {
        const result = await topologyApi.getTopology('pod-abc123')

        // Check segments
        const external = result.segments.find(s => s.name === 'external')
        expect(external).toBeDefined()
        expect(external?.vlan).toBe(100)
        expect(external?.subnet).toBe('10.0.100.0/24')

        const internal = result.segments.find(s => s.name === 'internal')
        expect(internal).toBeDefined()
        expect(internal?.vlan).toBe(101)
        expect(internal?.dhcp).toBe(true)
      })

      it('returns mock VMs with network connections', async () => {
        const result = await topologyApi.getTopology('pod-abc123')

        // Check firewall VM has multiple network connections
        const firewall = result.vms.find(vm => vm.name === 'pfsense-firewall')
        expect(firewall).toBeDefined()
        expect(firewall!.networks).toHaveLength(2)
        expect(firewall!.networks[0]!.segment).toBe('external')
        expect(firewall!.networks[1]!.segment).toBe('internal')
      })

      it('returns default mock topology for unknown pod', async () => {
        const result = await topologyApi.getTopology('unknown-pod-xyz')

        expect(result).toBeDefined()
        expect(result.podId).toBe('unknown-pod-xyz')
        expect(result.labTemplate).toBe('Unknown Lab')
        expect(result.segments).toHaveLength(2)
        expect(result.vms).toHaveLength(3)
      })

      it('returns second mock topology data set', async () => {
        const result = await topologyApi.getTopology('pod-def456')

        expect(result).toBeDefined()
        expect(result.podId).toBe('pod-def456')
        expect(result.labTemplate).toBe('Intrusion Detection with Snort')
        expect(result.segments).toHaveLength(1)
        expect(result.vms).toHaveLength(3)
      })

      it('does not call the actual API in mock mode', async () => {
        await topologyApi.getTopology('pod-abc123')

        expect(api.get).not.toHaveBeenCalled()
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        // Force real API mode
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls the correct API endpoint', async () => {
        const mockResponse = {
          data: {
            podId: 'test-pod',
            labTemplate: 'Test Lab',
            segments: [],
            vms: [],
          },
        }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        const result = await topologyApi.getTopology('test-pod')

        expect(api.get).toHaveBeenCalledWith('/pods/test-pod/topology')
        expect(result.podId).toBe('test-pod')
      })

      it('handles API response with segments and VMs', async () => {
        const mockResponse = {
          data: {
            podId: 'real-pod',
            labTemplate: 'Network Lab',
            segments: [
              { name: 'mgmt', vlan: 10, subnet: '192.168.1.0/24', gateway: '192.168.1.1', dhcp: true },
            ],
            vms: [
              {
                name: 'server1',
                platformId: 'vm-999',
                status: 'running',
                ipAddress: '192.168.1.10',
                template: 'ubuntu-22.04',
                networks: [{ segment: 'mgmt', ip: '192.168.1.10' }],
                resources: { cpu: 4, memory: 8192, disk: 100 },
              },
            ],
          },
        }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        const result = await topologyApi.getTopology('real-pod')

        expect(result.segments).toHaveLength(1)
        expect(result.segments[0]!.name).toBe('mgmt')
        expect(result.vms).toHaveLength(1)
        expect(result.vms[0]!.name).toBe('server1')
        expect(result.vms[0]!.resources.disk).toBe(100)
      })

      it('handles API error', async () => {
        vi.mocked(api.get).mockRejectedValue(new Error('Network error'))

        await expect(topologyApi.getTopology('error-pod')).rejects.toThrow('Network error')
      })
    })
  })
})
