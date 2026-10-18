/**
 * Shared types for progress tracking, achievements, and leaderboard
 * Used by both store and component layers
 */

export interface Achievement {
  id: string
  name: string
  description: string
  icon: string
  points: number
  category: 'progression' | 'skill' | 'special'
  unlocked: boolean
  unlockedAt?: string
  progress?: number
  progressMax?: number
}

export interface LeaderboardEntry {
  rank: number
  userId: string
  displayName: string
  avatar?: string
  points: number
  achievementCount: number
  labsCompleted: number
  isCurrentUser?: boolean
}
