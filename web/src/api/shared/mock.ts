/**
 * Mock Data Utilities
 * Helpers for mock data mode and simulated API delays.
 *
 * USE_MOCK_DATA is bound at build time via `define` in vite.config.ts, so
 * `if (USE_MOCK_DATA)` branches are statically eliminated from production
 * bundles. In dev builds we still allow a localStorage override so developers
 * can flip it at runtime without a full rebuild.
 */

// Build-time constant. In production bundles this is `true` or `false` literal,
// and the rollup dead-code-eliminator removes unreachable branches. In dev we
// additionally consult localStorage so the toggle in the UI still works.
export const USE_MOCK_DATA: boolean =
  __USE_MOCK_DATA__ ||
  (import.meta.env.DEV && localStorage.getItem('useMockData') === 'true')

// Helper to simulate API delay
export const simulateDelay = (ms: number = 300): Promise<void> => {
  return new Promise(resolve => setTimeout(resolve, ms))
}

// Helper to toggle mock data at runtime (dev only)
export const setMockDataEnabled = (enabled: boolean): void => {
  if (!import.meta.env.DEV) {
    console.warn('Mock data toggle is only available in development mode')
    return
  }
  localStorage.setItem('useMockData', String(enabled))
  // Reload to apply the change
  window.location.reload()
}

// Helper to check if mock data is currently enabled
export const isMockDataEnabled = (): boolean => USE_MOCK_DATA
