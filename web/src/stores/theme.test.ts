import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useThemeStore } from './theme'

// Get localStorage mock from test-setup.ts
type MockFn = ReturnType<typeof vi.fn> & ((...args: unknown[]) => unknown)
const localStorageMock = window.localStorage as unknown as {
  getItem: MockFn
  setItem: MockFn
  removeItem: MockFn
  clear: MockFn
}

// Mock matchMedia for system preference detection
const createMatchMediaMock = (matches: boolean) => {
  return vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('Theme Store', () => {
  beforeEach(() => {
    localStorageMock.clear()
    vi.clearAllMocks()
    // Reset document class list
    document.documentElement.classList.remove('dark')
    // Default to light mode system preference
    window.matchMedia = createMatchMediaMock(false)
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should default to light theme when no system preference', () => {
      window.matchMedia = createMatchMediaMock(false)

      setActivePinia(createPinia())
      const store = useThemeStore()

      expect(store.theme).toBe('light')
      expect(store.isDark).toBe(false)
    })

    it('should use system dark mode preference', () => {
      window.matchMedia = createMatchMediaMock(true) // System prefers dark

      setActivePinia(createPinia())
      const store = useThemeStore()

      expect(store.theme).toBe('dark')
      expect(store.isDark).toBe(true)
    })
  })

  describe('computed properties', () => {
    it('isDark should be true when theme is dark', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('dark')

      expect(store.isDark).toBe(true)
    })

    it('isDark should be false when theme is light', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')

      expect(store.isDark).toBe(false)
    })
  })

  describe('toggleTheme', () => {
    it('should toggle from light to dark', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')
      expect(store.theme).toBe('light')

      store.toggleTheme()

      expect(store.theme).toBe('dark')
      expect(store.isDark).toBe(true)
    })

    it('should toggle from dark to light', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('dark')
      expect(store.theme).toBe('dark')

      store.toggleTheme()

      expect(store.theme).toBe('light')
      expect(store.isDark).toBe(false)
    })

    it('should toggle multiple times correctly', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')

      store.toggleTheme()
      expect(store.theme).toBe('dark')

      store.toggleTheme()
      expect(store.theme).toBe('light')

      store.toggleTheme()
      expect(store.theme).toBe('dark')
    })
  })

  describe('setTheme', () => {
    it('should set theme to dark', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('dark')

      expect(store.theme).toBe('dark')
      expect(store.isDark).toBe(true)
    })

    it('should set theme to light', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')

      expect(store.theme).toBe('light')
      expect(store.isDark).toBe(false)
    })

    it('should be idempotent when setting same theme', () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('dark')
      store.setTheme('dark')

      expect(store.theme).toBe('dark')
    })
  })

  describe('DOM class management', () => {
    it('should add dark class to documentElement when dark theme', async () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('dark')

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })

    it('should remove dark class from documentElement when light theme', async () => {
      // Start with dark class
      document.documentElement.classList.add('dark')

      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('should update DOM class when toggling theme', async () => {
      setActivePinia(createPinia())
      const store = useThemeStore()

      store.setTheme('light')

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(false)

      store.toggleTheme()

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(true)

      store.toggleTheme()

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(false)
    })

    it('should apply dark class on initialization when system prefers dark', async () => {
      window.matchMedia = createMatchMediaMock(true)

      setActivePinia(createPinia())
      useThemeStore()

      // Wait for watcher to fire
      await new Promise(resolve => setTimeout(resolve, 0))

      expect(document.documentElement.classList.contains('dark')).toBe(true)
    })
  })
})
