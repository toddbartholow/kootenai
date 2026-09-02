import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// Previously this file also contained ~275 lines of type-shape
// assertions that built WebSocketMessage/SessionEventPayload/etc.
// literals and called expect() on a field. Those weren't runtime
// tests — TypeScript's own exhaustiveness check on the discriminated
// union in useWebSocket.ts (the `_exhaustiveCheck: never` in
// handleMessage) already enforces the same contract at compile time.
// The behavioral tests below exercise the composable's actual
// lifecycle: URL building, connect/disconnect, message routing,
// reconnect semantics, and send()-when-connected.

// Helper to run composables in a Vue component context
function withSetup<T>(composable: () => T): { result: T; unmount: () => void } {
  let result!: T
  const wrapper = mount(
    defineComponent({
      setup() {
        result = composable()
        return () => h('div')
      },
    }),
    {
      global: {
        plugins: [createPinia()],
      },
    }
  )
  return { result, unmount: () => wrapper.unmount() }
}

describe('useWebSocket unit tests', () => {
  // Store original globals
  let originalWebSocket: typeof WebSocket
  let originalLocation: Location
  let unmountFn: (() => void) | null = null

  // Mock WebSocket instances
  class MockWebSocket {
    static CONNECTING = 0
    static OPEN = 1
    static CLOSING = 2
    static CLOSED = 3

    url: string
    readyState: number = MockWebSocket.CONNECTING
    onopen: ((ev: Event) => void) | null = null
    onclose: ((ev: CloseEvent) => void) | null = null
    onerror: ((ev: Event) => void) | null = null
    onmessage: ((ev: MessageEvent) => void) | null = null

    constructor(url: string) {
      this.url = url
      mockInstances.push(this)
    }

    close = vi.fn((code?: number, reason?: string) => {
      this.readyState = MockWebSocket.CLOSED
      if (this.onclose) {
        this.onclose({ code: code || 1000, reason: reason || '', wasClean: true } as CloseEvent)
      }
    })

    send = vi.fn()

    simulateOpen() {
      this.readyState = MockWebSocket.OPEN
      if (this.onopen) this.onopen(new Event('open'))
    }

    simulateMessage(data: object) {
      if (this.onmessage) {
        this.onmessage({ data: JSON.stringify(data) } as MessageEvent)
      }
    }

    simulateError() {
      if (this.onerror) this.onerror(new Event('error'))
    }

    simulateClose(code = 1000, wasClean = true) {
      this.readyState = MockWebSocket.CLOSED
      if (this.onclose) {
        this.onclose({ code, wasClean, reason: '' } as CloseEvent)
      }
    }
  }

  let mockInstances: MockWebSocket[] = []

  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    mockInstances = []
    setActivePinia(createPinia())

    // Save originals
    originalWebSocket = global.WebSocket
    originalLocation = window.location

    // Mock WebSocket
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket

    // Mock location
    Object.defineProperty(window, 'location', {
      value: { protocol: 'http:', host: 'localhost:3000' },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    // Clean up component if mounted
    if (unmountFn) {
      unmountFn()
      unmountFn = null
    }

    vi.useRealTimers()
    vi.resetModules()

    // Restore originals
    global.WebSocket = originalWebSocket
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    })
  })

  async function getUseWebSocket() {
    vi.resetModules()
    const mod = await import('./useWebSocket')
    return mod.useWebSocket
  }

  // Helper to use composable in component context
  function useInContext<T>(composableFn: () => T): T {
    const { result, unmount } = withSetup(composableFn)
    unmountFn = unmount
    return result
  }

  it('should build correct WebSocket URL', async () => {
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true }))

    expect(mockInstances.length).toBe(1)
    expect(mockInstances[0]!.url).toBe('ws://localhost:3000/api/ws')
  })

  it('should use wss for https', async () => {
    Object.defineProperty(window, 'location', {
      value: { protocol: 'https:', host: 'secure.example.com' },
      writable: true,
      configurable: true,
    })

    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true }))

    expect(mockInstances[0]!.url).toBe('wss://secure.example.com/api/ws')
  })

  it('should append query params', async () => {
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ sessionId: 'sess-1', podId: 'pod-1', autoConnect: true }))

    expect(mockInstances[0]!.url).toContain('sessionId=sess-1')
    expect(mockInstances[0]!.url).toContain('podId=pod-1')
  })

  it('should set connected on open', async () => {
    const useWebSocket = await getUseWebSocket()
    const { connected } = useInContext(() => useWebSocket({ autoConnect: true }))

    expect(connected.value).toBe(false)
    mockInstances[0]!.simulateOpen()
    expect(connected.value).toBe(true)
  })

  it('should set connected false on close', async () => {
    const useWebSocket = await getUseWebSocket()
    const { connected } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateOpen()
    expect(connected.value).toBe(true)

    mockInstances[0]!.simulateClose()
    expect(connected.value).toBe(false)
  })

  it('should set error on WebSocket error', async () => {
    const useWebSocket = await getUseWebSocket()
    const { error } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateError()
    expect(error.value).toBe('WebSocket connection error')
  })

  it('should clear error on successful connection', async () => {
    const useWebSocket = await getUseWebSocket()
    const { error } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateError()
    expect(error.value).toBe('WebSocket connection error')

    mockInstances[0]!.simulateOpen()
    expect(error.value).toBeNull()
  })

  it('should not connect when autoConnect is false', async () => {
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: false }))

    expect(mockInstances.length).toBe(0)
  })

  it('should connect manually', async () => {
    const useWebSocket = await getUseWebSocket()
    const { connect } = useInContext(() => useWebSocket({ autoConnect: false }))

    expect(mockInstances.length).toBe(0)
    connect()
    expect(mockInstances.length).toBe(1)
  })

  it('should not create duplicate connection if already open', async () => {
    const useWebSocket = await getUseWebSocket()
    const { connect } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateOpen()
    connect()

    expect(mockInstances.length).toBe(1)
  })

  it('should disconnect', async () => {
    const useWebSocket = await getUseWebSocket()
    const { connected, disconnect } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateOpen()
    expect(connected.value).toBe(true)

    disconnect()
    expect(connected.value).toBe(false)
    expect(mockInstances[0]!.close).toHaveBeenCalledWith(1000, 'Client disconnecting')
  })

  it('should call callback on assessment message', async () => {
    const onAssessmentUpdate = vi.fn()
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true, onAssessmentUpdate }))

    mockInstances[0]!.simulateMessage({
      type: 'assessment',
      payload: { type: 'complete', sessionId: 's1', totalScore: 85, maxScore: 100, passedItems: 8, totalItems: 10, percentage: 85, timestamp: '2024-01-01T12:00:00Z' },
    })

    expect(onAssessmentUpdate).toHaveBeenCalledWith({
      type: 'complete',
      sessionId: 's1',
      totalScore: 85,
      maxScore: 100,
      passedItems: 8,
      totalItems: 10,
      percentage: 85,
      timestamp: '2024-01-01T12:00:00Z',
    })
  })

  it('should call callback on checkpoint message', async () => {
    const onCheckpointUpdate = vi.fn()
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true, onCheckpointUpdate }))

    mockInstances[0]!.simulateMessage({
      type: 'checkpoint',
      payload: {
        podId: 'pod-1',
        sessionId: 'session-1',
        checkpointId: 'cp-1',
        name: 'Test',
        status: 'passed',
        timestamp: '2024-01-01',
      },
    })

    expect(onCheckpointUpdate).toHaveBeenCalled()
  })

  it('should send message when connected', async () => {
    const useWebSocket = await getUseWebSocket()
    const { send } = useInContext(() => useWebSocket({ autoConnect: true }))

    mockInstances[0]!.simulateOpen()
    send({ type: 'subscribe', sessionId: 'session-1' })

    expect(mockInstances[0]!.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', sessionId: 'session-1' })
    )
  })

  it('should warn when sending without connection', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const useWebSocket = await getUseWebSocket()
    const { send } = useInContext(() => useWebSocket({ autoConnect: true }))

    send({ type: 'ping' })

    // Logger formats the message with prefix and level, so check for partial match
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('Cannot send - not connected'))
    expect(mockInstances[0]!.send).not.toHaveBeenCalled()
    warnSpy.mockRestore()
  })

  it('should handle invalid JSON in message', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true }))

    if (mockInstances[0]!.onmessage) {
      mockInstances[0]!.onmessage({ data: 'not json' } as MessageEvent)
    }

    expect(errorSpy).toHaveBeenCalled()
    errorSpy.mockRestore()
  })

  it('should reconnect on abnormal close when autoReconnect is true', async () => {
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true, autoReconnect: true, reconnectInterval: 1000 }))

    mockInstances[0]!.simulateOpen()
    mockInstances[0]!.simulateClose(1006, false) // Abnormal close

    vi.advanceTimersByTime(1000)

    expect(mockInstances.length).toBe(2)
  })

  it('should not reconnect on clean close', async () => {
    const useWebSocket = await getUseWebSocket()
    useInContext(() => useWebSocket({ autoConnect: true, autoReconnect: true, reconnectInterval: 1000 }))

    mockInstances[0]!.simulateOpen()
    mockInstances[0]!.simulateClose(1000, true) // Clean close

    vi.advanceTimersByTime(5000)

    expect(mockInstances.length).toBe(1)
  })

  it('should reset reconnect attempts on successful connection', async () => {
    const useWebSocket = await getUseWebSocket()
    const { reconnectAttempts } = useInContext(() => useWebSocket({
      autoConnect: true,
      autoReconnect: true,
      reconnectInterval: 1000,
    }))

    mockInstances[0]!.simulateClose(1006, false)
    expect(reconnectAttempts.value).toBe(1)

    vi.advanceTimersByTime(1000)
    mockInstances[1]!.simulateOpen()

    expect(reconnectAttempts.value).toBe(0)
  })
})
