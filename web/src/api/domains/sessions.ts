/**
 * Sessions API
 * Lab session management
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import type { UserAchievement } from './achievements'
import type { ApiSession } from '../generated'

// ============================================================================
// Types
// ============================================================================
//
// Like Pod, Session is being migrated onto the generated OpenAPI shape.
// The hand-rolled type below is the narrow UI-facing slice; the drift
// assertions at the bottom of the file verify it stays field-compatible
// with ApiSession as the backend model evolves. Phase 7 backlog:
// widen to use ApiSession directly and expose labTemplateId,
// canvasCourseId, dueAt, organizationId, etc. that the backend already
// serializes.
export interface Session {
  id: string
  podId: string
  userId: string
  labTemplateId: string
  labTemplateName?: string
  status: string
  earnedPoints: number
  maxPoints: number
  percentage: number
  passed: boolean
  startedAt: string
  endedAt?: string
}

export interface SessionsListResponse {
  sessions: Session[]
  count: number
}

export interface CreateSessionRequest {
  podId: string
  userId: string
  labTemplate: string
  // Optional pathway context - links session to pathway progress
  enrollmentId?: string
  moduleId?: string
}

export interface CreateSessionResponse {
  sessionId: string
  status: string
  maxPoints: number
}

// Progressive hint response type for checkpoints
export interface CheckpointHintResponse {
  checkpointId: string
  level: number  // Current hint level revealed
  maxLevel: number  // Total hints available
  hint: string  // The hint text for this level
  penalty: number  // Points deducted for this hint
  totalPenalty: number  // Total penalty applied so far
}

// Lab submission response types
export interface SubmissionCheckpoint {
  id: string
  description: string
  points: number
  earnedPoints: number
  passed: boolean
  completedAt?: string
}

/**
 * Module completion info returned when lab submission completes a module
 */
export interface ModuleCompletionInfo {
  moduleId: string
  moduleName: string
  pathwayId: string
  pathwayName: string
  /** Name of the next module that was unlocked, if any */
  unlockedModuleName?: string
  /** True if this submission completed the entire pathway */
  pathwayCompleted: boolean
}

export interface SubmissionResponse {
  sessionId: string
  status: 'graded' | 'already_submitted'
  earnedPoints: number
  maxPoints: number
  percentage: number
  passed: boolean
  passThreshold: number
  submittedAt: string
  checkpoints: SubmissionCheckpoint[]
  achievements?: UserAchievement[]
  message?: string
  /** Module completion info if this lab was part of a pathway module */
  moduleCompletion?: ModuleCompletionInfo
}

// ============================================================================
// Mock Data - dynamically imported to enable tree-shaking in production
// ============================================================================

/**
 * @deprecated Use dynamic import('./sessions.mock') instead. Kept for backward
 * compatibility with barrel re-exports; populated lazily in mock mode.
 */
export const mockSessions: Session[] = []

// Eagerly populate mockSessions for backward compat if mock mode is active
if (USE_MOCK_DATA) {
  import('./sessions.mock').then(({ mockSessions: data }) => {
    mockSessions.push(...data)
  })
}

/** @internal Cached mock sessions for use in API functions */
let _mockSessions: Session[] | null = null

async function getMockSessions(): Promise<Session[]> {
  if (!_mockSessions) {
    const { mockSessions: data } = await import('./sessions.mock')
    _mockSessions = data
  }
  return _mockSessions
}

// ============================================================================
// API
// ============================================================================
export const sessionsApi = {
  list: async (userId?: string, active?: boolean): Promise<Session[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const allSessions = await getMockSessions()
      let sessions = [...allSessions]
      if (userId) {
        sessions = sessions.filter(s => s.userId === userId)
      }
      if (active) {
        sessions = sessions.filter(s => s.status === 'active')
      }
      return sessions
    }
    const params: Record<string, string> = {}
    if (userId) params['userId'] = userId
    if (active) params['active'] = 'true'
    const response = await api.get<SessionsListResponse>('/sessions', { params })
    return response.data.sessions || []
  },

  get: async (id: string): Promise<Session> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const allSessions = await getMockSessions()
      const session = allSessions.find(s => s.id === id)
      if (!session) throw new Error(`Session not found: ${id}`)
      return session
    }
    const response = await api.get<Session>(`/sessions/${id}`)
    return response.data
  },

  create: async (req: CreateSessionRequest): Promise<CreateSessionResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const { getMockLabs } = await import('./labs')
      const labs = await getMockLabs()
      const lab = labs.find(l => l.id === req.labTemplate)
      return {
        sessionId: `session-${Date.now()}`,
        status: 'active',
        maxPoints: lab?.maxPoints || 1000,
      }
    }
    const response = await api.post<CreateSessionResponse>('/sessions', req)
    return response.data
  },

  end: async (id: string): Promise<{ status: string; earnedPoints: number; passed: boolean }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const allSessions = await getMockSessions()
      const session = allSessions.find(s => s.id === id)
      return {
        status: 'completed',
        earnedPoints: session?.earnedPoints || 0,
        passed: session?.passed || false,
      }
    }
    const response = await api.post(`/sessions/${id}/end`)
    return response.data
  },

  getProgress: async (id: string): Promise<{ sessionId: string; earnedPoints: number; maxPoints: number; checkpoints: unknown[] }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const allSessions = await getMockSessions()
      const session = allSessions.find(s => s.id === id)
      return {
        sessionId: id,
        earnedPoints: session?.earnedPoints || 0,
        maxPoints: session?.maxPoints || 1000,
        checkpoints: [],
      }
    }
    const response = await api.get(`/sessions/${id}/progress`)
    return response.data
  },

  submit: async (id: string): Promise<SubmissionResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(800)
      const allSessions = await getMockSessions()
      const session = allSessions.find(s => s.id === id)
      return {
        sessionId: id,
        status: 'graded',
        earnedPoints: session?.earnedPoints || 75,
        maxPoints: session?.maxPoints || 100,
        percentage: session?.percentage || 75,
        passed: session?.passed || true,
        passThreshold: 70,
        submittedAt: new Date().toISOString(),
        checkpoints: [
          { id: 'cp-1', description: 'Create directory', points: 20, earnedPoints: 20, passed: true },
          { id: 'cp-2', description: 'Create file', points: 20, earnedPoints: 20, passed: true },
          { id: 'cp-3', description: 'Write content', points: 30, earnedPoints: 20, passed: false },
          { id: 'cp-4', description: 'Check disk space', points: 10, earnedPoints: 10, passed: true },
          { id: 'cp-5', description: 'List files', points: 20, earnedPoints: 5, passed: false },
        ],
        achievements: [],
      }
    }
    const response = await api.post<SubmissionResponse>(`/sessions/${id}/submit`)
    return response.data
  },

  cleanup: async (maxAge: string = '24h'): Promise<{ cleaned: number; maxAge: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      return { cleaned: 2, maxAge }
    }
    const response = await api.post<{ cleaned: number; maxAge: string }>(`/sessions/cleanup?maxAge=${encodeURIComponent(maxAge)}`)
    return response.data
  },

  delete: async (id: string): Promise<{ status: string; sessionId: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const allSessions = await getMockSessions()
      const index = allSessions.findIndex(s => s.id === id)
      if (index !== -1) {
        allSessions.splice(index, 1)
      }
      return { status: 'deleted', sessionId: id }
    }
    const response = await api.delete(`/sessions/${id}`)
    return response.data
  },

  /**
   * Gets a progressive hint for a checkpoint
   * @param sessionId Session ID
   * @param checkpointId Checkpoint ID
   * @param level Optional hint level to request (default: next level)
   */
  getCheckpointHint: async (
    sessionId: string,
    checkpointId: string,
    level?: number
  ): Promise<CheckpointHintResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(200)
      const { resolveMockCheckpointHint } = await import('./sessions.mock')
      return resolveMockCheckpointHint(sessionId, checkpointId, level)
    }
    const params = level ? `?level=${level}` : ''
    const response = await api.post<CheckpointHintResponse>(
      `/sessions/${sessionId}/checkpoints/${checkpointId}/hint${params}`
    )
    return response.data
  },
}

// ============================================================================
// Drift assertions — verify the hand-rolled Session shape stays
// compatible with the backend's OpenAPI schema. See comments on the
// equivalent checks in pods.ts.
// ============================================================================
//
// UI-augmented fields: these don't exist on the backend Session struct,
// but the UI enriches the data with them client-side (e.g. joining lab
// template names in reservations.ts). Add to this union when a new
// UI-only field is introduced; the drift guard excludes them so it only
// checks genuine wire fields.
type SessionUIExtras = 'labTemplateName'

type _SessionPhantomFields = Exclude<
  keyof Session,
  keyof NonNullable<ApiSession> | SessionUIExtras
> extends never
  ? true
  : [
      'ERROR: Session declares a field not in ApiSession and not whitelisted as a UI extra',
      Exclude<keyof Session, keyof NonNullable<ApiSession> | SessionUIExtras>,
    ]
const _sessionFieldsOk: _SessionPhantomFields = true
void _sessionFieldsOk
