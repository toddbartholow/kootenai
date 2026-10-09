/**
 * Classroom Simulation API
 * AI student classroom orchestration (Enterprise feature: ai_classroom)
 */
import { api } from '../config'

// ============================================================================
// Types
// ============================================================================

export type PersonalityType = 'high_performer' | 'struggling' | 'industry_professional'
export type SimulationStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed'
export type ActivityType = 'lab_start' | 'lab_complete' | 'assignment_submit' | 'discussion_post' | 'quiz_attempt'

export interface Traits {
  motivation: number
  conscientiousness: number
  confidence: number
  anxiety: number
}

export interface TechSkills {
  linux: number
  networking: number
  security: number
  scripting: number
  cloud_ops: number
}

export interface BehavioralConfig {
  llm_temperature: number
  quiz_score_min: number
  quiz_score_max: number
  assignment_quality: string
  discussion_style: string
  submit_timing: string
  error_rate: number
}

export interface AIStudent {
  id: string
  simulationId: string
  userId?: string
  canvasUserId?: string
  name: string
  personality: PersonalityType
  traits: Traits
  techSkills: TechSkills
  behavioralConfig: BehavioralConfig
  state: Record<string, unknown>
  createdAt: string
}

export interface SimulationConfig {
  studentCount: number
  personalityMix: Partial<Record<PersonalityType, number>>
  enableCanvas: boolean
  enableVmLabs: boolean
  speedMultiplier: number
}

export interface ClassroomSimulation {
  id: string
  name: string
  status: SimulationStatus
  canvasCourseId?: string
  pathwayId?: string
  config: SimulationConfig
  startedAt?: string
  completedAt?: string
  createdAt: string
  updatedAt: string
  students?: AIStudent[]
}

export interface StudentActivity {
  id: number
  simulationId: string
  studentId: string
  activityType: ActivityType
  targetId?: string
  targetName?: string
  status: string
  content?: string
  score?: number
  maxScore?: number
  metadata: Record<string, unknown>
  createdAt: string
}

export interface CreateSimulationRequest {
  name: string
  pathwayId?: string | undefined
  canvasCourseId?: string | undefined
  config: SimulationConfig
}

export type FeedbackType = 'lab_quality' | 'pathway' | 'infrastructure' | 'content_suggestion'

export interface ClassroomFeedback {
  id: string
  simulationId: string
  studentId: string
  labTemplateId?: string
  pathwayId?: string
  sessionId?: string
  feedbackType: FeedbackType
  summary: string
  rating?: number
  details: Record<string, unknown>
  executionDurationMs?: number
  checkpointsPassed?: number
  checkpointsTotal?: number
  score?: number
  maxScore?: number
  executionLog?: string
  createdAt: string
}

export interface TypeSummary {
  count: number
  avgRating: number
}

export interface LabFeedbackSummary {
  labTemplateId: string
  count: number
  avgRating: number
}

export interface FeedbackSummary {
  totalFeedback: number
  avgRating: number
  byType: Record<string, TypeSummary>
  byLab: LabFeedbackSummary[]
  infrastructureIssues: number
}

// ============================================================================
// API
// ============================================================================

export const classroomApi = {
  create: async (req: CreateSimulationRequest): Promise<ClassroomSimulation> => {
    const response = await api.post<ClassroomSimulation>('/simulation/classrooms', req)
    return response.data
  },

  list: async (): Promise<ClassroomSimulation[]> => {
    const response = await api.get<{ simulations: ClassroomSimulation[]; count: number }>(
      '/simulation/classrooms',
    )
    return response.data.simulations || []
  },

  get: async (id: string): Promise<ClassroomSimulation> => {
    const response = await api.get<ClassroomSimulation>(`/simulation/classrooms/${id}`)
    return response.data
  },

  start: async (id: string): Promise<void> => {
    await api.post(`/simulation/classrooms/${id}/start`)
  },

  pause: async (id: string): Promise<void> => {
    await api.post(`/simulation/classrooms/${id}/pause`)
  },

  stop: async (id: string): Promise<void> => {
    await api.post(`/simulation/classrooms/${id}/stop`)
  },

  delete: async (id: string): Promise<void> => {
    await api.delete(`/simulation/classrooms/${id}`)
  },

  listActivities: async (
    id: string,
    limit = 50,
    offset = 0,
  ): Promise<{ activities: StudentActivity[]; total: number }> => {
    const response = await api.get<{ activities: StudentActivity[]; total: number }>(
      `/simulation/classrooms/${id}/activities`,
      { params: { limit, offset } },
    )
    return response.data
  },

  getStudent: async (simulationId: string, studentId: string): Promise<AIStudent> => {
    const response = await api.get<AIStudent>(
      `/simulation/classrooms/${simulationId}/students/${studentId}`,
    )
    return response.data
  },

  listFeedback: async (
    id: string,
    opts: { type?: string | undefined; labId?: string | undefined; limit?: number | undefined; offset?: number | undefined } = {},
  ): Promise<{ feedback: ClassroomFeedback[]; total: number }> => {
    const response = await api.get<{ feedback: ClassroomFeedback[]; total: number }>(
      `/simulation/classrooms/${id}/feedback`,
      { params: opts },
    )
    return response.data
  },

  getFeedbackSummary: async (id: string): Promise<FeedbackSummary> => {
    const response = await api.get<FeedbackSummary>(
      `/simulation/classrooms/${id}/feedback/summary`,
    )
    return response.data
  },
}
