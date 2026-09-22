import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'
import type { CheckpointUpdatePayload, SessionEventPayload, GradeUpdatePayload } from './useWebSocket'

// Mock useWebSocket
const mockWsConnect = vi.fn()
const mockWsDisconnect = vi.fn()
const mockWsConnected = ref(false)
const mockWsError = ref<string | null>(null)

let capturedCallbacks: {
  onAssessmentUpdate?: (update: any) => void
  onSessionUpdate?: (event: SessionEventPayload) => void
  onCheckpointUpdate?: (update: CheckpointUpdatePayload) => void
  onGradeUpdate?: (update: GradeUpdatePayload) => void
} = {}

vi.mock('./useWebSocket', () => ({
  useWebSocket: vi.fn((options: any) => {
    capturedCallbacks = {
      onAssessmentUpdate: options.onAssessmentUpdate,
      onSessionUpdate: options.onSessionUpdate,
      onCheckpointUpdate: options.onCheckpointUpdate,
      onGradeUpdate: options.onGradeUpdate,
    }
    return {
      connected: mockWsConnected,
      error: mockWsError,
      connect: mockWsConnect,
      disconnect: mockWsDisconnect,
    }
  }),
}))

// Mock useNotifications
const mockLabCheckpointPassed = vi.fn()
const mockLabCheckpointFailed = vi.fn()
const mockLabConnectionLost = vi.fn()
const mockLabConnectionRestored = vi.fn()

vi.mock('./useNotifications', () => ({
  useNotifications: vi.fn(() => ({
    lab: {
      checkpointPassed: mockLabCheckpointPassed,
      checkpointFailed: mockLabCheckpointFailed,
      connectionLost: mockLabConnectionLost,
      connectionRestored: mockLabConnectionRestored,
    },
  })),
}))

describe('useSessionWebSocket', () => {
  let mockAssessmentStore: any
  let mockSessionStore: any

  beforeEach(() => {
    vi.clearAllMocks()
    mockWsConnected.value = false
    mockWsError.value = null
    capturedCallbacks = {}

    mockAssessmentStore = {
      handleUpdate: vi.fn(),
    }

    mockSessionStore = {
      handleSessionUpdate: vi.fn(),
      handleCheckpointUpdate: vi.fn(),
      handleGradeUpdate: vi.fn(),
      fetchSession: vi.fn(),
    }
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  // Import dynamically to get fresh module with mocks
  async function getUseSessionWebSocket() {
    vi.resetModules()
    // Re-apply mocks after reset
    vi.doMock('./useWebSocket', () => ({
      useWebSocket: vi.fn((options: any) => {
        capturedCallbacks = {
          onAssessmentUpdate: options.onAssessmentUpdate,
          onSessionUpdate: options.onSessionUpdate,
          onCheckpointUpdate: options.onCheckpointUpdate,
          onGradeUpdate: options.onGradeUpdate,
        }
        return {
          connected: mockWsConnected,
          error: mockWsError,
          connect: mockWsConnect,
          disconnect: mockWsDisconnect,
        }
      }),
    }))
    vi.doMock('./useNotifications', () => ({
      useNotifications: vi.fn(() => ({
        lab: {
          checkpointPassed: mockLabCheckpointPassed,
          checkpointFailed: mockLabCheckpointFailed,
          connectionLost: mockLabConnectionLost,
          connectionRestored: mockLabConnectionRestored,
        },
      })),
    }))
    const mod = await import('./useSessionWebSocket')
    return mod.useSessionWebSocket
  }

  describe('WebSocket event handling', () => {
    it('should call assessmentStore.handleUpdate on assessment update', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      const update = { sessionId: 'session-123', status: 'completed', earnedPoints: 85, maxPoints: 100 }
      capturedCallbacks.onAssessmentUpdate?.(update)

      expect(mockAssessmentStore.handleUpdate).toHaveBeenCalledWith(update)
    })

    it('should call sessionStore.handleSessionUpdate on session update', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      const event: SessionEventPayload = {
        sessionId: 'session-123',
        podId: 'pod-1',
        status: 'active',
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onSessionUpdate?.(event)

      expect(mockSessionStore.handleSessionUpdate).toHaveBeenCalledWith(event)
    })

    it('should call sessionStore.handleCheckpointUpdate on checkpoint update', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-123',
        checkpointId: 'cp-1',
        name: 'Configure Router',
        status: 'passed',
        score: 25,
        maxScore: 25,
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onCheckpointUpdate?.(update)

      expect(mockSessionStore.handleCheckpointUpdate).toHaveBeenCalledWith(update)
    })

    it('should call sessionStore.handleGradeUpdate on grade update', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      const update: GradeUpdatePayload = {
        sessionId: 'session-123',
        userId: 'user-1',
        score: 85,
        maxScore: 100,
        percentage: 85,
        passed: true,
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onGradeUpdate?.(update)

      expect(mockSessionStore.handleGradeUpdate).toHaveBeenCalledWith(update)
    })
  })

  describe('checkpoint notifications', () => {
    it('should show checkpoint passed notification', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
        enableNotifications: true,
      })

      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-123',
        checkpointId: 'cp-1',
        name: 'Configure Router',
        status: 'passed',
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onCheckpointUpdate?.(update)

      expect(mockLabCheckpointPassed).toHaveBeenCalledWith('Configure Router')
    })

    it('should show checkpoint failed notification', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
        enableNotifications: true,
      })

      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-123',
        checkpointId: 'cp-1',
        name: 'Setup VLAN',
        status: 'failed',
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onCheckpointUpdate?.(update)

      expect(mockLabCheckpointFailed).toHaveBeenCalledWith('Setup VLAN')
    })

    it('should not show notification for pending status', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
        enableNotifications: true,
      })

      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-123',
        checkpointId: 'cp-1',
        name: 'Check Config',
        status: 'pending',
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onCheckpointUpdate?.(update)

      expect(mockLabCheckpointPassed).not.toHaveBeenCalled()
      expect(mockLabCheckpointFailed).not.toHaveBeenCalled()
    })

    it('should not show notifications when disabled', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
        enableNotifications: false,
      })

      const update: CheckpointUpdatePayload = {
        podId: 'pod-1',
        sessionId: 'session-123',
        checkpointId: 'cp-1',
        name: 'Configure Router',
        status: 'passed',
        timestamp: '2024-01-01T00:00:00Z',
      }
      capturedCallbacks.onCheckpointUpdate?.(update)

      expect(mockLabCheckpointPassed).not.toHaveBeenCalled()
    })
  })

  describe('initWebSocket()', () => {
    it('should disconnect existing WebSocket before creating new one', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      const { initWebSocket } = useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      initWebSocket('session-456')

      expect(mockWsDisconnect).toHaveBeenCalled()
    })
  })

  describe('disconnect()', () => {
    it('should disconnect WebSocket', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      const { disconnect } = useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      disconnect()

      expect(mockWsDisconnect).toHaveBeenCalled()
    })
  })

  describe('connection state', () => {
    it('should expose wsConnected state', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      const { wsConnected } = useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      expect(wsConnected.value).toBe(false)
    })

    it('should expose wsError state', async () => {
      const useSessionWebSocket = await getUseSessionWebSocket()
      const sessionIdRef = ref('session-123')

      const { wsError } = useSessionWebSocket(sessionIdRef, {
        assessmentStore: mockAssessmentStore,
        sessionStore: mockSessionStore,
      })

      expect(wsError.value).toBeNull()
    })
  })

  // See useSessionWebSocket.reconnect-toast.test.ts for the regression
  // tests that pin the Phase 1 fix for "connection restored" firing only
  // on real reconnects. Those tests live in their own file so they don't
  // inherit leaked Vue watchers from the dozen tests above (each call to
  // useSessionWebSocket in this file subscribes a watcher to the shared
  // module-level mockWsConnected ref that is never torn down).
})
