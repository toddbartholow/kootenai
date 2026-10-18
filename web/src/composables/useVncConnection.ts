import { ref, computed } from 'vue'
import type RFB from '@novnc/novnc/lib/rfb.js'
import type { RFBDisconnectEvent, RFBSecurityFailureEvent } from '@novnc/novnc/lib/rfb.js'
import { loggers } from '@/utils/logger'

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

export interface VncConnectionOptions {
  /** Maximum auto-reconnect attempts on unexpected disconnect. 0 disables. */
  maxReconnectAttempts?: number
  /** Base delay in ms between reconnect attempts (exponential backoff). */
  reconnectDelay?: number
  /** Called when connection status changes. */
  onStatusChange?: (status: ConnectionStatus, detail?: string) => void
}

// Pre-load the noVNC RFB class once per application (module-level singleton)
let RFBClass: typeof RFB | null = null
let rfbLoadPromise: Promise<typeof RFB | null> | null = null

function preloadRFB(): Promise<typeof RFB | null> {
  if (RFBClass) return Promise.resolve(RFBClass)
  if (!rfbLoadPromise) {
    rfbLoadPromise = import('@novnc/novnc/lib/rfb.js')
      .then(m => {
        RFBClass = m.default
        loggers.console.debug('noVNC RFB class pre-loaded')
        return RFBClass
      })
      .catch(() => {
        loggers.console.warn('Failed to pre-load noVNC')
        rfbLoadPromise = null // allow retry
        return null
      })
  }
  return rfbLoadPromise
}

// Start pre-loading immediately on first import
preloadRFB()

/**
 * Build a WebSocket proxy URL for the VNC connection.
 * Routes through the backend proxy at /api/v1/proxmox/vms/{vmid}/vnc.
 */
export function buildWsProxyUrl(vmPlatformId: string | number, node: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return `${protocol}//${host}/api/v1/proxmox/vms/${vmPlatformId}/vnc?node=${node}`
}

/**
 * Composable for managing a single noVNC RFB connection.
 *
 * Each call to `useVncConnection()` creates an independent connection instance.
 * For multi-VM scenarios (InlineVncConsole), create one instance per VM and
 * manage them in a Map.
 */
export function useVncConnection(options: VncConnectionOptions = {}) {
  const { maxReconnectAttempts = 0, reconnectDelay = 1000, onStatusChange } = options

  const connectionStatus = ref<ConnectionStatus>('disconnected')
  const errorMessage = ref('')
  const isConnected = computed(() => connectionStatus.value === 'connected')

  let rfb: RFB | null = null
  let reconnectAttempts = 0
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  let autoReconnect = maxReconnectAttempts > 0

  function setStatus(status: ConnectionStatus, detail?: string) {
    connectionStatus.value = status
    if (detail !== undefined) errorMessage.value = detail
    onStatusChange?.(status, detail)
  }

  /**
   * Connect to a VNC console.
   *
   * @param container - The DOM element noVNC will render into
   * @param wsUrl - Full WebSocket URL (use `buildWsProxyUrl` to construct)
   * @param password - VNC ticket/password for authentication
   */
  async function connect(container: HTMLElement, wsUrl: string, password: string) {
    // Disconnect any existing connection first
    disconnectInternal(false)

    setStatus('connecting', '')

    try {
      const RFB = RFBClass ?? (await preloadRFB())

      if (!RFB) {
        setStatus('error', 'noVNC library not available')
        return null
      }

      const instance = new RFB(container, wsUrl, {
        credentials: { password },
      })

      instance.addEventListener('connect', () => {
        setStatus('connected', '')
        reconnectAttempts = 0
        autoReconnect = maxReconnectAttempts > 0
      })

      instance.addEventListener('disconnect', (e: RFBDisconnectEvent) => {
        if (e.detail.clean) {
          setStatus('disconnected')
        } else {
          setStatus('disconnected', 'Connection lost')
          attemptReconnect(container, wsUrl, password)
        }
      })

      instance.addEventListener('securityfailure', (e: RFBSecurityFailureEvent) => {
        setStatus('error', `Security error: ${e.detail.reason}`)
        autoReconnect = false // Don't retry auth failures
      })

      // Configure scaling
      instance.scaleViewport = true
      instance.resizeSession = true
      instance.clipViewport = false

      rfb = instance
      return instance
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to connect'
      setStatus('error', msg)
      loggers.console.error('VNC connection error', err)
      return null
    }
  }

  function disconnectInternal(updateStatus: boolean) {
    cancelReconnect()
    if (rfb) {
      rfb.disconnect()
      rfb = null
    }
    if (updateStatus) {
      setStatus('disconnected', '')
    }
  }

  function disconnect() {
    disconnectInternal(true)
  }

  function attemptReconnect(container: HTMLElement, wsUrl: string, password: string) {
    if (!autoReconnect || reconnectAttempts >= maxReconnectAttempts) {
      if (reconnectAttempts >= maxReconnectAttempts) {
        setStatus('error', `Max reconnect attempts reached (${maxReconnectAttempts})`)
      }
      return
    }

    reconnectAttempts++
    const delay = reconnectDelay * Math.pow(1.5, reconnectAttempts - 1)
    loggers.console.debug(
      `Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${maxReconnectAttempts})`,
    )

    reconnectTimeout = setTimeout(() => {
      connect(container, wsUrl, password)
    }, delay)
  }

  function cancelReconnect() {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
  }

  /** Get the current RFB instance (for keyboard composable or direct access). */
  function getRfb(): RFB | null {
    return rfb
  }

  /** Focus the RFB canvas (gives it keyboard input). */
  function focus() {
    rfb?.focus()
  }

  return {
    connectionStatus,
    errorMessage,
    isConnected,
    connect,
    disconnect,
    getRfb,
    focus,
  }
}

export type UseVncConnectionReturn = ReturnType<typeof useVncConnection>
