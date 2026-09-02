/**
 * Teams API
 * Team management within organizations
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface Team {
  id: string
  organizationId: string
  name: string
  slug: string
  description?: string
  parentTeamId?: string
  isActive: boolean
  createdAt: string
}

export interface TeamMembership {
  id: string
  teamId: string
  userId: string
  role: 'lead' | 'member'
  createdAt: string
}

export interface TeamWithMembership {
  team: Team
  membership?: TeamMembership
  memberCount: number
}

// ============================================================================
// Mock Data
// ============================================================================
export const mockTeams: TeamWithMembership[] = [
  {
    team: {
      id: 'team-1',
      organizationId: 'org-1',
      name: 'Security Team',
      slug: 'security-team',
      description: 'Cybersecurity training team',
      isActive: true,
      createdAt: new Date().toISOString(),
    },
    membership: {
      id: 'tm-1',
      teamId: 'team-1',
      userId: '00000000-0000-0000-0000-000000000001',
      role: 'lead',
      createdAt: new Date().toISOString(),
    },
    memberCount: 5,
  },
  {
    team: {
      id: 'team-2',
      organizationId: 'org-1',
      name: 'Networking Team',
      slug: 'networking-team',
      description: 'Network operations training',
      isActive: true,
      createdAt: new Date().toISOString(),
    },
    memberCount: 3,
  },
]

// ============================================================================
// API
// ============================================================================
export const teamsApi = {
  get: async (teamId: string): Promise<TeamWithMembership> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const team = mockTeams.find(t => t.team.id === teamId)
      if (!team) throw new Error(`Team not found: ${teamId}`)
      return team
    }
    const response = await api.get<TeamWithMembership>(`/teams/${teamId}`)
    return response.data
  },

  create: async (orgId: string, data: { name: string; description?: string }): Promise<Team> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const newTeam: Team = {
        id: `team-${Date.now()}`,
        organizationId: orgId,
        name: data.name,
        slug: data.name.toLowerCase().replace(/\s+/g, '-'),
        ...(data.description ? { description: data.description } : {}),
        isActive: true,
        createdAt: new Date().toISOString(),
      }
      return newTeam
    }
    const response = await api.post<Team>(`/organizations/${orgId}/teams`, data)
    return response.data
  },

  update: async (teamId: string, data: { name?: string; description?: string }): Promise<Team> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const team = mockTeams.find(t => t.team.id === teamId)
      if (!team) throw new Error(`Team not found: ${teamId}`)
      return { ...team.team, ...data }
    }
    const response = await api.put<Team>(`/teams/${teamId}`, data)
    return response.data
  },

  delete: async (teamId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return
    }
    await api.delete(`/teams/${teamId}`)
  },

  getMembers: async (teamId: string): Promise<Array<{ user: { id: string; name: string; email: string }; membership: TeamMembership }>> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return []
    }
    const response = await api.get(`/teams/${teamId}/members`)
    return response.data.members || []
  },

  addMember: async (teamId: string, userId: string, role: 'lead' | 'member' = 'member'): Promise<TeamMembership> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return {
        id: `tm-${Date.now()}`,
        teamId,
        userId,
        role,
        createdAt: new Date().toISOString(),
      }
    }
    const response = await api.post<TeamMembership>(`/teams/${teamId}/members`, { userId, role })
    return response.data
  },

  removeMember: async (teamId: string, userId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return
    }
    await api.delete(`/teams/${teamId}/members/${userId}`)
  },
}
