/**
 * Audit API
 * Audit logging and compliance tracking
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface AuditEntry {
  id: string
  timestamp: string
  actorId: string
  actorType: string
  action: string
  resourceType: string
  resourceId?: string
  details?: Record<string, unknown>
  ipAddress?: string
  userAgent?: string
}

export interface AuditLogFilter {
  actor?: string
  action?: string
  resourceType?: string
  resourceId?: string
  startTime?: string
  endTime?: string
  limit?: number
  offset?: number
}

export interface AuditLogListResponse {
  entries: AuditEntry[]
  totalCount: number
  limit: number
  offset: number
}

// ============================================================================
// Mock Data
// ============================================================================
const mockAuditEntries: AuditEntry[] = [
  {
    id: 'audit-001',
    timestamp: new Date(Date.now() - 1000 * 60 * 5).toISOString(),
    actorId: 'user-001',
    actorType: 'user',
    action: 'member.invited',
    resourceType: 'membership',
    resourceId: 'membership-001',
    details: { userId: 'user-002', role: 'member', organizationId: 'org-001' },
    ipAddress: '192.168.1.100',
    userAgent: 'Mozilla/5.0',
  },
  {
    id: 'audit-002',
    timestamp: new Date(Date.now() - 1000 * 60 * 30).toISOString(),
    actorId: 'user-001',
    actorType: 'user',
    action: 'organization.updated',
    resourceType: 'organization',
    resourceId: 'org-001',
    details: { name: 'Demo Organization', slug: 'demo-org' },
    ipAddress: '192.168.1.100',
    userAgent: 'Mozilla/5.0',
  },
  {
    id: 'audit-003',
    timestamp: new Date(Date.now() - 1000 * 60 * 60).toISOString(),
    actorId: 'user-003',
    actorType: 'user',
    action: 'member.role_changed',
    resourceType: 'membership',
    resourceId: 'membership-002',
    details: { userId: 'user-004', oldRole: 'member', newRole: 'instructor' },
    ipAddress: '10.0.0.50',
    userAgent: 'Mozilla/5.0',
  },
  {
    id: 'audit-004',
    timestamp: new Date(Date.now() - 1000 * 60 * 120).toISOString(),
    actorId: 'user-001',
    actorType: 'user',
    action: 'auth.login.success',
    resourceType: 'session',
    resourceId: 'session-001',
    details: { username: 'admin' },
    ipAddress: '192.168.1.100',
    userAgent: 'Mozilla/5.0',
  },
]

// ============================================================================
// API
// ============================================================================
export const auditApi = {
  list: async (orgId: string, filter: AuditLogFilter = {}): Promise<AuditLogListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        entries: mockAuditEntries,
        totalCount: mockAuditEntries.length,
        limit: filter.limit || 50,
        offset: filter.offset || 0,
      }
    }
    const params = new URLSearchParams()
    if (filter.actor) params.set('actor', filter.actor)
    if (filter.action) params.set('action', filter.action)
    if (filter.resourceType) params.set('resource_type', filter.resourceType)
    if (filter.resourceId) params.set('resource_id', filter.resourceId)
    if (filter.startTime) params.set('start_time', filter.startTime)
    if (filter.endTime) params.set('end_time', filter.endTime)
    if (filter.limit) params.set('limit', filter.limit.toString())
    if (filter.offset) params.set('offset', filter.offset.toString())

    const response = await api.get<AuditLogListResponse>(
      `/organizations/${orgId}/audit?${params.toString()}`
    )
    return response.data
  },

  get: async (orgId: string, entryId: string): Promise<AuditEntry> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const entry = mockAuditEntries.find(e => e.id === entryId)
      if (!entry) throw new Error(`Audit entry not found: ${entryId}`)
      return entry
    }
    const response = await api.get<AuditEntry>(`/organizations/${orgId}/audit/${entryId}`)
    return response.data
  },

  export: async (orgId: string, startTime?: string, endTime?: string): Promise<AuditEntry[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockAuditEntries
    }
    const response = await api.post<{ entries: AuditEntry[] }>(
      `/organizations/${orgId}/audit/export`,
      { startTime, endTime }
    )
    return response.data.entries
  },
}
