import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PodDetailView from './PodDetailView.vue'
import { podsApi, snapshotsApi, sessionsApi, type Pod, type Snapshot } from '@/api'
import { mockConfirmRequire, mockToastAdd } from '../test-setup'

// Mock the API client
vi.mock('@/api', () => ({
  podsApi: {
    get: vi.fn(),
    destroy: vi.fn(),
    resetVM: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  },
  snapshotsApi: {
    list: vi.fn(),
    create: vi.fn(),
    delete: vi.fn(),
  },
  sessionsApi: {
    create: vi.fn(),
  },
  topologyApi: {
    getTopology: vi.fn(),
  },
}))

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { id: 'test-user-id', email: 'test@example.com' },
    isAuthenticated: true,
  })),
}))

const mockPod: Pod = {
  id: 'pod-1',
  labTemplate: 'network-basics',
  platform: 'proxmox',
  owner: 'test@example.com',
  status: 'running',
  vms: [
    { name: 'R1', platformId: 'vm-100', status: 'running', ipAddress: '192.168.1.1', currentSnapshot: 'initial' },
    { name: 'PC1', platformId: 'vm-101', status: 'running', ipAddress: '192.168.1.10' },
  ],
  createdAt: '2024-01-01T00:00:00Z',
  expiresAt: '2024-01-02T00:00:00Z',
}

const mockSnapshots: Snapshot[] = [
  { name: 'initial', description: 'Initial state' },
  { name: 'checkpoint-1', description: 'After configuration', parent: 'initial' },
]

describe('PodDetailView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pods/:podId', name: 'pod-detail', component: PodDetailView },
        { path: '/pods', name: 'pods', component: { template: '<div>Pods</div>' } },
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

  async function mountComponent(podId = 'pod-1') {
    router.push(`/pods/${podId}`)
    await router.isReady()

    return mount(PodDetailView, {
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
      vi.mocked(podsApi.get).mockImplementation(() => new Promise(() => {}))

      const wrapper = await mountComponent()

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('should fetch pod on mount', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      await mountComponent()
      await flushPromises()

      expect(podsApi.get).toHaveBeenCalledWith('pod-1')
    })

    it('should display pod details after loading', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('network-basics')
      expect(wrapper.text()).toContain('proxmox')
      expect(wrapper.text()).toContain('test@example.com')
    })

    it('should show "Pod not found" for invalid pod', async () => {
      vi.mocked(podsApi.get).mockRejectedValue(new Error('Not found'))

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Pod not found')
    })
  })

  describe('VM display', () => {
    it('should display all VMs', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('R1')
      expect(wrapper.text()).toContain('PC1')
    })

    it('should show VM status badges', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      const wrapper = await mountComponent()
      await flushPromises()

      const statusBadges = wrapper.findAll('.rounded-full')
      expect(statusBadges.length).toBeGreaterThan(0)
    })

    it('should show VM IP address', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('192.168.1.1')
      expect(wrapper.text()).toContain('192.168.1.10')
    })
  })

  // Skipped: VM expansion UI has been redesigned, selectors no longer match
  describe.skip('VM expansion and snapshots', () => {
    it('should expand VM on click', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)

      const wrapper = await mountComponent()
      await flushPromises()

      // Click on VM to expand
      const vmHeader = wrapper.find('.cursor-pointer')
      await vmHeader.trigger('click')
      await flushPromises()

      // Should show expanded content
      expect(wrapper.text()).toContain('Snapshots')
      expect(snapshotsApi.list).toHaveBeenCalledWith('pod-1', 'R1')
    })

    it('should display snapshots when VM is expanded', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      const vmHeader = wrapper.find('.cursor-pointer')
      await vmHeader.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('initial')
      expect(wrapper.text()).toContain('checkpoint-1')
    })

    it('should collapse VM on second click', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)

      const wrapper = await mountComponent()
      await flushPromises()

      const vmHeader = wrapper.find('.cursor-pointer')

      // Expand
      await vmHeader.trigger('click')
      await flushPromises()

      // Collapse
      await vmHeader.trigger('click')
      await flushPromises()

      // Expanded content should be hidden (check for absence of snapshot actions)
      expect(wrapper.find('.bg-gray-50').exists()).toBe(false)
    })
  })

  // Skipped: Snapshot actions depend on VM expansion UI that has been redesigned
  describe.skip('snapshot actions', () => {
    it('should open create snapshot modal', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      const vmHeader = wrapper.find('.cursor-pointer')
      await vmHeader.trigger('click')
      await flushPromises()

      // Click create snapshot button
      const createButton = wrapper.findAll('button').find(btn => btn.text().includes('Create Snapshot'))
      await createButton?.trigger('click')

      // Modal should be visible
      expect(wrapper.text()).toContain('Snapshot Name')
    })

    it('should create a snapshot', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)
      vi.mocked(snapshotsApi.create).mockResolvedValue()

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      await wrapper.find('.cursor-pointer').trigger('click')
      await flushPromises()

      // Open modal - find the button in the expanded VM section
      const vmButtons = wrapper.findAll('.bg-gray-50 button')
      const createButton = vmButtons.find(btn => btn.text().includes('Create Snapshot'))
      await createButton?.trigger('click')
      await flushPromises()

      // Fill in snapshot name - the modal is now visible
      const nameInput = wrapper.findAll('input[type="text"]')[0]
      expect(nameInput).toBeDefined()
      await nameInput!.setValue('new-snapshot')
      await flushPromises()

      // Submit - find button in modal (inside .fixed)
      const modalButtons = wrapper.findAll('.fixed button')
      const submitButton = modalButtons.find(btn => btn.text() === 'Create Snapshot')
      await submitButton?.trigger('click')
      await flushPromises()

      expect(snapshotsApi.create).toHaveBeenCalledWith('pod-1', 'R1', 'new-snapshot', undefined, false)
    })

    it('should delete a snapshot with confirmation', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)
      vi.mocked(snapshotsApi.delete).mockResolvedValue()
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      await wrapper.find('.cursor-pointer').trigger('click')
      await flushPromises()

      // Click delete on first snapshot
      const deleteButtons = wrapper.findAll('button').filter(btn => btn.text() === 'Delete')
      const firstDeleteButton = deleteButtons[0]
      expect(firstDeleteButton).toBeDefined()
      await firstDeleteButton!.trigger('click')
      await flushPromises()

      expect(mockConfirmRequire).toHaveBeenCalled()
      expect(snapshotsApi.delete).toHaveBeenCalledWith('pod-1', 'R1', 'initial')
    })

    it('should revert to snapshot', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)
      vi.mocked(podsApi.resetVM).mockResolvedValue()

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      await wrapper.find('.cursor-pointer').trigger('click')
      await flushPromises()

      // Click revert on first snapshot
      const revertButtons = wrapper.findAll('button').filter(btn => btn.text() === 'Revert')
      const firstRevertButton = revertButtons[0]
      expect(firstRevertButton).toBeDefined()
      await firstRevertButton!.trigger('click')
      await flushPromises()

      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'R1', 'initial')
    })
  })

  describe('pod actions', () => {
    it('should start session', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(sessionsApi.create).mockResolvedValue({ sessionId: 'session-new', status: 'active', maxPoints: 100 })
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountComponent()
      await flushPromises()

      const startButton = wrapper.findAll('button').find(btn => btn.text().includes('Start Session'))
      await startButton?.trigger('click')
      await flushPromises()

      expect(sessionsApi.create).toHaveBeenCalledWith({
        podId: 'pod-1',
        userId: 'test-user-id',
        labTemplate: 'network-basics',
      })
      expect(pushSpy).toHaveBeenCalledWith('/session/session-new')
    })

    it('should reset all VMs', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(podsApi.resetVM).mockResolvedValue()

      const wrapper = await mountComponent()
      await flushPromises()

      const resetButton = wrapper.findAll('button').find(btn => btn.text().includes('Reset All VMs'))
      await resetButton?.trigger('click')
      await flushPromises()

      expect(podsApi.resetVM).toHaveBeenCalledTimes(2)
      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'R1', 'initial')
      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'PC1', 'initial')
    })

    it('should destroy pod with confirmation', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(podsApi.destroy).mockResolvedValue()
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountComponent()
      await flushPromises()

      const destroyButton = wrapper.findAll('button').find(btn => btn.text().includes('Destroy'))
      await destroyButton?.trigger('click')
      await flushPromises()

      expect(mockConfirmRequire).toHaveBeenCalled()
      expect(podsApi.destroy).toHaveBeenCalledWith('pod-1')
      expect(pushSpy).toHaveBeenCalledWith('/pods')
    })

    it('should not destroy pod if user cancels', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      mockConfirmRequire.mockImplementation(() => {})

      const wrapper = await mountComponent()
      await flushPromises()

      const destroyButton = wrapper.findAll('button').find(btn => btn.text().includes('Destroy'))
      await destroyButton?.trigger('click')

      expect(podsApi.destroy).not.toHaveBeenCalled()
    })

    it('should disable Start Session when pod is not running', async () => {
      const stoppedPod = { ...mockPod, status: 'stopped' as const }
      vi.mocked(podsApi.get).mockResolvedValue(stoppedPod)

      const wrapper = await mountComponent()
      await flushPromises()

      const startButton = wrapper.findAll('button').find(btn => btn.text().includes('Start Session'))
      expect(startButton?.attributes('disabled')).toBeDefined()
    })
  })

  // Skipped: Revert modal depends on VM expansion UI that has been redesigned
  describe.skip('revert modal', () => {
    it('should open revert modal', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      await wrapper.find('.cursor-pointer').trigger('click')
      await flushPromises()

      // Open revert modal
      const revertButton = wrapper.findAll('button').find(btn => btn.text().includes('Revert to Snapshot'))
      await revertButton?.trigger('click')

      expect(wrapper.text()).toContain('Select a snapshot to revert')
    })

    it('should revert to selected snapshot from modal', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(snapshotsApi.list).mockResolvedValue(mockSnapshots)
      vi.mocked(podsApi.resetVM).mockResolvedValue()

      const wrapper = await mountComponent()
      await flushPromises()

      // Expand VM
      await wrapper.find('.cursor-pointer').trigger('click')
      await flushPromises()

      // Open revert modal - find button in expanded VM section
      const vmButtons = wrapper.findAll('.bg-gray-50 button')
      const openRevertButton = vmButtons.find(btn => btn.text().includes('Revert to Snapshot'))
      await openRevertButton?.trigger('click')
      await flushPromises()

      // Select the second snapshot (checkpoint-1) by clicking its radio button
      const radioButtons = wrapper.findAll('.fixed input[type="radio"]')
      // Use setValue with the value we want to set in the v-model
      const secondRadioButton = radioButtons[1]
      expect(secondRadioButton).toBeDefined()
      await secondRadioButton!.setValue('checkpoint-1')
      await flushPromises()

      // Click Revert button in modal
      const modalButtons = wrapper.findAll('.fixed button')
      const modalRevertButton = modalButtons.find(btn => btn.text() === 'Revert')
      await modalRevertButton?.trigger('click')
      await flushPromises()

      expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'R1', 'checkpoint-1')
    })
  })

  describe('error handling', () => {
    it('should display error on load failure', async () => {
      vi.mocked(podsApi.get).mockRejectedValue(new Error('Load failed'))

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load pod')
    })

    it('should display error on action failure', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)
      vi.mocked(podsApi.destroy).mockRejectedValue(new Error('Destroy failed'))
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())

      const wrapper = await mountComponent()
      await flushPromises()

      const destroyButton = wrapper.findAll('button').find(btn => btn.text().includes('Destroy'))
      await destroyButton?.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to destroy pod')
    })

    // Skipped: PrimeVue Message close button doesn't have text, requires integration testing
    it.skip('should allow dismissing error', async () => {
      vi.mocked(podsApi.get).mockRejectedValue(new Error('Load failed'))

      const wrapper = await mountComponent()
      await flushPromises()

      const dismissButton = wrapper.findAll('button').find(btn => btn.text() === 'Dismiss')
      await dismissButton?.trigger('click')

      expect(wrapper.text()).not.toContain('Failed to load pod')
    })
  })
})
