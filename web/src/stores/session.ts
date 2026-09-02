import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { sessionsApi, type Session, type SubmissionResponse, type UserAchievement, type ModuleCompletionInfo, type CheckpointHintResponse } from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'
import type {
  CheckpointUpdatePayload,
  GradeUpdatePayload,
  SessionEventPayload,
  HintNudgePayload,
} from '@/composables/useWebSocket'

export interface CheckpointProgress {
  id: string
  name: string
  status: 'pending' | 'passed' | 'failed'
  score: number
  maxScore: number
  timestamp?: string
  // Progressive hint fields
  hintCount?: number  // Number of hints available
  hintLevelShown?: number  // Current hint level revealed (0 = none)
  hintPenaltyApplied?: number  // Total penalty from hints
}

export type SubmissionStatus = 'idle' | 'submitting' | 'submitted' | 'error'

export const useSessionStore = defineStore('session', () => {
  // Current session state
  const currentSession = ref<Session | null>(null)
  const checkpoints = ref<CheckpointProgress[]>([])
  const loading = ref(false)
  const error = ref<ErrorState | null>(null)

  // Real-time status
  const sessionStatus = ref<'started' | 'active' | 'paused' | 'ended'>('started')
  const lastUpdate = ref<Date | null>(null)

  // Submission state
  const submissionStatus = ref<SubmissionStatus>('idle')
  const submissionResult = ref<SubmissionResponse | null>(null)
  const submissionError = ref<string | null>(null)
  const newAchievements = ref<UserAchievement[]>([])
  const moduleCompletion = ref<ModuleCompletionInfo | null>(null)

  // Computed properties
  const earnedPoints = computed(() => currentSession.value?.earnedPoints ?? 0)
  const maxPoints = computed(() => currentSession.value?.maxPoints ?? 0)
  const percentage = computed(() => {
    if (maxPoints.value === 0) return 0
    return Math.round((earnedPoints.value / maxPoints.value) * 100)
  })
  const passedCheckpoints = computed(() =>
    checkpoints.value.filter(c => c.status === 'passed').length
  )
  const totalCheckpoints = computed(() => checkpoints.value.length)

  /**
   * Fetches session details from the API
   */
  async function fetchSession(sessionId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      currentSession.value = await sessionsApi.get(sessionId)
      // Also fetch progress for checkpoints
      const progress = await sessionsApi.getProgress(sessionId)
      if (progress.checkpoints && Array.isArray(progress.checkpoints)) {
        checkpoints.value = progress.checkpoints as CheckpointProgress[]
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch session')
    } finally {
      loading.value = false
    }
  }

  /**
   * Ends the current session
   */
  async function endSession(sessionId: string): Promise<{ earnedPoints: number; passed: boolean } | null> {
    loading.value = true
    error.value = null
    try {
      const result = await sessionsApi.end(sessionId)
      sessionStatus.value = 'ended'
      if (currentSession.value) {
        currentSession.value.earnedPoints = result.earnedPoints
        currentSession.value.passed = result.passed
      }
      return result
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to end session')
      return null
    } finally {
      loading.value = false
    }
  }

  /**
   * Handles session status updates from WebSocket
   */
  function handleSessionUpdate(event: SessionEventPayload): void {
    sessionStatus.value = event.status
    lastUpdate.value = new Date(event.timestamp)

    if (currentSession.value && event.sessionId === currentSession.value.id) {
      currentSession.value.status = event.status
    }
  }

  /**
   * Handles checkpoint updates from WebSocket
   */
  function handleCheckpointUpdate(update: CheckpointUpdatePayload): void {
    lastUpdate.value = new Date(update.timestamp)

    // Find and update existing checkpoint, or add new one
    const existingIndex = checkpoints.value.findIndex(c => c.id === update.checkpointId)

    const checkpointData: CheckpointProgress = {
      id: update.checkpointId,
      name: update.name,
      status: update.status,
      score: update.score ?? 0,
      maxScore: update.maxScore ?? 0,
      timestamp: update.timestamp,
    }

    if (existingIndex >= 0) {
      checkpoints.value[existingIndex] = checkpointData
    } else {
      checkpoints.value.push(checkpointData)
    }

    // Update session earned points
    if (currentSession.value) {
      const totalEarned = checkpoints.value
        .filter(c => c.status === 'passed')
        .reduce((sum, c) => sum + c.score, 0)
      currentSession.value.earnedPoints = totalEarned
    }
  }

  /**
   * Handles grade updates from WebSocket
   */
  function handleGradeUpdate(update: GradeUpdatePayload): void {
    lastUpdate.value = new Date(update.timestamp)

    if (currentSession.value && update.sessionId === currentSession.value.id) {
      currentSession.value.earnedPoints = update.score
      currentSession.value.maxPoints = update.maxScore
      currentSession.value.percentage = update.percentage
      currentSession.value.passed = update.passed
    }
  }

  /**
   * Submits the lab session for grading
   */
  async function submitSession(sessionId: string): Promise<SubmissionResponse | null> {
    submissionStatus.value = 'submitting'
    submissionError.value = null
    try {
      const result = await sessionsApi.submit(sessionId)
      submissionResult.value = result
      submissionStatus.value = 'submitted'

      // Update session with final results
      if (currentSession.value) {
        currentSession.value.earnedPoints = result.earnedPoints
        currentSession.value.maxPoints = result.maxPoints
        currentSession.value.percentage = result.percentage
        currentSession.value.passed = result.passed
        currentSession.value.status = 'completed'
      }

      // Store any new achievements earned
      if (result.achievements && result.achievements.length > 0) {
        newAchievements.value = result.achievements
      }

      // Store module completion info if present
      if (result.moduleCompletion) {
        moduleCompletion.value = result.moduleCompletion
      }

      sessionStatus.value = 'ended'
      return result
    } catch (e: unknown) {
      submissionError.value = e instanceof Error ? e.message : 'Failed to submit session'
      submissionStatus.value = 'error'
      return null
    }
  }

  /**
   * Clears new achievements after they've been shown
   */
  function clearNewAchievements(): void {
    newAchievements.value = []
  }

  /**
   * Clears module completion info after it's been shown
   */
  function clearModuleCompletion(): void {
    moduleCompletion.value = null
  }

  // Hint nudge state (smart guided hints)
  const activeNudge = ref<HintNudgePayload | null>(null)
  const dismissedNudges = ref<Set<string>>(new Set())

  /**
   * Handles hint nudge from WebSocket (student stuck detection)
   */
  function handleHintNudge(nudge: HintNudgePayload): void {
    // Don't show if already dismissed for this checkpoint
    if (dismissedNudges.value.has(nudge.checkpointId)) return
    // Don't show if checkpoint already passed
    const cp = checkpoints.value.find(c => c.id === nudge.checkpointId)
    if (cp?.status === 'passed') return
    activeNudge.value = nudge
  }

  /**
   * Dismisses the current nudge
   */
  function dismissNudge(): void {
    if (activeNudge.value) {
      dismissedNudges.value.add(activeNudge.value.checkpointId)
      activeNudge.value = null
    }
  }

  // Track revealed hints for checkpoints
  const revealedCheckpointHints = ref<Record<string, { level: number; text: string }[]>>({})

  /**
   * Gets a progressive hint for a checkpoint
   * @param checkpointId Checkpoint ID
   * @param level Optional hint level to request (default: next level)
   */
  async function showCheckpointHint(
    checkpointId: string,
    level?: number
  ): Promise<CheckpointHintResponse | null> {
    if (!currentSession.value) return null

    try {
      const response = await sessionsApi.getCheckpointHint(
        currentSession.value.id,
        checkpointId,
        level
      )

      // Initialize array if needed
      if (!revealedCheckpointHints.value[checkpointId]) {
        revealedCheckpointHints.value[checkpointId] = []
      }

      // Add hint if not already present
      const existing = revealedCheckpointHints.value[checkpointId].find(
        h => h.level === response.level
      )
      if (!existing) {
        revealedCheckpointHints.value[checkpointId].push({
          level: response.level,
          text: response.hint,
        })
        // Sort by level
        revealedCheckpointHints.value[checkpointId].sort((a, b) => a.level - b.level)
      }

      // Update checkpoint progress
      const cpIndex = checkpoints.value.findIndex(c => c.id === checkpointId)
      if (cpIndex >= 0 && checkpoints.value[cpIndex]) {
        checkpoints.value[cpIndex].hintLevelShown = response.level
        checkpoints.value[cpIndex].hintPenaltyApplied = response.totalPenalty
      }

      return response
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to get hint')
      return null
    }
  }

  /**
   * Check if more hints are available for a checkpoint
   */
  function hasMoreCheckpointHints(checkpointId: string): boolean {
    const cp = checkpoints.value.find(c => c.id === checkpointId)
    if (!cp || !cp.hintCount) return false
    return (cp.hintLevelShown ?? 0) < cp.hintCount
  }

  /**
   * Get the next hint level for a checkpoint
   */
  function getNextCheckpointHintLevel(checkpointId: string): number {
    const cp = checkpoints.value.find(c => c.id === checkpointId)
    return (cp?.hintLevelShown ?? 0) + 1
  }

  /**
   * Resets the store state
   */
  function reset(): void {
    currentSession.value = null
    checkpoints.value = []
    loading.value = false
    error.value = null
    sessionStatus.value = 'started'
    lastUpdate.value = null
    submissionStatus.value = 'idle'
    submissionResult.value = null
    submissionError.value = null
    newAchievements.value = []
    moduleCompletion.value = null
    revealedCheckpointHints.value = {}
    activeNudge.value = null
    dismissedNudges.value = new Set()
  }

  /**
   * Clears the top-level `error` ref. Prefer this over `sessionStore.error = null`
   * from templates so that future changes to the error representation
   * (e.g. moving to a computed projection) don't silently break callers.
   */
  function clearError(): void {
    error.value = null
  }

  return {
    // State
    currentSession,
    checkpoints,
    loading,
    error,
    sessionStatus,
    lastUpdate,
    submissionStatus,
    submissionResult,
    submissionError,
    newAchievements,
    moduleCompletion,
    revealedCheckpointHints,
    activeNudge,
    dismissedNudges,
    // Computed
    earnedPoints,
    maxPoints,
    percentage,
    passedCheckpoints,
    totalCheckpoints,
    // Actions
    fetchSession,
    endSession,
    submitSession,
    clearNewAchievements,
    clearModuleCompletion,
    handleSessionUpdate,
    handleCheckpointUpdate,
    handleGradeUpdate,
    showCheckpointHint,
    hasMoreCheckpointHints,
    getNextCheckpointHintLevel,
    handleHintNudge,
    dismissNudge,
    clearError,
    reset,
  }
})
