/**
 * Organizations API
 * Multi-tenant organization management
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import { mockFeatures, type FeatureFlag } from './features'
import { mockTeams, type TeamWithMembership } from './teams'

// ============================================================================
// Types
// ============================================================================
export type Edition = 'community' | 'professional' | 'enterprise'
export type OrgRole = 'owner' | 'admin' | 'instructor' | 'member'
export type OrgType = 'standard' | 'educational' | 'enterprise'

export interface Organization {
  id: string
  name: string
  slug: string
  type: OrgType
  edition: Edition
  logoUrl?: string
  contactEmail?: string
  maxUsers?: number
  maxConcurrentPods?: number
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface OrganizationMembership {
  id: string
  organizationId: string
  userId: string
  role: OrgRole
  isPrimary: boolean
  invitedAt: string
  acceptedAt?: string
}

export interface OrganizationWithMembership {
  organization: Organization
  membership: OrganizationMembership
}

// ============================================================================
// Mock Data
// ============================================================================
const mockOrganizations: OrganizationWithMembership[] = [
  {
    organization: {
      id: 'org-1',
      name: 'Demo Organization',
      slug: 'demo-org',
      type: 'standard',
      edition: 'professional',
      isActive: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    membership: {
      id: 'mem-1',
      organizationId: 'org-1',
      userId: '00000000-0000-0000-0000-000000000001',
      role: 'admin',
      isPrimary: true,
      invitedAt: new Date().toISOString(),
      acceptedAt: new Date().toISOString(),
    },
  },
  {
    organization: {
      id: 'org-2',
      name: 'Acme University',
      slug: 'acme-university',
      type: 'educational',
      edition: 'enterprise',
      isActive: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    membership: {
      id: 'mem-2',
      organizationId: 'org-2',
      userId: '00000000-0000-0000-0000-000000000001',
      role: 'instructor',
      isPrimary: false,
      invitedAt: new Date().toISOString(),
      acceptedAt: new Date().toISOString(),
    },
  },
]

// Mock data imported from canonical sources (teams.ts, features.ts)

// Backend response type (flat structure)
interface BackendOrganization {
  id: string
  name: string
  slug: string
  type: OrgType
  edition: Edition
  role: OrgRole
  isPrimary: boolean
  memberCount?: number
  teamCount?: number
  isActive: boolean
  contactEmail?: string
  logoUrl?: string
  createdAt: string
}

// ============================================================================
// API
// ============================================================================
export const organizationsApi = {
  list: async (): Promise<OrganizationWithMembership[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockOrganizations
    }
    // Backend returns a flat structure, transform to nested OrganizationWithMembership
    const response = await api.get<{ organizations: BackendOrganization[] }>('/organizations')
    const orgs = response.data.organizations || []

    return orgs.map((org): OrganizationWithMembership => ({
      organization: {
        id: org.id,
        name: org.name,
        slug: org.slug,
        type: org.type,
        edition: org.edition,
        ...(org.logoUrl && { logoUrl: org.logoUrl }),
        ...(org.contactEmail && { contactEmail: org.contactEmail }),
        isActive: org.isActive,
        createdAt: org.createdAt,
        updatedAt: org.createdAt, // Backend doesn't return updatedAt separately
      },
      membership: {
        id: `mem-${org.id}`, // Generate membership ID
        organizationId: org.id,
        userId: '', // Will be filled by caller if needed
        role: org.role,
        isPrimary: org.isPrimary,
        invitedAt: org.createdAt,
        acceptedAt: org.createdAt,
      },
    }))
  },

  update: async (id: string, data: { name?: string; contactEmail?: string }): Promise<Organization> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const org = mockOrganizations.find(o => o.organization.id === id)
      if (!org) throw new Error(`Organization not found: ${id}`)
      if (data.name) org.organization.name = data.name
      if (data.contactEmail) org.organization.contactEmail = data.contactEmail
      org.organization.updatedAt = new Date().toISOString()
      return org.organization
    }
    const response = await api.put<Organization>(`/organizations/${id}`, data)
    return response.data
  },

  get: async (id: string): Promise<Organization> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const org = mockOrganizations.find(o => o.organization.id === id)
      if (!org) throw new Error(`Organization not found: ${id}`)
      return org.organization
    }
    const response = await api.get<Organization>(`/organizations/${id}`)
    return response.data
  },

  getFeatures: async (orgId: string): Promise<FeatureFlag[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockFeatures
    }
    const response = await api.get<FeatureFlag[]>(`/organizations/${orgId}/features`)
    return response.data
  },

  getTeams: async (orgId: string): Promise<TeamWithMembership[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockTeams.filter(t => t.team.organizationId === orgId)
    }
    const response = await api.get<{ teams: TeamWithMembership[] }>(`/organizations/${orgId}/teams`)
    return response.data.teams || []
  },

  getMembers: async (orgId: string): Promise<{ members: Array<{ user: { id: string; name: string; email: string }; membership: OrganizationMembership }> }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        members: [
          {
            user: { id: '00000000-0000-0000-0000-000000000001', name: 'Demo User', email: 'demo@example.com' },
            membership: mockOrganizations[0]!.membership,
          },
        ],
      }
    }
    const response = await api.get(`/organizations/${orgId}/members`)
    return response.data
  },
}
