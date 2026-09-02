import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import EditPathwayView from './EditPathwayView.vue'
import { labsApi, pathwaysApi, type Lab, type Pathway } from '@/api'
import { mockToastAdd, mockConfirmRequire } from '../test-setup'

// Mock the API client
vi.mock('@/api', () => ({
  labsApi: {
    list: vi.fn(),
  },
  pathwaysApi: {
    get: vi.fn(),
    update: vi.fn(),
    createModule: vi.fn(),
    updateModule: vi.fn(),
    deleteModule: vi.fn(),
    addLabToModule: vi.fn(),
    removeLabFromModule: vi.fn(),
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
  shortDescription: 'Short desc',
  difficulty: 'beginner',
  status: 'draft',
  visibility: 'global',
  displayOrder: 0,
  isFeatured: false,
  estimatedHours: 5,
  tags: ['linux', 'basics'],
  modules: [
    {
      id: 'module-1',
      pathwayId: 'pathway-123',
      name: 'Getting Started',
      slug: 'getting-started',
      description: 'First module',
      displayOrder: 0,
      unlockType: 'sequential',
      isActive: true,
      createdAt: '2024-01-01T00:00:00Z',
      labs: [
        {
          id: 'lab-entry-1',
          moduleId: 'module-1',
          labTemplateId: 'lab-1',
          labName: 'Linux Basics',
          displayOrder: 0,
          isRequired: true,
        },
      ],
    },
  ],
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
}

describe('EditPathwayView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pathways/:slug/edit', name: 'edit-pathway', component: EditPathwayView },
        { path: '/pathways', name: 'pathways', component: { template: '<div>Pathways</div>' } },
        { path: '/pathways/:slug', name: 'pathway-detail', component: { template: '<div>Pathway</div>' } },
      ],
    })

    // Reset auth store to instructor
    mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
    mockAuthStore.isAdmin = false

    vi.clearAllMocks()
    mockToastAdd.mockClear()
    mockConfirmRequire.mockClear()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  async function mountComponent(slug = 'test-pathway') {
    router.push(`/pathways/${slug}/edit`)
    await router.isReady()

    return mount(EditPathwayView, {
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
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      const pushSpy = vi.spyOn(router, 'push')

      await mountComponent()
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
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Edit Pathway')
    })
  })

  describe('initial render', () => {
    it('should show loading spinner while fetching pathway', async () => {
      vi.mocked(pathwaysApi.get).mockImplementation(() => new Promise(() => {}))
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('should fetch pathway on mount', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      await mountComponent()
      await flushPromises()

      expect(pathwaysApi.get).toHaveBeenCalledWith('test-pathway')
    })

    it('should populate form with existing pathway data', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Test Pathway')
      expect(wrapper.text()).toContain('Getting Started')  // Module name
    })

    it('should show draft badge for draft pathways', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Draft')
    })

    it('should show published badge for published pathways', async () => {
      const publishedPathway = { ...mockPathway, status: 'published' as const }
      vi.mocked(pathwaysApi.get).mockResolvedValue(publishedPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Published')
    })
  })

  describe('editing modules', () => {
    it('should display existing modules', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Getting Started')
      expect(wrapper.text()).toContain('sequential')
    })

    it('should add new module', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      // Fill in the new-module name input and click the "Add Module" button
      // exactly as a user would. Drives the component via its public UI.
      await wrapper.find('input[placeholder="e.g., Getting Started"]').setValue('New Module')
      const addBtn = wrapper.findAll('button').find(b => b.text().includes('Add Module'))
      await addBtn?.trigger('click')
      await flushPromises()

      // The existing mock module "Getting Started" should still be there,
      // and the new "New Module" should appear with a "New" badge.
      expect(wrapper.text()).toContain('Getting Started')
      expect(wrapper.text()).toContain('New Module')
    })

    it('should mark module as deleted', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      // Click the module-level trash button (pi-trash) to remove the
      // only module. Lab-level remove buttons use pi-times, so this
      // selector targets the module header delete.
      const trashBtn = wrapper.findAll('button').find(b => b.html().includes('pi-trash'))
      expect(trashBtn?.exists()).toBe(true)
      await trashBtn?.trigger('click')
      await flushPromises()

      // Once the existing module is marked deleted, the empty-state
      // message should appear.
      expect(wrapper.text()).toContain('No modules yet')
    })
  })

  describe('saving changes', () => {
    it('should save pathway changes', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.update).mockResolvedValue(mockPathway)
      vi.mocked(pathwaysApi.updateModule).mockResolvedValue({} as never)
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountComponent()
      await flushPromises()

      // Update the name via the real input and click "Save Changes".
      await wrapper
        .find('input[placeholder="e.g., Linux Fundamentals"]')
        .setValue('Updated Name')
      const saveBtn = wrapper.findAll('button').find(b => b.text().includes('Save Changes'))
      await saveBtn?.trigger('click')
      await flushPromises()

      expect(pathwaysApi.update).toHaveBeenCalledWith(
        'pathway-123',
        expect.objectContaining({
          name: 'Updated Name',
        })
      )
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Pathway Updated',
        })
      )
      expect(pushSpy).toHaveBeenCalledWith('/pathway/test-pathway')
    })

    it('should create new modules on save', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.update).mockResolvedValue(mockPathway)
      vi.mocked(pathwaysApi.updateModule).mockResolvedValue({} as never)
      vi.mocked(pathwaysApi.createModule).mockResolvedValue({
        id: 'new-module-1',
        pathwayId: 'pathway-123',
        name: 'New Module',
        slug: 'new-module',
        displayOrder: 1,
        unlockType: 'sequential',
        isActive: true,
        createdAt: '2024-01-01T00:00:00Z',
      })

      const wrapper = await mountComponent()
      await flushPromises()

      // Add a module via the UI, then click Save Changes.
      await wrapper
        .find('input[placeholder="e.g., Getting Started"]')
        .setValue('New Module')
      const addBtn = wrapper.findAll('button').find(b => b.text().includes('Add Module'))
      await addBtn?.trigger('click')
      const saveBtn = wrapper.findAll('button').find(b => b.text().includes('Save Changes'))
      await saveBtn?.trigger('click')
      await flushPromises()

      expect(pathwaysApi.createModule).toHaveBeenCalledWith(
        'pathway-123',
        expect.objectContaining({
          name: 'New Module',
        })
      )
    })

    it('should delete removed modules on save', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.update).mockResolvedValue(mockPathway)
      vi.mocked(pathwaysApi.deleteModule).mockResolvedValue(undefined)

      const wrapper = await mountComponent()
      await flushPromises()

      // Click the module-level trash (pi-trash) to remove the existing
      // module, then click Save Changes.
      const trashBtn = wrapper.findAll('button').find(b => b.html().includes('pi-trash'))
      await trashBtn?.trigger('click')
      const saveBtn = wrapper.findAll('button').find(b => b.text().includes('Save Changes'))
      await saveBtn?.trigger('click')
      await flushPromises()

      expect(pathwaysApi.deleteModule).toHaveBeenCalledWith('module-1')
    })

    it('should show error on save failure', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(pathwaysApi.update).mockRejectedValue(new Error('Update failed'))

      const wrapper = await mountComponent()
      await flushPromises()

      // Click Save Changes; the mocked pathwaysApi.update rejects.
      const saveBtn = wrapper.findAll('button').find(b => b.text().includes('Save Changes'))
      await saveBtn?.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('Update failed')
    })
  })

  describe('cancel', () => {
    it('should show confirmation dialog on cancel', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      const cancelButton = wrapper.findAll('button').find(btn => btn.text() === 'Cancel')
      await cancelButton?.trigger('click')

      expect(mockConfirmRequire).toHaveBeenCalledWith(
        expect.objectContaining({
          message: 'Are you sure you want to discard your changes?',
        })
      )
    })

    it('should navigate to pathway detail on confirm cancel', async () => {
      vi.mocked(pathwaysApi.get).mockResolvedValue(mockPathway)
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      mockConfirmRequire.mockImplementation(({ accept }) => accept?.())
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountComponent()
      await flushPromises()

      const cancelButton = wrapper.findAll('button').find(btn => btn.text() === 'Cancel')
      await cancelButton?.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathway/test-pathway')
    })
  })

  describe('error handling', () => {
    it('should display error when pathway fails to load', async () => {
      vi.mocked(pathwaysApi.get).mockRejectedValue(new Error('Not found'))
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load pathway')
    })
  })
})
