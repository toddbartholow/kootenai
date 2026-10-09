/**
 * Phase 4 regression tests for the useWebSocket reconnect strategy and
 * visibilitychange handler.
 *
 * Coverage:
 *   - Reconnect delay follows an exponential-backoff ladder, not linear.
 *   - Reconnect delay includes jitter in the requested band.
 *   - Delay is clamped by reconnectMaxDelay.
 *   - Reconnects are deferred while the tab is hidden.
 *   - When the tab becomes visible again, a disconnected socket reconnects
 *     immediately (no backoff) and onVisibilityRestored fires exactly once
 *     per hide→show cycle.
 *
 * We drive the composable against a mock WebSocket constructor that lets
 * us force open/close events synchronously, and fake timers so the
 * setTimeout-based scheduler is deterministic.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import { useWebSocket } from './useWebSocket'

// Captured arguments from the most recent setTimeout call inside the
// composable. We intercept setTimeout rather than using fake timers so we
// can assert on the computed delay directly.
let lastDelay: number | null
let scheduledCb: (() => void) | null
const realSetTimeout = globalThis.setTimeout

// Mock WebSocket — tracks all instances created during a test so assertions
// can reach in and fire open/close events.
interface MockWS {
  readyState: number
  url: string
  onopen: ((ev: Event) => void) | null
  onclose: ((ev: CloseEvent) => void) | null
  onerror: ((ev: Event) => void) | null
  onmessage: ((ev: MessageEvent<string>) => void) | null
  close: (code?: number, reason?: string) => void
  send: (data: string) => void
  _fireOpen: () => void
  _fireClose: (opts?: { code?: number; reason?: string; wasClean?: boolean }) => void
}
let createdSockets: MockWS[]
const OriginalWebSocket = globalThis.WebSocket

function installMockWebSocket(): void {
  createdSockets = []
  const ctor = function MockWebSocket(url: string): MockWS {
    const sock: MockWS = {
      readyState: 0, // CONNECTING
      url,
      onopen: null,
      onclose: null,
      onerror: null,
      onmessage: null,
      close(_code?: number, _reason?: string) {
        this.readyState = 3
      },
      send(_data: string) {},
      _fireOpen() {
        this.readyState = 1
        this.onopen?.(new Event('open'))
      },
      _fireClose(opts?: { code?: number; reason?: string; wasClean?: boolean }) {
        this.readyState = 3
        const ev = {
          code: opts?.code ?? 1006,
          reason: opts?.reason ?? '',
          wasClean: opts?.wasClean ?? false,
        } as unknown as CloseEvent
        this.onclose?.(ev)
      },
    }
    createdSockets.push(sock)
    return sock
  } as unknown as typeof globalThis.WebSocket
  // The real WebSocket has static OPEN/CLOSED constants — copy enough of
  // the shape that `WebSocket.OPEN` comparisons inside the composable work.
  ;(ctor as unknown as { OPEN: number }).OPEN = 1
  ;(ctor as unknown as { CLOSED: number }).CLOSED = 3
  globalThis.WebSocket = ctor as unknown as typeof globalThis.WebSocket
}

function restoreWebSocket(): void {
  globalThis.WebSocket = OriginalWebSocket
}

// Patch setTimeout to capture the delay the composable requests. We also
// synthesize an invocation path: tests can opt-in to invoking the captured
// callback themselves via runScheduled().
function installTimeoutInterceptor(): void {
  lastDelay = null
  scheduledCb = null
  globalThis.setTimeout = ((fn: () => void, delay?: number) => {
    lastDelay = delay ?? 0
    scheduledCb = fn
    // Return a sentinel — clearTimeout below is a no-op for these.
    return 1 as unknown as ReturnType<typeof setTimeout>
  }) as unknown as typeof setTimeout
  globalThis.clearTimeout = (() => {}) as unknown as typeof clearTimeout
}

function restoreTimers(): void {
  globalThis.setTimeout = realSetTimeout
}

function runScheduled(): void {
  const cb = scheduledCb
  scheduledCb = null
  lastDelay = null
  cb?.()
}

let scope: EffectScope
// Track composable instances so we can explicitly disconnect them in
// afterEach — Vue effectScope.stop() doesn't fire onUnmounted outside a
// real component, so the composable's visibilitychange listener otherwise
// stays attached and leaks into later tests.
type ComposableApi = ReturnType<typeof useWebSocket>
const activeComposables: ComposableApi[] = []

function runComposable(opts: Parameters<typeof useWebSocket>[0]): ComposableApi {
  let api: ComposableApi
  scope.run(() => {
    api = useWebSocket(opts)
    activeComposables.push(api)
  })
  return api!
}

// Visibility stub — we flip `document.hidden` via defineProperty and fire
// the event by hand so tests don't depend on jsdom/happy-dom quirks.
function setHidden(hidden: boolean): void {
  Object.defineProperty(document, 'hidden', {
    configurable: true,
    get: () => hidden,
  })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('useWebSocket — Phase 4 resilience', () => {
  beforeEach(() => {
    installMockWebSocket()
    installTimeoutInterceptor()
    activeComposables.length = 0
    // Set hidden=false BEFORE any listeners exist for this test.
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => false })
    scope = effectScope()
  })

  afterEach(() => {
    // Disconnect all composables first so their listeners detach cleanly.
    for (const api of activeComposables) {
      api.disconnect()
    }
    activeComposables.length = 0
    scope.stop()
    restoreTimers()
    restoreWebSocket()
  })

  describe('exponential backoff with jitter', () => {
    it('uses 2^n × base for the ladder, not linear', () => {
      runComposable({
        sessionId: 's1',
        reconnectBaseDelay: 1000,
        reconnectMaxDelay: 60_000,
        reconnectJitter: 0, // zero jitter so the exponent math is exact
      })
      const sock = createdSockets[0]!

      // First close → attempt 1 → base × 2^0 = 1000
      sock._fireClose()
      expect(lastDelay).toBe(1000)
      runScheduled()

      // Second close → attempt 2 → base × 2^1 = 2000
      createdSockets[1]!._fireClose()
      expect(lastDelay).toBe(2000)
      runScheduled()

      // Third close → attempt 3 → base × 2^2 = 4000
      createdSockets[2]!._fireClose()
      expect(lastDelay).toBe(4000)
    })

    it('caps the delay at reconnectMaxDelay', () => {
      runComposable({
        sessionId: 's1',
        reconnectBaseDelay: 1000,
        reconnectMaxDelay: 3000, // very low cap
        reconnectJitter: 0,
      })

      // Drive 6 reconnects; 2^5 × 1000 = 32000, but cap is 3000.
      createdSockets[0]!._fireClose(); runScheduled()
      createdSockets[1]!._fireClose(); runScheduled()
      createdSockets[2]!._fireClose(); runScheduled()
      createdSockets[3]!._fireClose(); runScheduled()
      createdSockets[4]!._fireClose(); runScheduled()
      createdSockets[5]!._fireClose()

      expect(lastDelay).toBe(3000)
    })

    it('applies jitter within the configured band', () => {
      // Drive three close/reconnect rounds on ONE composable, pinning
      // Math.random to hit both extremes and the midpoint.
      runComposable({
        sessionId: 's1',
        reconnectBaseDelay: 1000,
        reconnectMaxDelay: 60_000,
        reconnectJitter: 0.3,
      })

      const origRandom = Math.random
      const samples: number[] = []
      for (const r of [0, 0.5, 1]) {
        Math.random = () => r
        const sock = createdSockets[createdSockets.length - 1]!
        sock._fireClose()
        samples.push(lastDelay as number)
        runScheduled()
      }
      Math.random = origRandom

      // Attempt 1: base=1000, jitter=0.3 → delay in [700, 1300]
      //   Math.random=0   → multiplier = -1 → 1000 - 300 = 700
      // Attempt 2: base=2000, jitter=0.3 → delay in [1400, 2600]
      //   Math.random=0.5 → multiplier =  0 → 2000
      // Attempt 3: base=4000, jitter=0.3 → delay in [2800, 5200]
      //   Math.random=1   → multiplier = +1 → 4000 + 1200 = 5200
      expect(samples[0]).toBe(700)
      expect(samples[1]).toBe(2000)
      expect(samples[2]).toBe(5200)
    })
  })

  describe('visibilitychange behavior', () => {
    it('skips scheduling a reconnect while the tab is hidden', () => {
      runComposable({ sessionId: 's1', reconnectBaseDelay: 1000, reconnectJitter: 0 })
      // Tab goes hidden BEFORE the disconnect.
      setHidden(true)

      createdSockets[0]!._fireClose()
      expect(lastDelay).toBeNull() // no timer scheduled
    })

    it('reconnects immediately on return from hidden (no backoff)', () => {
      runComposable({
        sessionId: 's1',
        reconnectBaseDelay: 5000,
        reconnectMaxDelay: 60_000,
        reconnectJitter: 0,
      })

      // Force a drop and let the backoff schedule (attempt 1 = 5000ms).
      createdSockets[0]!._fireClose()
      expect(lastDelay).toBe(5000)

      // Tab becomes hidden, then visible. Visibility handler should reset
      // attempt counter and call connect() directly — no new setTimeout.
      setHidden(true)
      const socketsBefore = createdSockets.length
      lastDelay = null
      setHidden(false)

      // A fresh socket was created immediately (not scheduled).
      expect(createdSockets.length).toBe(socketsBefore + 1)
      expect(lastDelay).toBeNull()
    })

    it('fires onVisibilityRestored exactly once per hide→show cycle', () => {
      const restored = vi.fn()
      runComposable({
        sessionId: 's1',
        reconnectBaseDelay: 1000,
        reconnectJitter: 0,
        onVisibilityRestored: restored,
      })
      createdSockets[0]!._fireOpen()

      setHidden(true)
      setHidden(false)
      expect(restored).toHaveBeenCalledTimes(1)

      // A second "become visible" without any prior hide should not
      // re-fire: wasHiddenSinceConnect was cleared after the first return.
      setHidden(false)
      expect(restored).toHaveBeenCalledTimes(1)

      // But a fresh hide→show cycle does.
      setHidden(true)
      setHidden(false)
      expect(restored).toHaveBeenCalledTimes(2)
    })

    it('does NOT reconnect after an intentional disconnect()', () => {
      const api = runComposable({ sessionId: 's1', reconnectBaseDelay: 1000 })
      // Remove this composable from the teardown list — we're calling
      // disconnect() ourselves as the subject-under-test.
      activeComposables.pop()
      api.disconnect()

      // Simulate a subsequent close event (e.g. socket's own teardown
      // firing onclose after .close()). Reconnect must NOT schedule.
      lastDelay = null
      createdSockets[0]!._fireClose({ wasClean: false })
      expect(lastDelay).toBeNull()
    })
  })
})
