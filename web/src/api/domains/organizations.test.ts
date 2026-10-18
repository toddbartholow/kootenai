/**
 * Tests for Organizations and Teams APIs
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { organizationsApi } from './organizations'
import { teamsApi } from './teams'

// Mock axios
vi.mock('axios', () => {
  const mockAxios = {
    create: vi.fn(() => mockAxios),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    defaults: {
      headers: {
        common: {},
      },
    },
    interceptors: {
      request: {
        use: vi.fn(),
      },
      response: {
        use: vi.fn(),
      },
    },
  }
  return { default: mockAxios }
})

const mockedAxios = axios as unknown as {
  create: ReturnType<typeof vi.fn>
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

describe('Organizations API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('organizationsApi', () => {
    describe('list()', () => {
      it('should fetch organizations with memberships', async () => {
        // Backend returns flat structure
        const backendOrgs = [
          {
            id: 'org-1',
            name: 'Test Org',
            slug: 'test-org',
            type: 'standard',
            edition: 'professional',
            role: 'admin',
            isPrimary: true,
            isActive: true,
            createdAt: '2024-01-01T00:00:00Z',
          },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: { organizations: backendOrgs } })

        const result = await organizationsApi.list()

        // API transforms to nested OrganizationWithMembership
        expect(result).toHaveLength(1)
        const firstOrg = result[0]!
        expect(firstOrg.organization.id).toBe('org-1')
        expect(firstOrg.organization.name).toBe('Test Org')
        expect(firstOrg.membership.role).toBe('admin')
        expect(firstOrg.membership.isPrimary).toBe(true)
        expect(mockedAxios.get).toHaveBeenCalledWith('/organizations')
      })

      it('should return empty array when no organizations', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { organizations: null } })

        const result = await organizationsApi.list()

        expect(result).toEqual([])
      })
    })

    describe('get()', () => {
      it('should fetch single organization by id', async () => {
        const mockOrg = { id: 'org-1', name: 'Test Org', slug: 'test-org' }
        mockedAxios.get.mockResolvedValueOnce({ data: mockOrg })

        const result = await organizationsApi.get('org-1')

        expect(result).toEqual(mockOrg)
        expect(mockedAxios.get).toHaveBeenCalledWith('/organizations/org-1')
      })
    })

    describe('getFeatures()', () => {
      it('should fetch organization features', async () => {
        const mockFeatures = [
          { id: 'teams', name: 'Team Management', enabled: true },
          { id: 'sso', name: 'Single Sign-On', enabled: false },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: mockFeatures })

        const result = await organizationsApi.getFeatures('org-1')

        expect(result).toEqual(mockFeatures)
        expect(mockedAxios.get).toHaveBeenCalledWith('/organizations/org-1/features')
      })
    })

    describe('getTeams()', () => {
      it('should fetch organization teams', async () => {
        const mockTeams = [
          { team: { id: 'team-1', name: 'Security Team' }, memberCount: 5 },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: { teams: mockTeams } })

        const result = await organizationsApi.getTeams('org-1')

        expect(result).toEqual(mockTeams)
        expect(mockedAxios.get).toHaveBeenCalledWith('/organizations/org-1/teams')
      })

      it('should return empty array when no teams', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { teams: null } })

        const result = await organizationsApi.getTeams('org-1')

        expect(result).toEqual([])
      })
    })

    describe('getMembers()', () => {
      it('should fetch organization members', async () => {
        const mockMembers = {
          members: [
            { user: { id: 'u-1', name: 'John', email: 'john@example.com' }, membership: { role: 'admin' } },
          ],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockMembers })

        const result = await organizationsApi.getMembers('org-1')

        expect(result).toEqual(mockMembers)
        expect(mockedAxios.get).toHaveBeenCalledWith('/organizations/org-1/members')
      })
    })
  })

  describe('teamsApi', () => {
    describe('get()', () => {
      it('should fetch single team', async () => {
        const mockTeam = { team: { id: 'team-1', name: 'Security' }, memberCount: 5 }
        mockedAxios.get.mockResolvedValueOnce({ data: mockTeam })

        const result = await teamsApi.get('team-1')

        expect(result).toEqual(mockTeam)
        expect(mockedAxios.get).toHaveBeenCalledWith('/teams/team-1')
      })
    })

    describe('create()', () => {
      it('should create a new team', async () => {
        const newTeam = { id: 'team-new', name: 'New Team', organizationId: 'org-1' }
        mockedAxios.post.mockResolvedValueOnce({ data: newTeam })

        const result = await teamsApi.create('org-1', { name: 'New Team', description: 'A test team' })

        expect(result).toEqual(newTeam)
        expect(mockedAxios.post).toHaveBeenCalledWith('/organizations/org-1/teams', {
          name: 'New Team',
          description: 'A test team',
        })
      })
    })

    describe('update()', () => {
      it('should update team', async () => {
        const updatedTeam = { id: 'team-1', name: 'Updated Team' }
        mockedAxios.put.mockResolvedValueOnce({ data: updatedTeam })

        const result = await teamsApi.update('team-1', { name: 'Updated Team' })

        expect(result).toEqual(updatedTeam)
        expect(mockedAxios.put).toHaveBeenCalledWith('/teams/team-1', { name: 'Updated Team' })
      })
    })

    describe('delete()', () => {
      it('should delete team', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(teamsApi.delete('team-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/teams/team-1')
      })
    })

    describe('getMembers()', () => {
      it('should fetch team members', async () => {
        const mockMembers = [
          { user: { id: 'u-1', name: 'John', email: 'john@example.com' }, membership: { role: 'lead' } },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: { members: mockMembers } })

        const result = await teamsApi.getMembers('team-1')

        expect(result).toEqual(mockMembers)
        expect(mockedAxios.get).toHaveBeenCalledWith('/teams/team-1/members')
      })

      it('should return empty array when no members', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { members: null } })

        const result = await teamsApi.getMembers('team-1')

        expect(result).toEqual([])
      })
    })

    describe('addMember()', () => {
      it('should add member to team', async () => {
        const membership = { id: 'tm-1', teamId: 'team-1', userId: 'u-1', role: 'member' }
        mockedAxios.post.mockResolvedValueOnce({ data: membership })

        const result = await teamsApi.addMember('team-1', 'u-1', 'member')

        expect(result).toEqual(membership)
        expect(mockedAxios.post).toHaveBeenCalledWith('/teams/team-1/members', {
          userId: 'u-1',
          role: 'member',
        })
      })

      it('should add member as lead', async () => {
        const membership = { id: 'tm-1', teamId: 'team-1', userId: 'u-1', role: 'lead' }
        mockedAxios.post.mockResolvedValueOnce({ data: membership })

        const result = await teamsApi.addMember('team-1', 'u-1', 'lead')

        expect(result).toEqual(membership)
        expect(mockedAxios.post).toHaveBeenCalledWith('/teams/team-1/members', {
          userId: 'u-1',
          role: 'lead',
        })
      })
    })

    describe('removeMember()', () => {
      it('should remove member from team', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(teamsApi.removeMember('team-1', 'u-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/teams/team-1/members/u-1')
      })
    })
  })
})
