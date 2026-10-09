/**
 * API Configuration
 * Axios instance with base URL, timeout, and interceptors.
 *
 * Auth tokens live in an HttpOnly cookie (ADR-0002). The browser attaches the
 * cookie automatically via `withCredentials: true`. A double-submit CSRF
 * token is read from the non-HttpOnly `csrf_token` cookie and echoed as
 * `X-CSRF-Token` on mutating requests.
 */
import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { i18n } from '@/locales'
import { CSRF_COOKIE_NAME, CSRF_HEADER_NAME, requiresCSRF, getCookie } from './shared/cookie-auth'

// Build-time constant. Dev → empty (Vite proxy routes /api → backend).
// Prod → whatever was passed as VITE_API_BASE_URL at build time. See the
// `define` block in vite.config.ts.
const API_BASE_URL = __API_BASE_URL__

export const api = axios.create({
  baseURL: `${API_BASE_URL}/api/v1`,
  timeout: 30000,
  withCredentials: true,
})

// Token refresh state to prevent multiple simultaneous refresh attempts
let isRefreshing = false
let failedQueue: Array<{
  resolve: () => void
  reject: (error: unknown) => void
}> = []

const processQueue = (error: unknown): void => {
  failedQueue.forEach(prom => {
    if (error) {
      prom.reject(error)
    } else {
      prom.resolve()
    }
  })
  failedQueue = []
}

// Extend request config to track retry state
interface ExtendedRequestConfig extends InternalAxiosRequestConfig {
  _retry?: boolean
}

// Same-origin guard for the axios instance. Rejects requests whose resolved
// target is not the expected API origin, eliminating developer typos and
// XSS-controlled URL injection into the shared axios instance. Even with
// HttpOnly cookies the guard is valuable: it prevents the browser from
// sending the auth cookie to an attacker-controlled same-site URL.
const expectedApiOrigin = ((): string | null => {
  if (!API_BASE_URL) return null // dev-proxy mode → trust current page origin
  try {
    return new URL(API_BASE_URL).origin
  } catch {
    return null
  }
})()

function isExpectedOrigin(url: string): boolean {
  // Relative URLs resolve against baseURL — always same-origin-as-config.
  if (!/^(?:[a-z][a-z0-9+\-.]*:)?\/\//i.test(url)) return true
  try {
    const target = new URL(url, typeof window !== 'undefined' ? window.location.href : undefined)
    if (!expectedApiOrigin) {
      return typeof window !== 'undefined' && target.origin === window.location.origin
    }
    return target.origin === expectedApiOrigin
  } catch {
    return false
  }
}

// Request interceptor: enforce same-origin guard, attach locale, attach CSRF.
api.interceptors.request.use(config => {
  const targetUrl = config.url ?? ''
  if (targetUrl && !isExpectedOrigin(targetUrl)) {
    return Promise.reject(
      new Error(
        `[api] Refused to send request to non-API origin: ${targetUrl}. Use fetch() directly for third-party calls.`,
      ),
    )
  }

  const locale = i18n.global.locale.value
  if (locale) {
    config.headers.set('Accept-Language', locale)
  }

  // Attach the CSRF double-submit token on mutating methods.
  // The csrf_token cookie is NOT HttpOnly, so JS can read it.
  if (config.method && requiresCSRF(config.method)) {
    const csrfToken = getCookie(CSRF_COOKIE_NAME)
    if (csrfToken) {
      config.headers.set(CSRF_HEADER_NAME, csrfToken)
    }
  }

  return config
})

// Response interceptor to handle 401 errors with automatic token refresh.
// Delegates to the auth store's refreshToken() to avoid duplicating refresh logic.
api.interceptors.response.use(
  response => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as ExtendedRequestConfig | undefined

    // Only handle 401 errors
    if (error.response?.status !== 401 || !originalRequest) {
      return Promise.reject(error)
    }

    // Don't retry refresh or login requests to avoid infinite loops
    if (
      originalRequest.url?.includes('/auth/refresh') ||
      originalRequest.url?.includes('/auth/login')
    ) {
      return Promise.reject(error)
    }

    // Don't retry if we've already tried
    if (originalRequest._retry) {
      return Promise.reject(error)
    }

    // If already refreshing, queue this request
    if (isRefreshing) {
      return new Promise<void>((resolve, reject) => {
        failedQueue.push({ resolve, reject })
      })
        .then(() => api(originalRequest))
        .catch(err => Promise.reject(err))
    }

    // Mark as retrying and start refresh
    originalRequest._retry = true
    isRefreshing = true

    try {
      // Lazy import to avoid circular dependency (config -> store -> api -> config)
      const { useAuthStore } = await import('@/stores/auth')
      const authStore = useAuthStore()
      await authStore.refreshToken()

      processQueue(null)

      // The browser sends the rotated cookie automatically on retry.
      return api(originalRequest)
    } catch (refreshError) {
      // Refresh failed - the store already handled logout
      processQueue(refreshError)
      return Promise.reject(refreshError)
    } finally {
      isRefreshing = false
    }
  },
)

// Helper to set organization context header for multi-tenant requests
export const setOrganizationContext = (orgId: string | null): void => {
  if (orgId) {
    api.defaults.headers.common['X-Organization-ID'] = orgId
  } else {
    delete api.defaults.headers.common['X-Organization-ID']
  }
}

export default api
