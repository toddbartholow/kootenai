import { vi } from 'vitest'
import { config } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import { i18n } from '@/locales'

// Configure Vue Test Utils with PrimeVue + i18n plugins globally.
// The real `en` catalog is loaded so assertions on rendered text work without
// per-test wiring; tests that need a different locale can call
// `i18n.global.locale.value = '...'` in a beforeEach.
config.global.plugins = [
  [PrimeVue, { unstyled: true }],
  ToastService,
  ConfirmationService,
  i18n,
]

// Create mock functions for useConfirm and useToast that can be controlled per-test
export const mockConfirmRequire = vi.fn()
export const mockToastAdd = vi.fn()

// Mock PrimeVue composables globally
vi.mock('primevue/useconfirm', () => ({
  useConfirm: () => ({
    require: mockConfirmRequire,
    close: vi.fn(),
  }),
}))

vi.mock('primevue/usetoast', () => ({
  useToast: () => ({
    add: mockToastAdd,
    removeGroup: vi.fn(),
    removeAllGroups: vi.fn(),
  }),
}))

// Mock window.confirm and window.alert for happy-dom
Object.defineProperty(window, 'confirm', {
  writable: true,
  value: vi.fn(() => true),
})

Object.defineProperty(window, 'alert', {
  writable: true,
  value: vi.fn(),
})

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key]
    }),
    clear: vi.fn(() => {
      store = {}
    }),
  }
})()

Object.defineProperty(window, 'localStorage', { value: localStorageMock })

// Mock WebSocket for test environment
class MockWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readonly CONNECTING = 0
  readonly OPEN = 1
  readonly CLOSING = 2
  readonly CLOSED = 3

  url: string
  readyState: number = MockWebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null

  constructor(url: string) {
    this.url = url
    // Simulate async connection - don't auto-connect in tests to avoid unhandled errors
  }

  send(_data: string): void {
    // Mock send - do nothing in tests
  }

  close(_code?: number, _reason?: string): void {
    this.readyState = MockWebSocket.CLOSED
  }
}

// Only set if WebSocket doesn't exist (happy-dom may not have it)
if (typeof window.WebSocket === 'undefined') {
  Object.defineProperty(window, 'WebSocket', {
    writable: true,
    value: MockWebSocket,
  })
}

// Also set the global WebSocket for direct access
if (typeof globalThis.WebSocket === 'undefined') {
  ;(globalThis as unknown as { WebSocket: typeof MockWebSocket }).WebSocket = MockWebSocket
}
