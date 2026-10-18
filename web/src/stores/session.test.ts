import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSessionStore, type CheckpointProgress } from './session'
import { sessionsApi, type Session, type SubmissionResponse, type CheckpointHintResponse } from '@/api'
import type {
  CheckpointUpdatePayload,
  GradeUpdatePayload,
  SessionEventPayload,
} from '@/composables/useWebSocket'
import type { ErrorState } from '@/types/errors'

const testError: ErrorState = {
  message: 'Some error',
  type: 'unknown',
  timestamp: new Date(),
  retryable: true,
}

// Type for mock progress
interface MockProgress {
  sessionId: string
  earnedPoints: number
  maxPoints: number
  percentage: number
  checkpoints: Array<{ id: string; name: string; status: string; score: number; maxScore: number }>
}

// Mock the API client
vi.mock('@/api', () => ({
  sessionsApi: {
    get: vi.fn(),
    getProgress: vi.fn(),
    end: vi.fn(),
    submit: vi.fn(),
    getCheckpointHint: vi.fn(),
  },
}))

// Mock session data
const mockSession: Session = {
  id: 'session-1',
  podId: 'pod-1',
  userId: 'user@test.com',
  labTemplateId: 'linux-basics',
  status: 'active',
  earnedPoints: 50,
  maxPoints: 100,
  percentage: 50,
  passed: false,
  startedAt: '2024-01-01T10:00:00Z',
}

const mockProgress: MockProgress = {
  sessionId: 'session-1',
  earnedPoints: 50,
  maxPoints: 100,
  percentage: 50,
  checkpoints: [
    { id: 'cp-1', name: 'Checkpoint 1', status: 'passed', score: 30, maxScore: 30 },
    { id: 'cp-2', name: 'Checkpoint 2', status: 'pending', score: 0, maxScore: 40 },
    { id: 'cp-3', name: 'Checkpoint 3', status: 'pending', score: 0, maxScore: 30 },
  ],
}

describe('Session Store', () => {
  let store: ReturnType<typeof useSessionStore>

  beforeEach(() => {
    setActivePinia(createPinia())
    store = useSessionStore()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have empty initial state', () => {
      expect(store.currentSession).toBeNull()
      expect(store.checkpoints).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.sessionStatus).toBe('started')
      expect(store.submissionStatus).toBe('idle')
    })
  })

  describe('computed properties', () => {
    it('should compute earnedPoints from session', () => {
      store.currentSession = mockSession
      expect(store.earnedPoints).toBe(50)
    })

    it('should return 0 earnedPoints when no session', () => {
      expect(store.earnedPoints).toBe(0)
    })

    it('should compute maxPoints from session', () => {
      store.currentSession = mockSession
      expect(store.maxPoints).toBe(100)
    })

    it('should compute percentage correctly', () => {
      store.currentSession = mockSession
      expect(store.percentage).toBe(50)
    })

    it('should return 0 percentage when maxPoints is 0', () => {
      store.currentSession = { ...mockSession, maxPoints: 0 }
      expect(store.percentage).toBe(0)
    })

    it('should compute passedCheckpoints', () => {
      store.checkpoints = mockProgress.checkpoints as CheckpointProgress[]
      expect(store.passedCheckpoints).toBe(1)
    })

    it('should compute totalCheckpoints', () => {
      store.checkpoints = mockProgress.checkpoints as CheckpointProgress[]
      expect(store.totalCheckpoints).toBe(3)
    })
  })

  describe('fetchSession()', () => {
    it('should fetch session and progress', async () => {
      vi.mocked(sessionsApi.get).mockResolvedValue(mockSession)
      vi.mocked(sessionsApi.getProgress).mockResolvedValue(mockProgress)

      await store.fetchSession('session-1')

      expect(store.currentSession).toEqual(mockSession)
      expect(store.checkpoints).toHaveLength(3)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should set loading state during fetch', async () => {
      let resolveGet: (value: Session) => void
      vi.mocked(sessionsApi.get).mockImplementation(
        () => new Promise(resolve => { resolveGet = resolve })
      )
      vi.mocked(sessionsApi.getProgress).mockResolvedValue(mockProgress)

      const fetchPromise = store.fetchSession('session-1')
      expect(store.loading).toBe(true)

      resolveGet!(mockSession)
      await fetchPromise

      expect(store.loading).toBe(false)
    })

    it('should handle fetch error', async () => {
      vi.mocked(sessionsApi.get).mockRejectedValue(new Error('Session not found'))

      await store.fetchSession('session-999')

      expect(store.error?.message).toBe('Session not found')
      expect(store.currentSession).toBeNull()
    })
  })

  describe('endSession()', () => {
    it('should end session successfully', async () => {
      store.currentSession = { ...mockSession }
      vi.mocked(sessionsApi.end).mockResolvedValue({
        status: 'completed',
        earnedPoints: 75,
        passed: true
      })

      const result = await store.endSession('session-1')

      expect(result).toEqual({ earnedPoints: 75, passed: true, status: 'completed' })
      expect(store.sessionStatus).toBe('ended')
      expect(store.currentSession?.earnedPoints).toBe(75)
      expect(store.currentSession?.passed).toBe(true)
    })

    it('should handle end session error', async () => {
      store.currentSession = { ...mockSession }
      vi.mocked(sessionsApi.end).mockRejectedValue(new Error('Cannot end'))

      const result = await store.endSession('session-1')

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Cannot end')
    })
  })

  describe('submitSession()', () => {
    it('should submit session successfully', async () => {
      store.currentSession = { ...mockSession }
      const submissionResponse: SubmissionResponse = {
        sessionId: 'session-1',
        status: 'graded',
        earnedPoints: 85,
        maxPoints: 100,
        percentage: 85,
        passed: true,
        passThreshold: 70,
        submittedAt: '2024-01-01T12:00:00Z',
        checkpoints: [],
        achievements: [
          { id: 'ach-1', achievementId: 'first-lab', userId: 'user-1', earnedAt: '2024-01-01T12:00:00Z', notified: false }
        ],
        moduleCompletion: {
          moduleId: 'mod-1',
          moduleName: 'Linux Basics',
          pathwayId: 'pathway-1',
          pathwayName: 'Linux Path',
          pathwayCompleted: false,
        },
      }
      vi.mocked(sessionsApi.submit).mockResolvedValue(submissionResponse)

      const result = await store.submitSession('session-1')

      expect(result).toEqual(submissionResponse)
      expect(store.submissionStatus).toBe('submitted')
      expect(store.submissionResult).toEqual(submissionResponse)
      expect(store.currentSession?.status).toBe('completed')
      expect(store.newAchievements).toHaveLength(1)
      expect(store.moduleCompletion?.moduleName).toBe('Linux Basics')
      expect(store.sessionStatus).toBe('ended')
    })

    it('should handle submission error', async () => {
      store.currentSession = { ...mockSession }
      vi.mocked(sessionsApi.submit).mockRejectedValue(new Error('Submission failed'))

      const result = await store.submitSession('session-1')

      expect(result).toBeNull()
      expect(store.submissionStatus).toBe('error')
      expect(store.submissionError).toBe('Submission failed')
    })

    it('should set submitting status during submit', async () => {
      store.currentSession = { ...mockSession }
      let resolveSubmit: (value: SubmissionResponse) => void
      vi.mocked(sessionsApi.submit).mockImplementation(
        () => new Promise(resolve => { resolveSubmit = resolve })
      )

      const submitPromise = store.submitSession('session-1')
      expect(store.submissionStatus).toBe('submitting')

      resolveSubmit!({
        sessionId: 'session-1',
        status: 'graded',
        earnedPoints: 85,
        maxPoints: 100,
        percentage: 85,
        passed: true,
        passThreshold: 70,
        submittedAt: '2024-01-01T12:00:00Z',
        checkpoints: [],
      })
      await submitPromise

      expect(store.submissionStatus).toBe('submitted')
    })
  })

  describe('handleSessionUpdate()', () => {
    it('should update session status from WebSocket event', () => {
      store.currentSession = { ...mockSession }
      const event: SessionEventPayload = {
        sessionId: 'session-1',
        podId: 'pod-1',
        status: 'paused',
        timestamp: '2024-01-01T11:00:00Z',
      }

      store.handleSessionUpdate(event)

      expect(store.sessionStatus).toBe('paused')
      expect(store.currentSession?.status).toBe('paused')
      expect(store.lastUpdate).toEqual(new Date('2024-01-01T11:00:00Z'))
    })

    it('should not update session if sessionId does not match', () => {
      store.currentSession = { ...mockSession }
      const event: SessionEventPayload = {
        sessionId: 'session-other',
        podId: 'pod-1',
        status: 'ended',
        timestamp: '2024-01-01T11:00:00Z',
      }

      store.handleSessionUpdate(event)

      expect(store.sessionStatus).toBe('ended')
      expect(store.currentSession?.status).toBe('active') // unchanged
    })
  })

  describe('handleCheckpointUpdate()', () => {
    it('should update existing checkpoint', () => {
      store.checkpoints = mockProgress.checkpoints as CheckpointProgress[]
      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-1',
        checkpointId: 'cp-2',
        name: 'Checkpoint 2',
        status: 'passed',
        score: 40,
        maxScore: 40,
        timestamp: '2024-01-01T11:30:00Z',
      }

      store.handleCheckpointUpdate(update)

      expect(store.checkpoints[1]!.status).toBe('passed')
      expect(store.checkpoints[1]!.score).toBe(40)
    })

    it('should add new checkpoint if not exists', () => {
      store.checkpoints = []
      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-1',
        checkpointId: 'cp-new',
        name: 'New Checkpoint',
        status: 'passed',
        score: 25,
        maxScore: 25,
        timestamp: '2024-01-01T11:30:00Z',
      }

      store.handleCheckpointUpdate(update)

      expect(store.checkpoints).toHaveLength(1)
      expect(store.checkpoints[0]!.id).toBe('cp-new')
      expect(store.checkpoints[0]!.name).toBe('New Checkpoint')
    })

    it('should update session earnedPoints on checkpoint pass', () => {
      store.currentSession = { ...mockSession, earnedPoints: 30 }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'passed', score: 30, maxScore: 30 },
      ]
      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-1',
        checkpointId: 'cp-2',
        name: 'Checkpoint 2',
        status: 'passed',
        score: 40,
        maxScore: 40,
        timestamp: '2024-01-01T11:30:00Z',
      }

      store.handleCheckpointUpdate(update)

      expect(store.currentSession?.earnedPoints).toBe(70) // 30 + 40
    })
  })

  describe('handleGradeUpdate()', () => {
    it('should update session grades from WebSocket', () => {
      store.currentSession = { ...mockSession }
      const update: GradeUpdatePayload = {
        sessionId: 'session-1',
        userId: 'user@test.com',
        score: 90,
        maxScore: 100,
        percentage: 90,
        passed: true,
        timestamp: '2024-01-01T11:45:00Z',
      }

      store.handleGradeUpdate(update)

      expect(store.currentSession?.earnedPoints).toBe(90)
      expect(store.currentSession?.percentage).toBe(90)
      expect(store.currentSession?.passed).toBe(true)
      expect(store.lastUpdate).toEqual(new Date('2024-01-01T11:45:00Z'))
    })

    it('should set passed to false when below threshold', () => {
      store.currentSession = { ...mockSession }
      const update: GradeUpdatePayload = {
        sessionId: 'session-1',
        userId: 'user@test.com',
        score: 40,
        maxScore: 100,
        percentage: 40,
        passed: false,
        timestamp: '2024-01-01T11:45:00Z',
      }

      store.handleGradeUpdate(update)

      expect(store.currentSession?.passed).toBe(false)
    })

    it('should not update if sessionId does not match', () => {
      store.currentSession = { ...mockSession }
      const update: GradeUpdatePayload = {
        sessionId: 'session-other',
        userId: 'user@test.com',
        score: 100,
        maxScore: 100,
        percentage: 100,
        passed: true,
        timestamp: '2024-01-01T11:45:00Z',
      }

      store.handleGradeUpdate(update)

      expect(store.currentSession?.earnedPoints).toBe(50) // unchanged
    })
  })

  describe('clearNewAchievements()', () => {
    it('should clear new achievements', () => {
      store.newAchievements = [
        { id: 'ach-1', achievementId: 'first-lab', userId: 'user-1', earnedAt: '2024-01-01', notified: false }
      ]

      store.clearNewAchievements()

      expect(store.newAchievements).toEqual([])
    })
  })

  describe('clearModuleCompletion()', () => {
    it('should clear module completion info', () => {
      store.moduleCompletion = {
        moduleId: 'mod-1',
        moduleName: 'Test Module',
        pathwayId: 'pathway-1',
        pathwayName: 'Test Pathway',
        pathwayCompleted: false,
      }

      store.clearModuleCompletion()

      expect(store.moduleCompletion).toBeNull()
    })
  })

  describe('reset()', () => {
    it('should reset all state to initial values', () => {
      store.currentSession = mockSession
      store.checkpoints = mockProgress.checkpoints as CheckpointProgress[]
      store.loading = true
      store.error = testError
      store.sessionStatus = 'ended'
      store.lastUpdate = new Date()
      store.submissionStatus = 'submitted'
      store.submissionResult = { sessionId: 'session-1', status: 'graded', earnedPoints: 100, maxPoints: 100, percentage: 100, passed: true, passThreshold: 70, submittedAt: '2024-01-01', checkpoints: [] }
      store.submissionError = 'error'
      store.newAchievements = [{ id: '1', achievementId: 'a', userId: 'u', earnedAt: '2024-01-01', notified: false }]
      store.moduleCompletion = { moduleId: 'm', moduleName: 'M', pathwayId: 'p', pathwayName: 'P', pathwayCompleted: false }

      store.reset()

      expect(store.currentSession).toBeNull()
      expect(store.checkpoints).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.sessionStatus).toBe('started')
      expect(store.lastUpdate).toBeNull()
      expect(store.submissionStatus).toBe('idle')
      expect(store.submissionResult).toBeNull()
      expect(store.submissionError).toBeNull()
      expect(store.newAchievements).toEqual([])
      expect(store.moduleCompletion).toBeNull()
    })
  })

  describe('showCheckpointHint()', () => {
    const mockHintResponse: CheckpointHintResponse = {
      checkpointId: 'cp-1',
      level: 1,
      maxLevel: 3,
      hint: 'Look in the home directory',
      penalty: 0,
      totalPenalty: 0,
    }

    it('should return null when no current session', async () => {
      const result = await store.showCheckpointHint('cp-1')
      expect(result).toBeNull()
      expect(sessionsApi.getCheckpointHint).not.toHaveBeenCalled()
    })

    it('should fetch hint successfully', async () => {
      store.currentSession = { ...mockSession }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 0, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValueOnce(mockHintResponse)

      const result = await store.showCheckpointHint('cp-1')

      expect(sessionsApi.getCheckpointHint).toHaveBeenCalledWith('session-1', 'cp-1', undefined)
      expect(result).toEqual(mockHintResponse)
      expect(store.revealedCheckpointHints['cp-1']).toHaveLength(1)
      expect(store.revealedCheckpointHints['cp-1']![0]).toEqual({ level: 1, text: 'Look in the home directory' })
    })

    it('should fetch hint with specific level', async () => {
      store.currentSession = { ...mockSession }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 1, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      const level2Response: CheckpointHintResponse = {
        checkpointId: 'cp-1',
        level: 2,
        maxLevel: 3,
        hint: 'Check ~/.config directory',
        penalty: 2,
        totalPenalty: 2,
      }
      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValueOnce(level2Response)

      const result = await store.showCheckpointHint('cp-1', 2)

      expect(sessionsApi.getCheckpointHint).toHaveBeenCalledWith('session-1', 'cp-1', 2)
      expect(result).toEqual(level2Response)
    })

    it('should update checkpoint progress after getting hint', async () => {
      store.currentSession = { ...mockSession }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 0, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      const hintWithPenalty: CheckpointHintResponse = {
        checkpointId: 'cp-1',
        level: 2,
        maxLevel: 3,
        hint: 'Check ~/.config directory',
        penalty: 3,
        totalPenalty: 5,
      }
      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValueOnce(hintWithPenalty)

      await store.showCheckpointHint('cp-1', 2)

      expect(store.checkpoints[0]!.hintLevelShown).toBe(2)
      expect(store.checkpoints[0]!.hintPenaltyApplied).toBe(5)
    })

    it('should sort revealed hints by level', async () => {
      store.currentSession = { ...mockSession }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 0, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]

      // Reveal hint 2 first
      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValueOnce({
        checkpointId: 'cp-1', level: 2, maxLevel: 3, hint: 'Hint 2', penalty: 0, totalPenalty: 0,
      })
      await store.showCheckpointHint('cp-1', 2)

      // Then reveal hint 1
      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValueOnce({
        checkpointId: 'cp-1', level: 1, maxLevel: 3, hint: 'Hint 1', penalty: 0, totalPenalty: 0,
      })
      await store.showCheckpointHint('cp-1', 1)

      expect(store.revealedCheckpointHints['cp-1']).toHaveLength(2)
      expect(store.revealedCheckpointHints['cp-1']![0]!.level).toBe(1)
      expect(store.revealedCheckpointHints['cp-1']![1]!.level).toBe(2)
    })

    it('should not duplicate already revealed hints', async () => {
      store.currentSession = { ...mockSession }
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 1, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]

      vi.mocked(sessionsApi.getCheckpointHint).mockResolvedValue(mockHintResponse)

      await store.showCheckpointHint('cp-1')
      await store.showCheckpointHint('cp-1') // Same hint again

      expect(store.revealedCheckpointHints['cp-1']).toHaveLength(1)
    })

    it('should handle error and set error state', async () => {
      store.currentSession = { ...mockSession }
      vi.mocked(sessionsApi.getCheckpointHint).mockRejectedValueOnce(new Error('Checkpoint not found'))

      const result = await store.showCheckpointHint('cp-1')

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Checkpoint not found')
    })

    it('should handle non-Error exceptions', async () => {
      store.currentSession = { ...mockSession }
      vi.mocked(sessionsApi.getCheckpointHint).mockRejectedValueOnce('Unknown error')

      const result = await store.showCheckpointHint('cp-1')

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Failed to get hint')
    })
  })

  describe('hasMoreCheckpointHints()', () => {
    it('should return false when checkpoint not found', () => {
      store.checkpoints = []
      expect(store.hasMoreCheckpointHints('cp-unknown')).toBe(false)
    })

    it('should return false when checkpoint has no hints', () => {
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 0, hintLevelShown: 0, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      expect(store.hasMoreCheckpointHints('cp-1')).toBe(false)
    })

    it('should return true when more hints available', () => {
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 1, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      expect(store.hasMoreCheckpointHints('cp-1')).toBe(true)
    })

    it('should return false when all hints shown', () => {
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 3, hintPenaltyApplied: 5 },
      ] as CheckpointProgress[]
      expect(store.hasMoreCheckpointHints('cp-1')).toBe(false)
    })
  })

  describe('getNextCheckpointHintLevel()', () => {
    it('should return 1 for checkpoint with no hints shown', () => {
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 0, hintPenaltyApplied: 0 },
      ] as CheckpointProgress[]
      expect(store.getNextCheckpointHintLevel('cp-1')).toBe(1)
    })

    it('should return next level after current', () => {
      store.checkpoints = [
        { id: 'cp-1', name: 'Checkpoint 1', status: 'pending', score: 0, maxScore: 30, hintCount: 3, hintLevelShown: 2, hintPenaltyApplied: 3 },
      ] as CheckpointProgress[]
      expect(store.getNextCheckpointHintLevel('cp-1')).toBe(3)
    })

    it('should return 1 for unknown checkpoint', () => {
      store.checkpoints = []
      expect(store.getNextCheckpointHintLevel('cp-unknown')).toBe(1)
    })
  })
})
