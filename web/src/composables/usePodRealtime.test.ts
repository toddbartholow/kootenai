/**
 * Tests for usePodRealtime — the composable that subscribes to WebSocket
 * pod_status / vm_status events for a single pod and merges them into a
 * pod ref.
 *
 * The underlying useWebSocket is mocked so we can drive handler callbacks
 * directly and assert on the resulting pod-state mutations without
 * standing up a real socket.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, effectScope, type EffectScope } from 'vue'
import { setActivePinia, createPinia } from 'pinia'
import type { Pod, PodVM } from '@/api'
import type {
  PodStatusPayload,
  VMStatusPayload,
  UseWebSocketOptions,
} from './useWebSocket'

// Capture the options passed to useWebSocket so we can invoke handlers.
let capturedOptions: UseWebSocketOptions | null = null
const mockWsDisconnect = vi.fn()
const mockWsConnected = ref(false)

vi.mock('./useWebSocket', () => ({
  useWebSocket: vi.fn((options: UseWebSocketOptions) => {
    capturedOptions = options
    return {
      connected: mockWsConnected,
      error: ref<string | null>(null),
      disconnect: mockWsDisconnect,
      connect: vi.fn(),
      send: vi.fn(),
    }
  }),
}))

function makePod(overrides: Partial<Pod> = {}): Pod {
  return {
    id: 'p1',
    labTemplate: 'linux-intro',
    platform: 'proxmox',
    owner: 'user-1',
    status: 'provisioning',
    vms: [
      { name: 'web-01', platformId: '100', status: 'created' } as PodVM,
      { name: 'db-01', platformId: '101', status: 'created' } as PodVM,
    ],
    createdAt: '2025-01-01T00:00:00Z',
    ...overrides,
  }
}

let scope: EffectScope

describe('usePodRealtime', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    capturedOptions = null
    mockWsDisconnect.mockReset()
    mockWsConnected.value = false
    scope = effectScope()
  })

  afterEach(() => {
    scope.stop()
  })

  it('subscribes to WebSocket with the passed podId', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    scope.run(() => {
      const pod = ref<Pod | null>(makePod())
      usePodRealtime('p1', pod)
    })
    expect(capturedOptions?.podId).toBe('p1')
    expect(capturedOptions?.autoConnect).toBe(true)
    expect(capturedOptions?.autoReconnect).toBe(true)
  })

  it('merges pod_status updates into the pod ref when podId matches', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(makePod())
    let lastUpdate: ReturnType<typeof usePodRealtime>['lastUpdate'] | null = null
    scope.run(() => {
      lastUpdate = usePodRealtime('p1', pod).lastUpdate
    })

    const payload: PodStatusPayload = {
      podId: 'p1',
      status: 'running',
      timestamp: '2025-06-01T10:00:00Z',
    }
    capturedOptions!.onPodStatusUpdate!(payload)

    expect(pod.value?.status).toBe('running')
    expect(lastUpdate!.value?.toISOString()).toBe('2025-06-01T10:00:00.000Z')
  })

  it('ignores pod_status updates for a different pod', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(makePod())
    scope.run(() => {
      usePodRealtime('p1', pod)
    })

    capturedOptions!.onPodStatusUpdate!({
      podId: 'different-pod',
      status: 'destroyed',
      timestamp: '2025-06-01T10:00:00Z',
    })

    // Local pod ref must not be mutated — the message was for someone else.
    expect(pod.value?.status).toBe('provisioning')
  })

  it('merges vm_status updates into the matching VM, preserving others', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(makePod())
    scope.run(() => {
      usePodRealtime('p1', pod)
    })

    const payload: VMStatusPayload = {
      podId: 'p1',
      vmName: 'web-01',
      status: 'running',
      ipAddress: '10.0.0.5',
      currentSnapshot: 'initial',
      timestamp: '2025-06-01T10:00:00Z',
    }
    capturedOptions!.onVMStatusUpdate!(payload)

    const web = pod.value!.vms!.find((v) => v.name === 'web-01')!
    const db = pod.value!.vms!.find((v) => v.name === 'db-01')!
    expect(web.status).toBe('running')
    expect(web.ipAddress).toBe('10.0.0.5')
    expect(web.currentSnapshot).toBe('initial')
    // Unrelated VM untouched.
    expect(db.status).toBe('created')
    expect(db.ipAddress).toBeUndefined()
  })

  it('mirrors the WebSocket connected state onto wsConnected', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(makePod())
    let wsConnected: ReturnType<typeof usePodRealtime>['wsConnected'] | null = null
    scope.run(() => {
      wsConnected = usePodRealtime('p1', pod).wsConnected
    })

    expect(wsConnected!.value).toBe(false)
    mockWsConnected.value = true
    // Watchers fire asynchronously.
    await new Promise((r) => setTimeout(r, 0))
    expect(wsConnected!.value).toBe(true)
  })

  it('tears down the socket when disconnect() is called', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(makePod())
    let api: ReturnType<typeof usePodRealtime> | null = null
    scope.run(() => {
      api = usePodRealtime('p1', pod)
    })
    api!.disconnect()
    expect(mockWsDisconnect).toHaveBeenCalledTimes(1)
  })

  it('does not update pod fields when pod ref is null', async () => {
    const { usePodRealtime } = await import('./usePodRealtime')
    const pod = ref<Pod | null>(null)
    scope.run(() => {
      usePodRealtime('p1', pod)
    })

    // These must not throw — the handlers have null guards.
    capturedOptions!.onPodStatusUpdate!({
      podId: 'p1',
      status: 'running',
      timestamp: '2025-06-01T10:00:00Z',
    })
    capturedOptions!.onVMStatusUpdate!({
      podId: 'p1',
      vmName: 'web-01',
      status: 'running',
      timestamp: '2025-06-01T10:00:00Z',
    })

    expect(pod.value).toBeNull()
  })
})
