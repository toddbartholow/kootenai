import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from './auth'
import { api } from '@/api'

// Mock the API module
vi.mock('@/api', () => {
  const headers: Record<string, string> = {}
  return {
    api: {
      post: vi.fn(),
      get: vi.fn(),
      defaults: {
        headers: {
          common: headers,
        },
      },
    },
    USE_MOCK_DATA: false,
    simulateDelay: vi.fn(),
    oauth2Api: {
      getProviders: vi.fn(),
      initiateLogin: vi.fn(),
    },
  }
})

// Get localStorage mock from test-setup.ts
type MockFn = ReturnType<typeof vi.fn> & ((...args: unknown[]) => unknown)
const localStorageMock = window.localStorage as unknown as {
  getItem: MockFn
  setItem: MockFn
  removeItem: MockFn
  clear: MockFn
}

describe('Auth Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorageMock.clear()
    const headers = api.defaults.headers.common as Record<string, string>
    Object.keys(headers).forEach(key => delete headers[key])
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have null user initially', () => {
      const store = useAuthStore()

      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('should restore user from localStorage (non-secret cache)', () => {
      const mockUser = { id: '1', email: 'test@example.com', name: 'Test User', roles: ['student'] }
      localStorageMock.getItem.mockImplementation((key: string) => {
        if (key === 'auth_user') return JSON.stringify(mockUser)
        return null
      })

      const pinia = createPinia()
      setActivePinia(pinia)
      const store = useAuthStore()

      expect(store.user).toEqual(mockUser)
      expect(store.isAuthenticated).toBe(true)
    })

    it('isAuthenticated should be based on user presence', () => {
      const store = useAuthStore()

      expect(store.isAuthenticated).toBe(false)

      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }
      expect(store.isAuthenticated).toBe(true)
    })
  })

  describe('computed properties', () => {
    it('isAdmin should return true when user has admin role', () => {
      const store = useAuthStore()
      store.user = { id: '1', email: 'admin@example.com', name: 'Admin', roles: ['admin'] }

      expect(store.isAdmin).toBe(true)
    })

    it('isAdmin should return false when user lacks admin role', () => {
      const store = useAuthStore()
      store.user = { id: '1', email: 'user@example.com', name: 'User', roles: ['student'] }

      expect(store.isAdmin).toBe(false)
    })

    it('isInstructor should return true when user has instructor role', () => {
      const store = useAuthStore()
      store.user = {
        id: '1',
        email: 'instructor@example.com',
        name: 'Instructor',
        roles: ['instructor'],
      }

      expect(store.isInstructor).toBe(true)
    })
  })

  describe('login()', () => {
    it('should store user but not token in localStorage', async () => {
      const mockResponse = {
        data: {
          token: 'server-token',
          user: { id: '1', email: 'test@example.com', name: 'Test User', roles: ['student'] },
        },
      }
      vi.mocked(api.post).mockResolvedValueOnce(mockResponse)

      const store = useAuthStore()
      await store.login('test@example.com', 'password')

      expect(store.user).toEqual(mockResponse.data.user)
      expect(store.isAuthenticated).toBe(true)
      expect(localStorageMock.setItem).toHaveBeenCalledWith(
        'auth_user',
        JSON.stringify(mockResponse.data.user),
      )

      // Token should NOT be stored (HttpOnly cookie handles it)
      expect(localStorageMock.setItem).not.toHaveBeenCalledWith('auth_token', expect.anything())

      // Authorization header should NOT be set
      expect(api.defaults.headers.common['Authorization']).toBeUndefined()
    })

    it('should set loading state during login', async () => {
      const mockResponse = {
        data: {
          token: 'token',
          user: { id: '1', email: 'test@example.com', name: 'Test', roles: [] },
        },
      }
      let resolvePromise: (value: unknown) => void
      const pendingPromise = new Promise(resolve => {
        resolvePromise = resolve
      })
      vi.mocked(api.post).mockReturnValueOnce(pendingPromise as never)

      const store = useAuthStore()
      const loginPromise = store.login('test@example.com', 'password')

      expect(store.loading).toBe(true)

      resolvePromise!(mockResponse)
      await loginPromise

      expect(store.loading).toBe(false)
    })

    it('should set error on login failure', async () => {
      const mockError = { response: { data: { error: 'Invalid credentials' } } }
      vi.mocked(api.post).mockRejectedValueOnce(mockError)

      const store = useAuthStore()

      await expect(store.login('test@example.com', 'wrong-password')).rejects.toEqual(mockError)
      expect(store.error).toBe('Invalid credentials')
      expect(store.isAuthenticated).toBe(false)
    })
  })

  describe('logout()', () => {
    it('should call POST /auth/logout to clear server cookies', async () => {
      vi.mocked(api.post).mockResolvedValueOnce({ data: {} })

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await store.logout()

      expect(api.post).toHaveBeenCalledWith('/auth/logout')
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('should still clear local state even if server call fails', async () => {
      vi.mocked(api.post).mockRejectedValueOnce(new Error('Network error'))

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await store.logout()

      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('should clear user cache from localStorage', async () => {
      vi.mocked(api.post).mockResolvedValueOnce({ data: {} })

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await store.logout()

      expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_user')
      expect(localStorageMock.removeItem).toHaveBeenCalledWith('must_change_password')
    })
  })

  describe('refreshToken()', () => {
    it('should send empty body (server reads HttpOnly cookie)', async () => {
      const mockResponse = {
        data: {
          token: 'refreshed-token',
          user: { id: '1', email: 'test@example.com', name: 'Test', roles: [] },
        },
      }
      vi.mocked(api.post).mockResolvedValueOnce(mockResponse)

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await store.refreshToken()

      expect(api.post).toHaveBeenCalledWith('/auth/refresh', {})
      expect(localStorageMock.setItem).not.toHaveBeenCalledWith('auth_token', expect.anything())
    })

    it('should logout on refresh failure', async () => {
      vi.mocked(api.post)
        .mockRejectedValueOnce(new Error('Refresh failed')) // refresh
        .mockResolvedValueOnce({ data: {} }) // logout

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await expect(store.refreshToken()).rejects.toThrow('Refresh failed')
      expect(store.user).toBeNull()
    })
  })

  describe('fetchCurrentUser()', () => {
    it('should fetch user (browser sends cookie automatically)', async () => {
      const mockUser = { id: '1', email: 'test@example.com', name: 'Test User', roles: ['student'] }
      vi.mocked(api.get).mockResolvedValueOnce({ data: { user: mockUser } })

      const store = useAuthStore()

      await store.fetchCurrentUser()

      expect(api.get).toHaveBeenCalledWith('/auth/me')
      expect(store.user).toEqual(mockUser)
    })

    it('should logout on 401 response', async () => {
      const mockError = { response: { status: 401 } }
      vi.mocked(api.get).mockRejectedValueOnce(mockError)
      vi.mocked(api.post).mockResolvedValueOnce({ data: {} }) // logout

      const store = useAuthStore()
      store.user = { id: '1', email: 'test@example.com', name: 'Test', roles: [] }

      await expect(store.fetchCurrentUser()).rejects.toEqual(mockError)
      expect(store.user).toBeNull()
    })
  })
})
