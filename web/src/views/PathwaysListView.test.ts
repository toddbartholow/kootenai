import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import PathwaysListView from './PathwaysListView.vue'
import { pathwaysApi, enrollmentsApi, type Pathway, type PathwayEnrollment } from '@/api'

// Mock the API client
vi.mock('@/api', () => ({
  pathwaysApi: {
    list: vi.fn(),
    enroll: vi.fn(),
  },
  enrollmentsApi: {
    list: vi.fn(),
  },
}))

// Mock the auth store
const mockAuthStore = {
  user: { email: 'student@example.com', name: 'Student', roles: ['student'] },
  isAuthenticated: true,
  isAdmin: false,
}

vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => mockAuthStore),
}))

const mockPublishedPathways: Pathway[] = [
  {
    id: 'pathway-1',
    name: 'Linux Fundamentals',
    slug: 'linux-fundamentals',
    description: 'Learn Linux basics',
    shortDescription: 'Linux basics course',
    difficulty: 'beginner',
    status: 'published',
    visibility: 'global',
    displayOrder: 0,
    isFeatured: true,
    estimatedHours: 10,
    moduleCount: 5,
    tags: ['linux', 'fundamentals'],
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
  },
  {
    id: 'pathway-2',
    name: 'Advanced Networking',
    slug: 'advanced-networking',
    description: 'Advanced networking concepts',
    difficulty: 'advanced',
    status: 'published',
    visibility: 'global',
    displayOrder: 1,
    isFeatured: false,
    estimatedHours: 20,
    moduleCount: 8,
    tags: ['networking'],
    createdAt: '2024-01-02T00:00:00Z',
    updatedAt: '2024-01-02T00:00:00Z',
  },
]

const mockDraftPathways: Pathway[] = [
  {
    id: 'pathway-3',
    name: 'Draft Pathway',
    slug: 'draft-pathway',
    description: 'A draft pathway',
    difficulty: 'intermediate',
    status: 'draft',
    visibility: 'global',
    displayOrder: 2,
    isFeatured: false,
    estimatedHours: 5,
    moduleCount: 2,
    createdAt: '2024-01-03T00:00:00Z',
    updatedAt: '2024-01-03T00:00:00Z',
  },
]

const mockEnrollments: PathwayEnrollment[] = [
  {
    id: 'enrollment-1',
    pathwayId: 'pathway-1',
    userId: 'student@example.com',
    status: 'in_progress',
    percentage: 40,
    completedModules: 2,
    totalModules: 5,
    earnedPoints: 200,
    maxPoints: 500,
    enrolledAt: '2024-01-01T00:00:00Z',
    startedAt: '2024-01-01T00:00:00Z',
    certificateIssued: false,
  },
]

describe('PathwaysListView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/pathways', name: 'pathways', component: PathwaysListView },
        { path: '/pathways/create', name: 'create-pathway', component: { template: '<div>Create</div>' } },
        { path: '/pathway/:slug', name: 'pathway-interactive', component: { template: '<div>Pathway</div>' } },
        { path: '/enrollments/:id', name: 'enrollment', component: { template: '<div>Enrollment</div>' } },
      ],
    })

    // Reset auth store to student
    mockAuthStore.user = { email: 'student@example.com', name: 'Student', roles: ['student'] }
    mockAuthStore.isAdmin = false

    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(PathwaysListView, {
      global: {
        plugins: [router],
        stubs: {
          // Use component that renders slot content instead of stubbing to true
          RouterLink: { template: '<a><slot /></a>' },
          SkeletonCardGrid: { template: '<div class="skeleton-loading">Loading...</div>' },
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner while fetching pathways', async () => {
      vi.mocked(pathwaysApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()

      expect(wrapper.find('.skeleton-loading').exists()).toBe(true)
    })

    it('should display page title', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Learning Pathways')
      expect(wrapper.text()).toContain('Structured learning tracks')
    })

    it('should display published pathways', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Linux Fundamentals')
      expect(wrapper.text()).toContain('Advanced Networking')
    })

    it('should display featured pathways section', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Featured Pathways')
      expect(wrapper.text()).toContain('Linux Fundamentals')  // Featured pathway
    })
  })

  describe('instructor/admin features', () => {
    it('should show Create Pathway button for instructors', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Create Pathway')
    })

    it('should show Create Pathway button for admins', async () => {
      mockAuthStore.isAdmin = true
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Create Pathway')
    })

    it('should NOT show Create Pathway button for students', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).not.toContain('Create Pathway')
    })

    it('should fetch draft pathways for instructors', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }

      // First call returns published, second returns drafts
      vi.mocked(pathwaysApi.list)
        .mockResolvedValueOnce(mockPublishedPathways)
        .mockResolvedValueOnce(mockDraftPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      mountComponent()
      await flushPromises()

      expect(pathwaysApi.list).toHaveBeenCalledWith({ status: 'published' })
      expect(pathwaysApi.list).toHaveBeenCalledWith({ status: 'draft' })
    })

    it('should display draft pathways with badge for instructors', async () => {
      mockAuthStore.user = { email: 'instructor@example.com', name: 'Instructor', roles: ['instructor'] }

      vi.mocked(pathwaysApi.list)
        .mockResolvedValueOnce(mockPublishedPathways)
        .mockResolvedValueOnce(mockDraftPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Draft Pathway')
      expect(wrapper.text()).toContain('Draft')  // Badge
    })

    it('should NOT fetch draft pathways for students', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      mountComponent()
      await flushPromises()

      // Should only be called once for published pathways
      expect(pathwaysApi.list).toHaveBeenCalledTimes(1)
      expect(pathwaysApi.list).toHaveBeenCalledWith({ status: 'published' })
    })
  })

  describe('enrollments', () => {
    it('should display continue learning section for enrolled users', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Continue Learning')
      expect(wrapper.text()).toContain('40%')  // Progress
    })

    it('should show Continue button for enrolled pathways', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Continue')
    })

    it('should show Enroll button for non-enrolled pathways', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Enroll')
    })
  })

  describe('filtering', () => {
    // Helper: find the search input by its aria-label. Survives
    // refactors of the surrounding template as long as the a11y hook
    // stays in place.
    function searchInput(wrapper: ReturnType<typeof mount>) {
      return wrapper.find('input[aria-label="Search pathways by name, description, or tags"]')
    }

    it('should filter pathways by search query', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      // Drive the filter via the real input instead of reaching into
      // the component's internal refs. Works with v-model exactly as
      // a human typing would trigger it.
      await searchInput(wrapper).setValue('Linux')
      await flushPromises()

      const text = wrapper.text()
      expect(text).toContain('Linux Fundamentals')
      expect(text).toContain('1 of 2')
    })

    it('should filter pathways by difficulty', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      // The difficulty dropdown is a PrimeVue Select wrapping an
      // internal <select>. Emit an update:modelValue from the Select
      // stub mirrors the public two-way-binding API without poking
      // at internal refs.
      const difficultySelect = wrapper.findComponent({ name: 'Select' })
      await difficultySelect.vm.$emit('update:modelValue', 'beginner')
      await flushPromises()

      expect(wrapper.text()).toContain('1 of 2')
    })

    it('should show enrolled pathways only when filter is active', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)

      const wrapper = mountComponent()
      await flushPromises()

      // Click the "My Enrollments" toggle button. It's the only button
      // with that label in the filter toolbar.
      const toggle = wrapper
        .findAll('button')
        .find((b) => b.text().includes('My Enrollments'))
      expect(toggle).toBeDefined()
      await toggle!.trigger('click')
      await flushPromises()

      expect(wrapper.text()).toContain('1 of 2')
    })

    it('should clear filters', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      // Apply a search so the Clear button becomes visible (v-if="hasActiveFilters").
      await searchInput(wrapper).setValue('test')
      await flushPromises()

      const clearBtn = wrapper
        .findAll('button')
        .find((b) => b.text().trim() === 'Clear')
      expect(clearBtn).toBeDefined()
      await clearBtn!.trigger('click')
      await flushPromises()

      // After clear, the Clear button is hidden again and the full
      // list is visible.
      const clearBtnAfter = wrapper
        .findAll('button')
        .find((b) => b.text().trim() === 'Clear')
      expect(clearBtnAfter).toBeUndefined()
      expect(wrapper.text()).toContain('2 of 2')
    })
  })

  describe('empty states', () => {
    it('should show empty state when no pathways', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue([])
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No learning pathways available')
    })

    it('should show no results message when filters match nothing', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      const search = wrapper.find(
        'input[aria-label="Search pathways by name, description, or tags"]',
      )
      await search.setValue('nonexistent')
      await flushPromises()

      expect(wrapper.text()).toContain('No pathways match your filters')
    })
  })

  describe('error handling', () => {
    it('should display error when pathways fail to load', async () => {
      vi.mocked(pathwaysApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Failed to load learning pathways')
    })
  })

  describe('navigation', () => {
    it('should navigate to pathway detail on click', async () => {
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPublishedPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Click the first pathway card. Cards are rendered as <article>
      // elements with role="button" per the a11y-audit fix in Phase 5.
      const card = wrapper.find('article[role="button"]')
      expect(card.exists()).toBe(true)
      await card.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathways/linux-fundamentals')
    })
  })
})
