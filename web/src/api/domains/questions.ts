/**
 * Questions API
 * Question-based assessment within sessions
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export type QuestionType = 'text' | 'multiple_choice'
export type QuestionResponseStatus = 'pending' | 'correct' | 'incorrect' | 'partial'

export interface QuestionOption {
  id: string
  text: string
}

export interface QuestionProgress {
  id: string
  type: QuestionType
  description: string
  points: number
  hint?: string // Legacy single hint (for backward compatibility)
  hintCount?: number // Number of progressive hints available
  dependsOn?: string[]
  order?: number
  options?: QuestionOption[]
  multiSelect?: boolean
  status: QuestionResponseStatus
  earnedPoints: number
  attemptCount: number
  isLocked: boolean
  hintAvailable: boolean
  hintLevelShown: number // Highest hint level revealed (0 = none)
  hintPenaltyApplied: number // Total penalty from hints
}

export interface QuestionsListResponse {
  sessionId: string
  questions: QuestionProgress[]
  totalPoints: number
  earnedPoints: number
}

export interface SubmitAnswerRequest {
  responseText?: string
  selectedOptions?: string[]
}

export interface SubmitAnswerResponse {
  questionId: string
  status: QuestionResponseStatus
  isCorrect: boolean
  earnedPoints: number
  maxPoints: number
  feedback?: string
  attemptCount: number
}

export interface GetHintResponse {
  questionId?: string
  checkpointId?: string
  level: number // Current hint level revealed
  maxLevel: number // Total hints available
  hint: string // The hint text for this level
  penalty: number // Points deducted for this hint
  totalPenalty: number // Total penalty applied so far
}

// ============================================================================
// Mock Data
// ============================================================================
// Mock progressive hints for questions
const mockHints: Record<string, { level: number; text: string; penalty: number }[]> = {
  'q1-file-owner': [
    { level: 1, text: 'Use the ls command with a flag to see file ownership', penalty: 0 },
    { level: 2, text: 'Try: ls -l /etc/passwd', penalty: 1 },
    { level: 3, text: 'The answer is "root"', penalty: 2 },
  ],
  'q2-permissions': [
    { level: 1, text: 'Use stat command or ls -l and convert', penalty: 0 },
    { level: 2, text: 'stat -c %a /etc/shadow', penalty: 2 },
  ],
  'q3-services': [
    { level: 1, text: 'Use systemctl or service commands to check', penalty: 0 },
    { level: 2, text: 'Try: systemctl list-units --type=service --state=running', penalty: 3 },
  ],
}

const mockQuestions: QuestionProgress[] = [
  {
    id: 'q1-file-owner',
    type: 'text',
    description: 'What user owns the file /etc/passwd?',
    points: 5,
    hint: 'Use the ls -l command to see file ownership',
    hintCount: 3,
    status: 'pending',
    earnedPoints: 0,
    attemptCount: 0,
    isLocked: false,
    hintAvailable: true,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
  },
  {
    id: 'q2-permissions',
    type: 'text',
    description: 'What are the octal permissions of /etc/shadow?',
    points: 10,
    hint: 'Use stat command or ls -l and convert',
    hintCount: 2,
    dependsOn: ['q1-file-owner'],
    status: 'pending',
    earnedPoints: 0,
    attemptCount: 0,
    isLocked: true,
    hintAvailable: true,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
  },
  {
    id: 'q3-services',
    type: 'multiple_choice',
    description: 'Which services are running on this system?',
    points: 15,
    multiSelect: true,
    options: [
      { id: 'A', text: 'Apache HTTP Server' },
      { id: 'B', text: 'MySQL Database' },
      { id: 'C', text: 'SSH Server' },
      { id: 'D', text: 'FTP Server' },
    ],
    hint: 'Use systemctl or service commands to check',
    hintCount: 2,
    dependsOn: ['q1-file-owner'],
    status: 'pending',
    earnedPoints: 0,
    attemptCount: 0,
    isLocked: true,
    hintAvailable: true,
    hintLevelShown: 0,
    hintPenaltyApplied: 0,
  },
]

// ============================================================================
// API
// ============================================================================
export const questionsApi = {
  getForSession: async (sessionId: string): Promise<QuestionsListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        sessionId,
        questions: mockQuestions,
        totalPoints: mockQuestions.reduce((sum, q) => sum + q.points, 0),
        earnedPoints: mockQuestions.reduce((sum, q) => sum + q.earnedPoints, 0),
      }
    }
    const response = await api.get<QuestionsListResponse>(`/sessions/${sessionId}/questions`)
    return response.data
  },

  submitAnswer: async (
    sessionId: string,
    questionId: string,
    answer: SubmitAnswerRequest
  ): Promise<SubmitAnswerResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const question = mockQuestions.find(q => q.id === questionId)
      // Simple mock validation - "root" is correct for q1, "640" for q2, ["A", "C"] for q3
      let isCorrect = false
      if (questionId === 'q1-file-owner' && answer.responseText?.toLowerCase() === 'root') {
        isCorrect = true
      } else if (questionId === 'q2-permissions' && ['640', '0640'].includes(answer.responseText || '')) {
        isCorrect = true
      } else if (questionId === 'q3-services') {
        const selected = answer.selectedOptions || []
        isCorrect = selected.length === 2 && selected.includes('A') && selected.includes('C')
      }

      // Update mock state
      if (question) {
        question.attemptCount++
        if (isCorrect) {
          question.status = 'correct'
          question.earnedPoints = question.points
          // Unlock dependent questions
          mockQuestions.forEach(q => {
            if (q.dependsOn?.includes(questionId)) {
              q.isLocked = false
            }
          })
        } else {
          question.status = 'incorrect'
        }
      }

      return {
        questionId,
        status: isCorrect ? 'correct' : 'incorrect',
        isCorrect,
        earnedPoints: isCorrect ? (question?.points || 0) : 0,
        maxPoints: question?.points || 0,
        feedback: isCorrect ? 'Correct!' : 'Incorrect answer. Please try again.',
        attemptCount: question?.attemptCount || 1,
      }
    }
    const response = await api.post<SubmitAnswerResponse>(
      `/sessions/${sessionId}/questions/${questionId}/answer`,
      answer
    )
    return response.data
  },

  /**
   * Gets a progressive hint for a question
   * @param sessionId Session ID
   * @param questionId Question ID
   * @param level Optional hint level to request (default: next level)
   */
  getHint: async (sessionId: string, questionId: string, level?: number): Promise<GetHintResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(200)
      const question = mockQuestions.find(q => q.id === questionId)
      const hints = mockHints[questionId] || []

      if (!question || hints.length === 0) {
        return {
          questionId,
          level: 0,
          maxLevel: 0,
          hint: 'No hint available',
          penalty: 0,
          totalPenalty: 0,
        }
      }

      // Determine which level to show (default: next level after current)
      const requestedLevel = level ?? (question.hintLevelShown + 1)
      const maxLevel = hints.length

      // If already shown this level, return without penalty
      if (requestedLevel <= question.hintLevelShown) {
        const hint = hints.find(h => h.level === requestedLevel) || hints[requestedLevel - 1]
        return {
          questionId,
          level: requestedLevel,
          maxLevel,
          hint: hint?.text || 'No hint available',
          penalty: 0,
          totalPenalty: question.hintPenaltyApplied,
        }
      }

      // Get new hint and apply penalty
      const hint = hints.find(h => h.level === requestedLevel) || hints[requestedLevel - 1]
      if (hint) {
        question.hintLevelShown = requestedLevel
        question.hintPenaltyApplied += hint.penalty
      }

      return {
        questionId,
        level: requestedLevel,
        maxLevel,
        hint: hint?.text || 'No more hints available',
        penalty: hint?.penalty || 0,
        totalPenalty: question.hintPenaltyApplied,
      }
    }
    const params = level ? `?level=${level}` : ''
    const response = await api.post<GetHintResponse>(
      `/sessions/${sessionId}/questions/${questionId}/hint${params}`
    )
    return response.data
  },
}
