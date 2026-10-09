import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import LabsView from './LabsView.vue'
import { labsApi, podsApi, type Lab } from '@/api'

// Mock the API client
vi.mock('@/api', () => ({
  labsApi: {
    list: vi.fn(),
  },
  podsApi: {
    create: vi.fn(),
  },
}))

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { email: 'test@example.com' },
    isAuthenticated: true,
  })),
}))

const mockLabs: Lab[] = [
  {
    id: 'lab-1',
    name: 'Network Basics',
    description: 'Learn networking fundamentals',
    difficulty: 'beginner',
    durationMinutes: 60,
    platform: 'proxmox',
    tags: ['networking', 'beginner'],
    maxPoints: 100,
  },
  {
    id: 'lab-2',
    name: 'Advanced Routing',
    description: 'Configure complex routing scenarios',
    difficulty: 'advanced',
    durationMinutes: 120,
    platform: 'proxmox',
    tags: ['routing', 'advanced'],
    maxPoints: 200,
  },
]
const mockLabsResponse = { labs: mockLabs, count: mockLabs.length }

describe('LabsView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/labs', name: 'labs', component: LabsView },
        { path: '/pods/:podId', name: 'pod-detail', component: { template: '<div>Pod</div>' } },
      ],
    })

    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(LabsView, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: true,
          SkeletonCardGrid: { template: '<div class="skeleton-loading">Loading...</div>' },
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner initially', async () => {
      vi.mocked(labsApi.list).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      expect(wrapper.find('.skeleton-loading').exists()).toBe(true)
    })

    it('should fetch labs on mount', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      mountComponent()
      await flushPromises()

      expect(labsApi.list).toHaveBeenCalled()
    })

    it('should display labs after loading', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Network Basics')
      expect(wrapper.text()).toContain('Advanced Routing')
    })

    it('should show empty state when no labs', async () => {
      vi.mocked(labsApi.list).mockResolvedValue({ labs: [], count: 0 })

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No labs available')
    })
  })

  describe('lab card display', () => {
    it('should display lab name and description', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Network Basics')
      expect(wrapper.text()).toContain('Learn networking fundamentals')
    })

    it('should display difficulty badge', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('beginner')
      expect(wrapper.text()).toContain('advanced')
    })

    it('should display duration', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Duration is formatted as hours (60 min = 1h, 120 min = 2h)
      expect(wrapper.text()).toContain('1h')
      expect(wrapper.text()).toContain('2h')
    })

    it('should display tags', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('networking')
      expect(wrapper.text()).toContain('routing')
    })
  })

  describe('launch pod', () => {
    it('should launch pod when clicking Launch button', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.create).mockResolvedValue({
        id: 'pod-new',
        labTemplate: 'lab-1',
        platform: 'proxmox',
        owner: 'test@example.com',
        status: 'provisioning',
        vms: [],
        createdAt: new Date().toISOString(),
      })
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      const launchButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Launch'))
      const firstLaunchButton = launchButtons[0]
      expect(firstLaunchButton).toBeDefined()
      await firstLaunchButton!.trigger('click')
      await flushPromises()

      expect(podsApi.create).toHaveBeenCalledWith('lab-1', 'test@example.com')
      expect(pushSpy).toHaveBeenCalledWith('/pods/pod-new')
    })

    it('should show loading state while launching', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      let resolveCreate: (value: unknown) => void
      const createPromise = new Promise(resolve => {
        resolveCreate = resolve
      })
      vi.mocked(podsApi.create).mockReturnValue(createPromise as never)

      const wrapper = mountComponent()
      await flushPromises()

      const launchButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Launch'))
      const firstLaunchButton = launchButtons[0]
      expect(firstLaunchButton).toBeDefined()
      const launchPromise = firstLaunchButton!.trigger('click')

      await flushPromises()

      // Check for loading state - button should be disabled during launch
      expect(firstLaunchButton!.attributes('disabled')).toBeDefined()

      resolveCreate!({
        id: 'pod-new',
        labTemplate: 'lab-1',
        platform: 'proxmox',
        owner: 'test@example.com',
        status: 'provisioning',
        vms: [],
        createdAt: new Date().toISOString(),
      })
      await launchPromise
      await flushPromises()
    })

    it('should show error on launch failure', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.create).mockRejectedValue(new Error('Failed to create pod'))

      const wrapper = mountComponent()
      await flushPromises()

      const launchButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Launch'))
      const firstLaunchButton = launchButtons[0]
      expect(firstLaunchButton).toBeDefined()
      await firstLaunchButton!.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to launch Network Basics')
    })
  })

  describe('error handling', () => {
    it('should display error on load failure', async () => {
      vi.mocked(labsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load lab catalog')
    })

    it('should show empty state with error message', async () => {
      vi.mocked(labsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      // The component shows error and empty state together
      expect(wrapper.text()).toContain('Failed to load lab catalog')
      expect(wrapper.text()).toContain('No labs available')
    })

    it('should load correctly on fresh mount', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Network Basics')
    })
  })
})
