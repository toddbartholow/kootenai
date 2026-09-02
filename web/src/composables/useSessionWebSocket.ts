import { ref, watch, onUnmounted, type Ref, type WatchStopHandle } from 'vue'
import { useWebSocket } from './useWebSocket'
import { useNotifications } from './useNotifications'
import type { useAssessmentStore as UseAssessmentStore } from '@/stores/assessment'
import type { useSessionStore as UseSessionStore } from '@/stores/session'

export interface SessionWebSocketOptions {
  assessmentStore: ReturnType<typeof UseAssessmentStore>
  sessionStore: ReturnType<typeof UseSessionStore>
  enableNotifications?: boolean
}

export interface UseSessionWebSocketReturn {
  wsConnected: Ref<boolean>
  wsError: Ref<string | null>
  initWebSocket: (sessionId: string) => void
  disconnect: () => void
}

/**
 * Composable for managing WebSocket connections in session view
 */
export function useSessionWebSocket(
  sessionIdRef: Ref<string>,
  options: SessionWebSocketOptions
): UseSessionWebSocketReturn {
  const { assessmentStore, sessionStore, enableNotifications = true } = options

  const wsConnected = ref(false)
  const wsError = ref<string | null>(null)
  // `wasConnected` is tri-state:
  //   null   → initial mount, no connection has been attempted yet
  //   false  → we were disconnected after previously being connected
  //   true   → currently connected
  // The previous bug used `ref(false)` with a `!== undefined` check that
  // was a tautology, so "connection restored" toasts never fired on real
  // reconnects — they only fired on first mount.
  const wasConnected = ref<boolean | null>(null)

  // Initialize notifications if enabled
  const notify = enableNotifications ? useNotifications() : null

  let wsInstance: ReturnType<typeof useWebSocket> | null = null
  // Track watcher stop-handles for the current wsInstance. Without this,
  // a subsequent initWebSocket() would register a second pair of watchers
  // that race with the first pair when writing into wsConnected/wsError.
  const instanceWatchers: WatchStopHandle[] = []

  /**
   * Initialize WebSocket connection for a session
   */
  function initWebSocket(sessionId: string): void {
    // Tear down any existing connection + its watchers before starting a new
    // one. Calling disconnect() ensures watcher handles registered against
    // the old instance don't leak.
    disconnect()

    wsInstance = useWebSocket({
      sessionId,
      autoConnect: true,
      autoReconnect: true,
      onAssessmentUpdate: (update) => {
        assessmentStore.handleUpdate(update)
      },
      onSessionUpdate: (event) => {
        sessionStore.handleSessionUpdate(event)
      },
      onCheckpointUpdate: (update) => {
        sessionStore.handleCheckpointUpdate(update)
        // Show notification for checkpoint updates
        if (notify && update.name) {
          if (update.status === 'passed') {
            notify.lab.checkpointPassed(update.name)
          } else if (update.status === 'failed') {
            notify.lab.checkpointFailed(update.name)
          }
        }
      },
      onGradeUpdate: (update) => {
        sessionStore.handleGradeUpdate(update)
      },
      onHintNudge: (nudge) => {
        sessionStore.handleHintNudge(nudge)
      },
    })

    // Sync connection state and show notifications.
    instanceWatchers.push(
      watch(
        () => wsInstance?.connected.value,
        (connected) => {
          const isConnected = connected ?? false
          wsConnected.value = isConnected

          // "restored" fires only on a genuine reconnect (we were true, went
          // false, now true again); "lost" fires only on a real drop.
          // Initial connect (wasConnected === null) is silent.
          if (notify) {
            if (isConnected && wasConnected.value === false) {
              notify.lab.connectionRestored()
            } else if (!isConnected && wasConnected.value === true) {
              notify.lab.connectionLost()
            }
          }
          wasConnected.value = isConnected
        },
        { immediate: true }
      )
    )

    instanceWatchers.push(
      watch(
        () => wsInstance?.error.value,
        (error) => {
          wsError.value = error ?? null
        },
        { immediate: true }
      )
    )
  }

  /**
   * Disconnect the WebSocket and stop all watchers bound to the current
   * wsInstance. Safe to call multiple times.
   */
  function disconnect(): void {
    while (instanceWatchers.length > 0) {
      const stop = instanceWatchers.pop()
      stop?.()
    }
    if (wsInstance) {
      wsInstance.disconnect()
      wsInstance = null
    }
  }

  // Watch for sessionId changes and reinitialize WebSocket
  watch(
    sessionIdRef,
    (newSessionId) => {
      if (newSessionId) {
        initWebSocket(newSessionId)
      }
    },
    { immediate: true }
  )

  // Cleanup on unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    wsConnected,
    wsError,
    initWebSocket,
    disconnect,
  }
}
