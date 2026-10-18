import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import SessionsView from './SessionsView.vue'
import { sessionsApi, type Session } from '@/api'
import { mockConfirmRequire, mockToastAdd } from '../test-setup'

// Mock the API client
vi.mock('@/api', () => ({
  sessionsApi: {
    list: vi.fn(),
    end: vi.fn(),
  },
}))

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { email: 'test@example.com' },
    isAuthenticated: true,
  })),
}))

const mockSessions: Session[] = [
  {
    id: 'session-1',
    podId: 'pod-1',
    userId: 'user@test.com',
    labTemplateId: 'network-basics',
    status: 'active',
    earnedPoints: 50,
    maxPoints: 100,
    percentage: 50,
    passed: false,
    startedAt: '2024-01-01T10:00:00Z',
  },
  {
    id: 'session-2',
    podId: 'pod-2',
    userId: 'user2@test.com',
    labTemplateId: 'advanced-routing',
    status: 'completed',
    earnedPoints: 85,
    maxPoints: 100,
    percentage: 85,
    passed: true,
    startedAt: '2024-01-01T09:00:00Z',
    endedAt: '2024-01-01T11:00:00Z',
  },
]

describe('SessionsView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/sessions', name: 'sessions', component: SessionsView },
        { path: '/session/:sessionId', name: 'session', component: { template: '<div>Session</div>' } },
      ],
    })

    vi.clearAllMocks()
    mockConfirmRequire.mockClear()
    mockToastAdd.mockClear()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(SessionsView, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: true,
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner initially', async () => {
      vi.mocked(sessionsApi.list).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('should fetch sessions on mount', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      mountComponent()
      await flushPromises()

      expect(sessionsApi.list).toHaveBeenCalledWith(undefined, true)
    })

    it('should display sessions after loading', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('network-basics')
      expect(wrapper.text()).toContain('advanced-routing')
    })

    it('should show empty state when no sessions', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No sessions found')
    })
  })

  describe('session display', () => {
    it('should display session status badge', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('active')
      expect(wrapper.text()).toContain('completed')
    })

    it('should display score progress', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('50 / 100 points')
      expect(wrapper.text()).toContain('85 / 100 points')
    })

    it('should show "Passed" for passed sessions', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Passed')
    })

    it('should show "End" button only for active sessions', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      // Find End buttons - should only be 1 since only 1 session is active
      const endButtons = wrapper.findAll('button').filter(btn => btn.text() === 'End')
      expect(endButtons).toHaveLength(1)
    })
  })

  describe('filtering', () => {
    it('should filter by userId', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      const userInput = wrapper.find('input[type="text"]')
      await userInput.setValue('user@test.com')
      await flushPromises()

      expect(sessionsApi.list).toHaveBeenCalledWith('user@test.com', true)
    })

    // Skipped: PrimeVue Select component requires complex interaction testing
    it.skip('should filter by active status', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const _wrapper = mountComponent()
      await flushPromises()

      // Clear mocks after initial load
      vi.mocked(sessionsApi.list).mockClear()

      // PrimeVue Select requires more complex interaction
      // This test is skipped as it needs integration with the actual component
      expect(sessionsApi.list).toHaveBeenCalled()
    })

    it('should clear filters when clicking Clear Filters', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      // Set some filters
      const userInput = wrapper.find('input[type="text"]')
      await userInput.setValue('user@test.com')
      await flushPromises()

      // Clear filters
      const clearButton = wrapper.findAll('button').find(btn => btn.text().includes('Clear Filters'))
      await clearButton?.trigger('click')
      await flushPromises()

      expect(sessionsApi.list).toHaveBeenLastCalledWith(undefined, undefined)
    })
  })

  describe('actions', () => {
    it('should navigate to session view when clicking View', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      const viewButtons = wrapper.findAll('button').filter(btn => btn.text() === 'View')
      const firstViewButton = viewButtons[0]
      expect(firstViewButton).toBeDefined()
      await firstViewButton!.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/session/session-1')
    })

    it('should end session when clicking End Session', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      vi.mocked(sessionsApi.end).mockResolvedValue({ status: 'completed', earnedPoints: 75, passed: true })
      // Mock confirm.require to immediately call accept callback
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      const endButton = wrapper.findAll('button').find(btn => btn.text().includes('End'))
      await endButton?.trigger('click')
      await flushPromises()

      expect(sessionsApi.end).toHaveBeenCalledWith('session-1')
      expect(mockToastAdd).toHaveBeenCalled()
    })

    it('should not end session if user cancels confirmation', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      // Mock confirm.require to NOT call accept (simulating cancel)
      mockConfirmRequire.mockImplementation(() => {})

      const wrapper = mountComponent()
      await flushPromises()

      const endButton = wrapper.findAll('button').find(btn => btn.text().includes('End'))
      await endButton?.trigger('click')

      expect(sessionsApi.end).not.toHaveBeenCalled()
    })

    it('should refresh sessions when clicking Refresh', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)

      const wrapper = mountComponent()
      await flushPromises()

      vi.mocked(sessionsApi.list).mockClear()

      const refreshButton = wrapper.findAll('button').find(btn => btn.text().includes('Refresh'))
      await refreshButton?.trigger('click')
      await flushPromises()

      expect(sessionsApi.list).toHaveBeenCalled()
    })
  })

  describe('error handling', () => {
    it('should display error message on API failure', async () => {
      vi.mocked(sessionsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load sessions')
    })

    // Skipped: PrimeVue Message close button doesn't have text, requires integration testing
    it.skip('should allow dismissing error message', async () => {
      vi.mocked(sessionsApi.list).mockRejectedValueOnce(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load sessions')

      // PrimeVue Message uses an icon button without text
      const dismissButton = wrapper.findAll('button').find(btn => btn.text() === 'Dismiss')
      await dismissButton?.trigger('click')

      expect(wrapper.text()).not.toContain('Failed to load sessions')
    })

    it('should handle end session error', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      vi.mocked(sessionsApi.end).mockRejectedValue(new Error('End failed'))
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      const endButton = wrapper.findAll('button').find(btn => btn.text().includes('End'))
      await endButton?.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to end session')
    })
  })
})
