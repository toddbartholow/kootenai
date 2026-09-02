import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  organizationsApi,
  teamsApi,
  setOrganizationContext,
  USE_MOCK_DATA,
  type Organization,
  type OrganizationWithMembership,
  type OrganizationMembership,
  type TeamWithMembership,
  type OrgRole,
} from '@/api'

const ORG_KEY = 'current_organization_id'

/**
 * Role hierarchy for permission checks
 * Higher number = more privileges
 */
const ROLE_HIERARCHY: Record<OrgRole, number> = {
  owner: 100,
  admin: 80,
  instructor: 50,
  member: 10,
}

export const useOrganizationStore = defineStore('organization', () => {
  // State
  const organizations = ref<OrganizationWithMembership[]>([])
  const currentOrganization = ref<Organization | null>(null)
  const currentMembership = ref<OrganizationMembership | null>(null)
  const features = ref<Map<string, boolean>>(new Map())
  const teams = ref<TeamWithMembership[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Initialize from localStorage
  const storedOrgId = localStorage.getItem(ORG_KEY)

  // Computed properties
  const hasOrganization = computed(() => !!currentOrganization.value)

  const currentRole = computed((): OrgRole | null => {
    return currentMembership.value?.role || null
  })

  const isOwner = computed(() => currentRole.value === 'owner')
  const isAdmin = computed(() => {
    const role = currentRole.value
    return role === 'owner' || role === 'admin'
  })
  const isInstructor = computed(() => {
    const role = currentRole.value
    return role === 'owner' || role === 'admin' || role === 'instructor'
  })

  const edition = computed(() => currentOrganization.value?.edition || 'community')

  /**
   * Check if the current user has at least the specified role
   */
  function hasRole(minRole: OrgRole): boolean {
    const userRole = currentRole.value
    if (!userRole) return false
    return ROLE_HIERARCHY[userRole] >= ROLE_HIERARCHY[minRole]
  }

  /**
   * Check if a feature is enabled for the current organization
   */
  function hasFeature(featureId: string): boolean {
    return features.value.get(featureId) ?? false
  }

  /**
   * Check if any of the specified features is enabled
   */
  function hasAnyFeature(...featureIds: string[]): boolean {
    return featureIds.some(id => hasFeature(id))
  }

  /**
   * Check if all specified features are enabled
   */
  function hasAllFeatures(...featureIds: string[]): boolean {
    return featureIds.every(id => hasFeature(id))
  }

  /**
   * Fetch all organizations the current user belongs to
   */
  async function fetchOrganizations(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const rawOrgs = await organizationsApi.list()
      // Filter out any malformed entries that don't have required fields
      organizations.value = rawOrgs.filter(o => o.organization && o.membership)

      // Auto-select organization if needed
      if (!currentOrganization.value && organizations.value.length > 0) {
        // Try stored org first
        if (storedOrgId) {
          const stored = organizations.value.find(o => o.organization.id === storedOrgId)
          if (stored) {
            await selectOrganization(stored.organization.id)
            return
          }
        }

        // Otherwise, select primary or first org
        const primary = organizations.value.find(o => o.membership.isPrimary)
        const first = organizations.value[0]
        const toSelect = primary || first
        if (toSelect) {
          await selectOrganization(toSelect.organization.id)
        }
      }
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch organizations'
    } finally {
      loading.value = false
    }
  }

  /**
   * Select an organization as the current context
   */
  async function selectOrganization(orgId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      // Find in cached list or fetch
      let orgWithMembership = organizations.value.find(o => o.organization.id === orgId)

      if (!orgWithMembership) {
        // Fetch fresh if not in cache
        await organizationsApi.get(orgId)
        // We don't have membership info, so refetch list
        await fetchOrganizations()
        orgWithMembership = organizations.value.find(o => o.organization.id === orgId)
        if (!orgWithMembership) {
          throw new Error('Organization not found or access denied')
        }
      }

      currentOrganization.value = orgWithMembership.organization
      currentMembership.value = orgWithMembership.membership

      // Save to localStorage
      localStorage.setItem(ORG_KEY, orgId)

      // Set API header for subsequent requests
      setOrganizationContext(orgId)

      // Load features and teams for the org
      await Promise.all([
        fetchFeatures(),
        fetchTeams(),
      ])
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to select organization'
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * Fetch features for the current organization
   */
  async function fetchFeatures(): Promise<void> {
    if (!currentOrganization.value) return

    try {
      const featureList = await organizationsApi.getFeatures(currentOrganization.value.id)
      features.value = new Map(
        featureList.map(f => [f.id, f.enabled ?? f.isGlobal])
      )
    } catch (e: unknown) {
      console.error('Failed to fetch features:', e)
    }
  }

  /**
   * Fetch teams for the current organization
   */
  async function fetchTeams(): Promise<void> {
    if (!currentOrganization.value) return

    // Only fetch if teams feature is enabled or we're in mock mode
    if (!USE_MOCK_DATA && !hasFeature('teams')) {
      teams.value = []
      return
    }

    try {
      teams.value = await organizationsApi.getTeams(currentOrganization.value.id)
    } catch (e: unknown) {
      console.error('Failed to fetch teams:', e)
      teams.value = []
    }
  }

  /**
   * Create a new team
   */
  async function createTeam(data: { name: string; description?: string }): Promise<TeamWithMembership> {
    if (!currentOrganization.value) {
      throw new Error('No organization selected')
    }

    loading.value = true
    error.value = null
    try {
      const team = await teamsApi.create(currentOrganization.value.id, data)
      const newTeamWithMembership: TeamWithMembership = {
        team,
        memberCount: 1,
      }
      teams.value.push(newTeamWithMembership)
      return newTeamWithMembership
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to create team'
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * Update a team
   */
  async function updateTeam(teamId: string, data: { name?: string; description?: string }): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const updatedTeam = await teamsApi.update(teamId, data)
      const index = teams.value.findIndex(t => t.team.id === teamId)
      if (index >= 0) {
        teams.value[index] = {
          ...teams.value[index]!,
          team: updatedTeam,
        }
      }
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to update team'
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * Delete a team
   */
  async function deleteTeam(teamId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await teamsApi.delete(teamId)
      teams.value = teams.value.filter(t => t.team.id !== teamId)
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Failed to delete team'
      throw e
    } finally {
      loading.value = false
    }
  }

  /**
   * Clear organization context (e.g., on logout)
   */
  function clearOrganization(): void {
    currentOrganization.value = null
    currentMembership.value = null
    features.value = new Map()
    teams.value = []
    localStorage.removeItem(ORG_KEY)
    setOrganizationContext(null)
  }

  /**
   * Reset the entire store
   */
  function reset(): void {
    organizations.value = []
    clearOrganization()
    loading.value = false
    error.value = null
  }

  return {
    // State
    organizations,
    currentOrganization,
    currentMembership,
    features,
    teams,
    loading,
    error,
    // Computed
    hasOrganization,
    currentRole,
    isOwner,
    isAdmin,
    isInstructor,
    edition,
    // Actions
    hasRole,
    hasFeature,
    hasAnyFeature,
    hasAllFeatures,
    fetchOrganizations,
    selectOrganization,
    fetchFeatures,
    fetchTeams,
    createTeam,
    updateTeam,
    deleteTeam,
    clearOrganization,
    reset,
  }
})
