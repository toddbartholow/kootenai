import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSessionActions } from './useSessionActions'
import type { SubmissionResponse, UserAchievement } from '@/api'

// Mock implementations
const mockRouterPush = vi.fn()
const mockConfirmRequire = vi.fn()
const mockToastAdd = vi.fn()

const createMockRouter = () => ({
  push: mockRouterPush,
})

const createMockConfirm = () => ({
  require: mockConfirmRequire,
})

const createMockToast = () => ({
  add: mockToastAdd,
})

const createMockSessionStore = (overrides = {}) => ({
  endSession: vi.fn(),
  submitSession: vi.fn(),
  passedCheckpoints: 3,
  totalCheckpoints: 5,
  earnedPoints: 75,
  maxPoints: 100,
  percentage: 75,
  ...overrides,
})

describe('useSessionActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const { showAchievementModal, unlockedAchievements } = useSessionActions({
        sessionStore: createMockSessionStore() as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      expect(showAchievementModal.value).toBe(false)
      expect(unlockedAchievements.value).toEqual([])
    })
  })

  describe('handleEndSession()', () => {
    it('should show confirmation dialog', () => {
      const { handleEndSession } = useSessionActions({
        sessionStore: createMockSessionStore() as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleEndSession('session-123')

      expect(mockConfirmRequire).toHaveBeenCalledWith(
        expect.objectContaining({
          message: 'Are you sure you want to end this session? This will stop tracking your progress.',
          header: 'Confirm End Session',
          icon: 'pi pi-stop-circle',
        })
      )
    })

    it('should call endSession and redirect on accept', async () => {
      const mockEndSession = vi.fn().mockResolvedValue({ earnedPoints: 75 })
      const sessionStore = createMockSessionStore({ endSession: mockEndSession })

      // Capture the accept callback
      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleEndSession } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleEndSession('session-123')

      // Wait for async accept callback
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockEndSession).toHaveBeenCalledWith('session-123')
      expect(mockRouterPush).toHaveBeenCalledWith('/sessions')
    })

    it('should not redirect if endSession returns null', async () => {
      const mockEndSession = vi.fn().mockResolvedValue(null)
      const sessionStore = createMockSessionStore({ endSession: mockEndSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleEndSession } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleEndSession('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockEndSession).toHaveBeenCalled()
      expect(mockRouterPush).not.toHaveBeenCalled()
    })
  })

  describe('handleSubmitLab()', () => {
    it('should show confirmation dialog with progress info', () => {
      const sessionStore = createMockSessionStore({
        passedCheckpoints: 3,
        totalCheckpoints: 5,
        earnedPoints: 75,
        maxPoints: 100,
        percentage: 75,
      })

      const { handleSubmitLab } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      expect(mockConfirmRequire).toHaveBeenCalledWith(
        expect.objectContaining({
          header: 'Confirm Lab Submission',
          icon: 'pi pi-send',
        })
      )

      // Check message contains progress info
      const call = mockConfirmRequire.mock.calls[0]![0]
      expect(call.message).toContain('3 of 5 checkpoints')
      expect(call.message).toContain('75 of 100 points')
      expect(call.message).toContain('75%')
    })

    it('should submit lab and show success toast when passed', async () => {
      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        passed: true,
        percentage: 85,
        earnedPoints: 85,
        maxPoints: 100,
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockSubmitSession).toHaveBeenCalledWith('session-123')
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Lab Passed!',
          detail: 'Score: 85% (85/100 points)',
        })
      )
    })

    it('should show warning toast when not passed', async () => {
      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        passed: false,
        percentage: 50,
        earnedPoints: 50,
        maxPoints: 100,
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'warn',
          summary: 'Lab Not Passed',
        })
      )
    })

    it('should show achievement modal when achievements earned', async () => {
      const achievements: UserAchievement[] = [
        {
          id: 'ua-1',
          achievementId: 'first-lab',
          userId: 'user-1',
          earnedAt: '2024-01-01',
          notified: false,
          achievement: { id: 'first-lab', name: 'First Lab', description: 'Complete your first lab', points: 50, iconUrl: 'star.png', type: 'lab_completion', tier: 'bronze', isSecret: false, isActive: true, criteria: {}, createdAt: '2024-01-01', updatedAt: '2024-01-01' },
        },
      ]

      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        passed: true,
        percentage: 100,
        earnedPoints: 100,
        maxPoints: 100,
        achievements,
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab, showAchievementModal, unlockedAchievements } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(showAchievementModal.value).toBe(true)
      expect(unlockedAchievements.value).toEqual(achievements)

      // Should show achievement toast
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Achievement Unlocked!',
          detail: 'You earned 50 points!',
        })
      )
    })

    it('should show multiple achievements notification', async () => {
      const achievements: UserAchievement[] = [
        {
          id: 'ua-1',
          achievementId: 'first-lab',
          userId: 'user-1',
          earnedAt: '2024-01-01',
          notified: false,
          achievement: { id: 'first-lab', name: 'First Lab', description: 'desc', points: 50, iconUrl: 'star.png', type: 'lab_completion', tier: 'bronze', isSecret: false, isActive: true, criteria: {}, createdAt: '2024-01-01', updatedAt: '2024-01-01' },
        },
        {
          id: 'ua-2',
          achievementId: 'speed-demon',
          userId: 'user-1',
          earnedAt: '2024-01-01',
          notified: false,
          achievement: { id: 'speed-demon', name: 'Speed Demon', description: 'desc', points: 25, iconUrl: 'bolt.png', type: 'speed', tier: 'silver', isSecret: false, isActive: true, criteria: {}, createdAt: '2024-01-01', updatedAt: '2024-01-01' },
        },
      ]

      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        passed: true,
        percentage: 100,
        earnedPoints: 100,
        maxPoints: 100,
        achievements,
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          summary: '2 Achievements Unlocked!',
          detail: 'You earned 75 points!',
        })
      )
    })

    it('should not show achievement modal if no achievements', async () => {
      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        status: 'graded',
        passed: true,
        percentage: 100,
        earnedPoints: 100,
        maxPoints: 100,
        passThreshold: 70,
        submittedAt: '2024-01-01',
        checkpoints: [],
        achievements: [],
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab, showAchievementModal } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(showAchievementModal.value).toBe(false)
    })

    it('should not show toasts if submission returns null', async () => {
      const mockSubmitSession = vi.fn().mockResolvedValue(null)
      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')

      await new Promise(resolve => setTimeout(resolve, 0))

      expect(mockToastAdd).not.toHaveBeenCalled()
    })
  })

  describe('closeAchievementModal()', () => {
    it('should close modal and clear achievements', async () => {
      const achievements: UserAchievement[] = [
        {
          id: 'ua-1',
          achievementId: 'first-lab',
          userId: 'user-1',
          earnedAt: '2024-01-01',
          notified: false,
          achievement: { id: 'first-lab', name: 'First Lab', description: 'desc', points: 50, iconUrl: 'star.png', type: 'lab_completion', tier: 'bronze', isSecret: false, isActive: true, criteria: {}, createdAt: '2024-01-01', updatedAt: '2024-01-01' },
        },
      ]

      const mockSubmitSession = vi.fn().mockResolvedValue({
        sessionId: 'session-123',
        passed: true,
        percentage: 100,
        earnedPoints: 100,
        maxPoints: 100,
        achievements,
      } as SubmissionResponse)

      const sessionStore = createMockSessionStore({ submitSession: mockSubmitSession })

      mockConfirmRequire.mockImplementation((options) => {
        options.accept()
      })

      const { handleSubmitLab, closeAchievementModal, showAchievementModal, unlockedAchievements } = useSessionActions({
        sessionStore: sessionStore as any,
        router: createMockRouter() as any,
        confirm: createMockConfirm() as any,
        toast: createMockToast() as any,
      })

      handleSubmitLab('session-123')
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(showAchievementModal.value).toBe(true)
      expect(unlockedAchievements.value).toHaveLength(1)

      closeAchievementModal()

      expect(showAchievementModal.value).toBe(false)
      expect(unlockedAchievements.value).toEqual([])
    })
  })
})
