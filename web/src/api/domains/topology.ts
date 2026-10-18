/**
 * Topology API
 * Network topology visualization data
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================

export interface NetworkSegment {
  name: string
  vlan: number
  subnet: string
  gateway?: string
  dhcp: boolean
}

export interface TopologyVM {
  name: string
  platformId: string
  status: string
  ipAddress?: string
  template: string
  networks: {
    segment: string
    ip?: string
  }[]
  resources: {
    cpu: number
    memory: number
    disk?: number
  }
}

export interface TopologyData {
  podId: string
  labTemplate: string
  segments: NetworkSegment[]
  vms: TopologyVM[]
}

// ============================================================================
// Mock Data
// ============================================================================

const mockTopologyData: Record<string, TopologyData> = {
  'pod-abc123': {
    podId: 'pod-abc123',
    labTemplate: 'Firewall Configuration 101',
    segments: [
      { name: 'external', vlan: 100, subnet: '10.0.100.0/24', gateway: '10.0.100.1', dhcp: false },
      { name: 'internal', vlan: 101, subnet: '10.0.101.0/24', gateway: '10.0.101.1', dhcp: true },
    ],
    vms: [
      {
        name: 'kali-attacker',
        platformId: 'vm-101',
        status: 'running',
        ipAddress: '10.0.100.10',
        template: 'kali-linux',
        networks: [{ segment: 'external', ip: '10.0.100.10' }],
        resources: { cpu: 2, memory: 4096, disk: 32 },
      },
      {
        name: 'pfsense-firewall',
        platformId: 'vm-102',
        status: 'running',
        ipAddress: '10.0.100.1',
        template: 'pfsense',
        networks: [
          { segment: 'external', ip: '10.0.100.1' },
          { segment: 'internal', ip: '10.0.101.1' },
        ],
        resources: { cpu: 1, memory: 2048, disk: 16 },
      },
      {
        name: 'windows-target',
        platformId: 'vm-103',
        status: 'running',
        ipAddress: '10.0.101.20',
        template: 'windows-10',
        networks: [{ segment: 'internal', ip: '10.0.101.20' }],
        resources: { cpu: 2, memory: 4096, disk: 64 },
      },
    ],
  },
  'pod-def456': {
    podId: 'pod-def456',
    labTemplate: 'Intrusion Detection with Snort',
    segments: [
      { name: 'monitor', vlan: 101, subnet: '10.0.101.0/24', dhcp: false },
    ],
    vms: [
      {
        name: 'security-onion',
        platformId: 'vm-201',
        status: 'running',
        ipAddress: '10.0.101.10',
        template: 'security-onion',
        networks: [{ segment: 'monitor', ip: '10.0.101.10' }],
        resources: { cpu: 4, memory: 8192, disk: 128 },
      },
      {
        name: 'attack-host',
        platformId: 'vm-202',
        status: 'running',
        ipAddress: '10.0.101.20',
        template: 'kali-linux',
        networks: [{ segment: 'monitor', ip: '10.0.101.20' }],
        resources: { cpu: 2, memory: 4096, disk: 32 },
      },
      {
        name: 'victim-server',
        platformId: 'vm-203',
        status: 'stopped',
        ipAddress: '10.0.101.30',
        template: 'ubuntu-server',
        networks: [{ segment: 'monitor', ip: '10.0.101.30' }],
        resources: { cpu: 2, memory: 2048, disk: 32 },
      },
    ],
  },
}

// Default mock data for unknown pods
const defaultMockTopology = (podId: string): TopologyData => ({
  podId,
  labTemplate: 'Unknown Lab',
  segments: [
    { name: 'network-a', vlan: 101, subnet: '10.10.101.0/24', gateway: '10.10.101.1', dhcp: false },
    { name: 'network-b', vlan: 102, subnet: '10.10.102.0/24', gateway: '10.10.102.1', dhcp: false },
  ],
  vms: [
    {
      name: 'router',
      platformId: 'vm-1',
      status: 'running',
      template: 'ubuntu-server',
      networks: [
        { segment: 'network-a', ip: '10.10.101.1' },
        { segment: 'network-b', ip: '10.10.102.1' },
      ],
      resources: { cpu: 1, memory: 1024 },
    },
    {
      name: 'client-a',
      platformId: 'vm-2',
      status: 'running',
      ipAddress: '10.10.101.10',
      template: 'ubuntu-desktop',
      networks: [{ segment: 'network-a', ip: '10.10.101.10' }],
      resources: { cpu: 1, memory: 2048 },
    },
    {
      name: 'client-b',
      platformId: 'vm-3',
      status: 'stopped',
      ipAddress: '10.10.102.10',
      template: 'ubuntu-desktop',
      networks: [{ segment: 'network-b', ip: '10.10.102.10' }],
      resources: { cpu: 1, memory: 2048 },
    },
  ],
})

// ============================================================================
// API
// ============================================================================

export const topologyApi = {
  /**
   * Get topology data for a pod
   */
  getTopology: async (podId: string): Promise<TopologyData> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockTopologyData[podId] || defaultMockTopology(podId)
    }
    const response = await api.get<TopologyData>(`/pods/${podId}/topology`)
    return response.data
  },
}
