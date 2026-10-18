import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import LeaderboardView from './LeaderboardView.vue'
import {
  dashboardApi,
  type LeaderboardResponse,
} from '@/api'

// Mock the API client
vi.mock('@/api', () => ({
  dashboardApi: {
    getLeaderboard: vi.fn(),
  },
}))

// Mock data
const mockLeaderboardResponse: LeaderboardResponse = {
  entries: [
    { rank: 1, userId: 'u1', displayName: 'Alice Chen', totalPoints: 24500, achievementCount: 28, labsCompleted: 32 },
    { rank: 2, userId: 'u2', displayName: 'Bob Williams', totalPoints: 21000, achievementCount: 24, labsCompleted: 28 },
    { rank: 3, userId: 'u3', displayName: 'Carol Martinez', totalPoints: 19500, achievementCount: 22, labsCompleted: 26 },
    { rank: 4, userId: 'u4', displayName: 'David Kim', totalPoints: 18200, achievementCount: 20, labsCompleted: 24 },
    { rank: 5, userId: 'u5', displayName: 'Eva Thompson', totalPoints: 17100, achievementCount: 19, labsCompleted: 23 },
  ],
  currentUser: { rank: 12, userId: 'current', displayName: 'You', totalPoints: 15420, achievementCount: 12, labsCompleted: 14, isCurrentUser: true },
  totalUsers: 247,
}

const mockLeaderboardWithCurrentUser: LeaderboardResponse = {
  entries: [
    { rank: 1, userId: 'u1', displayName: 'Alice Chen', totalPoints: 24500, achievementCount: 28, labsCompleted: 32 },
    { rank: 2, userId: 'u2', displayName: 'Bob Williams', totalPoints: 21000, achievementCount: 24, labsCompleted: 28 },
    { rank: 3, userId: 'current', displayName: 'You', totalPoints: 19500, achievementCount: 22, labsCompleted: 26, isCurrentUser: true },
  ],
  currentUser: { rank: 3, userId: 'current', displayName: 'You', totalPoints: 19500, achievementCount: 22, labsCompleted: 26, isCurrentUser: true },
  totalUsers: 100,
}

describe('LeaderboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(LeaderboardView, {
      global: {
        stubs: {
          Card: { template: '<div class="card"><slot name="content" /></div>' },
          Button: { template: '<button class="btn" @click="$emit(\'click\')">{{ label }}</button>', props: ['label', 'loading', 'icon', 'outlined'] },
          SelectButton: { template: '<div class="select-button"></div>', props: ['modelValue', 'options'] },
          ProgressSpinner: { template: '<div class="spinner">Loading...</div>' },
        },
      },
    })
  }

  describe('initial render', () => {
    it('should show loading spinner initially', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockImplementation(() => new Promise(() => {}))

      const wrapper = mountComponent()

      expect(wrapper.find('.spinner').exists()).toBe(true)
      expect(wrapper.text()).toContain('Loading')
    })

    it('should display leaderboard header', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Leaderboard')
      expect(wrapper.text()).toContain('See how you rank against other learners')
    })

    it('should fetch leaderboard data on mount', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      mountComponent()
      await flushPromises()

      expect(dashboardApi.getLeaderboard).toHaveBeenCalledWith(25)
    })
  })

  describe('stats display', () => {
    it('should display total learners count', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('247')
      expect(wrapper.text()).toContain('Total Learners')
    })

    it('should display current user rank', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('#12')
      expect(wrapper.text()).toContain('Your Rank')
    })

    it('should display current user points', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('15,420')
      expect(wrapper.text()).toContain('Your Points')
    })
  })

  describe('leaderboard table', () => {
    it('should display all leaderboard entries', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Alice Chen')
      expect(wrapper.text()).toContain('Bob Williams')
      expect(wrapper.text()).toContain('Carol Martinez')
      expect(wrapper.text()).toContain('David Kim')
      expect(wrapper.text()).toContain('Eva Thompson')
    })

    it('should display points for each entry', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('24,500')
      expect(wrapper.text()).toContain('21,000')
      expect(wrapper.text()).toContain('19,500')
    })

    it('should display labs completed for each entry', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('32')
      expect(wrapper.text()).toContain('28')
      expect(wrapper.text()).toContain('26')
    })

    it('should highlight current user if in list', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardWithCurrentUser)

      const wrapper = mountComponent()
      await flushPromises()

      // Current user entry should have "(You)" label
      expect(wrapper.text()).toContain('(You)')
      // Should have highlighted row with bg-blue class
      const rows = wrapper.findAll('tr')
      const currentUserRow = rows.find(row => row.text().includes('(You)'))
      expect(currentUserRow?.classes()).toContain('bg-blue-50')
    })
  })

  describe('current user card', () => {
    it('should show current user card when not in visible list', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Current user is rank 12, not in the top 5 entries
      expect(wrapper.text()).toContain('Your Position')
      expect(wrapper.text()).toContain('14 labs completed')
    })

    it('should not show current user card when in visible list', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardWithCurrentUser)

      const wrapper = mountComponent()
      await flushPromises()

      // Current user is rank 3, in the visible entries
      expect(wrapper.text()).not.toContain('Your Position')
    })
  })

  describe('load more', () => {
    it('should show load more button when entries reach limit', async () => {
      // Create exactly 25 entries to trigger "load more"
      const manyEntries = Array.from({ length: 25 }, (_, i) => ({
        rank: i + 1,
        userId: `u${i}`,
        displayName: `User ${i + 1}`,
        totalPoints: 25000 - (i * 100),
        achievementCount: 30 - i,
        labsCompleted: 35 - i,
      }))

      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue({
        entries: manyEntries,
        currentUser: mockLeaderboardResponse.currentUser!,
        totalUsers: 100,
      })

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Load More')
    })

    it('should fetch more entries when load more clicked', async () => {
      const manyEntries = Array.from({ length: 25 }, (_, i) => ({
        rank: i + 1,
        userId: `u${i}`,
        displayName: `User ${i + 1}`,
        totalPoints: 25000 - (i * 100),
        achievementCount: 30 - i,
        labsCompleted: 35 - i,
      }))

      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue({
        entries: manyEntries,
        currentUser: mockLeaderboardResponse.currentUser!,
        totalUsers: 100,
      })

      const wrapper = mountComponent()
      await flushPromises()

      // Click load more
      const loadMoreBtn = wrapper.findAll('button').find(btn => btn.text().includes('Load More'))
      expect(loadMoreBtn).toBeDefined()
      await loadMoreBtn!.trigger('click')
      await flushPromises()

      // Should have called with increased limit
      expect(dashboardApi.getLeaderboard).toHaveBeenCalledWith(50)
    })
  })

  describe('empty state', () => {
    it('should show empty state when no entries', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue({
        entries: [],
        totalUsers: 0,
      })

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('No leaderboard data yet')
      expect(wrapper.text()).toContain('Complete labs to earn points and appear on the leaderboard!')
    })
  })

  describe('error handling', () => {
    it('should display error message on API failure', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      vi.mocked(dashboardApi.getLeaderboard).mockRejectedValue(new Error('Network error'))

      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.text()).toContain('Network error')
      expect(wrapper.text()).toContain('Try Again')

      consoleSpy.mockRestore()
    })

    it('should retry on try again click', async () => {
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
      vi.mocked(dashboardApi.getLeaderboard).mockRejectedValue(new Error('Network error'))

      const wrapper = mountComponent()
      await flushPromises()

      // Error state should be showing
      expect(wrapper.text()).toContain('Network error')

      // Now set up successful response for retry
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      // Click try again
      const tryAgainBtn = wrapper.findAll('button').find(btn => btn.text().includes('Try Again'))
      expect(tryAgainBtn).toBeDefined()
      await tryAgainBtn!.trigger('click')
      await flushPromises()

      // Should now show leaderboard
      expect(wrapper.text()).toContain('Alice Chen')

      consoleSpy.mockRestore()
    })
  })

  describe('time range filter', () => {
    it('should display time range options', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // SelectButton is stubbed but should exist
      expect(wrapper.find('.select-button').exists()).toBe(true)
    })
  })

  describe('rank styling', () => {
    it('should display rank badges for top 3', async () => {
      vi.mocked(dashboardApi.getLeaderboard).mockResolvedValue(mockLeaderboardResponse)

      const wrapper = mountComponent()
      await flushPromises()

      // Top 3 ranks should have special styling (gradient backgrounds)
      const rankCells = wrapper.findAll('td').filter(td => td.find('.rounded-full').exists())
      expect(rankCells.length).toBeGreaterThan(0)

      // Check for gradient classes on top ranks
      const gradientRanks = wrapper.findAll('.bg-gradient-to-br')
      expect(gradientRanks.length).toBeGreaterThan(0)
    })
  })
})
