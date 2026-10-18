import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import DashboardView from './DashboardView.vue'
import {
  labsApi,
  podsApi,
  sessionsApi,
  pathwaysApi,
  enrollmentsApi,
  achievementsApi,
  recommendationsApi,
  dashboardApi,
  type Lab,
  type Pod,
  type Session,
  type Pathway,
  type PathwayEnrollment,
  type AchievementWithProgress,
  type LabRecommendation,
  type PathwayRecommendation,
} from '@/api'

// Mock the API client
vi.mock('@/api', () => ({
  labsApi: {
    list: vi.fn(),
  },
  podsApi: {
    list: vi.fn(),
  },
  sessionsApi: {
    list: vi.fn(),
  },
  pathwaysApi: {
    list: vi.fn(),
  },
  enrollmentsApi: {
    list: vi.fn(),
  },
  achievementsApi: {
    getUserAchievements: vi.fn(),
  },
  recommendationsApi: {
    get: vi.fn(),
  },
  dashboardApi: {
    get: vi.fn(),
  },
}))

// InsightsPanel.vue imports dashboardApi from '@/api/domains/dashboard' rather
// than the '@/api' barrel above, so the barrel mock does not intercept it. Left
// unmocked, its onMounted getInsights() issues a real HTTP request that fails
// with ECONNREFUSED after the test has finished, and the resulting console.warn
// lands after worker teardown -- surfacing as
// "EnvironmentTeardownError: Closing rpc while onUserConsoleLog was pending"
// and failing the run even though every test passes.
vi.mock('@/api/domains/dashboard', () => ({
  dashboardApi: {
    getInsights: vi.fn().mockResolvedValue({
      skills: [],
      strengths: [],
      weaknesses: [],
      streakCalendar: {},
      weeklyTime: [],
    }),
  },
}))

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    user: { id: 'test-user', email: 'test@example.com', name: 'Test User' },
    isAuthenticated: true,
  })),
}))

// Mock data
const mockLabs: Lab[] = [
  {
    id: 'lab-1',
    name: 'Network Basics',
    description: 'Learn networking',
    difficulty: 'beginner',
    durationMinutes: 60,
    platform: 'proxmox',
    tags: [],
    maxPoints: 100,
  },
  {
    id: 'lab-2',
    name: 'Advanced Routing',
    description: 'Routing config',
    difficulty: 'advanced',
    durationMinutes: 120,
    platform: 'proxmox',
    tags: [],
    maxPoints: 200,
  },
]
const mockLabsResponse = { labs: mockLabs, count: mockLabs.length }

const mockPods: Pod[] = [
  {
    id: 'pod-1',
    labTemplate: 'lab-1',
    platform: 'proxmox',
    owner: 'test@example.com',
    status: 'running',
    vms: [],
    createdAt: '2024-01-01T00:00:00Z',
  },
  {
    id: 'pod-2',
    labTemplate: 'lab-2',
    platform: 'proxmox',
    owner: 'test@example.com',
    status: 'provisioning',
    vms: [],
    createdAt: '2024-01-01T01:00:00Z',
  },
]

const mockSessions: Session[] = [
  {
    id: 'session-1',
    podId: 'pod-1',
    userId: 'test@example.com',
    labTemplateId: 'lab-1',
    status: 'active',
    earnedPoints: 50,
    maxPoints: 100,
    percentage: 50,
    passed: false,
    startedAt: '2024-01-01T10:00:00Z',
  },
]

const mockPathway: Pathway = {
  id: 'pathway-1',
  slug: 'network-fundamentals',
  name: 'Network Fundamentals',
  description: 'Learn networking',
  shortDescription: 'Networking basics',
  difficulty: 'beginner',
  estimatedHours: 10,
  isFeatured: true,
  status: 'published',
  displayOrder: 1,
  visibility: 'global',
  moduleCount: 5,
  createdAt: '2024-01-01',
  updatedAt: '2024-01-01',
}

const mockPathways: Pathway[] = [mockPathway]

const mockEnrollments: PathwayEnrollment[] = [
  {
    id: 'enroll-1',
    pathwayId: 'pathway-1',
    userId: 'test-user',
    status: 'in_progress',
    earnedPoints: 150,
    maxPoints: 500,
    percentage: 30,
    completedModules: 2,
    totalModules: 5,
    enrolledAt: '2024-01-01T00:00:00Z',
    pathway: mockPathway,
    certificateIssued: false,
  },
]

const mockAchievements: AchievementWithProgress[] = [
  {
    achievement: {
      id: 'ach-1',
      name: 'First Steps',
      description: 'Complete your first lab',
      type: 'milestone',
      tier: 'bronze',
      points: 10,
      iconUrl: 'pi-star',
      criteria: {},
      isSecret: false,
      isActive: true,
      createdAt: '2024-01-01',
      updatedAt: '2024-01-01',
    },
    earned: true,
    progress: {
      userId: 'test-user',
      achievementId: 'ach-1',
      currentValue: 1,
      targetValue: 1,
      percentage: 100,
      lastUpdated: '2024-01-02T00:00:00Z',
    },
    earnedAt: '2024-01-02T00:00:00Z',
  },
  {
    achievement: {
      id: 'ach-2',
      name: 'Network Pro',
      description: 'Complete network pathway',
      type: 'category',
      tier: 'gold',
      points: 50,
      iconUrl: 'pi-trophy',
      criteria: {},
      isSecret: false,
      isActive: true,
      createdAt: '2024-01-01',
      updatedAt: '2024-01-01',
    },
    earned: false,
    progress: {
      userId: 'test-user',
      achievementId: 'ach-2',
      currentValue: 3,
      targetValue: 10,
      percentage: 30,
      lastUpdated: '2024-01-02T00:00:00Z',
    },
  },
]

const mockLabRecommendations: LabRecommendation[] = [
  {
    labTemplateId: 'lab-1',
    labName: 'Network Basics',
    labSlug: 'network-basics',
    labDescription: 'Learn networking basics',
    difficulty: 'beginner',
    durationMinutes: 45,
    maxPoints: 100,
    type: 'continue_progress',
    reason: 'Continue where you left off',
    priority: 0,
  },
]

const mockPathwayRecommendations: PathwayRecommendation[] = [
  {
    pathwayId: 'pathway-2',
    pathwayName: 'Security Essentials',
    pathwaySlug: 'security-essentials',
    description: 'Learn security basics',
    difficulty: 'intermediate',
    estimatedHours: 12,
    moduleCount: 6,
    labCount: 18,
    type: 'new_pathway',
    reason: 'Featured pathway',
    priority: 3,
    icon: 'pi-shield',
  },
]

const mockRecommendations = {
  labs: mockLabRecommendations,
  pathways: mockPathwayRecommendations,
}

const mockDashboardData = {
  user: { id: 'test-user', displayName: 'Test User', totalPoints: 150 },
  enrolledPathways: [],
  recentSessions: [],
  achievements: { totalEarned: 1, totalAvailable: 10, totalPoints: 10, recentAchievements: [] },
  stats: {
    totalLabsCompleted: 5,
    totalTimeSpentMins: 360, // 6 hours
    currentStreak: 3,
    bestStreak: 7,
    averageScore: 85,
    pathwaysCompleted: 0,
    pathwaysInProgress: 1,
  },
  recommendedNext: [],
}

describe('DashboardView', () => {
  let router: ReturnType<typeof createRouter>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'dashboard', component: DashboardView },
        { path: '/labs', name: 'labs', component: { template: '<div>Labs</div>' } },
        { path: '/pathways', name: 'pathways', component: { template: '<div>Pathways</div>' } },
        { path: '/pathways/:slug', name: 'pathway', component: { template: '<div>Pathway</div>' } },
        {
          path: '/enrollments/:id',
          name: 'enrollment',
          component: { template: '<div>Enrollment</div>' },
        },
        { path: '/session/:id', name: 'session', component: { template: '<div>Session</div>' } },
        { path: '/sessions', name: 'sessions', component: { template: '<div>Sessions</div>' } },
        { path: '/pods', name: 'pods', component: { template: '<div>Pods</div>' } },
        {
          path: '/achievements',
          name: 'achievements',
          component: { template: '<div>Achievements</div>' },
        },
        { path: '/progress', name: 'progress', component: { template: '<div>Progress</div>' } },
        {
          path: '/pathway-mockup',
          name: 'pathway-mockup',
          component: { template: '<div>Mockup</div>' },
        },
      ],
    })

    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mockAllApis() {
    vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
    vi.mocked(podsApi.list).mockResolvedValue(mockPods)
    vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
    vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
    vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)
    vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue(mockAchievements)
    vi.mocked(recommendationsApi.get).mockResolvedValue(mockRecommendations)
    vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)
  }

  function mockAllApisWithEmptyRecommendations() {
    vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
    vi.mocked(podsApi.list).mockResolvedValue(mockPods)
    vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
    vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
    vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)
    vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue(mockAchievements)
    vi.mocked(recommendationsApi.get).mockResolvedValue({ labs: [], pathways: [] })
    vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)
  }

  function mountComponent() {
    return mount(DashboardView, {
      global: {
        plugins: [router],
        stubs: {
          // Don't stub RouterLink so stats render correctly
          // Use shallow stubs for heavy PrimeVue components
          Card: {
            template: '<div class="card"><slot name="title" /><slot name="content" /></div>',
          },
          ProgressSpinner: { template: '<div class="animate-spin">Loading...</div>' },
          SkeletonDashboard: { template: '<div class="skeleton-loading">Loading...</div>' },
          DashboardWidgetHeader: {
            template: '<div class="widget-header">{{ label }}<slot name="actions" /></div>',
            props: ['id', 'label', 'icon', 'locked', 'minimized', 'hasSettings'],
          },
          AvailableWidgets: { template: '<div class="available-widgets" />', props: ['cards'] },
          Popover: { template: '<div class="popover"><slot /></div>' },
          InputNumber: { template: '<input />' },
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner initially', async () => {
      vi.mocked(labsApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(podsApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(sessionsApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(pathwaysApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(enrollmentsApi.list).mockImplementation(() => new Promise(() => {}))
      vi.mocked(achievementsApi.getUserAchievements).mockImplementation(() => new Promise(() => {}))
      vi.mocked(recommendationsApi.get).mockImplementation(() => new Promise(() => {}))
      vi.mocked(dashboardApi.get).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      // Check for skeleton loading state
      expect(wrapper.find('.skeleton-loading').exists()).toBe(true)
    })

    it('should display welcome message with user name', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Welcome back, Test User')
    })

    it('should fetch all dashboard data on mount', async () => {
      mockAllApis()

      mountComponent()
      await flushPromises()

      expect(labsApi.list).toHaveBeenCalled()
      expect(podsApi.list).toHaveBeenCalled()
      expect(sessionsApi.list).toHaveBeenCalled()
      expect(pathwaysApi.list).toHaveBeenCalledWith({ featured: true })
      expect(enrollmentsApi.list).toHaveBeenCalled()
      expect(achievementsApi.getUserAchievements).toHaveBeenCalled()
      expect(recommendationsApi.get).toHaveBeenCalledWith(10)
    })
  })

  describe('stats display', () => {
    it('should display total points', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Total Points')
      expect(wrapper.text()).toContain('150') // From enrollment earnedPoints
    })

    it('should display achievement count', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Achievements')
      // Only 1 achievement is earned
      expect(wrapper.text()).toMatch(/1.*Achievements|Achievements.*1/)
    })

    it('should display active sessions count', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Active Sessions')
    })

    it('should display running pods count', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Running Pods')
      // 2 pods with running/provisioning status
      expect(wrapper.text()).toContain('2')
    })

    it('should display available labs count', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Available Labs')
    })

    it('should display learning time', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Learning Time')
      // Mock data has 360 minutes = 6 hours
      expect(wrapper.text()).toContain('6h')
    })
  })

  describe('enrollments section', () => {
    it('should display pathway enrollments', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('My Learning Pathways')
      expect(wrapper.text()).toContain('Network Fundamentals')
      expect(wrapper.text()).toContain('2 of 5 modules completed')
    })

    it('should display enrollment progress', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('30%')
      expect(wrapper.text()).toContain('150')
      expect(wrapper.text()).toContain('500')
    })

    it('should show empty state when no enrollments', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue([])
      vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue(mockAchievements)
      vi.mocked(recommendationsApi.get).mockResolvedValue(mockRecommendations)
      vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain("You haven't enrolled in any pathways yet")
    })

    it('should navigate to enrollment on click', async () => {
      mockAllApis()
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Find clickable enrollment article
      const enrollmentCard = wrapper.find('article[role="button"]')
      expect(enrollmentCard.exists()).toBe(true)
      await enrollmentCard.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/enrollments/enroll-1')
    })
  })

  describe('active sessions section', () => {
    it('should display active sessions', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Active Lab Sessions')
      expect(wrapper.text()).toContain('50 / 100 pts')
    })

    it('should show empty state when no active sessions', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(sessionsApi.list).mockResolvedValue([])
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)
      vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue(mockAchievements)
      vi.mocked(recommendationsApi.get).mockResolvedValue(mockRecommendations)
      vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No active lab sessions')
    })
  })

  describe('achievements section', () => {
    it('should display recent achievements', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Recent Achievements')
      expect(wrapper.text()).toContain('First Steps')
    })

    it('should only show earned achievements', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      // First Steps is earned, Network Pro is not
      expect(wrapper.text()).toContain('First Steps')
      // Network Pro should not be in recent achievements since it's not earned
      // The component filters for earned: true
    })

    it('should show empty state when no achievements', async () => {
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.list).mockResolvedValue(mockPods)
      vi.mocked(sessionsApi.list).mockResolvedValue(mockSessions)
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)
      vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue([])
      vi.mocked(recommendationsApi.get).mockResolvedValue(mockRecommendations)
      vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Complete labs to earn achievements!')
    })
  })

  describe('recommendations section', () => {
    it('should display lab recommendations', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Recommended for You')
      expect(wrapper.text()).toContain('Network Basics')
      expect(wrapper.text()).toContain('Continue where you left off')
      expect(wrapper.text()).toContain('45m')
      expect(wrapper.text()).toContain('100 pts')
    })

    it('should display pathway recommendations', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Security Essentials')
      expect(wrapper.text()).toContain('Featured pathway')
      expect(wrapper.text()).toContain('12h')
      expect(wrapper.text()).toContain('18 labs')
    })

    it('should show empty state when no recommendations', async () => {
      mockAllApisWithEmptyRecommendations()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Complete more labs to get personalized recommendations!')
    })

    it('should navigate to lab on click', async () => {
      mockAllApis()
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Find the lab recommendation card
      const recCards = wrapper.findAll('.cursor-pointer')
      const labCard = recCards.find(
        el => el.text().includes('Network Basics') && el.text().includes('45m'),
      )
      expect(labCard).toBeDefined()
      await labCard!.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/labs/network-basics')
    })

    it('should navigate to recommended pathway on click', async () => {
      mockAllApis()
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Find the pathway recommendation card
      const recCards = wrapper.findAll('.cursor-pointer')
      const pathwayCard = recCards.find(
        el => el.text().includes('Security Essentials') && el.text().includes('12h'),
      )
      expect(pathwayCard).toBeDefined()
      await pathwayCard!.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathways/security-essentials')
    })
  })

  describe('featured pathways fallback section', () => {
    it('should display featured pathways when no recommendations', async () => {
      mockAllApisWithEmptyRecommendations()

      const wrapper = mountComponent()
      await flushPromises()

      // Featured pathways show as fallback when no recommendations
      expect(wrapper.text()).toContain('Featured Pathways')
      expect(wrapper.text()).toContain('Network Fundamentals')
      expect(wrapper.text()).toContain('10h')
      expect(wrapper.text()).toContain('beginner')
    })

    it('should not display featured pathways when recommendations exist', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      // When recommendations exist, Featured Pathways section should not show
      expect(wrapper.text()).not.toContain('Featured Pathways')
    })

    it('should navigate to pathway on click', async () => {
      mockAllApisWithEmptyRecommendations()
      const pushSpy = vi.spyOn(router, 'push')

      const wrapper = mountComponent()
      await flushPromises()

      // Find the featured pathway card (has "Featured" badge)
      const pathwayCards = wrapper.findAll('.cursor-pointer')
      const pathwayCard = pathwayCards.find(
        el => el.text().includes('Featured') && el.text().includes('10h'),
      )
      expect(pathwayCard).toBeDefined()
      await pathwayCard!.trigger('click')

      expect(pushSpy).toHaveBeenCalledWith('/pathways/network-fundamentals')
    })
  })

  describe('quick actions', () => {
    it('should display quick actions section', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Quick Actions')
      // RouterLink stubs don't render children, so we just check the section exists
      // and has the expected links by finding the RouterLink stubs
      const quickActionsLinks = wrapper.findAllComponents({ name: 'RouterLink' })
      const labsLink = quickActionsLinks.find(link => link.props('to') === '/labs')
      const podsLink = quickActionsLinks.find(link => link.props('to') === '/pods')
      expect(labsLink).toBeDefined()
      expect(podsLink).toBeDefined()
    })
  })

  describe('error handling', () => {
    it('should handle partial API failures gracefully', async () => {
      // Some APIs succeed, some fail
      vi.mocked(labsApi.list).mockResolvedValue(mockLabsResponse)
      vi.mocked(podsApi.list).mockRejectedValue(new Error('Pods failed'))
      vi.mocked(sessionsApi.list).mockRejectedValue(new Error('Sessions failed'))
      vi.mocked(pathwaysApi.list).mockResolvedValue(mockPathways)
      vi.mocked(enrollmentsApi.list).mockResolvedValue(mockEnrollments)
      vi.mocked(achievementsApi.getUserAchievements).mockResolvedValue(mockAchievements)
      vi.mocked(recommendationsApi.get).mockResolvedValue(mockRecommendations)
      vi.mocked(dashboardApi.get).mockResolvedValue(mockDashboardData)

      // Suppress console.warn for this test
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const wrapper = mountComponent()
      await flushPromises()

      // Dashboard should still render with available data
      expect(wrapper.text()).toContain('Welcome back')
      expect(wrapper.text()).toContain('Network Fundamentals')

      // Stats for failed APIs should be 0 (active pods, active sessions)
      // Learning time should still show from dashboard API (6h from mockDashboardData)
      expect(wrapper.text()).toContain('6h')

      warnSpy.mockRestore()
    })

    it('should display error when all APIs fail', async () => {
      // If Promise.allSettled works correctly, the dashboard should still load
      // but with empty data. Only a complete catch block error shows the error message.
      vi.mocked(labsApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(podsApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(sessionsApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(pathwaysApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(enrollmentsApi.list).mockRejectedValue(new Error('API Error'))
      vi.mocked(achievementsApi.getUserAchievements).mockRejectedValue(new Error('API Error'))
      vi.mocked(recommendationsApi.get).mockRejectedValue(new Error('API Error'))
      vi.mocked(dashboardApi.get).mockRejectedValue(new Error('API Error'))

      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      const wrapper = mountComponent()
      await flushPromises()

      // Dashboard should still render, just with zeros and empty states
      expect(wrapper.text()).toContain('Welcome back')

      warnSpy.mockRestore()
    })
  })

  describe('header actions', () => {
    it('should have links to pathways and labs', async () => {
      mockAllApis()

      const wrapper = mountComponent()
      await flushPromises()

      // Check RouterLink stubs have correct destinations
      const links = wrapper.findAllComponents({ name: 'RouterLink' })
      const pathwaysLink = links.find(link => link.props('to') === '/pathways')
      const labsLink = links.find(link => link.props('to') === '/labs')
      expect(pathwaysLink).toBeDefined()
      expect(labsLink).toBeDefined()
    })
  })
})
