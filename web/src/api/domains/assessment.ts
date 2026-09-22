/**
 * Assessment API
 * Automated assessment and grading
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export type AssessmentStatus = 'correct' | 'incorrect' | 'incomplete' | 'pending' | 'error'

// Verification types for assessment checks
export type VerifyType =
  | 'config_value'
  | 'interface_status'
  | 'interface_ip'
  | 'subnet_mask'
  | 'default_gateway'
  | 'route_exists'
  | 'acl_rule'
  | 'service_running'
  | 'file_content'
  | 'connectivity'
  | 'command'

export interface VerifyConfig {
  type: VerifyType
  command?: string
  path?: string
  field?: string
  expected: string
  regex?: string
  operator?: 'eq' | 'ne' | 'gt' | 'lt' | 'contains' | 'matches'
}

export interface AssessmentCheckConfig {
  id: string
  description: string
  component: string
  points: number
  verify: VerifyConfig
}

export interface AssessmentInterfaceConfig {
  name: string
  checks: AssessmentCheckConfig[]
}

export interface AssessmentDeviceConfig {
  name: string
  type: 'router' | 'switch' | 'host' | 'firewall'
  checks?: AssessmentCheckConfig[]
  interfaces?: AssessmentInterfaceConfig[]
}

export interface AssessmentComponentConfig {
  id: string
  description: string
  weight: number
}

export interface AssessmentTemplate {
  components: AssessmentComponentConfig[]
  devices: AssessmentDeviceConfig[]
}

export interface CheckResult {
  id: string
  description: string
  component: string
  status: AssessmentStatus
  expected: string
  actual: string
  points: number
  earnedPoints: number
  feedback: string
  checkedAt?: string
  error?: string
}

export interface InterfaceResult {
  name: string
  status: AssessmentStatus
  checks: CheckResult[]
  totalItems: number
  passedItems: number
  earnedPoints: number
  maxPoints: number
}

export interface DeviceResult {
  name: string
  type: string
  status: AssessmentStatus
  checks?: CheckResult[]
  interfaces?: InterfaceResult[]
  totalItems: number
  passedItems: number
  earnedPoints: number
  maxPoints: number
}

export interface ComponentResult {
  id: string
  description: string
  totalItems: number
  passedItems: number
  maxPoints: number
  earnedPoints: number
  percentage: number
}

export interface AssessmentResult {
  sessionId: string
  score: number
  maxScore: number
  percentage: number
  itemCount: number
  passedCount: number
  status: string
  startedAt: string
  lastChecked: string
  components: ComponentResult[]
  devices: DeviceResult[]
  timeElapsed: string
}

export interface AssessmentStatusResponse {
  sessionId: string
  status: string
  score?: number
  maxScore?: number
  percentage?: number
  itemCount?: number
  passedCount?: number
  lastChecked?: string
  timeElapsed?: string
}

// Discriminated union for type-safe WebSocket updates
export interface CheckUpdatePayload {
  type: 'check_update'
  sessionId: string
  deviceName: string
  interfaceName?: string
  checkId: string
  status: AssessmentStatus
  expected?: string
  actual?: string
  points?: number
  earnedPoints?: number
  feedback?: string
  timestamp: string
}

export interface DeviceUpdatePayload {
  type: 'device_update'
  sessionId: string
  deviceName: string
  status: AssessmentStatus
  earnedPoints?: number
  passedItems?: number
  timestamp: string
}

export interface ComponentUpdatePayload {
  type: 'component_update'
  sessionId: string
  componentId: string
  earnedPoints: number
  maxPoints: number
  passedItems: number
  totalItems: number
  percentage: number
  timestamp: string
}

export interface AssessmentCompletePayload {
  type: 'complete'
  sessionId: string
  totalScore: number
  maxScore: number
  passedItems: number
  totalItems: number
  percentage: number
  timestamp: string
}

export type AssessmentUpdate =
  | CheckUpdatePayload
  | DeviceUpdatePayload
  | ComponentUpdatePayload
  | AssessmentCompletePayload

// ============================================================================
// Mock Data
// ============================================================================
export const mockAssessmentResult: AssessmentResult = {
  sessionId: 'session-001',
  score: 650,
  maxScore: 1000,
  percentage: 65,
  itemCount: 10,
  passedCount: 6,
  status: 'in_progress',
  startedAt: new Date(Date.now() - 45 * 60 * 1000).toISOString(),
  lastChecked: new Date(Date.now() - 2 * 60 * 1000).toISOString(),
  timeElapsed: '45m 00s',
  components: [
    {
      id: 'firewall-rules',
      description: 'Firewall Rule Configuration',
      totalItems: 5,
      passedItems: 4,
      maxPoints: 500,
      earnedPoints: 400,
      percentage: 80,
    },
    {
      id: 'nat-config',
      description: 'NAT Configuration',
      totalItems: 3,
      passedItems: 2,
      maxPoints: 300,
      earnedPoints: 200,
      percentage: 67,
    },
    {
      id: 'security-best-practices',
      description: 'Security Best Practices',
      totalItems: 2,
      passedItems: 0,
      maxPoints: 200,
      earnedPoints: 50,
      percentage: 25,
    },
  ],
  devices: [
    {
      name: 'pfsense-firewall',
      type: 'firewall',
      status: 'incomplete',
      totalItems: 8,
      passedItems: 5,
      earnedPoints: 550,
      maxPoints: 800,
      interfaces: [
        {
          name: 'WAN',
          status: 'correct',
          totalItems: 2,
          passedItems: 2,
          earnedPoints: 200,
          maxPoints: 200,
          checks: [
            {
              id: 'wan-ip',
              description: 'WAN IP configured correctly',
              component: 'firewall-rules',
              status: 'correct',
              expected: 'DHCP',
              actual: 'DHCP',
              points: 100,
              earnedPoints: 100,
              feedback: 'WAN interface correctly configured for DHCP',
            },
            {
              id: 'wan-gateway',
              description: 'Default gateway set',
              component: 'firewall-rules',
              status: 'correct',
              expected: 'Auto',
              actual: 'Auto',
              points: 100,
              earnedPoints: 100,
              feedback: 'Gateway automatically configured',
            },
          ],
        },
        {
          name: 'LAN',
          status: 'incomplete',
          totalItems: 3,
          passedItems: 2,
          earnedPoints: 200,
          maxPoints: 300,
          checks: [
            {
              id: 'lan-ip',
              description: 'LAN IP configured',
              component: 'firewall-rules',
              status: 'correct',
              expected: '10.0.100.1/24',
              actual: '10.0.100.1/24',
              points: 100,
              earnedPoints: 100,
              feedback: 'LAN IP correctly configured',
            },
            {
              id: 'lan-dhcp',
              description: 'DHCP server enabled',
              component: 'firewall-rules',
              status: 'correct',
              expected: 'enabled',
              actual: 'enabled',
              points: 100,
              earnedPoints: 100,
              feedback: 'DHCP server is running',
            },
            {
              id: 'lan-dns',
              description: 'DNS forwarder configured',
              component: 'nat-config',
              status: 'incorrect',
              expected: '8.8.8.8, 8.8.4.4',
              actual: 'not configured',
              points: 100,
              earnedPoints: 0,
              feedback: 'DNS forwarder needs to be configured with upstream servers',
            },
          ],
        },
      ],
    },
    {
      name: 'windows-target',
      type: 'host',
      status: 'incomplete',
      totalItems: 2,
      passedItems: 1,
      earnedPoints: 100,
      maxPoints: 200,
      checks: [
        {
          id: 'windows-firewall',
          description: 'Windows Firewall enabled',
          component: 'security-best-practices',
          status: 'correct',
          expected: 'enabled',
          actual: 'enabled',
          points: 100,
          earnedPoints: 100,
          feedback: 'Windows Firewall is active',
        },
        {
          id: 'rdp-disabled',
          description: 'RDP disabled for security',
          component: 'security-best-practices',
          status: 'incorrect',
          expected: 'disabled',
          actual: 'enabled',
          points: 100,
          earnedPoints: 0,
          feedback: 'RDP should be disabled or restricted to specific IPs',
        },
      ],
    },
  ],
}

// ============================================================================
// API
// ============================================================================
export const assessmentApi = {
  get: async (sessionId: string) => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return { data: { ...mockAssessmentResult, sessionId } }
    }
    return api.get<AssessmentResult>(`/assessment/${sessionId}`)
  },

  run: async (sessionId: string, template?: AssessmentTemplate) => {
    if (USE_MOCK_DATA) {
      await simulateDelay(1500) // Longer delay for assessment run
      return { data: { ...mockAssessmentResult, sessionId, lastChecked: new Date().toISOString() } }
    }
    return api.post<AssessmentResult>(`/assessment/${sessionId}/verify`, { template })
  },

  getStatus: async (sessionId: string) => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        data: {
          sessionId,
          status: mockAssessmentResult.status,
          score: mockAssessmentResult.score,
          maxScore: mockAssessmentResult.maxScore,
          percentage: mockAssessmentResult.percentage,
          itemCount: mockAssessmentResult.itemCount,
          passedCount: mockAssessmentResult.passedCount,
          lastChecked: mockAssessmentResult.lastChecked,
          timeElapsed: mockAssessmentResult.timeElapsed,
        },
      }
    }
    return api.get<AssessmentStatusResponse>(`/assessment/${sessionId}/status`)
  },

  getComponents: async (sessionId: string) => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return { data: { sessionId, components: mockAssessmentResult.components } }
    }
    return api.get<{ sessionId: string; components: ComponentResult[] }>(`/assessment/${sessionId}/components`)
  },

  getDevice: async (sessionId: string, deviceName: string) => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const device = mockAssessmentResult.devices.find(d => d.name === deviceName)
      if (!device) throw new Error(`Device not found: ${deviceName}`)
      return { data: device }
    }
    return api.get<DeviceResult>(`/assessment/${sessionId}/devices/${deviceName}`)
  },
}
