import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PathwayDetailView from './PathwayDetailView.vue'
import { pathwaysApi, type Pathway } from '@/api'
import { mockToastAdd } from '../test-setup'

// Mock the pathway store
const mockPathwayStore = {
  pathway: null as Pathway | null,
  enrollment: null,
  loading: false,
  enrolling: false,
  errorMessage: null as string | null,
  isEnrolled: false,
  isPathwayComplete: false,
  overallProgress: 0,
  completedModules: 0,
  earnedPoints: 0,
  fetchPathway: vi.fn(),
  fetchEnrollment: vi.fn(),
  enroll: vi.fn(),
  unenroll: vi.fn(),
  getModuleStatus: vi.fn(() => 'locked'),
  getModuleProgress: vi.fn(() => 0),
  reset: vi.fn(),
  clearError: vi.fn(),
}

vi.mock('@/stores/pathway', () => ({
  usePathwayStore: vi.fn(() => mockPathwayStore),
}))

// Mock the auth store
const mockAuthStore = {
  user: { email: 'student@example.com', name: 'Student', roles: ['student'] },
  isAuthenticated: true,
  isAdmin: false,
  isInstructor: false,
}

vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn(() => mockAuthStore),
}))

// Mock the API client for publish
vi.mock('@/api', () => ({
  pathwaysApi: {
    publish: vi.fn(),
  },
}))

const mockDraftPathway: Pathway = {
  id: 'pathway-123',
  name: 'Test Pathway',
  slug: 'test-pathway',
  description: 'A test pathway',
  difficulty: 'beginner',
  status: 'draft',
  visibility: 'global',
  displayOrder: 0,
  isFeatured: false,
  estimatedHours: 10,
  moduleCount: 3,
  modules: [
    {
      id: 'module-1',
      pathwayId: 'pathway-123',
      name: 'Getting Started',
      slug: 'getting-started',
      displayOrder: 0,
      unlockType: 'always',
      isActive: true,
      createdAt: '2024-01-01T00:00:00Z',
      labs: [],
    },
  ],
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-01T00:00:00Z',
}

const mockPublishedPathway: Pathway = {
  ...mockDraftPathway,
  status: 'published',
}

describe('PathwayDetailView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pathways/:slug', name: 'pathway-detail', component: PathwayDetailView },
        { path: '/pathways', name: 'pathways', component: { template: '<div>Pathways</div>' } },
        { path: '/pathways/:slug/edit', name: 'edit-pathway', component: { template: '<div>Edit</div>' } },
        { path: '/labs/:labId', name: 'lab-detail', component: { template: '<div>Lab</div>' } },
      ],
    })

    // Reset stores
    mockPathwayStore.pathway = null
    mockPathwayStore.loading = false
    mockPathwayStore.errorMessage = null
    mockAuthStore.user = { email: 'student@example.com', name: 'Student', roles: ['student'] }
    mockAuthStore.isAdmin = false
    mockAuthStore.isInstructor = false

    vi.clearAllMocks()
    mockToastAdd.mockClear()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  async function mountComponent(slug = 'test-pathway') {
    router.push(`/pathways/${slug}`)
    await router.isReady()

    return mount(PathwayDetailView, {
      global: {
        plugins: [router],
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          PathwayHeader: true,
          PathwayProgressCard: true,
          PathwayLearningRoadmap: true,
          PathwayPrerequisites: true,
          ConfirmDialog: true,
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner while fetching pathway', async () => {
      mockPathwayStore.loading = true

      const wrapper = await mountComponent()

      expect(wrapper.find('.animate-spin').exists()).toBe(true)
    })

    it('should fetch pathway on mount', async () => {
      mockPathwayStore.pathway = mockPublishedPathway

      await mountComponent()
      await flushPromises()

      expect(mockPathwayStore.fetchPathway).toHaveBeenCalledWith('test-pathway')
    })

    it('should display pathway name in breadcrumb', async () => {
      mockPathwayStore.pathway = mockPublishedPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Test Pathway')
    })

    it('should display error when pathway load fails', async () => {
      mockPathwayStore.pathway = null
      mockPathwayStore.errorMessage = 'Pathway not found'

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Pathway not found')
    })
  })

  describe('draft badge', () => {
    it('should show Draft badge for draft pathways', async () => {
      mockPathwayStore.pathway = mockDraftPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Draft')
    })

    it('should NOT show Draft badge for published pathways', async () => {
      mockPathwayStore.pathway = mockPublishedPathway

      const wrapper = await mountComponent()
      await flushPromises()

      // Check that only "Learning Pathways" and pathway name are in breadcrumb
      const breadcrumbArea = wrapper.find('.flex.items-center.justify-between')
      expect(breadcrumbArea.text()).not.toContain('Draft')
    })
  })

  describe('instructor/admin actions', () => {
    it('should show Publish button for instructors on draft pathways', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Publish')
    })

    it('should show Edit button for instructors', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Edit')
    })

    it('should show Publish and Edit buttons for admins', async () => {
      mockAuthStore.isAdmin = true
      mockPathwayStore.pathway = mockDraftPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Publish')
      expect(wrapper.text()).toContain('Edit')
    })

    it('should NOT show Publish/Edit buttons for students', async () => {
      mockPathwayStore.pathway = mockDraftPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).not.toContain('Publish')
      expect(wrapper.text()).not.toContain('Edit')
    })

    it('should NOT show Publish button for already published pathways', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockPublishedPathway

      const wrapper = await mountComponent()
      await flushPromises()

      // Should still have Edit but not Publish
      expect(wrapper.text()).toContain('Edit')
      expect(wrapper.text()).not.toContain('Publish')
    })
  })

  describe('publish pathway', () => {
    it('should publish pathway when clicking Publish button', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway
      vi.mocked(pathwaysApi.publish).mockResolvedValue(mockPublishedPathway)

      const wrapper = await mountComponent()
      await flushPromises()

      const publishButton = wrapper.findAll('button').find(btn => btn.text().includes('Publish'))
      await publishButton?.trigger('click')
      await flushPromises()

      expect(pathwaysApi.publish).toHaveBeenCalledWith('pathway-123')
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Pathway Published',
        })
      )
    })

    it('should refresh pathway after publishing', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway
      vi.mocked(pathwaysApi.publish).mockResolvedValue(mockPublishedPathway)

      const wrapper = await mountComponent()
      await flushPromises()

      // Clear the initial call count
      mockPathwayStore.fetchPathway.mockClear()

      const publishButton = wrapper.findAll('button').find(btn => btn.text().includes('Publish'))
      await publishButton?.trigger('click')
      await flushPromises()

      expect(mockPathwayStore.fetchPathway).toHaveBeenCalledWith('test-pathway')
    })

    it('should show error toast on publish failure', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway
      vi.mocked(pathwaysApi.publish).mockRejectedValue(new Error('Publish failed'))

      const wrapper = await mountComponent()
      await flushPromises()

      const publishButton = wrapper.findAll('button').find(btn => btn.text().includes('Publish'))
      await publishButton?.trigger('click')
      await flushPromises()

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'error',
          summary: 'Publish Failed',
        })
      )
    })

    it('should show loading state during publishing', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway

      let resolvePublish: () => void
      vi.mocked(pathwaysApi.publish).mockImplementation(
        () => new Promise(resolve => {
          resolvePublish = () => resolve(mockPublishedPathway)
        })
      )

      const wrapper = await mountComponent()
      await flushPromises()

      const publishButton = wrapper.findAll('button').find(btn => btn.text().includes('Publish'))
      const publishPromise = publishButton?.trigger('click')
      await flushPromises()

      // Button should be disabled/loading
      expect(publishButton?.attributes('disabled')).toBeDefined()

      resolvePublish!()
      await publishPromise
    })
  })

  describe('edit navigation', () => {
    it('should navigate to edit page when clicking Edit button', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      mockAuthStore.isInstructor = true
      mockPathwayStore.pathway = mockDraftPathway
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = await mountComponent()
      await flushPromises()

      const editButton = wrapper.findAll('button').find(btn => btn.text().includes('Edit'))
      await editButton?.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathways/test-pathway/edit')
    })
  })

  describe('breadcrumb navigation', () => {
    it('should have link to pathways list', async () => {
      mockPathwayStore.pathway = mockPublishedPathway

      const wrapper = await mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Learning Pathways')
    })
  })
})
