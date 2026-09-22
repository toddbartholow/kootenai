import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LeaderboardCard from './LeaderboardCard.vue'
import type { LeaderboardEntry } from '@/types/progress'

const mockEntries: LeaderboardEntry[] = [
  {
    rank: 1,
    userId: 'u1',
    displayName: 'Alice Chen',
    points: 24500,
    achievementCount: 28,
    labsCompleted: 32,
  },
  {
    rank: 2,
    userId: 'u2',
    displayName: 'Bob Williams',
    points: 21000,
    achievementCount: 24,
    labsCompleted: 28,
  },
  {
    rank: 3,
    userId: 'u3',
    displayName: 'Carol Martinez',
    points: 19500,
    achievementCount: 22,
    labsCompleted: 26,
  },
  {
    rank: 4,
    userId: 'u4',
    displayName: 'David Kim',
    points: 18200,
    achievementCount: 20,
    labsCompleted: 24,
    isCurrentUser: true,
  },
]

describe('LeaderboardCard', () => {
  function mountComponent(props: {
    entries: LeaderboardEntry[]
    title?: string
    showAchievements?: boolean
  }) {
    return mount(LeaderboardCard, {
      props,
    })
  }

  describe('rendering', () => {
    it('should display default title', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('Leaderboard')
    })

    it('should display custom title', () => {
      const wrapper = mountComponent({ entries: mockEntries, title: 'Top Performers' })

      expect(wrapper.text()).toContain('Top Performers')
    })

    it('should display the trophy icon', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.find('i.pi-trophy').exists()).toBe(true)
    })
  })

  describe('entries display', () => {
    it('should display all entries', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('Alice Chen')
      expect(wrapper.text()).toContain('Bob Williams')
      expect(wrapper.text()).toContain('Carol Martinez')
      expect(wrapper.text()).toContain('David Kim')
    })

    it('should display points for each entry', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('24,500')
      expect(wrapper.text()).toContain('21,000')
      expect(wrapper.text()).toContain('19,500')
      expect(wrapper.text()).toContain('18,200')
    })

    it('should display labs completed for each entry', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('32 labs completed')
      expect(wrapper.text()).toContain('28 labs completed')
      expect(wrapper.text()).toContain('26 labs completed')
      expect(wrapper.text()).toContain('24 labs completed')
    })

    it('should display user initials in avatar', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      // Check for first letters of names
      const text = wrapper.text()
      expect(text).toContain('A') // Alice
      expect(text).toContain('B') // Bob
      expect(text).toContain('C') // Carol
      expect(text).toContain('D') // David
    })
  })

  describe('rank display', () => {
    it('should display a rank-1 trophy', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.find('i.pi-trophy[data-rank="1"]').exists()).toBe(true)
    })

    it('should display a rank-2 trophy', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.find('i.pi-trophy[data-rank="2"]').exists()).toBe(true)
    })

    it('should display a rank-3 trophy', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.find('i.pi-trophy[data-rank="3"]').exists()).toBe(true)
    })

    it('should display numeric rank for rank 4+', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('#4')
    })
  })

  describe('current user highlighting', () => {
    it('should highlight current user entry', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      // Find the entry with current user
      const entries = wrapper.findAll('.flex.items-center.gap-4')
      const currentUserEntry = entries.find(entry => entry.text().includes('David Kim'))

      expect(currentUserEntry?.classes()).toContain('bg-blue-50')
    })

    it('should display (You) label for current user', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      expect(wrapper.text()).toContain('(You)')
    })
  })

  describe('achievements display', () => {
    it('should not show achievements by default', () => {
      const wrapper = mountComponent({ entries: mockEntries })

      // The "badges" text should not appear
      expect(wrapper.text()).not.toContain('badges')
    })

    it('should show achievements when showAchievements is true', () => {
      const wrapper = mountComponent({ entries: mockEntries, showAchievements: true })

      expect(wrapper.text()).toContain('badges')
      expect(wrapper.text()).toContain('28') // Achievement count for rank 1
    })
  })

  describe('empty state', () => {
    it('should render without entries', () => {
      const wrapper = mountComponent({ entries: [] })

      // Should still show the header
      expect(wrapper.text()).toContain('Leaderboard')
      // But no user entries
      expect(wrapper.text()).not.toContain('labs completed')
    })
  })
})
