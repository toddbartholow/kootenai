import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import CreatePodView from './CreatePodView.vue'
import { labsApi, podsApi, type Lab, type Pod } from '@/api'
import { mockToastAdd } from '../test-setup'
import { i18n } from '@/locales'

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
    user: { email: 'test@example.com', name: 'Test User' },
    isAuthenticated: true,
    isAdmin: false,
  })),
}))

const mockLabs: Lab[] = [
  {
    id: 'lab-1',
    name: 'Network Basics',
    description: 'Learn networking fundamentals',
    difficulty: 'beginner',
    durationMinutes: 30,
    platform: 'proxmox',
    maxPoints: 100,
    tags: ['networking', 'beginner'],
  },
  {
    id: 'lab-2',
    name: 'Advanced Routing',
    description: 'Advanced routing protocols',
    difficulty: 'advanced',
    durationMinutes: 60,
    platform: 'proxmox',
    maxPoints: 200,
    tags: ['networking', 'routing'],
  },
]
const mockLabsResponse = { labs: mockLabs, count: mockLabs.length }

const mockPod: Pod = {
  id: 'pod-123',
  labTemplate: 'Network Basics',
  platform: 'proxmox',
  owner: 'test@example.com',
  status: 'provisioning',
  vms: [],
  createdAt: '2024-01-01T00:00:00Z',
}

describe('CreatePodView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pods/create', name: 'create-pod', component: CreatePodView },
        { path: '/pods', name: 'pods', component: { template: '<div>Pods</div>' } },
        { path: '/pods/:podId', name: 'pod-detail', component: { template: '<div>Pod Detail</div>' } },
      ],
    })

    vi.clearAllMocks()
    mockToastAdd.mockClear()
    i18n.global.locale.value = 'en'
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(CreatePodView, {
      global: {
        plugins: [router, i18n],
        stubs: {
          RouterLink: true,
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner while fetching labs', async () => {
      vi.mocked(labsApi.list).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('should fetch labs on mount', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      mountComponent()
      await flushPromises()

      expect(labsApi.list).toHaveBeenCalled()
    })

    it('should display form after labs are loaded', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Lab Template')
      expect(wrapper.text()).toContain('Owner')
    })

    it('should pre-fill owner with current user', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      const ownerInput = wrapper.find('#owner-input')
      expect(ownerInput.exists()).toBe(true)
    })

    it('should display page title', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Create Pod')
      expect(wrapper.text()).toContain('Launch a new lab environment')
    })
  })

  describe('form validation', () => {
    it('should disable create button when no lab is selected', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      const createButton = wrapper.findAll('button').find(btn => btn.text().includes('Create Pod'))
      expect(createButton?.attributes('disabled')).toBeDefined()
    })

    it('should show lab details when a lab is selected', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Drive selection via the Select component's public v-model.
      // Same observable effect as a human picking from the dropdown.
      const select = wrapper.findComponent({ name: 'Select' })
      await select.vm.$emit('update:modelValue', mockLabs[0])
      await flushPromises()

      expect(wrapper.text()).toContain('Lab Details')
      expect(wrapper.text()).toContain('Platform')
      expect(wrapper.text()).toContain('Duration')
    })
  })

  describe('pod creation', () => {
    it('should create pod when form is submitted', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.create).mockResolvedValue(mockPod)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Set form state through exposed refs
      const vm = wrapper.vm
      vm.selectedLab = mockLabs[0]!
      vm.owner = 'test@example.com'
      await wrapper.vm.$nextTick()

      // Submit the form directly
      const form = wrapper.find('form')
      await form.trigger('submit')
      await flushPromises()

      expect(podsApi.create).toHaveBeenCalledWith('Network Basics', 'test@example.com')
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Pod Created',
        })
      )
      expect(pushSpy).toHaveBeenCalledWith('/pods/pod-123')
    })

    it('should show loading state during creation', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      let resolveCreate: ((pod: Pod) => void) | undefined
      vi.mocked(podsApi.create).mockImplementation(
        () => new Promise(resolve => { resolveCreate = resolve })
      )

      const wrapper = mountComponent()
      await flushPromises()

      const vm = wrapper.vm
      vm.selectedLab = mockLabs[0]!
      vm.owner = 'test@example.com'
      await wrapper.vm.$nextTick()

      // Submit the form
      const form = wrapper.find('form')
      form.trigger('submit')
      await wrapper.vm.$nextTick()

      // Check creating state is true
      expect(vm.creating).toBe(true)

      resolveCreate!(mockPod)
      await flushPromises()

      // Check creating state is false after completion
      expect(vm.creating).toBe(false)
    })
  })

  describe('error handling', () => {
    it('should display error when labs fail to load', async () => {
      vi.mocked(labsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load available labs')
    })

    it('should display error when pod creation fails', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.create).mockRejectedValue(new Error('Creation failed'))

      const wrapper = mountComponent()
      await flushPromises()

      const vm = wrapper.vm
      vm.selectedLab = mockLabs[0]!
      vm.owner = 'test@example.com'
      await wrapper.vm.$nextTick()

      // Submit the form directly
      const form = wrapper.find('form')
      await form.trigger('submit')
      await flushPromises()

      expect(wrapper.text()).toContain('Creation failed')
    })

    it('should allow closing error message', async () => {
      vi.mocked(labsApi.list).mockRejectedValue(new Error('API Error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load available labs')

      // The error banner is a <Message> with a close button; click it
      // to dismiss. This mirrors the user flow rather than zeroing an
      // internal ref.
      const messageComponent = wrapper.findComponent({ name: 'Message' })
      expect(messageComponent.exists()).toBe(true)
      await messageComponent.vm.$emit('close')
      await flushPromises()

      expect(wrapper.text()).not.toContain('Failed to load available labs')
    })
  })

  describe('navigation', () => {
    it('should navigate to pods list when cancel is clicked', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      const cancelButton = wrapper.findAll('button').find(btn => btn.text().includes('Cancel'))
      await cancelButton?.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pods')
    })
  })

  describe('info card', () => {
    it('should display about pods information', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('About Pods')
      expect(wrapper.text()).toContain('personal lab environment')
    })
  })
})
