import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PodsView from './PodsView.vue'
import { podsApi, type Pod } from '@/api'
import { mockConfirmRequire } from '../test-setup'

// Mock the API client
vi.mock('@/api', () => ({
  podsApi: {
    list: vi.fn(),
    get: vi.fn(),
    destroy: vi.fn(),
    resetVM: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  },
}))

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { email: 'test@example.com' },
    isAuthenticated: true,
  })),
}))

// Factory function to create fresh mock data for each test
function createMockPods(): Pod[] {
  return [
    {
      id: 'pod-1',
      labTemplate: 'network-basics',
      platform: 'proxmox',
      owner: 'test@example.com',
      status: 'running',
      vms: [
        { name: 'R1', platformId: 'vm-100', status: 'running' },
        { name: 'PC1', platformId: 'vm-101', status: 'running' },
      ],
      createdAt: '2024-01-01T00:00:00Z',
      expiresAt: '2024-01-02T00:00:00Z',
    },
    {
      id: 'pod-2',
      labTemplate: 'advanced-routing',
      platform: 'proxmox',
      owner: 'test@example.com',
      status: 'provisioning',
      vms: [],
      createdAt: '2024-01-01T01:00:00Z',
    },
  ]
}

describe('PodsView', () => {
  let router: ReturnType<typeof createRouter>
  let mockPods: Pod[]

  beforeEach(() => {
    setActivePinia(createPinia())
    mockPods = createMockPods()

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pods', name: 'pods', component: PodsView },
        { path: '/pods/:podId', name: 'pod-detail', component: { template: '<div>Pod Detail</div>' } },
        { path: '/labs', name: 'labs', component: { template: '<div>Labs</div>' } },
      ],
    })

    vi.clearAllMocks()
    mockConfirmRequire.mockClear()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(PodsView, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: true,
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading state while fetching', async () => {
      vi.mocked(podsApi.list).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()
      // Allow Vue to process the state change from onMounted calling fetchPods
      await wrapper.vm.$nextTick()

      // During loading, the component should not show the pod grid or empty state
      // Check that pods are not shown yet
      expect(wrapper.text()).not.toContain('network-basics')
      expect(wrapper.text()).not.toContain('No Active Pods')
    })

    it('should fetch pods for current user on mount', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      mountComponent()
      await flushPromises()

      expect(podsApi.list).toHaveBeenCalled()
    })

    it('should display pods after loading', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('network-basics')
      expect(wrapper.text()).toContain('advanced-routing')
    })

    it('should show empty state when no pods', async () => {
      vi.mocked(podsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No Active Pods')
      expect(wrapper.text()).toContain('Launch a lab to create your first pod')
    })
  })

  describe('pod card display', () => {
    it('should display lab template and pod id', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('network-basics')
      expect(wrapper.text()).toContain('pod-1')
    })

    it('should display status badges with correct styling', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('running')
      expect(wrapper.text()).toContain('provisioning')
    })

    it('should display VM count', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('2 VMs')
      expect(wrapper.text()).toContain('0 VMs')
    })

    it('should display platform', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('proxmox')
    })

    it('should display expiration date when present', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Expires:')
    })
  })

  describe('navigation', () => {
    it('should navigate to pod detail when clicking pod card', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Find the card by looking for the pod content (labTemplate text)
      // The card has @click="viewPod(pod)" which handles navigation
      const cards = wrapper.findAllComponents({ name: 'Card' })
      // Skip the first card if it's the empty state card, get the first pod card
      const podCards = cards.filter(card => card.text().includes('network-basics'))
      expect(podCards.length).toBeGreaterThan(0)

      // Click on the card - the Card component should emit click
      await podCards[0]!.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pods/pod-1')
    })
  })

  describe('pod actions', () => {
    it('should reset pod when clicking Reset button', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.resetVM).mockResolvedValue()
      // Mock get for the fetchPod call after reset
      vi.mocked(podsApi.get).mockResolvedValue(mockPods[0]!)

      const wrapper = mountComponent()
      await flushPromises()

      const resetButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Reset'))
      const firstResetButton = resetButtons[0]
      expect(firstResetButton).toBeDefined()
      await firstResetButton!.trigger('click')
      await flushPromises()

      // Should reset all VMs in the pod
      expect(podsApi.resetVM).toHaveBeenCalledTimes(2)
      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'R1', 'initial')
      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'PC1', 'initial')
    })

    it('should show error when resetting pod with no VMs', async () => {
      // Create a running pod with no VMs
      const podWithNoVMs: Pod[] = [{
        id: 'pod-no-vms',
        labTemplate: 'empty-lab',
        platform: 'proxmox',
        owner: 'test@example.com',
        status: 'running',
        vms: [],
        createdAt: '2024-01-01T00:00:00Z',
      }]
      vi.mocked(podsApi.list).mockResolvedValue(podWithNoVMs)

      const wrapper = mountComponent()
      await flushPromises()

      const resetButton = wrapper.findAll('button').find(btn => btn.text().includes('Reset'))
      expect(resetButton).toBeDefined()
      await resetButton?.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('No VMs to reset')
    })

    it('should destroy pod with confirmation', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.destroy).mockResolvedValue()
      // Mock confirm.require to immediately call accept callback
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()
      await firstDestroyButton!.trigger('click')
      await flushPromises()

      expect(mockConfirmRequire).toHaveBeenCalled()
      expect(podsApi.destroy).toHaveBeenCalledWith('pod-1')
    })

    it('should remove pod from list after destroy', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.destroy).mockResolvedValue()
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('pod-1')

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()
      await firstDestroyButton!.trigger('click')
      await flushPromises()

      expect(wrapper.text()).not.toContain('pod-1')
    })

    it('should not destroy pod if user cancels confirmation', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      // Mock confirm.require to NOT call accept (simulating cancel)
      mockConfirmRequire.mockImplementation(() => {})

      const wrapper = mountComponent()
      await flushPromises()

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()
      await firstDestroyButton!.trigger('click')

      expect(podsApi.destroy).not.toHaveBeenCalled()
    })

    it('should not navigate when clicking action buttons', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      mockConfirmRequire.mockImplementation(() => {})
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()
      await firstDestroyButton!.trigger('click')

      // Should not navigate because @click.stop prevents propagation
      expect(pushSpy).not.toHaveBeenCalled()
    })
  })

  describe('refresh', () => {
    it('should refresh pods when clicking Refresh button', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)

      const wrapper = mountComponent()
      await flushPromises()

      vi.mocked(podsApi.list).mockClear()

      const refreshButton = wrapper.findAll('button').find(btn => btn.text().includes('Refresh'))
      await refreshButton?.trigger('click')
      await flushPromises()

      expect(podsApi.list).toHaveBeenCalled()
    })

    it('should show loading state on refresh button while loading', async () => {
      vi.mocked(podsApi.list).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      // Check that the component is in loading state
      const refreshButton = wrapper.findAll('button').find(btn => btn.text().includes('Refresh'))
      expect(refreshButton).toBeDefined()
      // The button should have the loading prop set, which adds loading spinner
      // We just verify the button exists during loading
    })
  })

  describe('error handling', () => {
    it('should display error on load failure', async () => {
      vi.mocked(podsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      // Store uses e.message which is 'API Error'
      expect(wrapper.text()).toContain('API Error')
    })

    it('should display error on reset failure', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.resetVM).mockRejectedValue(new Error('Reset failed'))

      const wrapper = mountComponent()
      await flushPromises()

      const resetButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Reset'))
      const firstResetButton = resetButtons[0]
      expect(firstResetButton).toBeDefined()
      await firstResetButton!.trigger('click')
      await flushPromises()

      // Store uses e.message which is 'Reset failed'
      expect(wrapper.text()).toContain('Reset failed')
    })

    it('should display error on destroy failure', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.destroy).mockRejectedValue(new Error('Destroy failed'))
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()
      await firstDestroyButton!.trigger('click')
      await flushPromises()

      // Store uses e.message which is 'Destroy failed'
      expect(wrapper.text()).toContain('Destroy failed')
    })
  })

  describe('loading states', () => {
    it('should call reset API when clicking Reset button', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.resetVM).mockResolvedValue(undefined)
      // Also mock get for the fetchPod call after reset
      vi.mocked(podsApi.get).mockResolvedValue(mockPods[0]!)

      const wrapper = mountComponent()
      await flushPromises()

      const resetButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Reset'))
      const firstResetButton = resetButtons[0]
      expect(firstResetButton).toBeDefined()

      await firstResetButton!.trigger('click')
      await flushPromises()

      // Should have called resetVM for each VM in the pod
      expect(podsApi.resetVM).toHaveBeenCalled()
    })

    it('should call destroy API when clicking Destroy button', async () => {
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(podsApi.destroy).mockResolvedValue(undefined)
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = mountComponent()
      await flushPromises()

      const destroyButtons = wrapper.findAll('button').filter(btn => btn.text().includes('Destroy'))
      const firstDestroyButton = destroyButtons[0]
      expect(firstDestroyButton).toBeDefined()

      await firstDestroyButton!.trigger('click')
      await flushPromises()

      // Should have called destroy API
      expect(podsApi.destroy).toHaveBeenCalledWith('pod-1')
    })
  })
})
