/**
 * Pathways API
 * Learning pathways and module management
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import { getMockLabs } from './labs'

// ============================================================================
// Types
// ============================================================================
export type PathwayStatus = 'draft' | 'published' | 'archived'
export type ModuleProgressStatus = 'locked' | 'unlocked' | 'in_progress' | 'completed'
export type UnlockType = 'sequential' | 'all_previous' | 'manual' | 'always'
export type LabVisibility = 'global' | 'organization' | 'private'

export interface Pathway {
  id: string
  name: string
  slug: string
  description?: string
  shortDescription?: string
  difficulty?: string
  estimatedHours?: number
  displayOrder: number
  status: PathwayStatus
  isFeatured: boolean
  organizationId?: string
  visibility: LabVisibility
  createdBy?: string
  prerequisites?: string[]
  tags?: string[]
  icon?: string
  color?: string
  coverImageUrl?: string
  createdAt: string
  updatedAt: string
  // Computed stats
  moduleCount?: number
  labCount?: number
  totalPoints?: number
  totalDurationMinutes?: number
  enrollmentCount?: number
  completionCount?: number
  // Nested data
  modules?: PathwayModule[]
}

export interface PathwayModule {
  id: string
  pathwayId: string
  name: string
  slug: string
  description?: string
  displayOrder: number
  unlockType: UnlockType
  isActive: boolean
  icon?: string
  estimatedMinutes?: number
  createdAt: string
  // Computed
  labCount?: number
  totalPoints?: number
  // Nested
  labs?: ModuleLab[]
}

export interface ModuleLab {
  id: string
  moduleId: string
  labTemplateId: string
  displayOrder: number
  isRequired: boolean
  passThresholdOverride?: number
  // Joined from lab_templates
  labName?: string
  labDescription?: string
  labDifficulty?: string
  labDurationMinutes?: number
  labMaxPoints?: number
  labPlatform?: string
}

export interface PathwayStats {
  pathwayId: string
  moduleCount: number
  labCount: number
  totalPoints: number
  totalDurationMinutes: number
  enrollmentCount: number
  completionCount: number
  completionRate?: number
  averageScore?: number
}

// Request types
export interface CreatePathwayRequest {
  name: string
  slug?: string
  description?: string
  shortDescription?: string
  difficulty?: 'beginner' | 'intermediate' | 'advanced' | 'mixed'
  visibility?: 'global' | 'organization' | 'private'
  estimatedHours?: number
  tags?: string[]
  icon?: string
  color?: string
  coverImageUrl?: string
}

export interface UpdatePathwayRequest {
  name?: string
  description?: string
  shortDescription?: string
  difficulty?: 'beginner' | 'intermediate' | 'advanced' | 'mixed'
  visibility?: 'global' | 'organization' | 'private'
  estimatedHours?: number
  tags?: string[]
  icon?: string
  color?: string
  coverImageUrl?: string
  isFeatured?: boolean
}

export interface CreateModuleRequest {
  name: string
  slug?: string
  description?: string
  unlockType?: 'sequential' | 'all_previous' | 'manual' | 'always'
  estimatedMinutes?: number
  icon?: string
}

export interface UpdateModuleRequest {
  name?: string
  description?: string
  unlockType?: 'sequential' | 'all_previous' | 'manual' | 'always'
  estimatedMinutes?: number
  icon?: string
  isActive?: boolean
}

export interface AddLabToModuleRequest {
  labTemplateId: string
  displayOrder?: number
  isRequired?: boolean
  passThresholdOverride?: number
}

// Enrollment types (defined here to avoid circular dependency)
export type EnrollmentStatus = 'enrolled' | 'in_progress' | 'completed' | 'abandoned'

export interface PathwayEnrollment {
  id: string
  userId: string
  pathwayId: string
  status: EnrollmentStatus
  completedModules: number
  totalModules: number
  earnedPoints: number
  maxPoints: number
  percentage: number
  enrolledAt: string
  startedAt?: string
  completedAt?: string
  lastActivityAt?: string
  organizationId?: string
  certificateIssued: boolean
  certificateUrl?: string
  // Nested
  pathway?: Pathway
  moduleProgress?: ModuleProgress[]
}

export interface ModuleProgress {
  id: string
  enrollmentId: string
  moduleId: string
  status: ModuleProgressStatus
  completedLabs: number
  totalLabs: number
  earnedPoints: number
  maxPoints: number
  unlockedAt?: string
  startedAt?: string
  completedAt?: string
  // Joined from pathway_modules
  moduleName?: string
  moduleDescription?: string
  // Nested
  module?: PathwayModule
  labProgress?: LabProgress[]
  labs?: LabProgress[] // Alternative field name
}

export interface LabProgress {
  id: string
  enrollmentId: string
  moduleId: string
  labTemplateId: string
  bestSessionId?: string
  attemptCount: number
  bestScore: number
  maxPoints: number
  passed: boolean
  firstAttemptAt?: string
  completedAt?: string
  // Joined from lab_templates
  labName?: string
  labDescription?: string
  labDifficulty?: string
  durationMinutes?: number
}

// Unlock requirements for a module
export interface UnlockRequirement {
  moduleId: string
  moduleName: string
  status: string
  required: boolean
}

export interface UnlockRequirements {
  moduleId: string
  moduleName: string
  unlockType: UnlockType
  currentStatus: ModuleProgressStatus
  requirements: UnlockRequirement[]
  message: string
}

// ============================================================================
// Mock Data (exported for cross-domain lookups)
// ============================================================================
export const mockPathways: Pathway[] = [
  {
    id: 'pathway-linux',
    name: 'Linux Fundamentals',
    slug: 'linux-fundamentals',
    description: 'Master the essentials of Linux system administration, from navigation to networking.',
    shortDescription: 'Complete Linux learning track from basics to advanced',
    difficulty: 'beginner',
    estimatedHours: 12,
    displayOrder: 0,
    status: 'published',
    isFeatured: true,
    visibility: 'global',
    tags: ['linux', 'beginner', 'fundamentals'],
    icon: 'pi-server',
    color: '#f97316',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    moduleCount: 8,
    labCount: 8,
    totalPoints: 1030,
    enrollmentCount: 150,
    completionCount: 45,
  },
  {
    id: 'pathway-security',
    name: 'Security Essentials',
    slug: 'security-essentials',
    description: 'Learn fundamental cybersecurity concepts and practices.',
    shortDescription: 'Introduction to cybersecurity',
    difficulty: 'intermediate',
    estimatedHours: 8,
    displayOrder: 1,
    status: 'published',
    isFeatured: false,
    visibility: 'global',
    tags: ['security', 'intermediate'],
    icon: 'pi-shield',
    color: '#22c55e',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    moduleCount: 4,
    labCount: 6,
    totalPoints: 600,
    enrollmentCount: 80,
    completionCount: 20,
  },
]

// ============================================================================
// API
// ============================================================================
export const pathwaysApi = {
  list: async (options?: {
    status?: PathwayStatus
    difficulty?: string
    featured?: boolean
    search?: string
    includeStats?: boolean
  }): Promise<Pathway[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      let pathways = [...mockPathways]
      if (options?.status) {
        pathways = pathways.filter(p => p.status === options.status)
      }
      if (options?.difficulty) {
        pathways = pathways.filter(p => p.difficulty === options.difficulty)
      }
      if (options?.featured) {
        pathways = pathways.filter(p => p.isFeatured)
      }
      if (options?.search) {
        const term = options.search.toLowerCase()
        pathways = pathways.filter(p =>
          p.name.toLowerCase().includes(term) ||
          p.description?.toLowerCase().includes(term)
        )
      }
      return pathways
    }
    const params: Record<string, string> = {}
    if (options?.status) params['status'] = options.status
    if (options?.difficulty) params['difficulty'] = options.difficulty
    if (options?.featured) params['featured'] = 'true'
    if (options?.search) params['search'] = options.search
    if (options?.includeStats) params['include_stats'] = 'true'
    const response = await api.get<{ pathways: Pathway[]; count: number }>('/pathways', { params })
    return response.data.pathways || []
  },

  get: async (idOrSlug: string): Promise<Pathway> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const pathway = mockPathways.find(p => p.id === idOrSlug || p.slug === idOrSlug)
      if (!pathway) throw new Error(`Pathway not found: ${idOrSlug}`)
      // Return with mock modules
      return {
        ...pathway,
        modules: [
          {
            id: 'mod-1',
            pathwayId: pathway.id,
            name: 'Linux Foundations',
            slug: 'linux-foundations',
            description: 'Navigation and file editing basics',
            displayOrder: 0,
            unlockType: 'always',
            isActive: true,
            icon: 'pi-compass',
            estimatedMinutes: 45,
            createdAt: new Date().toISOString(),
            labCount: 1,
            totalPoints: 100,
          },
          {
            id: 'mod-2',
            pathwayId: pathway.id,
            name: 'Shell Essentials',
            slug: 'shell-essentials',
            description: 'Shell environment and scripting basics',
            displayOrder: 1,
            unlockType: 'sequential',
            isActive: true,
            icon: 'pi-code',
            estimatedMinutes: 45,
            createdAt: new Date().toISOString(),
            labCount: 1,
            totalPoints: 100,
          },
        ],
      }
    }
    const response = await api.get<Pathway>(`/pathways/${idOrSlug}`)
    return response.data
  },

  getStats: async (pathwayId: string): Promise<PathwayStats> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const pathway = mockPathways.find(p => p.id === pathwayId)
      return {
        pathwayId,
        moduleCount: pathway?.moduleCount || 0,
        labCount: pathway?.labCount || 0,
        totalPoints: pathway?.totalPoints || 0,
        totalDurationMinutes: (pathway?.estimatedHours || 0) * 60,
        enrollmentCount: pathway?.enrollmentCount || 0,
        completionCount: pathway?.completionCount || 0,
        completionRate: pathway?.enrollmentCount
          ? ((pathway?.completionCount || 0) / pathway.enrollmentCount) * 100
          : 0,
      }
    }
    const response = await api.get<PathwayStats>(`/pathways/${pathwayId}/stats`)
    return response.data
  },

  // Enrollment operations delegate to enrollmentsApi to avoid duplication.
  // Import lazily to avoid circular dependency (enrollments.ts imports from pathways.ts).
  enroll: async (pathwayId: string): Promise<PathwayEnrollment> => {
    const { enrollmentsApi } = await import('./enrollments')
    return enrollmentsApi.enroll(pathwayId)
  },

  unenroll: async (pathwayId: string): Promise<void> => {
    const { enrollmentsApi } = await import('./enrollments')
    return enrollmentsApi.unenroll(pathwayId)
  },

  // Pathway management (instructor/admin)
  create: async (data: CreatePathwayRequest): Promise<Pathway> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const slug = data.slug || data.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
      const newPathway: Pathway = {
        id: `pathway-${Date.now()}`,
        name: data.name,
        slug,
        ...(data.description && { description: data.description }),
        ...(data.shortDescription && { shortDescription: data.shortDescription }),
        difficulty: data.difficulty || 'beginner',
        ...(data.estimatedHours !== undefined && { estimatedHours: data.estimatedHours }),
        displayOrder: mockPathways.length,
        status: 'draft',
        isFeatured: false,
        visibility: data.visibility || 'global',
        ...(data.tags && { tags: data.tags }),
        ...(data.icon && { icon: data.icon }),
        ...(data.color && { color: data.color }),
        ...(data.coverImageUrl && { coverImageUrl: data.coverImageUrl }),
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        moduleCount: 0,
        labCount: 0,
        totalPoints: 0,
        modules: [],
      }
      mockPathways.push(newPathway)
      return newPathway
    }
    const response = await api.post<Pathway>('/pathways', data)
    return response.data
  },

  update: async (pathwayId: string, data: UpdatePathwayRequest): Promise<Pathway> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const pathway = mockPathways.find(p => p.id === pathwayId)
      if (!pathway) throw new Error(`Pathway not found: ${pathwayId}`)
      Object.assign(pathway, data, { updatedAt: new Date().toISOString() })
      return pathway
    }
    const response = await api.put<Pathway>(`/pathways/${pathwayId}`, data)
    return response.data
  },

  delete: async (pathwayId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const index = mockPathways.findIndex(p => p.id === pathwayId)
      if (index !== -1) mockPathways.splice(index, 1)
      return
    }
    await api.delete(`/pathways/${pathwayId}`)
  },

  publish: async (pathwayId: string): Promise<Pathway> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const pathway = mockPathways.find(p => p.id === pathwayId)
      if (!pathway) throw new Error(`Pathway not found: ${pathwayId}`)
      pathway.status = 'published'
      pathway.updatedAt = new Date().toISOString()
      return pathway
    }
    const response = await api.post<Pathway>(`/pathways/${pathwayId}/publish`)
    return response.data
  },

  archive: async (pathwayId: string): Promise<Pathway> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const pathway = mockPathways.find(p => p.id === pathwayId)
      if (!pathway) throw new Error(`Pathway not found: ${pathwayId}`)
      pathway.status = 'archived'
      pathway.updatedAt = new Date().toISOString()
      return pathway
    }
    const response = await api.post<Pathway>(`/pathways/${pathwayId}/archive`)
    return response.data
  },

  // Module management
  listModules: async (pathwayId: string): Promise<PathwayModule[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      // Return mock modules for the pathway
      return [
        {
          id: 'mod-1',
          pathwayId,
          name: 'Getting Started',
          slug: 'getting-started',
          description: 'Introduction and basics',
          displayOrder: 0,
          unlockType: 'always',
          isActive: true,
          estimatedMinutes: 30,
          createdAt: new Date().toISOString(),
          labCount: 1,
          totalPoints: 100,
        },
      ]
    }
    const response = await api.get<{ modules: PathwayModule[] }>(`/pathways/${pathwayId}/modules`)
    return response.data.modules || []
  },

  createModule: async (pathwayId: string, data: CreateModuleRequest): Promise<PathwayModule> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(400)
      const slug = data.slug || data.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '')
      const newModule: PathwayModule = {
        id: `mod-${Date.now()}`,
        pathwayId,
        name: data.name,
        slug,
        ...(data.description && { description: data.description }),
        displayOrder: 0,
        unlockType: data.unlockType || 'sequential',
        isActive: true,
        ...(data.icon && { icon: data.icon }),
        ...(data.estimatedMinutes !== undefined && { estimatedMinutes: data.estimatedMinutes }),
        createdAt: new Date().toISOString(),
        labCount: 0,
        totalPoints: 0,
        labs: [],
      }
      return newModule
    }
    const response = await api.post<PathwayModule>(`/pathways/${pathwayId}/modules`, data)
    return response.data
  },

  updateModule: async (moduleId: string, data: UpdateModuleRequest): Promise<PathwayModule> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return {
        id: moduleId,
        pathwayId: 'mock-pathway',
        name: data.name || 'Updated Module',
        slug: 'updated-module',
        ...(data.description && { description: data.description }),
        displayOrder: 0,
        unlockType: data.unlockType || 'sequential',
        isActive: data.isActive ?? true,
        ...(data.icon && { icon: data.icon }),
        ...(data.estimatedMinutes !== undefined && { estimatedMinutes: data.estimatedMinutes }),
        createdAt: new Date().toISOString(),
      }
    }
    const response = await api.put<PathwayModule>(`/modules/${moduleId}`, data)
    return response.data
  },

  deleteModule: async (moduleId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return
    }
    await api.delete(`/modules/${moduleId}`)
  },

  // Lab management within modules
  addLabToModule: async (moduleId: string, data: AddLabToModuleRequest): Promise<ModuleLab> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const labs = await getMockLabs()
      const lab = labs.find(l => l.id === data.labTemplateId)
      const newModuleLab: ModuleLab = {
        id: `mlab-${Date.now()}`,
        moduleId,
        labTemplateId: data.labTemplateId,
        displayOrder: data.displayOrder || 0,
        isRequired: data.isRequired ?? true,
        ...(data.passThresholdOverride !== undefined && { passThresholdOverride: data.passThresholdOverride }),
        ...(lab?.name && { labName: lab.name }),
        ...(lab?.description && { labDescription: lab.description }),
        ...(lab?.difficulty && { labDifficulty: lab.difficulty }),
        ...(lab?.durationMinutes !== undefined && { labDurationMinutes: lab.durationMinutes }),
        ...(lab?.maxPoints !== undefined && { labMaxPoints: lab.maxPoints }),
        ...(lab?.platform && { labPlatform: lab.platform }),
      }
      return newModuleLab
    }
    const response = await api.post<ModuleLab>(`/modules/${moduleId}/labs`, data)
    return response.data
  },

  removeLabFromModule: async (moduleId: string, labTemplateId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(200)
      return
    }
    await api.delete(`/modules/${moduleId}/labs/${labTemplateId}`)
  },
}
