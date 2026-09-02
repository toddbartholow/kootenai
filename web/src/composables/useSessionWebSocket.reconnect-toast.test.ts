/**
 * Regression tests for the Phase 1 fix to the reconnect-toast bug in
 * useSessionWebSocket.
 *
 * Before the fix, `wasConnected` was initialized as `ref(false)` and the
 * "restored" branch guarded on `wasConnected.value !== undefined` — a
 * tautology — so the "Connected" toast fired on the FIRST connect (users
 * saw "connection restored" on page load) and NEVER fired on a real
 * reconnect after a drop.
 *
 * The fix made `wasConnected` tri-state (`null | boolean`): null means
 * "never connected yet", false means "was connected and dropped", true
 * means "currently connected". The toast now only fires on genuine
 * drop→reconnect transitions.
 *
 * These tests live in a separate file from useSessionWebSocket.test.ts so
 * they don't inherit Vue watchers leaked by the other tests in that file
 * (each prior useSessionWebSocket invocation subscribes a watcher to the
 * shared mock ref that is never torn down outside of component unmount).
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, nextTick, effectScope, type Ref, type EffectScope } from 'vue'

// Per-test mock state. Re-created from scratch in each beforeEach so no
// watcher from a previous case is left subscribed.
let mockWsConnected: Ref<boolean>
let mockWsError: Ref<string | null>
const mockLabConnectionLost = vi.fn()
const mockLabConnectionRestored = vi.fn()
const mockLabCheckpointPassed = vi.fn()
const mockLabCheckpointFailed = vi.fn()

let scope: EffectScope

// Reset the modules + remount mocks so the composable closes over the
// fresh per-test refs.
async function loadComposable() {
  vi.resetModules()
  vi.doMock('./useWebSocket', () => ({
    useWebSocket: vi.fn(() => ({
      connected: mockWsConnected,
      error: mockWsError,
      connect: vi.fn(),
      disconnect: vi.fn(),
    })),
  }))
  vi.doMock('./useNotifications', () => ({
    useNotifications: vi.fn(() => ({
      lab: {
        connectionLost: mockLabConnectionLost,
        connectionRestored: mockLabConnectionRestored,
        checkpointPassed: mockLabCheckpointPassed,
        checkpointFailed: mockLabCheckpointFailed,
      },
    })),
  }))
  const mod = await import('./useSessionWebSocket')
  return mod.useSessionWebSocket
}

function mountInScope(
  use: Awaited<ReturnType<typeof loadComposable>>,
): void {
  scope.run(() => {
    const sessionIdRef = ref('session-123')
    use(sessionIdRef, {
      // Minimal mocks — these tests only care about the notification side effect.
      assessmentStore: { handleUpdate: vi.fn() } as never,
      sessionStore: {
        handleSessionUpdate: vi.fn(),
        handleCheckpointUpdate: vi.fn(),
        handleGradeUpdate: vi.fn(),
        handleHintNudge: vi.fn(),
      } as never,
      enableNotifications: true,
    })
  })
}

describe('useSessionWebSocket — reconnect-toast semantics (Phase 1 regression)', () => {
  beforeEach(() => {
    // Fresh refs per test so no prior watcher can fire on our mutations.
    mockWsConnected = ref(false)
    mockWsError = ref<string | null>(null)
    mockLabConnectionLost.mockReset()
    mockLabConnectionRestored.mockReset()
    mockLabCheckpointPassed.mockReset()
    mockLabCheckpointFailed.mockReset()
    scope = effectScope()
  })

  afterEach(() => {
    scope.stop()
  })

  it('silently handles the initial mount when already connected', async () => {
    const use = await loadComposable()
    mockWsConnected.value = true

    mountInScope(use)
    await nextTick()

    // "Connected" toast must NOT fire on initial mount — that would read
    // as "connection restored" to users who just loaded the page.
    expect(mockLabConnectionRestored).not.toHaveBeenCalled()
    expect(mockLabConnectionLost).not.toHaveBeenCalled()
  })

  it('fires "connection lost" when a live connection drops', async () => {
    const use = await loadComposable()
    mockWsConnected.value = true
    mountInScope(use)
    await nextTick()

    // Drop.
    mockWsConnected.value = false
    await nextTick()

    expect(mockLabConnectionLost).toHaveBeenCalledTimes(1)
    expect(mockLabConnectionRestored).not.toHaveBeenCalled()
  })

  it('fires "connection restored" only on a real reconnect (not first mount)', async () => {
    const use = await loadComposable()
    mockWsConnected.value = true
    mountInScope(use)
    await nextTick()

    // connected → disconnected → reconnected
    mockWsConnected.value = false
    await nextTick()
    mockWsConnected.value = true
    await nextTick()

    expect(mockLabConnectionLost).toHaveBeenCalledTimes(1)
    expect(mockLabConnectionRestored).toHaveBeenCalledTimes(1)
  })

  it('stays silent when the composable mounts while disconnected', async () => {
    const use = await loadComposable()
    mockWsConnected.value = false

    mountInScope(use)
    await nextTick()

    // `wasConnected` transitions from null → false. We were never actually
    // connected, so "connection lost" must not fire.
    expect(mockLabConnectionLost).not.toHaveBeenCalled()
    expect(mockLabConnectionRestored).not.toHaveBeenCalled()
  })

  it('fires "restored" on every subsequent reconnect, not just the first', async () => {
    const use = await loadComposable()
    mockWsConnected.value = true
    mountInScope(use)
    await nextTick()

    // Simulate a flaky network: drop and restore twice.
    mockWsConnected.value = false
    await nextTick()
    mockWsConnected.value = true
    await nextTick()
    mockWsConnected.value = false
    await nextTick()
    mockWsConnected.value = true
    await nextTick()

    expect(mockLabConnectionLost).toHaveBeenCalledTimes(2)
    expect(mockLabConnectionRestored).toHaveBeenCalledTimes(2)
  })
})
