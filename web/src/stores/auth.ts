import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api, USE_MOCK_DATA, simulateDelay, oauth2Api } from '@/api'
import type { OAuth2Provider } from '@/api'
import { createErrorState } from '@/types/errors'
import { resolveLocale } from '@/locales'

export interface User {
  id: string
  email: string
  name: string
  roles: string[]
}

export interface LoginResponse {
  token: string
  user: User
  mustChangePassword?: boolean
}

const USER_KEY = 'auth_user'
const MUST_CHANGE_PASSWORD_KEY = 'must_change_password'

/**
 * Type guard to validate user data from localStorage
 * Prevents malicious or corrupted data from being loaded
 */
function isValidUser(data: unknown): data is User {
  if (typeof data !== 'object' || data === null) {
    return false
  }
  const obj = data as Record<string, unknown>
  return (
    typeof obj['id'] === 'string' &&
    typeof obj['email'] === 'string' &&
    typeof obj['name'] === 'string' &&
    Array.isArray(obj['roles']) &&
    obj['roles'].every((role: unknown) => typeof role === 'string')
  )
}

export const useAuthStore = defineStore('auth', () => {
  // The JWT lives in an HttpOnly cookie — invisible to JS (ADR-0002).
  // Only the user object (non-secret) is cached in localStorage for
  // instant hydration across page loads.
  const user = ref<User | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const mustChangePassword = ref<boolean>(localStorage.getItem(MUST_CHANGE_PASSWORD_KEY) === 'true')

  // OAuth2 state
  const oauth2Providers = ref<OAuth2Provider[]>([])
  const oauth2Loading = ref(false)

  // Initialize user from localStorage with validation
  const storedUser = localStorage.getItem(USER_KEY)
  if (storedUser) {
    try {
      const parsed: unknown = JSON.parse(storedUser)
      if (isValidUser(parsed)) {
        user.value = parsed
      } else {
        console.warn('[Auth] Invalid user data in localStorage, clearing')
        localStorage.removeItem(USER_KEY)
      }
    } catch {
      localStorage.removeItem(USER_KEY)
    }
  }

  // ---------------------------------------------------------------------------
  // Cross-tab sync via BroadcastChannel
  // ---------------------------------------------------------------------------
  // The auth cookie is shared across tabs automatically. BroadcastChannel
  // syncs the local reactive state so sibling tabs update instantly.
  // Safari ≤ 15.4 lacks BroadcastChannel — fall back to re-checking auth
  // on visibilitychange.
  if (typeof window !== 'undefined') {
    if (typeof BroadcastChannel !== 'undefined') {
      const authChannel = new BroadcastChannel('kootenai-auth')
      authChannel.onmessage = (e: MessageEvent<{ type: string }>) => {
        if (e.data?.type === 'logout') {
          user.value = null
          mustChangePassword.value = false
          localStorage.removeItem(USER_KEY)
          localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
        } else if (e.data?.type === 'login') {
          fetchCurrentUser().catch(() => {
            /* ignore */
          })
        }
      }
    } else {
      // Safari fallback: re-check auth when tab regains focus
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible' && user.value) {
          fetchCurrentUser().catch(() => {
            /* ignore — will log out if 401 */
          })
        }
      })
    }
  }

  // Computed properties
  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.roles.includes('admin') ?? false)
  const isInstructor = computed(() => user.value?.roles.includes('instructor') ?? false)

  async function login(email: string, password?: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      if (USE_MOCK_DATA) {
        await simulateDelay(300)
        const mockUser: User = {
          id: '00000000-0000-0000-0000-000000000001',
          email: email || 'demo@example.com',
          name: email?.split('@')[0] || 'Demo User',
          roles: ['student', 'instructor'],
        }
        user.value = mockUser
        localStorage.setItem(USER_KEY, JSON.stringify(mockUser))
        return
      }

      const response = await api.post<LoginResponse>('/auth/login', { email, password })
      const { user: newUser, mustChangePassword: needsPasswordChange } = response.data

      // The server already set the HttpOnly auth cookie via Set-Cookie.
      // We store the user object (non-secret) for instant hydration.
      user.value = newUser
      mustChangePassword.value = needsPasswordChange ?? false
      localStorage.setItem(USER_KEY, JSON.stringify(newUser))
      if (needsPasswordChange) {
        localStorage.setItem(MUST_CHANGE_PASSWORD_KEY, 'true')
      } else {
        localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
      }

      broadcastAuthEvent('login')
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Login failed').message
      throw e
    } finally {
      loading.value = false
    }
  }

  async function logout(): Promise<void> {
    // Tell the server to clear the HttpOnly cookie.
    // Best-effort — we clear local state even if the call fails.
    try {
      await api.post('/auth/logout')
    } catch {
      // Server unreachable or token already expired — proceed with local cleanup
    }
    broadcastAuthEvent('logout')

    // Clear state
    user.value = null
    mustChangePassword.value = false

    // Clear localStorage (non-secret user cache + password flag)
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  }

  function clearMustChangePassword(): void {
    mustChangePassword.value = false
    localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  }

  // OAuth2 methods
  async function fetchOAuth2Providers(): Promise<void> {
    if (USE_MOCK_DATA) {
      oauth2Providers.value = [
        { id: 'google', name: 'Google', type: 'google' },
        { id: 'github', name: 'GitHub', type: 'github' },
      ]
      return
    }

    oauth2Loading.value = true
    try {
      oauth2Providers.value = await oauth2Api.getProviders()
    } catch {
      oauth2Providers.value = []
    } finally {
      oauth2Loading.value = false
    }
  }

  function loginWithOAuth2(providerId: string, redirectUrl?: string): void {
    oauth2Api.initiateLogin(providerId, redirectUrl)
  }

  /**
   * Handle OAuth2 callback. The server set the auth cookie during the
   * redirect — we just need to fetch the user.
   */
  async function handleOAuth2Callback(): Promise<boolean> {
    // Check for error query param (server may redirect with ?error=...).
    const urlParams = new URLSearchParams(window.location.search)
    const oauthError = urlParams.get('error')
    if (oauthError) {
      error.value = oauthError
      return false
    }

    // Try to fetch user — if the cookie is valid we're logged in
    try {
      await fetchCurrentUser()
      if (user.value) {
        broadcastAuthEvent('login')
        return true
      }
      return false
    } catch {
      return false
    }
  }

  async function refreshToken(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      // Send empty body — the server reads the token from the HttpOnly
      // cookie and rotates both auth + CSRF cookies.
      const response = await api.post<LoginResponse>('/auth/refresh', {})
      const { user: newUser } = response.data

      user.value = newUser
      localStorage.setItem(USER_KEY, JSON.stringify(newUser))
    } catch (e: unknown) {
      await logout()
      throw e
    } finally {
      loading.value = false
    }
  }

  async function fetchCurrentUser(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      if (USE_MOCK_DATA) {
        await simulateDelay(100)
        return
      }
      const response = await api.get<{ user: User; preferredLocale?: string | null }>('/auth/me')
      user.value = response.data.user
      localStorage.setItem(USER_KEY, JSON.stringify(response.data.user))
      if (response.data.preferredLocale) {
        const { useLocaleStore } = await import('@/stores/locale')
        useLocaleStore().setLocale(resolveLocale(response.data.preferredLocale), { persist: false })
      }
    } catch (e: unknown) {
      const err = e as { response?: { status?: number } }
      if (err.response?.status === 401) {
        await logout()
      }
      throw e
    } finally {
      loading.value = false
    }
  }

  // ---------------------------------------------------------------------------
  // BroadcastChannel helper
  // ---------------------------------------------------------------------------
  function broadcastAuthEvent(type: 'login' | 'logout'): void {
    if (typeof BroadcastChannel === 'undefined') return
    try {
      const ch = new BroadcastChannel('kootenai-auth')
      ch.postMessage({ type })
      ch.close()
    } catch {
      // BroadcastChannel not available — ignore
    }
  }

  return {
    // State
    user,
    loading,
    error,
    mustChangePassword,
    oauth2Providers,
    oauth2Loading,
    // Computed
    isAuthenticated,
    isAdmin,
    isInstructor,
    // Actions
    login,
    logout,
    refreshToken,
    fetchCurrentUser,
    clearMustChangePassword,
    // OAuth2 actions
    fetchOAuth2Providers,
    loginWithOAuth2,
    handleOAuth2Callback,
  }
})
