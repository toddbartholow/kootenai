/**
 * Recommendations API
 * Personalized lab and pathway recommendations for users
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export type RecommendationType =
  | 'next_in_pathway'
  | 'continue_progress'
  | 'new_pathway'
  | 'similar_difficulty'
  | 'popular'

export interface LabRecommendation {
  labTemplateId: string
  labName: string
  labSlug: string
  labDescription?: string
  difficulty?: string
  durationMinutes?: number
  maxPoints?: number
  type: RecommendationType
  reason: string
  priority: number
  pathwayId?: string
  pathwayName?: string
  moduleId?: string
  moduleName?: string
}

export interface PathwayRecommendation {
  pathwayId: string
  pathwayName: string
  pathwaySlug: string
  description?: string
  difficulty?: string
  estimatedHours?: number
  moduleCount?: number
  labCount?: number
  type: RecommendationType
  reason: string
  priority: number
  coverImageUrl?: string
  icon?: string
}

export interface RecommendationsResponse {
  labs: LabRecommendation[]
  pathways: PathwayRecommendation[]
}

// ============================================================================
// Mock Data
// ============================================================================
const mockLabRecommendations: LabRecommendation[] = [
  {
    labTemplateId: 'lab-linux-basics',
    labName: 'Linux Command Line Basics',
    labSlug: 'linux-basics',
    labDescription: 'Learn essential Linux commands for navigation and file management',
    difficulty: 'beginner',
    durationMinutes: 45,
    maxPoints: 100,
    type: 'continue_progress',
    reason: 'Continue where you left off',
    priority: 0,
  },
  {
    labTemplateId: 'lab-firewall-config',
    labName: 'Firewall Configuration',
    labSlug: 'firewall-config',
    labDescription: 'Configure iptables rules for network security',
    difficulty: 'intermediate',
    durationMinutes: 60,
    maxPoints: 150,
    type: 'next_in_pathway',
    reason: 'Next lab in Network Security Fundamentals',
    priority: 1,
    pathwayId: 'pathway-netsec',
    pathwayName: 'Network Security Fundamentals',
    moduleId: 'mod-firewalls',
    moduleName: 'Firewalls & Access Control',
  },
  {
    labTemplateId: 'lab-log-analysis',
    labName: 'Log Analysis with Grep',
    labSlug: 'log-analysis',
    labDescription: 'Analyze system logs to identify security events',
    difficulty: 'intermediate',
    durationMinutes: 45,
    maxPoints: 120,
    type: 'popular',
    reason: 'Popular with other learners',
    priority: 2,
  },
]

const mockPathwayRecommendations: PathwayRecommendation[] = [
  {
    pathwayId: 'pathway-incident-response',
    pathwayName: 'Incident Response',
    pathwaySlug: 'incident-response',
    description: 'Master the art of detecting, analyzing, and responding to security incidents',
    difficulty: 'intermediate',
    estimatedHours: 12,
    moduleCount: 6,
    labCount: 18,
    type: 'new_pathway',
    reason: 'Featured pathway',
    priority: 3,
    icon: 'pi-shield',
  },
  {
    pathwayId: 'pathway-cloud-security',
    pathwayName: 'Cloud Security Essentials',
    pathwaySlug: 'cloud-security',
    description: 'Secure cloud infrastructure on AWS, Azure, and GCP',
    difficulty: 'intermediate',
    estimatedHours: 16,
    moduleCount: 8,
    labCount: 24,
    type: 'popular',
    reason: 'Popular with other students',
    priority: 4,
    icon: 'pi-cloud',
  },
]

const _mockRecommendationsResponse: RecommendationsResponse = {
  labs: mockLabRecommendations,
  pathways: mockPathwayRecommendations,
}

// ============================================================================
// API
// ============================================================================
export const recommendationsApi = {
  /**
   * Get personalized recommendations for the current user
   */
  get: async (limit?: number): Promise<RecommendationsResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        labs: mockLabRecommendations.slice(0, limit || 5),
        pathways: mockPathwayRecommendations.slice(0, limit || 5),
      }
    }
    const params = limit ? { limit } : {}
    const response = await api.get<RecommendationsResponse>('/recommendations', { params })
    return response.data
  },

  /**
   * Get the next recommended lab in a specific pathway
   */
  getNextLabInPathway: async (pathwayId: string): Promise<LabRecommendation | null> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      // Return first lab recommendation with matching pathway
      const match = mockLabRecommendations.find(l => l.pathwayId === pathwayId)
      return match || null
    }
    try {
      const response = await api.get<LabRecommendation>(`/pathways/${pathwayId}/next-lab`)
      return response.data
    } catch {
      // 404 means no next lab (pathway completed or not enrolled)
      return null
    }
  },
}
