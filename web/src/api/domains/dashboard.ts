/**
 * Dashboard API
 * User dashboard, activity feed, and leaderboard
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface DashboardUser {
  id: string
  displayName: string
  email?: string
  totalPoints: number
  currentRank?: string
}

export interface DashboardPathway {
  id: string
  name: string
  slug: string
  description?: string
  iconUrl?: string
  completedModules: number
  totalModules: number
  percentage: number
  enrolledAt: string
  lastActivityAt?: string
  status: string
  currentModule?: string
}

export interface DashboardSession {
  id: string
  labName: string
  labTemplateId: string
  startedAt: string
  endedAt?: string
  earnedPoints: number
  maxPoints: number
  passed: boolean
  durationMinutes?: number
}

export interface DashboardAchievement {
  id: string
  name: string
  description: string
  tier: string
  iconUrl?: string
  earnedAt: string
  points: number
}

export interface DashboardAchievements {
  totalEarned: number
  totalAvailable: number
  totalPoints: number
  recentAchievements: DashboardAchievement[]
}

export interface DashboardStats {
  totalLabsCompleted: number
  totalTimeSpentMins: number
  currentStreak: number
  bestStreak: number
  averageScore: number
  pathwaysCompleted: number
  pathwaysInProgress: number
}

export interface DashboardRecommendation {
  type: string
  title: string
  description: string
  action: string
  priority: number
}

export interface DashboardResponse {
  user: DashboardUser
  enrolledPathways: DashboardPathway[]
  recentSessions: DashboardSession[]
  achievements: DashboardAchievements
  stats: DashboardStats
  recommendedNext: DashboardRecommendation[]
}

export interface ActivityItem {
  id: string
  type: string
  title: string
  description: string
  timestamp: string
  metadata?: Record<string, unknown>
}

export interface ActivityResponse {
  activities: ActivityItem[]
  hasMore: boolean
}

export interface LeaderboardEntry {
  rank: number
  userId: string
  displayName: string
  totalPoints: number
  achievementCount: number
  labsCompleted: number
  isCurrentUser?: boolean
}

export interface LeaderboardResponse {
  entries: LeaderboardEntry[]
  currentUser?: LeaderboardEntry
  totalUsers: number
}

// Time Analytics Types
export interface DailyTimeStats {
  date: string // YYYY-MM-DD format
  timeMinutes: number
  sessionCount: number
}

export interface LabTimeStats {
  labId: string
  labName: string
  labSlug: string
  totalMinutes: number
  sessionCount: number
  averageMinutes: number
  bestScore?: number
}

export interface WeeklyTimeStats {
  weekStart: string // YYYY-MM-DD (Monday)
  timeMinutes: number
  sessionCount: number
}

export interface TimeStats {
  totalLabsCompleted: number
  averageScore: number
  currentStreak: number
  bestStreak: number
  pointsPerMinute?: number
  mostActiveDay?: string
}

export interface TimeAnalyticsResponse {
  userId: string
  period: string // "week", "month", "all"
  totalTimeMinutes: number
  sessionCount: number
  averageSessionMinutes: number
  longestSessionMinutes: number
  dailyBreakdown: DailyTimeStats[]
  labBreakdown: LabTimeStats[]
  weeklyTrend: WeeklyTimeStats[]
  stats: TimeStats
}

// Insights Types
export interface SkillProficiency {
  tag: string
  proficiency: number
  labsCompleted: number
  avgScore: number
}

export interface WeeklyTimeEntry {
  week: string
  minutes: number
}

export interface InsightsResponse {
  skills: SkillProficiency[]
  strengths: string[]
  weaknesses: string[]
  streakCalendar: Record<string, number>
  weeklyTime: WeeklyTimeEntry[]
}

// ============================================================================
// Mock Data
// ============================================================================
const mockDashboardData: DashboardResponse = {
  user: {
    id: 'user-1',
    displayName: 'Demo User',
    email: 'demo@example.com',
    totalPoints: 15420
  },
  enrolledPathways: [
    {
      id: 'pathway-1',
      name: 'Network Security Fundamentals',
      slug: 'netsec-fundamentals',
      description: 'Learn the basics of network security',
      completedModules: 6,
      totalModules: 8,
      percentage: 75,
      enrolledAt: '2024-01-01T00:00:00Z',
      status: 'active'
    },
    {
      id: 'pathway-2',
      name: 'Incident Response',
      slug: 'incident-response',
      description: 'Master incident response procedures',
      completedModules: 4,
      totalModules: 8,
      percentage: 50,
      enrolledAt: '2024-01-05T00:00:00Z',
      status: 'active'
    }
  ],
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
      durationMinutes: 90
    }
  ],
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
        earnedAt: '2024-01-15T10:30:00Z',
        points: 350
      },
      {
        id: 'ach-2',
        name: 'Firewall Master',
        description: 'Configure 50 firewall rules',
        tier: 'silver',
        earnedAt: '2024-01-14T15:22:00Z',
        points: 500
      }
    ]
  },
  stats: {
    totalLabsCompleted: 14,
    totalTimeSpentMins: 1260,
    currentStreak: 7,
    bestStreak: 12,
    averageScore: 85,
    pathwaysCompleted: 1,
    pathwaysInProgress: 2
  },
  recommendedNext: [
    {
      type: 'continue_pathway',
      title: 'Continue Network Security',
      description: "You're 75% complete",
      action: '/pathways/netsec-fundamentals',
      priority: 1
    }
  ]
}

const mockActivityData: ActivityResponse = {
  activities: [
    {
      id: 'act-1',
      type: 'lab_passed',
      title: 'Firewall Configuration',
      description: 'Scored 850/1000 points',
      timestamp: '2024-01-15T11:30:00Z',
      metadata: { labTemplateId: 'lab-1', earnedPoints: 850, maxPoints: 1000, passed: true }
    },
    {
      id: 'act-2',
      type: 'achievement_earned',
      title: 'On Fire',
      description: 'Complete labs 7 days in a row',
      timestamp: '2024-01-15T10:30:00Z',
      metadata: { achievementId: 'ach-1', tier: 'gold', points: 350 }
    }
  ],
  hasMore: false
}

const mockLeaderboardData: LeaderboardResponse = {
  entries: [
    { rank: 1, userId: 'u1', displayName: 'Alice Chen', totalPoints: 24500, achievementCount: 28, labsCompleted: 32 },
    { rank: 2, userId: 'u2', displayName: 'Bob Williams', totalPoints: 21000, achievementCount: 24, labsCompleted: 28 },
    { rank: 3, userId: 'u3', displayName: 'Carol Martinez', totalPoints: 19500, achievementCount: 22, labsCompleted: 26 },
    { rank: 4, userId: 'u4', displayName: 'David Kim', totalPoints: 18200, achievementCount: 20, labsCompleted: 24 },
    { rank: 5, userId: 'u5', displayName: 'Eva Thompson', totalPoints: 17100, achievementCount: 19, labsCompleted: 23 },
    { rank: 12, userId: 'current', displayName: 'You', totalPoints: 15420, achievementCount: 12, labsCompleted: 14, isCurrentUser: true }
  ],
  currentUser: { rank: 12, userId: 'current', displayName: 'You', totalPoints: 15420, achievementCount: 12, labsCompleted: 14, isCurrentUser: true },
  totalUsers: 247
}

// Generate mock daily data for last 7 days
const generateMockDailyData = (): DailyTimeStats[] => {
  const days: DailyTimeStats[] = []
  const today = new Date()
  for (let i = 6; i >= 0; i--) {
    const date = new Date(today)
    date.setDate(date.getDate() - i)
    const dateStr = date.toISOString().split('T')[0] ?? ''
    days.push({
      date: dateStr,
      timeMinutes: Math.floor(Math.random() * 120) + 30,
      sessionCount: Math.floor(Math.random() * 3) + 1
    })
  }
  return days
}

// Generate mock weekly data for last 4 weeks
const generateMockWeeklyData = (): WeeklyTimeStats[] => {
  const weeks: WeeklyTimeStats[] = []
  const today = new Date()
  for (let i = 3; i >= 0; i--) {
    const date = new Date(today)
    date.setDate(date.getDate() - (i * 7))
    // Move to Monday
    const day = date.getDay()
    const diff = date.getDate() - day + (day === 0 ? -6 : 1)
    date.setDate(diff)
    const weekStartStr = date.toISOString().split('T')[0] ?? ''
    weeks.push({
      weekStart: weekStartStr,
      timeMinutes: Math.floor(Math.random() * 400) + 200,
      sessionCount: Math.floor(Math.random() * 10) + 5
    })
  }
  return weeks
}

const mockTimeAnalyticsData: TimeAnalyticsResponse = {
  userId: 'user-1',
  period: 'month',
  totalTimeMinutes: 1260,
  sessionCount: 28,
  averageSessionMinutes: 45,
  longestSessionMinutes: 120,
  dailyBreakdown: generateMockDailyData(),
  labBreakdown: [
    { labId: 'lab-1', labName: 'Firewall Configuration', labSlug: 'firewall-config', totalMinutes: 320, sessionCount: 8, averageMinutes: 40, bestScore: 95 },
    { labId: 'lab-2', labName: 'Network Scanning', labSlug: 'network-scanning', totalMinutes: 280, sessionCount: 7, averageMinutes: 40, bestScore: 88 },
    { labId: 'lab-3', labName: 'Linux Fundamentals', labSlug: 'linux-fundamentals', totalMinutes: 240, sessionCount: 6, averageMinutes: 40, bestScore: 92 },
    { labId: 'lab-4', labName: 'Docker Basics', labSlug: 'docker-basics', totalMinutes: 200, sessionCount: 4, averageMinutes: 50, bestScore: 85 },
    { labId: 'lab-5', labName: 'SQL Injection', labSlug: 'sql-injection', totalMinutes: 220, sessionCount: 3, averageMinutes: 73, bestScore: 78 }
  ],
  weeklyTrend: generateMockWeeklyData(),
  stats: {
    totalLabsCompleted: 14,
    averageScore: 85,
    currentStreak: 7,
    bestStreak: 12,
    pointsPerMinute: 12.24,
    mostActiveDay: 'Wednesday'
  }
}

const mockInsightsData: InsightsResponse = {
  skills: [
    { tag: 'linux', proficiency: 88, labsCompleted: 12, avgScore: 88 },
    { tag: 'networking', proficiency: 72, labsCompleted: 8, avgScore: 72 },
    { tag: 'security', proficiency: 65, labsCompleted: 6, avgScore: 65 },
    { tag: 'scripting', proficiency: 82, labsCompleted: 5, avgScore: 82 },
    { tag: 'cloud', proficiency: 45, labsCompleted: 3, avgScore: 45 },
    { tag: 'forensics', proficiency: 55, labsCompleted: 4, avgScore: 55 },
  ],
  strengths: ['linux', 'scripting', 'networking'],
  weaknesses: ['cloud', 'forensics', 'security'],
  streakCalendar: (() => {
    const cal: Record<string, number> = {}
    const today = new Date()
    for (let i = 0; i < 90; i++) {
      const d = new Date(today)
      d.setDate(d.getDate() - i)
      const key = d.toISOString().split('T')[0] ?? ''
      if (Math.random() > 0.4) cal[key] = Math.floor(Math.random() * 4) + 1
    }
    return cal
  })(),
  weeklyTime: [
    { week: '2026-W01', minutes: 220 },
    { week: '2026-W02', minutes: 310 },
    { week: '2026-W03', minutes: 180 },
    { week: '2026-W04', minutes: 260 },
  ],
}

// ============================================================================
// API
// ============================================================================
export const dashboardApi = {
  get: async (): Promise<DashboardResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockDashboardData
    }
    const response = await api.get<DashboardResponse>('/dashboard')
    return response.data
  },

  getActivity: async (): Promise<ActivityResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockActivityData
    }
    const response = await api.get<ActivityResponse>('/activity')
    return response.data
  },

  getLeaderboard: async (limit?: number): Promise<LeaderboardResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockLeaderboardData
    }
    const params = limit ? { limit } : {}
    const response = await api.get<LeaderboardResponse>('/leaderboard', { params })
    return response.data
  },

  getInsights: async (): Promise<InsightsResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockInsightsData
    }
    const response = await api.get<InsightsResponse>('/dashboard/insights')
    return response.data
  },

  getTimeAnalytics: async (period: 'week' | 'month' | 'all' = 'month'): Promise<TimeAnalyticsResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        ...mockTimeAnalyticsData,
        period,
        dailyBreakdown: generateMockDailyData(),
        weeklyTrend: generateMockWeeklyData()
      }
    }
    const response = await api.get<TimeAnalyticsResponse>('/analytics/time', { params: { period } })
    return response.data
  }
}
