/**
 * Labs API
 * Lab templates and instructions
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import type { PaginationMeta, PaginationParams } from '../shared/pagination'

// ============================================================================
// Types
// ============================================================================
export interface Lab {
  id: string
  name: string
  slug?: string
  description: string
  difficulty: string
  durationMinutes: number
  platform: string
  category?: string
  tags?: string[]
  version?: string
  maxPoints?: number
  passThreshold?: number
  isActive?: boolean
  // Omitting this is how the edit wizard came to hardcode 'private' and demote
  // every lab it saved.
  visibility?: string
  createdBy?: string
}

// Lab with full spec for editing
export interface LabWithSpec extends Lab {
  spec: string | Record<string, unknown> // API may return object or string
  createdBy?: string
  organizationId?: string
}

// Response wrappers
export interface LabsListResponse {
  labs: Lab[]
  count: number
  /** @deprecated Use pagination.total instead */
  total?: number
  pagination?: PaginationMeta
}

// Request type for creating a lab template
export interface CreateLabRequest {
  name: string
  description: string
  version: string
  platform: string
  durationMinutes: number
  difficulty: string
  category?: string
  tags: string[]
  maxPoints: number
  passThreshold: number
  spec: string // JSON-serialized LabTemplate spec
  isActive: boolean
  visibility: string
  organizationId?: string
}

// Response from lab creation
export interface CreateLabResponse {
  id: string
  name: string
  slug: string
}

// Lab Instructions types
export interface InstructionResource {
  title: string
  url: string
}

export interface InstructionStep {
  id: string
  title: string
  objective_id?: string
  content: string
}

export interface LabInstructions {
  overview?: string
  learning_objectives?: string[]
  prerequisites?: string[]
  steps?: InstructionStep[]
  summary?: string
  tips?: string[]
  resources?: InstructionResource[]
}

// ============================================================================
// Mock Data - dynamically imported to enable tree-shaking in production
// ============================================================================

/** @internal Cached mock labs for cross-domain lookups */
let _mockLabs: Lab[] | null = null

/**
 * Lazily loads mock labs data. Only fetches the module once.
 * For cross-domain mock lookups (sessions, pathways, reservations).
 */
export async function getMockLabs(): Promise<Lab[]> {
  if (!_mockLabs) {
    const { mockLabs: data } = await import('./labs.mock')
    _mockLabs = data
  }
  return _mockLabs
}

/**
 * @deprecated Use getMockLabs() for lazy loading. Kept for backward compatibility
 * with barrel re-exports; resolves to an empty array in production builds.
 */
export const mockLabs: Lab[] = []

// Eagerly populate mockLabs for backward compat if mock mode is active
if (USE_MOCK_DATA) {
  import('./labs.mock').then(({ mockLabs: data }) => {
    mockLabs.push(...data)
    _mockLabs = mockLabs
  })
}

// ============================================================================
// API
// ============================================================================
export const labsApi = {
  /**
   * List labs with optional pagination
   * @param params - Pagination parameters (limit, offset)
   * @returns Labs list with pagination metadata
   */
  list: async (params?: PaginationParams): Promise<LabsListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const labs = await getMockLabs()
      const offset = params?.offset ?? 0
      const limit = params?.limit ?? labs.length
      const paginatedLabs = labs.slice(offset, offset + limit)
      return {
        labs: paginatedLabs,
        count: paginatedLabs.length,
        total: labs.length,
        pagination: {
          total: labs.length,
          limit,
          offset,
          hasMore: offset + limit < labs.length,
          totalPages: Math.ceil(labs.length / limit),
          page: Math.floor(offset / limit) + 1,
        },
      }
    }
    const response = await api.get<LabsListResponse>('/labs', { params })
    return response.data
  },

  get: async (id: string): Promise<Lab> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const labs = await getMockLabs()
      const lab = labs.find(l => l.id === id)
      if (!lab) throw new Error(`Lab not found: ${id}`)
      return lab
    }
    const response = await api.get<Lab>(`/labs/${id}`)
    return response.data
  },

  create: async (request: CreateLabRequest): Promise<CreateLabResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(1000)
      return {
        id: `lab-${Date.now()}`,
        name: request.name,
        slug: request.name.toLowerCase().replace(/\s+/g, '-'),
      }
    }
    const response = await api.post<CreateLabResponse>('/labs', request)
    return response.data
  },

  getWithSpec: async (id: string): Promise<LabWithSpec> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const labs = await getMockLabs()
      const lab = labs.find(l => l.id === id)
      if (!lab) throw new Error(`Lab not found: ${id}`)
      return { ...lab, spec: '{}' }
    }
    const response = await api.get<LabWithSpec>(`/labs/${id}`, { params: { include_spec: true } })
    return response.data
  },

  update: async (id: string, request: CreateLabRequest): Promise<CreateLabResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      return {
        id,
        name: request.name,
        slug: request.name.toLowerCase().replace(/\s+/g, '-'),
      }
    }
    const response = await api.put<CreateLabResponse>(`/labs/${id}`, request)
    return response.data
  },
}

// ============================================================================
// Version History Types
// ============================================================================
export interface LabVersionSummary {
  id: string
  versionNumber: number
  name: string
  version: string
  changeSummary: string
  createdAt: string
}

export interface LabVersion {
  id: string
  labTemplateId: string
  versionNumber: number
  name: string
  slug: string
  description: string
  version: string
  platform: string
  durationMinutes: number
  difficulty: string
  maxPoints: number
  passThreshold: number
  spec: Record<string, unknown>
  checkpoints: unknown
  instructions: unknown
  isActive: boolean
  visibility: string
  changeSummary: string
  createdAt: string
}

export interface VersionListResponse {
  versions: LabVersionSummary[]
  total: number
}

// ============================================================================
// Version / Export / Import API
// ============================================================================
export const labVersionsApi = {
  list: async (labId: string, limit = 50, offset = 0): Promise<VersionListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return { versions: [], total: 0 }
    }
    const response = await api.get<VersionListResponse>(`/labs/${labId}/versions`, {
      params: { limit, offset },
    })
    return response.data
  },

  get: async (labId: string, versionNumber: number): Promise<LabVersion> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      throw new Error('Not available in mock mode')
    }
    const response = await api.get<LabVersion>(`/labs/${labId}/versions/${versionNumber}`)
    return response.data
  },

  restore: async (
    labId: string,
    versionNumber: number,
    changeSummary?: string,
  ): Promise<{ message: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return { message: 'Restored' }
    }
    const response = await api.post<{ message: string }>(
      `/labs/${labId}/versions/${versionNumber}/restore`,
      { changeSummary },
    )
    return response.data
  },

  exportYaml: async (labId: string): Promise<string> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return '# mock yaml'
    }
    const response = await api.get(`/labs/${labId}/export`, { responseType: 'text' })
    return response.data as string
  },

  importYaml: async (yamlContent: string, visibility?: string): Promise<CreateLabResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return { id: `lab-${Date.now()}`, name: 'Imported Lab', slug: 'imported-lab' }
    }
    const response = await api.post<CreateLabResponse>('/labs/import', {
      yaml: yamlContent,
      visibility,
    })
    return response.data
  },
}

// Lab Templates API (for instructions)
export const labTemplatesApi = {
  getInstructions: async (labTemplateId: string): Promise<LabInstructions> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const { mockInstructions } = await import('./labs.mock')
      // Try to find instructions by ID or name
      const instructions = mockInstructions[labTemplateId] || mockInstructions['simple-linux-intro'] // fallback for demo
      if (!instructions) {
        throw new Error(`No instructions found for lab: ${labTemplateId}`)
      }
      return instructions
    }
    const response = await api.get<LabInstructions>(`/labs/${labTemplateId}/instructions`)
    return response.data
  },
}
