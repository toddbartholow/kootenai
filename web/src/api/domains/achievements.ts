/**
 * Achievements API
 * Badge and achievement tracking for gamification
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export type AchievementType = 'lab_completion' | 'perfect_score' | 'speed' | 'streak' | 'category' | 'milestone' | 'special'
export type AchievementTier = 'bronze' | 'silver' | 'gold' | 'platinum' | 'diamond'
export type AchievementCriteria = Record<string, unknown>

export interface Achievement {
  id: string
  name: string
  description: string
  type: AchievementType
  tier: AchievementTier
  iconUrl?: string
  points: number
  isSecret: boolean
  isActive: boolean
  criteria: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

export interface UserAchievement {
  id: string
  userId: string
  achievementId: string
  achievement?: Achievement
  earnedAt: string
  sessionId?: string
  notified: boolean
}

export interface AchievementProgress {
  userId: string
  achievementId: string
  achievement?: Achievement
  currentValue: number
  targetValue: number
  percentage: number
  lastUpdated: string
}

// API returns flat structure (Go embedded struct), but mock uses nested
// This interface supports both for compatibility
export interface AchievementWithProgress {
  // Nested structure (mock data)
  achievement?: Achievement
  // Flat structure (API response - Go embedded struct fields)
  id?: string
  name?: string
  description?: string
  type?: AchievementType
  tier?: AchievementTier
  points?: number
  isSecret?: boolean
  isActive?: boolean
  criteria?: AchievementCriteria
  iconUrl?: string
  createdAt?: string
  updatedAt?: string
  // Common fields
  earned: boolean
  earnedAt?: string
  progress?: AchievementProgress
}

export interface UserAchievementSummary {
  userId: string
  totalPoints: number
  achievementsEarned: number
  totalAchievements: number
  tierCounts: Record<AchievementTier, number>
  recentAchievements: UserAchievement[]
}

export interface AchievementsListResponse {
  achievements: Achievement[]
  count: number
}

// ============================================================================
// Mock Data
// ============================================================================
const mockAchievements: Achievement[] = [
  {
    id: 'first-lab',
    name: 'First Steps',
    description: 'Complete your first lab',
    type: 'lab_completion',
    tier: 'bronze',
    points: 10,
    isSecret: false,
    isActive: true,
    criteria: { totalLabs: 1 },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'perfectionist',
    name: 'Perfectionist',
    description: 'Earn a perfect score on any lab',
    type: 'perfect_score',
    tier: 'silver',
    points: 25,
    isSecret: false,
    isActive: true,
    criteria: { requirePerfect: true },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'speed-demon',
    name: 'Speed Demon',
    description: 'Complete a lab in record time',
    type: 'speed',
    tier: 'gold',
    points: 75,
    isSecret: false,
    isActive: true,
    criteria: { maxDurationMins: 15 },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'on-fire',
    name: 'On Fire!',
    description: 'Complete 3 labs in a row',
    type: 'streak',
    tier: 'bronze',
    points: 30,
    isSecret: false,
    isActive: true,
    criteria: { streakCount: 3, streakType: 'any' },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
  {
    id: 'lab-veteran',
    name: 'Lab Veteran',
    description: 'Complete 10 labs',
    type: 'milestone',
    tier: 'silver',
    points: 50,
    isSecret: false,
    isActive: true,
    criteria: { totalLabs: 10 },
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  },
]

// ============================================================================
// API
// ============================================================================
export const achievementsApi = {
  list: async (type?: AchievementType, tier?: AchievementTier): Promise<Achievement[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      let achievements = [...mockAchievements]
      if (type) achievements = achievements.filter(a => a.type === type)
      if (tier) achievements = achievements.filter(a => a.tier === tier)
      return achievements
    }
    const params: Record<string, string> = {}
    if (type) params['type'] = type
    if (tier) params['tier'] = tier
    const response = await api.get<AchievementsListResponse>('/achievements', { params })
    return response.data.achievements || []
  },
  get: async (id: string): Promise<Achievement> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const achievement = mockAchievements.find(a => a.id === id)
      if (!achievement) throw new Error(`Achievement not found: ${id}`)
      return achievement
    }
    const response = await api.get<Achievement>(`/achievements/${id}`)
    return response.data
  },
  getRecent: async (limit: number = 10): Promise<UserAchievement[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return []
    }
    const response = await api.get<{ achievements: UserAchievement[]; count: number }>(`/achievements/recent?limit=${limit}`)
    return response.data.achievements || []
  },
  getUserAchievements: async (userId: string): Promise<AchievementWithProgress[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockAchievements.map(a => {
        const earned = a.id === 'first-lab'
        const result: AchievementWithProgress = {
          achievement: a,
          earned,
        }
        if (earned) {
          result.earnedAt = new Date().toISOString()
        } else {
          result.progress = {
            userId: userId,
            achievementId: a.id,
            currentValue: Math.floor(Math.random() * 3),
            targetValue: 3,
            percentage: Math.floor(Math.random() * 100),
            lastUpdated: new Date().toISOString(),
          }
        }
        return result
      })
    }
    const response = await api.get<{ achievements: AchievementWithProgress[] }>(`/users/${userId}/achievements`)
    return response.data.achievements || []
  },
  getUserSummary: async (userId: string): Promise<UserAchievementSummary> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        userId,
        totalPoints: 10,
        achievementsEarned: 1,
        totalAchievements: mockAchievements.length,
        tierCounts: { bronze: 1, silver: 0, gold: 0, platinum: 0, diamond: 0 },
        recentAchievements: [],
      }
    }
    const response = await api.get<UserAchievementSummary>(`/users/${userId}/achievements/summary`)
    return response.data
  },
}
