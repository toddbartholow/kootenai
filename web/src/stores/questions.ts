import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  questionsApi,
  type QuestionProgress,
  type SubmitAnswerResponse,
  type QuestionResponseStatus,
  type GetHintResponse,
} from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'

export const useQuestionsStore = defineStore('questions', () => {
  // Questions state
  const questions = ref<QuestionProgress[]>([])
  const loading = ref(false)
  const error = ref<ErrorState | null>(null)
  const submitting = ref<string | null>(null) // questionId being submitted

  // Computed properties
  const totalPoints = computed(() =>
    questions.value.reduce((sum, q) => sum + q.points, 0)
  )

  const earnedPoints = computed(() =>
    questions.value.reduce((sum, q) => sum + q.earnedPoints, 0)
  )

  const percentage = computed(() => {
    if (totalPoints.value === 0) return 0
    return Math.round((earnedPoints.value / totalPoints.value) * 100)
  })

  const answeredCount = computed(() =>
    questions.value.filter(q => q.status === 'correct').length
  )

  const totalCount = computed(() => questions.value.length)

  const hasQuestions = computed(() => questions.value.length > 0)

  /**
   * Fetches questions for a session
   */
  async function fetchQuestions(sessionId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const response = await questionsApi.getForSession(sessionId)
      questions.value = response.questions
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to load questions')
    } finally {
      loading.value = false
    }
  }

  /**
   * Submits an answer for a question
   */
  async function submitAnswer(
    sessionId: string,
    questionId: string,
    answer: { responseText?: string; selectedOptions?: string[] }
  ): Promise<SubmitAnswerResponse | null> {
    submitting.value = questionId
    error.value = null
    try {
      const result = await questionsApi.submitAnswer(sessionId, questionId, answer)

      // Update local state
      const idx = questions.value.findIndex(q => q.id === questionId)
      const question = questions.value[idx]
      if (idx >= 0 && question) {
        question.status = result.status as QuestionResponseStatus
        question.earnedPoints = result.earnedPoints
        question.attemptCount = result.attemptCount

        // If correct, unlock dependent questions
        if (result.isCorrect) {
          questions.value.forEach(q => {
            if (q.dependsOn?.includes(questionId)) {
              q.isLocked = false
            }
          })
        }
      }

      return result
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to submit answer')
      return null
    } finally {
      submitting.value = null
    }
  }

  /**
   * Gets a progressive hint for a question
   * @param sessionId Session ID
   * @param questionId Question ID
   * @param level Optional hint level to request (default: next level)
   * @returns GetHintResponse with hint text and penalty info, or null on error
   */
  async function showHint(
    sessionId: string,
    questionId: string,
    level?: number
  ): Promise<GetHintResponse | null> {
    try {
      const response = await questionsApi.getHint(sessionId, questionId, level)

      // Update local state
      const idx = questions.value.findIndex(q => q.id === questionId)
      const question = questions.value[idx]
      if (idx >= 0 && question) {
        // Update hint level shown (only if higher than current)
        if (response.level > question.hintLevelShown) {
          question.hintLevelShown = response.level
        }
        // Update penalty applied
        question.hintPenaltyApplied = response.totalPenalty
        // Mark hint as available if there are more levels
        question.hintAvailable = response.level < response.maxLevel
      }

      return response
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to get hint')
      return null
    }
  }

  /**
   * Gets a specific hint level that was already revealed (no penalty)
   * @param sessionId Session ID
   * @param questionId Question ID
   * @param level The hint level to retrieve
   * @returns GetHintResponse with hint text, or null on error
   */
  async function getRevealedHint(
    sessionId: string,
    questionId: string,
    level: number
  ): Promise<GetHintResponse | null> {
    const question = questions.value.find(q => q.id === questionId)
    if (!question || level > question.hintLevelShown) {
      return null // Can't retrieve hints that haven't been revealed yet
    }
    return showHint(sessionId, questionId, level)
  }

  /**
   * Clears the hint shown error
   */
  function clearError(): void {
    error.value = null
  }

  /**
   * Resets the store state
   */
  function reset(): void {
    questions.value = []
    loading.value = false
    error.value = null
    submitting.value = null
  }

  return {
    // State
    questions,
    loading,
    error,
    submitting,
    // Computed
    totalPoints,
    earnedPoints,
    percentage,
    answeredCount,
    totalCount,
    hasQuestions,
    // Actions
    fetchQuestions,
    submitAnswer,
    showHint,
    getRevealedHint,
    clearError,
    reset,
  }
})
