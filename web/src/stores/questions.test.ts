import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useQuestionsStore } from './questions'
import { questionsApi, type QuestionProgress, type SubmitAnswerResponse } from '@/api'
import type { ErrorState } from '@/types/errors'

const testError: ErrorState = {
  message: 'Some error',
  type: 'unknown',
  timestamp: new Date(),
  retryable: true,
}

// Mock the questions API
vi.mock('@/api', () => ({
  questionsApi: {
    getForSession: vi.fn(),
    submitAnswer: vi.fn(),
    getHint: vi.fn(),
  },
}))

const mockQuestions: QuestionProgress[] = [
  {
    id: 'q1',
    type: 'text',
    description: 'What user owns /etc/passwd?',
    points: 10,
    earnedPoints: 0,
    status: 'pending',
    attemptCount: 0,
    hintAvailable: true,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
    isLocked: false,
  },
  {
    id: 'q2',
    type: 'multiple_choice',
    description: 'Which services are running?',
    points: 15,
    earnedPoints: 15,
    status: 'correct',
    attemptCount: 1,
    hintAvailable: false,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
    isLocked: false,
    options: [
      { id: 'A', text: 'Apache' },
      { id: 'B', text: 'MySQL' },
      { id: 'C', text: 'SSH' },
    ],
    multiSelect: true,
  },
  {
    id: 'q3',
    type: 'text',
    description: 'Locked question',
    points: 5,
    earnedPoints: 0,
    status: 'pending',
    attemptCount: 0,
    hintAvailable: false,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
    isLocked: true,
    dependsOn: ['q1'],
  },
]

describe('Questions Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const store = useQuestionsStore()

      expect(store.questions).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.submitting).toBeNull()
    })

    it('should have correct computed initial values', () => {
      const store = useQuestionsStore()

      expect(store.totalPoints).toBe(0)
      expect(store.earnedPoints).toBe(0)
      expect(store.percentage).toBe(0)
      expect(store.answeredCount).toBe(0)
      expect(store.totalCount).toBe(0)
      expect(store.hasQuestions).toBe(false)
    })
  })

  describe('computed properties with data', () => {
    it('should calculate totalPoints correctly', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.totalPoints).toBe(30) // 10 + 15 + 5
    })

    it('should calculate earnedPoints correctly', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.earnedPoints).toBe(15) // only q2 has earned points
    })

    it('should calculate percentage correctly', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.percentage).toBe(50) // 15/30 = 50%
    })

    it('should calculate answeredCount correctly', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.answeredCount).toBe(1) // only q2 is correct
    })

    it('should calculate totalCount correctly', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.totalCount).toBe(3)
    })

    it('should return hasQuestions as true when questions exist', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions

      expect(store.hasQuestions).toBe(true)
    })

    it('should handle zero totalPoints in percentage calculation', () => {
      const store = useQuestionsStore()
      store.questions = []

      expect(store.percentage).toBe(0)
    })
  })

  describe('fetchQuestions()', () => {
    it('should fetch questions successfully', async () => {
      vi.mocked(questionsApi.getForSession).mockResolvedValueOnce({
        sessionId: 'session-1',
        questions: mockQuestions,
        totalPoints: 30,
        earnedPoints: 15,
      })

      const store = useQuestionsStore()
      await store.fetchQuestions('session-1')

      expect(questionsApi.getForSession).toHaveBeenCalledWith('session-1')
      expect(store.questions).toEqual(mockQuestions)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      const promise = new Promise((resolve) => {
        resolvePromise = resolve
      })
      vi.mocked(questionsApi.getForSession).mockReturnValueOnce(promise as any)

      const store = useQuestionsStore()
      const fetchPromise = store.fetchQuestions('session-1')

      expect(store.loading).toBe(true)

      resolvePromise!({ questions: [], totalPoints: 0, earnedPoints: 0 })
      await fetchPromise

      expect(store.loading).toBe(false)
    })

    it('should handle fetch error', async () => {
      vi.mocked(questionsApi.getForSession).mockRejectedValueOnce(new Error('Network error'))

      const store = useQuestionsStore()
      await store.fetchQuestions('session-1')

      expect(store.questions).toEqual([])
      expect(store.error?.message).toBe('Network error')
      expect(store.loading).toBe(false)
    })

    it('should handle non-Error exceptions', async () => {
      vi.mocked(questionsApi.getForSession).mockRejectedValueOnce('Unknown error')

      const store = useQuestionsStore()
      await store.fetchQuestions('session-1')

      expect(store.error?.message).toBe('Failed to load questions')
    })
  })

  describe('submitAnswer()', () => {
    it('should submit text answer successfully', async () => {
      const mockResponse: SubmitAnswerResponse = {
        questionId: 'q1',
        isCorrect: true,
        earnedPoints: 10,
        maxPoints: 10,
        status: 'correct',
        attemptCount: 1,
      }
      vi.mocked(questionsApi.submitAnswer).mockResolvedValueOnce(mockResponse)

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.submitAnswer('session-1', 'q1', { responseText: 'root' })

      expect(questionsApi.submitAnswer).toHaveBeenCalledWith('session-1', 'q1', { responseText: 'root' })
      expect(result).toEqual(mockResponse)
      expect(store.questions[0]!.status).toBe('correct')
      expect(store.questions[0]!.earnedPoints).toBe(10)
      expect(store.questions[0]!.attemptCount).toBe(1)
    })

    it('should submit multiple choice answer successfully', async () => {
      const mockResponse: SubmitAnswerResponse = {
        questionId: 'q2',
        isCorrect: false,
        earnedPoints: 0,
        maxPoints: 15,
        status: 'incorrect',
        attemptCount: 2,
      }
      vi.mocked(questionsApi.submitAnswer).mockResolvedValueOnce(mockResponse)

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.submitAnswer('session-1', 'q2', { selectedOptions: ['A', 'B'] })

      expect(questionsApi.submitAnswer).toHaveBeenCalledWith('session-1', 'q2', { selectedOptions: ['A', 'B'] })
      expect(result).toEqual(mockResponse)
    })

    it('should unlock dependent questions when answer is correct', async () => {
      const mockResponse: SubmitAnswerResponse = {
        questionId: 'q1',
        isCorrect: true,
        earnedPoints: 10,
        maxPoints: 10,
        status: 'correct',
        attemptCount: 1,
      }
      vi.mocked(questionsApi.submitAnswer).mockResolvedValueOnce(mockResponse)

      const store = useQuestionsStore()
      // Deep copy to avoid test interference
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      expect(store.questions[2]!.isLocked).toBe(true)

      await store.submitAnswer('session-1', 'q1', { responseText: 'root' })

      expect(store.questions[2]!.isLocked).toBe(false)
    })

    it('should set submitting state during submission', async () => {
      let resolvePromise: (value: unknown) => void
      const promise = new Promise((resolve) => {
        resolvePromise = resolve
      })
      vi.mocked(questionsApi.submitAnswer).mockReturnValueOnce(promise as any)

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))
      const submitPromise = store.submitAnswer('session-1', 'q1', { responseText: 'test' })

      expect(store.submitting).toBe('q1')

      resolvePromise!({ questionId: 'q1', isCorrect: false, earnedPoints: 0, status: 'incorrect', attemptCount: 1 })
      await submitPromise

      expect(store.submitting).toBeNull()
    })

    it('should handle submit error', async () => {
      vi.mocked(questionsApi.submitAnswer).mockRejectedValueOnce(new Error('Server error'))

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.submitAnswer('session-1', 'q1', { responseText: 'test' })

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Server error')
      expect(store.submitting).toBeNull()
    })
  })

  describe('showHint()', () => {
    it('should fetch hint successfully', async () => {
      const mockHintResponse = {
        questionId: 'q1',
        level: 1,
        maxLevel: 3,
        hint: 'Use ls -l to see ownership',
        penalty: 0,
        totalPenalty: 0,
      }
      vi.mocked(questionsApi.getHint).mockResolvedValueOnce(mockHintResponse)

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.showHint('session-1', 'q1')

      expect(questionsApi.getHint).toHaveBeenCalledWith('session-1', 'q1', undefined)
      expect(result).toEqual(mockHintResponse)
      expect(store.questions[0]!.hintLevelShown).toBe(1)
      expect(store.questions[0]!.hintPenaltyApplied).toBe(0)
    })

    it('should handle hint fetch error', async () => {
      vi.mocked(questionsApi.getHint).mockRejectedValueOnce(new Error('Not found'))

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.showHint('session-1', 'q1')

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Not found')
    })

    it('should apply penalty for progressive hints', async () => {
      const mockHintResponse = {
        questionId: 'q1',
        level: 2,
        maxLevel: 3,
        hint: 'Check /etc/passwd',
        penalty: 2,
        totalPenalty: 2,
      }
      vi.mocked(questionsApi.getHint).mockResolvedValueOnce(mockHintResponse)

      const store = useQuestionsStore()
      store.questions = JSON.parse(JSON.stringify(mockQuestions))

      const result = await store.showHint('session-1', 'q1', 2)

      expect(questionsApi.getHint).toHaveBeenCalledWith('session-1', 'q1', 2)
      expect(result).toEqual(mockHintResponse)
      expect(store.questions[0]!.hintLevelShown).toBe(2)
      expect(store.questions[0]!.hintPenaltyApplied).toBe(2)
    })
  })

  describe('clearError()', () => {
    it('should clear error state', () => {
      const store = useQuestionsStore()
      store.error = testError

      store.clearError()

      expect(store.error).toBeNull()
    })
  })

  describe('reset()', () => {
    it('should reset all state', () => {
      const store = useQuestionsStore()
      store.questions = mockQuestions
      store.loading = true
      store.error = testError
      store.submitting = 'q1'

      store.reset()

      expect(store.questions).toEqual([])
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.submitting).toBeNull()
    })
  })
})
