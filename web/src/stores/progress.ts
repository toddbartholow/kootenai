import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  dashboardApi,
  type DashboardResponse,
  type ActivityResponse,
  type LeaderboardResponse,
  type TimeAnalyticsResponse,
} from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'
import type { Achievement } from '@/types/progress'
import type { LeaderboardEntry } from '@/types/progress'

export const useProgressStore = defineStore('progress', () => {
  // Dashboard state
  const dashboard = ref<DashboardResponse | null>(null)
  const activity = ref<ActivityResponse | null>(null)
  const leaderboard = ref<LeaderboardResponse | null>(null)
  const timeAnalytics = ref<TimeAnalyticsResponse | null>(null)

  // Loading states
  const dashboardLoading = ref(false)
  const activityLoading = ref(false)
  const leaderboardLoading = ref(false)
  const timeAnalyticsLoading = ref(false)

  // Error states
  const dashboardError = ref<ErrorState | null>(null)
  const activityError = ref<ErrorState | null>(null)
  const leaderboardError = ref<ErrorState | null>(null)
  const timeAnalyticsError = ref<ErrorState | null>(null)

  // Computed properties for user stats.
  //
  // maxPoints / totalLabs are typed `number | null` on purpose: the
  // /dashboard API does not return a platform-wide points ceiling or lab
  // count. Older versions of this store fabricated them (maxPoints =
  // totalPoints, totalLabs = labsCompleted), which made every dashboard
  // display as "100% complete". Callers must guard nulls and render
  // "N/A" (or hide the "of N" denominator) when unknown. See the audit
  // remediation plan 5E for the rationale.
  const userStats = computed(() => {
    if (!dashboard.value) {
      return {
        rank: 0,
        totalRanked: 0,
        totalPoints: 0,
        maxPoints: null as number | null,
        achievementsUnlocked: 0,
        totalAchievements: 0,
        coursesCompleted: 0,
        totalCourses: 0,
        labsCompleted: 0,
        totalLabs: null as number | null,
        currentStreak: 0,
        longestStreak: 0,
      }
    }

    const d = dashboard.value
    return {
      rank: leaderboard.value?.currentUser?.rank ?? 0,
      totalRanked: leaderboard.value?.totalUsers ?? 0,
      totalPoints: d.user.totalPoints,
      maxPoints: null as number | null,
      achievementsUnlocked: d.achievements.totalEarned,
      totalAchievements: d.achievements.totalAvailable,
      coursesCompleted: d.stats.pathwaysCompleted,
      totalCourses: d.stats.pathwaysCompleted + d.stats.pathwaysInProgress,
      labsCompleted: d.stats.totalLabsCompleted,
      totalLabs: null as number | null,
      currentStreak: d.stats.currentStreak,
      longestStreak: d.stats.bestStreak,
    }
  })

  const enrolledCourses = computed(() => {
    if (!dashboard.value) return []
    // DashboardPathway exposes module counts and percentage, but not raw
    // points. The previous per-module "100 pts" estimate was fabricated;
    // return null so the UI can omit/N/A the points column rather than
    // display a made-up number. Module counts ARE real — keep those.
    return dashboard.value.enrolledPathways.map(p => ({
      id: p.id,
      name: p.name,
      icon: getCourseIcon(p.name),
      totalLabs: p.totalModules,
      completedLabs: p.completedModules,
      totalPoints: null as number | null,
      earnedPoints: null as number | null,
      percentage: p.percentage,
    }))
  })

  const recentAchievements = computed<Achievement[]>(() => {
    if (!dashboard.value) return []
    return dashboard.value.achievements.recentAchievements.map(a => ({
      id: a.id,
      name: a.name,
      description: a.description,
      icon: getAchievementIcon(a.tier),
      points: a.points,
      category: mapTierToCategory(a.tier) as 'progression' | 'skill' | 'special',
      unlocked: true,
      unlockedAt: a.earnedAt,
    }))
  })

  const leaderboardEntries = computed<LeaderboardEntry[]>(() => {
    if (!leaderboard.value) return []
    return leaderboard.value.entries.map(e => ({
      rank: e.rank,
      userId: e.userId,
      displayName: e.displayName,
      points: e.totalPoints,
      achievementCount: e.achievementCount,
      labsCompleted: e.labsCompleted,
      isCurrentUser: e.isCurrentUser ?? false,
    }))
  })

  const platformStats = computed(() => {
    return {
      totalUsers: leaderboard.value?.totalUsers ?? 0,
      activeLabs: dashboard.value?.recentSessions.length ?? 0,
      totalCompletions: dashboard.value?.stats.totalLabsCompleted ?? 0,
      averageScore: dashboard.value?.stats.averageScore ?? 0,
    }
  })

  // Actions
  async function fetchDashboard(): Promise<void> {
    dashboardLoading.value = true
    dashboardError.value = null
    try {
      dashboard.value = await dashboardApi.get()
    } catch (e: unknown) {
      dashboardError.value = createErrorState(e, 'Failed to fetch dashboard')
    } finally {
      dashboardLoading.value = false
    }
  }

  async function fetchActivity(): Promise<void> {
    activityLoading.value = true
    activityError.value = null
    try {
      activity.value = await dashboardApi.getActivity()
    } catch (e: unknown) {
      activityError.value = createErrorState(e, 'Failed to fetch activity')
    } finally {
      activityLoading.value = false
    }
  }

  async function fetchLeaderboard(limit?: number): Promise<void> {
    leaderboardLoading.value = true
    leaderboardError.value = null
    try {
      leaderboard.value = await dashboardApi.getLeaderboard(limit)
    } catch (e: unknown) {
      leaderboardError.value = createErrorState(e, 'Failed to fetch leaderboard')
    } finally {
      leaderboardLoading.value = false
    }
  }

  async function fetchTimeAnalytics(period: 'week' | 'month' | 'all' = 'month'): Promise<void> {
    timeAnalyticsLoading.value = true
    timeAnalyticsError.value = null
    try {
      timeAnalytics.value = await dashboardApi.getTimeAnalytics(period)
    } catch (e: unknown) {
      timeAnalyticsError.value = createErrorState(e, 'Failed to fetch time analytics')
    } finally {
      timeAnalyticsLoading.value = false
    }
  }

  async function fetchAll(): Promise<void> {
    await Promise.all([fetchDashboard(), fetchActivity(), fetchLeaderboard(), fetchTimeAnalytics()])
  }

  function reset(): void {
    dashboard.value = null
    activity.value = null
    leaderboard.value = null
    timeAnalytics.value = null
    dashboardLoading.value = false
    activityLoading.value = false
    leaderboardLoading.value = false
    timeAnalyticsLoading.value = false
    dashboardError.value = null
    activityError.value = null
    leaderboardError.value = null
    timeAnalyticsError.value = null
  }

  return {
    // State
    dashboard,
    activity,
    leaderboard,
    timeAnalytics,
    dashboardLoading,
    activityLoading,
    leaderboardLoading,
    timeAnalyticsLoading,
    dashboardError,
    activityError,
    leaderboardError,
    timeAnalyticsError,
    // Computed
    userStats,
    enrolledCourses,
    recentAchievements,
    leaderboardEntries,
    platformStats,
    // Actions
    fetchDashboard,
    fetchActivity,
    fetchLeaderboard,
    fetchTimeAnalytics,
    fetchAll,
    reset,
  }
})

// Helper functions
function getCourseIcon(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes('network') || lower.includes('security')) return '🛡️'
  if (lower.includes('incident')) return '🚨'
  if (lower.includes('cloud')) return '☁️'
  if (lower.includes('forensic')) return '🔍'
  if (lower.includes('malware')) return '🦠'
  return '📚'
}

function getAchievementIcon(tier: string): string {
  switch (tier.toLowerCase()) {
    case 'legendary':
      return 'crown'
    case 'epic':
      return 'star'
    case 'gold':
      return 'fire'
    case 'silver':
      return 'shield'
    case 'bronze':
      return 'lightning'
    default:
      return 'trophy'
  }
}

function mapTierToCategory(tier: string): string {
  switch (tier.toLowerCase()) {
    case 'legendary':
    case 'epic':
      return 'special'
    case 'gold':
    case 'silver':
      return 'skill'
    case 'bronze':
      return 'progression'
    default:
      return 'general'
  }
}
