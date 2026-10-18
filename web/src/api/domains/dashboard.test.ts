/**
 * Tests for Dashboard API
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { dashboardApi } from './dashboard'

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

describe('Dashboard API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('dashboardApi', () => {
    describe('get()', () => {
      it('should fetch dashboard data', async () => {
        const mockData = {
          user: {
            id: 'user-1',
            displayName: 'Test User',
            email: 'test@example.com',
            totalPoints: 15000,
          },
          enrolledPathways: [
            {
              id: 'p-1',
              name: 'Network Security',
              slug: 'netsec',
              completedModules: 3,
              totalModules: 6,
              percentage: 50,
              enrolledAt: '2024-01-01T00:00:00Z',
              status: 'active',
            },
          ],
          recentSessions: [
            {
              id: 's-1',
              labName: 'Firewall Lab',
              labTemplateId: 'lab-1',
              startedAt: '2024-01-15T10:00:00Z',
              earnedPoints: 800,
              maxPoints: 1000,
              passed: true,
            },
          ],
          achievements: {
            totalEarned: 10,
            totalAvailable: 50,
            totalPoints: 4500,
            recentAchievements: [],
          },
          stats: {
            totalLabsCompleted: 12,
            totalTimeSpentMins: 960,
            currentStreak: 5,
            bestStreak: 10,
            averageScore: 82,
            pathwaysCompleted: 1,
            pathwaysInProgress: 2,
          },
          recommendedNext: [],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.get()

        expect(result).toEqual(mockData)
        expect(mockedAxios.get).toHaveBeenCalledWith('/dashboard')
      })

      it('should return user info', async () => {
        const mockData = {
          user: {
            id: 'user-1',
            displayName: 'Demo User',
            totalPoints: 15420,
            currentRank: 'Gold',
          },
          enrolledPathways: [],
          recentSessions: [],
          achievements: { totalEarned: 0, totalAvailable: 0, totalPoints: 0, recentAchievements: [] },
          stats: { totalLabsCompleted: 0, totalTimeSpentMins: 0, currentStreak: 0, bestStreak: 0, averageScore: 0, pathwaysCompleted: 0, pathwaysInProgress: 0 },
          recommendedNext: [],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.get()

        expect(result.user.displayName).toBe('Demo User')
        expect(result.user.totalPoints).toBe(15420)
        expect(result.user.currentRank).toBe('Gold')
      })

      it('should return enrolled pathways with progress', async () => {
        const mockData = {
          user: { id: 'u-1', displayName: 'User', totalPoints: 0 },
          enrolledPathways: [
            {
              id: 'p-1',
              name: 'Network Security',
              slug: 'netsec',
              description: 'Learn network security basics',
              completedModules: 6,
              totalModules: 8,
              percentage: 75,
              enrolledAt: '2024-01-01T00:00:00Z',
              lastActivityAt: '2024-01-15T10:00:00Z',
              status: 'active',
              currentModule: 'Firewalls',
            },
          ],
          recentSessions: [],
          achievements: { totalEarned: 0, totalAvailable: 0, totalPoints: 0, recentAchievements: [] },
          stats: { totalLabsCompleted: 0, totalTimeSpentMins: 0, currentStreak: 0, bestStreak: 0, averageScore: 0, pathwaysCompleted: 0, pathwaysInProgress: 0 },
          recommendedNext: [],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.get()

        expect(result.enrolledPathways).toHaveLength(1)
        expect(result.enrolledPathways[0]!.percentage).toBe(75)
        expect(result.enrolledPathways[0]!.currentModule).toBe('Firewalls')
      })
    })

    describe('getActivity()', () => {
      it('should fetch activity feed', async () => {
        const mockData = {
          activities: [
            {
              id: 'act-1',
              type: 'lab_passed',
              title: 'Firewall Configuration',
              description: 'Scored 850/1000 points',
              timestamp: '2024-01-15T11:30:00Z',
            },
            {
              id: 'act-2',
              type: 'achievement_earned',
              title: 'On Fire',
              description: 'Complete labs 7 days in a row',
              timestamp: '2024-01-15T10:30:00Z',
            },
          ],
          hasMore: true,
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.getActivity()

        expect(result).toEqual(mockData)
        expect(mockedAxios.get).toHaveBeenCalledWith('/activity')
      })

      it('should return empty activities array', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { activities: [], hasMore: false } })

        const result = await dashboardApi.getActivity()

        expect(result.activities).toEqual([])
        expect(result.hasMore).toBe(false)
      })

      it('should include activity metadata', async () => {
        const mockData = {
          activities: [
            {
              id: 'act-1',
              type: 'lab_passed',
              title: 'Test Lab',
              description: 'Completed',
              timestamp: '2024-01-15T11:30:00Z',
              metadata: {
                labTemplateId: 'lab-1',
                earnedPoints: 850,
                maxPoints: 1000,
                passed: true,
              },
            },
          ],
          hasMore: false,
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.getActivity()

        expect(result.activities[0]!.metadata).toBeDefined()
        expect(result.activities[0]!.metadata?.['earnedPoints']).toBe(850)
      })
    })

    describe('getLeaderboard()', () => {
      it('should fetch leaderboard', async () => {
        const mockData = {
          entries: [
            { rank: 1, userId: 'u1', displayName: 'Alice', totalPoints: 24500, achievementCount: 28, labsCompleted: 32 },
            { rank: 2, userId: 'u2', displayName: 'Bob', totalPoints: 21000, achievementCount: 24, labsCompleted: 28 },
          ],
          currentUser: { rank: 10, userId: 'current', displayName: 'You', totalPoints: 15000, achievementCount: 12, labsCompleted: 14, isCurrentUser: true },
          totalUsers: 100,
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.getLeaderboard()

        expect(result).toEqual(mockData)
        expect(mockedAxios.get).toHaveBeenCalledWith('/leaderboard', { params: {} })
      })

      it('should pass limit as query param', async () => {
        const mockData = { entries: [], totalUsers: 0 }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        await dashboardApi.getLeaderboard(50)

        expect(mockedAxios.get).toHaveBeenCalledWith('/leaderboard', { params: { limit: 50 } })
      })

      it('should return entries with all fields', async () => {
        const mockData = {
          entries: [
            {
              rank: 1,
              userId: 'u1',
              displayName: 'Alice Chen',
              totalPoints: 24500,
              achievementCount: 28,
              labsCompleted: 32,
              isCurrentUser: false,
            },
          ],
          totalUsers: 247,
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.getLeaderboard()
        const entry = result.entries[0]!

        expect(entry.rank).toBe(1)
        expect(entry.userId).toBe('u1')
        expect(entry.displayName).toBe('Alice Chen')
        expect(entry.totalPoints).toBe(24500)
        expect(entry.achievementCount).toBe(28)
        expect(entry.labsCompleted).toBe(32)
      })

      it('should include current user info', async () => {
        const mockData = {
          entries: [
            { rank: 1, userId: 'u1', displayName: 'Alice', totalPoints: 24500, achievementCount: 28, labsCompleted: 32 },
          ],
          currentUser: {
            rank: 12,
            userId: 'current',
            displayName: 'You',
            totalPoints: 15420,
            achievementCount: 12,
            labsCompleted: 14,
            isCurrentUser: true,
          },
          totalUsers: 247,
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await dashboardApi.getLeaderboard()

        expect(result.currentUser).toBeDefined()
        expect(result.currentUser?.rank).toBe(12)
        expect(result.currentUser?.isCurrentUser).toBe(true)
      })

      it('should handle empty leaderboard', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { entries: [], totalUsers: 0 } })

        const result = await dashboardApi.getLeaderboard()

        expect(result.entries).toEqual([])
        expect(result.totalUsers).toBe(0)
        expect(result.currentUser).toBeUndefined()
      })
    })
  })

  describe('Dashboard Stats', () => {
    it('should return all stat fields', async () => {
      const mockData = {
        user: { id: 'u-1', displayName: 'User', totalPoints: 0 },
        enrolledPathways: [],
        recentSessions: [],
        achievements: { totalEarned: 0, totalAvailable: 0, totalPoints: 0, recentAchievements: [] },
        stats: {
          totalLabsCompleted: 14,
          totalTimeSpentMins: 1260,
          currentStreak: 7,
          bestStreak: 12,
          averageScore: 85,
          pathwaysCompleted: 1,
          pathwaysInProgress: 2,
        },
        recommendedNext: [],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await dashboardApi.get()

      expect(result.stats.totalLabsCompleted).toBe(14)
      expect(result.stats.totalTimeSpentMins).toBe(1260)
      expect(result.stats.currentStreak).toBe(7)
      expect(result.stats.bestStreak).toBe(12)
      expect(result.stats.averageScore).toBe(85)
      expect(result.stats.pathwaysCompleted).toBe(1)
      expect(result.stats.pathwaysInProgress).toBe(2)
    })
  })

  describe('Achievements', () => {
    it('should return achievements summary', async () => {
      const mockData = {
        user: { id: 'u-1', displayName: 'User', totalPoints: 0 },
        enrolledPathways: [],
        recentSessions: [],
        achievements: {
          totalEarned: 12,
          totalAvailable: 45,
          totalPoints: 5000,
          recentAchievements: [
            {
              id: 'ach-1',
              name: 'On Fire',
              description: 'Complete labs 7 days in a row',
              tier: 'gold',
              iconUrl: '/icons/fire.png',
              earnedAt: '2024-01-15T10:30:00Z',
              points: 350,
            },
          ],
        },
        stats: { totalLabsCompleted: 0, totalTimeSpentMins: 0, currentStreak: 0, bestStreak: 0, averageScore: 0, pathwaysCompleted: 0, pathwaysInProgress: 0 },
        recommendedNext: [],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await dashboardApi.get()

      expect(result.achievements.totalEarned).toBe(12)
      expect(result.achievements.totalAvailable).toBe(45)
      expect(result.achievements.totalPoints).toBe(5000)
      expect(result.achievements.recentAchievements).toHaveLength(1)
      expect(result.achievements.recentAchievements[0]!.name).toBe('On Fire')
      expect(result.achievements.recentAchievements[0]!.tier).toBe('gold')
    })
  })

  describe('Recent Sessions', () => {
    it('should return recent session data', async () => {
      const mockData = {
        user: { id: 'u-1', displayName: 'User', totalPoints: 0 },
        enrolledPathways: [],
        recentSessions: [
          {
            id: 'session-1',
            labName: 'Firewall Configuration',
            labTemplateId: 'lab-1',
            startedAt: '2024-01-15T10:00:00Z',
            endedAt: '2024-01-15T11:30:00Z',
            earnedPoints: 850,
            maxPoints: 1000,
            passed: true,
            durationMinutes: 90,
          },
          {
            id: 'session-2',
            labName: 'Network Basics',
            labTemplateId: 'lab-2',
            startedAt: '2024-01-14T14:00:00Z',
            endedAt: '2024-01-14T15:00:00Z',
            earnedPoints: 600,
            maxPoints: 800,
            passed: false,
            durationMinutes: 60,
          },
        ],
        achievements: { totalEarned: 0, totalAvailable: 0, totalPoints: 0, recentAchievements: [] },
        stats: { totalLabsCompleted: 0, totalTimeSpentMins: 0, currentStreak: 0, bestStreak: 0, averageScore: 0, pathwaysCompleted: 0, pathwaysInProgress: 0 },
        recommendedNext: [],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await dashboardApi.get()

      expect(result.recentSessions).toHaveLength(2)
      expect(result.recentSessions[0]!.labName).toBe('Firewall Configuration')
      expect(result.recentSessions[0]!.passed).toBe(true)
      expect(result.recentSessions[0]!.durationMinutes).toBe(90)
      expect(result.recentSessions[1]!.passed).toBe(false)
    })
  })

  describe('Recommendations', () => {
    it('should return recommended next actions', async () => {
      const mockData = {
        user: { id: 'u-1', displayName: 'User', totalPoints: 0 },
        enrolledPathways: [],
        recentSessions: [],
        achievements: { totalEarned: 0, totalAvailable: 0, totalPoints: 0, recentAchievements: [] },
        stats: { totalLabsCompleted: 0, totalTimeSpentMins: 0, currentStreak: 0, bestStreak: 0, averageScore: 0, pathwaysCompleted: 0, pathwaysInProgress: 0 },
        recommendedNext: [
          {
            type: 'continue_pathway',
            title: 'Continue Network Security',
            description: "You're 75% complete",
            action: '/pathways/netsec-fundamentals',
            priority: 1,
          },
          {
            type: 'new_lab',
            title: 'Try Log Analysis',
            description: 'Popular with other learners',
            action: '/labs/log-analysis',
            priority: 2,
          },
        ],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await dashboardApi.get()

      expect(result.recommendedNext).toHaveLength(2)
      expect(result.recommendedNext[0]!.type).toBe('continue_pathway')
      expect(result.recommendedNext[0]!.action).toBe('/pathways/netsec-fundamentals')
      expect(result.recommendedNext[1]!.type).toBe('new_lab')
    })
  })
})
