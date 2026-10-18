import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import CreatePathwayView from './CreatePathwayView.vue'
import { labsApi, pathwaysApi, type Lab, type Pathway } from '@/api'
import { mockToastAdd } from '../test-setup'

// Mock the API client
vi.mock('@/api', () => ({
  labsApi: {
    list: vi.fn(),
  },
  pathwaysApi: {
    create: vi.fn(),
    createModule: vi.fn(),
    addLabToModule: vi.fn(),
  },
}))

// Mock the auth store - instructor by default
const mockAuthStore = {
  user: { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] },
  isAuthenticated: true,
  isAdmin: false,
}

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => mockAuthStore),
}))

const mockLabs: Lab[] = [
  {
    id: 'lab-1',
    name: 'Linux Basics',
    description: 'Learn Linux fundamentals',
    difficulty: 'beginner',
    durationMinutes: 30,
    platform: 'proxmox',
    maxPoints: 100,
  },
  {
    id: 'lab-2',
    name: 'Shell Scripting',
    description: 'Learn bash scripting',
    difficulty: 'intermediate',
    durationMinutes: 45,
    platform: 'proxmox',
    maxPoints: 150,
  },
]
const mockLabsResponse = { labs: mockLabs, count: mockLabs.length }

const mockPathway: Pathway = {
  id: 'pathway-123',
  name: 'Test Pathway',
  slug: 'test-pathway',
  description: 'A test pathway',
  difficulty: 'beginner',
  status: 'draft',
  visibility: 'global',
  displayOrder: 0,
  isFeatured: false,
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
}

const mockModule = {
  id: 'module-1',
  pathwayId: 'pathway-123',
  name: 'Getting Started',
  slug: 'getting-started',
  displayOrder: 0,
  unlockType: 'sequential' as const,
  isActive: true,
  createdAt: '2024-01-01T00:00:00Z',
}

describe('CreatePathwayView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pathways/create', name: 'create-pathway', component: CreatePathwayView },
        { path: '/pathways', name: 'pathways', component: { template: '<div>Pathways</div>' } },
        { path: '/pathways/:slug', name: 'pathway-detail', component: { template: '<div>Pathway</div>' } },
      ],
    })

    // Reset auth store to instructor
    mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
    mockAuthStore.isAdmin = false

    vi.clearAllMocks()
    mockToastAdd.mockClear()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(CreatePathwayView, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
  }

  describe('access control', () => {
    it('should redirect non-instructors to pathways list', async () => {
      mockAuthStore.user = { email: 'student@example.com', name: 'Student', roles: ['student'] }
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      const pushSpy = vi.spyOn(router, 'push')

      mountComponent()
      await flushPromises()

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'error',
          summary: 'Access Denied',
        })
      )
      expect(pushSpy).toHaveBeenCalledWith('/pathways')
    })

    it('should allow instructors to access', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Create Pathway')
    })

    it('should allow admins to access', async () => {
      mockAuthStore.isAdmin = true
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Create Pathway')
    })
  })

  describe('initial render', () => {
    it('should display wizard step indicator', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Basic Info')
      expect(wrapper.text()).toContain('Modules')
      expect(wrapper.text()).toContain('Labs')
      expect(wrapper.text()).toContain('Review')
    })

    it('should start on step 1', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Basic Information')
      expect(wrapper.text()).toContain('Pathway Name')
    })

    it('should fetch labs on mount', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      mountComponent()
      await flushPromises()

      expect(labsApi.list).toHaveBeenCalled()
    })
  })

  describe('step 1 - basic info', () => {
    it('should require pathway name', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      const nextButton = wrapper.findAll('button').find(btn => btn.text().includes('Next'))
      expect(nextButton?.attributes('disabled')).toBeDefined()
    })

    it('should enable next button when name is provided', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      await wrapper.vm.$nextTick()

      const nextButton = wrapper.findAll('button').find(btn => btn.text().includes('Next'))
      expect(nextButton?.attributes('disabled')).toBeUndefined()
    })

    it('should allow adding tags', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test'
      wrapper.vm.tagInput = 'linux'
      wrapper.vm.addTag()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.pathwayData.tags).toContain('linux')
    })
  })

  describe('step 2 - modules', () => {
    it('should show module form on step 2', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Navigate to step 2
      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.currentStep = 2
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('Add Module')
      expect(wrapper.text()).toContain('Module Name')
      expect(wrapper.text()).toContain('Unlock Type')
    })

    it('should require at least one module to proceed', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.currentStep = 2
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('No modules added yet')
    })

    it('should add module when form is submitted', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.currentStep = 2
      wrapper.vm.newModuleName = 'Getting Started'
      wrapper.vm.addModule()
      await wrapper.vm.$nextTick()

      expect(wrapper.vm.modules.length).toBe(1)
      expect(wrapper.vm.modules[0]!.name).toBe('Getting Started')
    })
  })

  describe('step 3 - labs', () => {
    it('should show labs assignment on step 3', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.modules.splice(0, wrapper.vm.modules.length, {
        id: 'temp-1',
        name: 'Module 1',
        unlockType: 'sequential',
        labs: [],
      })
      wrapper.vm.currentStep = 3
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('Assign Labs to Modules')
      expect(wrapper.text()).toContain('Module 1')
    })
  })

  describe('step 4 - review', () => {
    it('should show summary on step 4', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.modules.splice(0, wrapper.vm.modules.length, {
        id: 'temp-1',
        name: 'Module 1',
        unlockType: 'sequential',
        labs: [{ labTemplateId: mockLabs[0]!.id, lab: mockLabs[0]!, isRequired: true }],
      })
      wrapper.vm.currentStep = 4
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('Review & Create')
      expect(wrapper.text()).toContain('1')  // Module count
    })
  })

  describe('pathway creation', () => {
    it('should create pathway with modules and labs', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.create).mockResolvedValue(mockPathway)
      vi.mocked(pathwaysApi.createModule).mockResolvedValue(mockModule)
      vi.mocked(pathwaysApi.addLabToModule).mockResolvedValue({} as never)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.pathwayData.description = 'A test'
      wrapper.vm.pathwayData.difficulty = 'beginner'
      wrapper.vm.modules.splice(0, wrapper.vm.modules.length, {
        id: 'temp-1',
        name: 'Module 1',
        unlockType: 'sequential',
        labs: [{ labTemplateId: 'lab-1', lab: mockLabs[0]!, isRequired: true }],
      })
      wrapper.vm.currentStep = 4
      await wrapper.vm.$nextTick()

      await wrapper.vm.createPathway()
      await flushPromises()

      expect(pathwaysApi.create).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Pathway',
          description: 'A test',
          difficulty: 'beginner',
        })
      )
      expect(pathwaysApi.createModule).toHaveBeenCalledWith(
        'pathway-123',
        expect.objectContaining({
          name: 'Module 1',
          unlockType: 'sequential',
        })
      )
      expect(pathwaysApi.addLabToModule).toHaveBeenCalledWith(
        'module-1',
        expect.objectContaining({
          labTemplateId: 'lab-1',
          isRequired: true,
        })
      )
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Pathway Created',
        })
      )
      expect(pushSpy).toHaveBeenCalledWith('/pathways/test-pathway')
    })

    it('should show error on creation failure', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.create).mockRejectedValue(new Error('Creation failed'))

      const wrapper = mountComponent()
      await flushPromises()

      wrapper.vm.pathwayData.name = 'Test Pathway'
      wrapper.vm.modules.splice(0, wrapper.vm.modules.length, {
        id: 'temp-1',
        name: 'Module 1',
        unlockType: 'sequential',
        labs: [],
      })
      wrapper.vm.currentStep = 4
      await wrapper.vm.$nextTick()

      await wrapper.vm.createPathway()
      await flushPromises()

      expect(wrapper.text()).toContain('Creation failed')
    })
  })

  describe('navigation', () => {
    it('should navigate between steps', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Step 1 -> 2
      wrapper.vm.pathwayData.name = 'Test'
      wrapper.vm.nextStep()
      expect(wrapper.vm.currentStep).toBe(2)

      // Step 2 -> 1
      wrapper.vm.prevStep()
      expect(wrapper.vm.currentStep).toBe(1)
    })

    it('should cancel and navigate to pathways', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      const cancelButton = wrapper.findAll('button').find(btn => btn.text().includes('Cancel'))
      await cancelButton?.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathways')
    })
  })
})
