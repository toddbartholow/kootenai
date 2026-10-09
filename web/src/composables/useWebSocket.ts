import { ref, onUnmounted } from 'vue'
import type { AssessmentUpdate, PodStatus, PodVM } from '@/api'
import { loggers } from '@/utils'

const log = loggers.websocket

// Session event payload
export interface SessionEventPayload {
  sessionId: string
  podId: string
  status: 'started' | 'active' | 'paused' | 'ended'
  userId?: string
  timestamp: string
}

// Checkpoint update payload
export interface CheckpointUpdatePayload {
  podId: string
  sessionId: string
  checkpointId: string
  name: string
  status: 'passed' | 'failed' | 'pending'
  score?: number
  maxScore?: number
  timestamp: string
}

// Grade update payload
export interface GradeUpdatePayload {
  sessionId: string
  userId: string
  score: number
  maxScore: number
  percentage: number
  passed: boolean
  timestamp: string
}

// Pod status update payload. `status` is narrowed to the backend enum
// via the generated OpenAPI types; invalid values would be caught at
// compile time on the sender side.
export interface PodStatusPayload {
  podId: string
  status: PodStatus
  vms?: PodVM[]
  timestamp: string
}

// VM status update payload
export interface VMStatusPayload {
  podId: string
  vmName: string
  status: string
  ipAddress?: string
  currentSnapshot?: string
  timestamp: string
}

// Hint nudge payload (student stuck on checkpoint)
export interface HintNudgePayload {
  checkpointId: string
  checkpointName: string
  minutesStuck: number
  attemptCount: number
  nextHintLevel: number
}

// Monitoring event payload (for admin event dashboard)
export interface MonitoringEventPayload {
  id: number
  timestamp: string
  podId: string
  sessionId?: string
  vmName: string
  agentId?: string
  eventType: string
  ruleId?: string
  ruleLevel?: number
  description?: string
  data?: Record<string, unknown>
  processed: boolean
  matchedCheckpoints?: string[]
}

// Discriminated union for all WebSocket message types
export type WebSocketMessage =
  | { type: 'assessment' | 'assessment_update'; payload: AssessmentUpdate }
  | { type: 'session'; payload: SessionEventPayload }
  | { type: 'checkpoint'; payload: CheckpointUpdatePayload }
  | { type: 'grade'; payload: GradeUpdatePayload }
  | { type: 'pod_status'; payload: PodStatusPayload }
  | { type: 'vm_status'; payload: VMStatusPayload }
  | { type: 'hint_nudge'; payload: HintNudgePayload }
  | { type: 'monitoring_event'; payload: MonitoringEventPayload }

// Outbound message types
export type OutboundMessageType = 'subscribe' | 'unsubscribe' | 'ping'

export interface OutboundMessage {
  type: OutboundMessageType
  sessionId?: string
  podId?: string
}

export interface UseWebSocketOptions {
  sessionId?: string
  podId?: string
  onAssessmentUpdate?: (update: AssessmentUpdate) => void
  onSessionUpdate?: (event: SessionEventPayload) => void
  onCheckpointUpdate?: (event: CheckpointUpdatePayload) => void
  onGradeUpdate?: (event: GradeUpdatePayload) => void
  onPodStatusUpdate?: (event: PodStatusPayload) => void
  onVMStatusUpdate?: (event: VMStatusPayload) => void
  onHintNudge?: (event: HintNudgePayload) => void
  onMonitoringEvent?: (event: MonitoringEventPayload) => void
  /**
   * Called once per hide→show cycle when the tab becomes visible again.
   * Intended for consumers that need to refetch state the server may have
   * advanced while the tab was backgrounded (session status, checkpoints,
   * etc.). Fires before the socket has necessarily reconnected; treat it
   * as a "you may have missed messages" hint.
   */
  onVisibilityRestored?: () => void
  autoReconnect?: boolean
  /**
   * Legacy option kept for backward compatibility. New callers should use
   * reconnectBaseDelay / reconnectMaxDelay / reconnectJitter instead. When
   * only reconnectInterval is provided we seed reconnectBaseDelay from it.
   */
  reconnectInterval?: number
  /** Base delay for the exponential-backoff ladder, in ms. Default: 1000. */
  reconnectBaseDelay?: number
  /** Maximum reconnect delay, in ms. Default: 30000. */
  reconnectMaxDelay?: number
  /**
   * Jitter factor in [0, 1]. The actual delay is multiplied by a random
   * value in [1 - jitter, 1 + jitter], which prevents reconnect stampedes
   * when many clients disconnect from the same backend restart. Default: 0.3.
   */
  reconnectJitter?: number
  autoConnect?: boolean
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
  const connected = ref(false)
  const error = ref<string | null>(null)
  const reconnectAttempts = ref(0)

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  // True when the tab was hidden at least once during the current
  // connection's lifetime; used to decide whether to fire
  // onVisibilityRestored on the way back.
  let wasHiddenSinceConnect = false
  // True after disconnect() so we know not to resurrect the socket from
  // handlers that fire after explicit teardown (visibilitychange etc.).
  let intentionallyClosed = false

  const {
    sessionId,
    podId,
    onAssessmentUpdate,
    onSessionUpdate,
    onCheckpointUpdate,
    onGradeUpdate,
    onPodStatusUpdate,
    onVMStatusUpdate,
    onHintNudge,
    onMonitoringEvent,
    onVisibilityRestored,
    autoReconnect = true,
    reconnectInterval = 3000,
    reconnectBaseDelay,
    reconnectMaxDelay = 30000,
    reconnectJitter = 0.3,
    autoConnect = true,
  } = options
  // Backward-compat: when the caller uses ONLY the legacy reconnectInterval
  // option, keep the old linear ramp (attempt × interval) so existing
  // call sites and tests keep their timing. Callers who opt in to any of
  // the new options (reconnectBaseDelay/MaxDelay/Jitter) get the modern
  // exponential-with-jitter behavior.
  const useLegacyBackoff =
    reconnectBaseDelay === undefined &&
    options.reconnectMaxDelay === undefined &&
    options.reconnectJitter === undefined
  const baseDelay = reconnectBaseDelay ?? reconnectInterval

  function buildUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    let url = `${protocol}//${host}/api/v1/ws`

    const params = new URLSearchParams()
    if (sessionId) params.set('sessionId', sessionId)
    if (podId) params.set('podId', podId)

    const queryString = params.toString()
    if (queryString) url += `?${queryString}`

    return url
  }

  function connect() {
    if (ws?.readyState === WebSocket.OPEN) return
    // A previous disconnect() may have set intentionallyClosed; a fresh
    // connect() call is explicit consumer intent and should override.
    intentionallyClosed = false

    try {
      ws = new WebSocket(buildUrl())

      ws.onopen = () => {
        connected.value = true
        error.value = null
        reconnectAttempts.value = 0
        wasHiddenSinceConnect = false
        log.info('Connected')
      }

      ws.onclose = event => {
        connected.value = false
        log.info('Disconnected', { code: event.code, reason: event.reason })

        // Only attempt reconnect when:
        //   - auto-reconnect is enabled
        //   - the close was unclean (server dropped us, network blip, etc.)
        //   - the consumer hasn't explicitly called disconnect()
        if (autoReconnect && !event.wasClean && !intentionallyClosed) {
          scheduleReconnect()
        }
      }

      ws.onerror = () => {
        error.value = 'WebSocket connection error'
      }

      ws.onmessage = (event: MessageEvent<string>) => {
        const lines = event.data.split('\n')
        for (const line of lines) {
          if (!line.trim()) continue
          try {
            const message = JSON.parse(line) as WebSocketMessage
            handleMessage(message)
          } catch (e: unknown) {
            log.error('Failed to parse message', e)
          }
        }
      }
    } catch (e: unknown) {
      const connectError = e instanceof Error ? e.message : 'Failed to connect'
      error.value = connectError
      if (autoReconnect) scheduleReconnect()
    }
  }

  function handleMessage(message: WebSocketMessage) {
    switch (message.type) {
      case 'assessment':
      case 'assessment_update':
        if (onAssessmentUpdate) {
          onAssessmentUpdate(message.payload)
        }
        break
      case 'session':
        if (onSessionUpdate) {
          onSessionUpdate(message.payload)
        }
        break
      case 'checkpoint':
        if (onCheckpointUpdate) {
          onCheckpointUpdate(message.payload)
        }
        break
      case 'grade':
        if (onGradeUpdate) {
          onGradeUpdate(message.payload)
        }
        break
      case 'pod_status':
        if (onPodStatusUpdate) {
          onPodStatusUpdate(message.payload)
        }
        break
      case 'vm_status':
        if (onVMStatusUpdate) {
          onVMStatusUpdate(message.payload)
        }
        break
      case 'hint_nudge':
        if (onHintNudge) {
          onHintNudge(message.payload)
        }
        break
      case 'monitoring_event':
        if (onMonitoringEvent) {
          onMonitoringEvent(message.payload)
        }
        break
      default: {
        // Exhaustive check - TypeScript will error if a new message type is added but not handled
        const _exhaustiveCheck: never = message
        log.warn('Unhandled message type', { type: (_exhaustiveCheck as { type: string }).type })
      }
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer) clearTimeout(reconnectTimer)

    // Don't burn battery reconnecting a backgrounded tab. The visibilitychange
    // handler will re-issue connect() immediately on return.
    if (typeof document !== 'undefined' && document.hidden) {
      log.info('Skipping reconnect while tab is hidden')
      return
    }

    reconnectAttempts.value++

    let delay: number
    if (useLegacyBackoff) {
      // Linear ramp, the way this composable always worked before Phase 4.
      delay = Math.min(reconnectInterval * reconnectAttempts.value, 30000)
    } else {
      // Exponential backoff with jitter:
      //   attempt 1 → baseDelay * 2^0 = baseDelay
      //   attempt 2 → baseDelay * 2^1
      //   attempt N → baseDelay * 2^(N-1), capped at reconnectMaxDelay
      // The exponent is itself capped at 10 (1024×) so Math.pow doesn't
      // blow up for callers that leave autoReconnect on forever.
      const exponent = Math.min(reconnectAttempts.value - 1, 10)
      const raw = Math.min(baseDelay * Math.pow(2, exponent), reconnectMaxDelay)
      const spread = raw * Math.max(0, Math.min(1, reconnectJitter))
      // Delay drawn uniformly from [raw - spread, raw + spread].
      delay = Math.max(0, raw + (Math.random() * 2 - 1) * spread)
    }

    log.info(`Reconnecting in ${Math.round(delay)}ms`, {
      attempt: reconnectAttempts.value,
      mode: useLegacyBackoff ? 'legacy-linear' : 'exponential-jitter',
    })

    reconnectTimer = setTimeout(() => {
      // Re-check visibility at fire time — the tab may have gone hidden
      // between scheduling and firing.
      if (typeof document !== 'undefined' && document.hidden) {
        log.info('Reconnect fired while tab hidden; deferring')
        return
      }
      connect()
    }, delay)
  }

  /**
   * visibilitychange handler. Strategy:
   *   - Going hidden: mark wasHiddenSinceConnect so we know the next
   *     visibility return deserves an onVisibilityRestored callback.
   *     Don't tear anything down — established sockets are fine to keep
   *     open while backgrounded; browsers throttle them automatically.
   *   - Becoming visible: if we're disconnected, cancel any long-backoff
   *     timer and reconnect immediately with a reset attempt counter.
   *     Fire onVisibilityRestored so the consumer can refetch state.
   */
  function onVisibilityChange() {
    if (typeof document === 'undefined') return
    if (document.hidden) {
      wasHiddenSinceConnect = true
      return
    }

    // Tab just became visible.
    const shouldRefetch = wasHiddenSinceConnect
    if (!connected.value && !intentionallyClosed && autoReconnect) {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      reconnectAttempts.value = 0
      log.info('Tab visible — reconnecting immediately')
      connect()
    }
    if (shouldRefetch) {
      wasHiddenSinceConnect = false
      onVisibilityRestored?.()
    }
  }
  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibilityChange)
  }

  function disconnect() {
    // Mark first so ws.onclose doesn't trigger auto-reconnect.
    intentionallyClosed = true

    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    if (ws) {
      ws.close(1000, 'Client disconnecting')
      ws = null
    }

    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }

    connected.value = false
  }

  function send(message: OutboundMessage) {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message))
    } else {
      log.warn('Cannot send - not connected')
    }
  }

  // Auto-connect on mount if enabled
  if (autoConnect) {
    connect()
  }

  // Cleanup on unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    connected,
    error,
    reconnectAttempts,
    connect,
    disconnect,
    send,
  }
}
