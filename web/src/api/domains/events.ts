/**
 * Events API
 * Monitoring events from OSSEC/Wazuh
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface MonitoringEvent {
  id: number
  timestamp: string
  podId: string
  sessionId?: string
  vmName: string
  eventType: string
  ruleId?: string
  ruleLevel?: number
  description?: string
  processed: boolean
  matchedCheckpoints?: string[]
}

export interface EventsListResponse {
  events: MonitoringEvent[]
  count: number
  limit: number
  offset?: number
}

export interface EventStatsResponse {
  totalEvents: number
  eventsToday: number
  eventsThisHour: number
  processedEvents: number
  pendingEvents: number
  eventsByType: Record<string, number>
  checkpointsPassed: number
  checkpointsMatched: number
  generatedAt: string
}

export interface EventsFilterParams {
  limit?: number
  offset?: number
  eventType?: string
  podId?: string
  sessionId?: string
  vmName?: string
  start?: string
  end?: string
}

// ============================================================================
// Mock Data
// ============================================================================
const mockEvents: MonitoringEvent[] = [
  {
    id: 1,
    timestamp: new Date(Date.now() - 60000).toISOString(),
    podId: 'pod-001',
    sessionId: 'session-001',
    vmName: 'linux-vm',
    eventType: 'syscheck',
    description: 'File created: /home/student/notes.txt',
    processed: true,
    matchedCheckpoints: ['vim-file']
  },
  {
    id: 2,
    timestamp: new Date(Date.now() - 120000).toISOString(),
    podId: 'pod-001',
    sessionId: 'session-001',
    vmName: 'linux-vm',
    eventType: 'audit',
    description: 'Command executed: pwd',
    processed: true,
    matchedCheckpoints: ['pwd-command']
  },
  {
    id: 3,
    timestamp: new Date(Date.now() - 180000).toISOString(),
    podId: 'pod-001',
    sessionId: 'session-001',
    vmName: 'linux-vm',
    eventType: 'auth',
    description: 'SSH login successful',
    processed: true,
    matchedCheckpoints: []
  }
]

const mockEventStats: EventStatsResponse = {
  totalEvents: 156,
  eventsToday: 42,
  eventsThisHour: 12,
  processedEvents: 150,
  pendingEvents: 6,
  eventsByType: {
    'syscheck': 45,
    'audit': 38,
    'auth': 52,
    'ssh': 21
  },
  checkpointsPassed: 28,
  checkpointsMatched: 35,
  generatedAt: new Date().toISOString()
}

// ============================================================================
// API
// ============================================================================
export const eventsApi = {
  list: async (params?: EventsFilterParams): Promise<EventsListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        events: mockEvents,
        count: mockEvents.length,
        limit: params?.limit || 50
      }
    }
    const response = await api.get<EventsListResponse>('/monitoring/events', { params })
    return response.data
  },

  getStats: async (): Promise<EventStatsResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockEventStats
    }
    const response = await api.get<EventStatsResponse>('/monitoring/stats')
    return response.data
  },

  getBySession: async (sessionId: string, limit?: number): Promise<EventsListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const filtered = mockEvents.filter(e => e.sessionId === sessionId)
      return {
        events: filtered,
        count: filtered.length,
        limit: limit || 100
      }
    }
    const params = limit ? { limit } : {}
    const response = await api.get<EventsListResponse>(`/monitoring/events/session/${sessionId}`, { params })
    return response.data
  },

  getByPod: async (podId: string, limit?: number): Promise<EventsListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const filtered = mockEvents.filter(e => e.podId === podId)
      return {
        events: filtered,
        count: filtered.length,
        limit: limit || 100
      }
    }
    const params = limit ? { limit } : {}
    const response = await api.get<EventsListResponse>(`/monitoring/events/pod/${podId}`, { params })
    return response.data
  }
}
