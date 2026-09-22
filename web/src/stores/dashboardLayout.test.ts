import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDashboardLayoutStore } from './dashboardLayout'

describe('useDashboardLayoutStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (typeof localStorage !== 'undefined') localStorage.clear()
  })

  describe('initial state', () => {
    it('starts locked', () => {
      const store = useDashboardLayoutStore()
      expect(store.locked).toBe(true)
    })

    it('seeds the default set of widgets in display order', () => {
      const store = useDashboardLayoutStore()
      const ids = store.cards.map(c => c.id)
      expect(ids).toEqual([
        'stats',
        'insights',
        'enrollments',
        'achievements',
        'sessions',
        'recommendations',
        'quickActions',
      ])
      expect(store.cards.every(c => c.visible)).toBe(true)
    })

    it('isolates state between store instances (test isolation)', () => {
      const a = useDashboardLayoutStore()
      a.toggleLocked()
      expect(a.locked).toBe(false)

      // A fresh Pinia + fresh store should be locked again.
      setActivePinia(createPinia())
      localStorage.clear()
      const b = useDashboardLayoutStore()
      expect(b.locked).toBe(true)
    })
  })

  describe('lock', () => {
    it('toggleLocked flips the lock flag', () => {
      const store = useDashboardLayoutStore()
      store.toggleLocked()
      expect(store.locked).toBe(false)
      store.toggleLocked()
      expect(store.locked).toBe(true)
    })
  })

  describe('visibility', () => {
    it('isVisible reflects setCardVisible', () => {
      const store = useDashboardLayoutStore()
      store.setCardVisible('stats', false)
      expect(store.isVisible('stats')).toBe(false)
      store.setCardVisible('stats', true)
      expect(store.isVisible('stats')).toBe(true)
    })

    it('addWidget and removeWidget proxy to setCardVisible', () => {
      const store = useDashboardLayoutStore()
      store.removeWidget('insights')
      expect(store.isVisible('insights')).toBe(false)
      store.addWidget('insights')
      expect(store.isVisible('insights')).toBe(true)
    })
  })

  describe('size and settings', () => {
    it('setSize updates the widget size', () => {
      const store = useDashboardLayoutStore()
      store.setSize('stats', 'large')
      expect(store.getSize('stats')).toBe('large')
    })

    it('updateSettings merges new values without dropping existing keys', () => {
      const store = useDashboardLayoutStore()
      store.updateSettings('enrollments', { itemCount: 7 })
      expect(store.getSettings('enrollments')['itemCount']).toBe(7)
      store.updateSettings('enrollments', { extra: 'x' })
      expect(store.getSettings('enrollments')['itemCount']).toBe(7)
      expect(store.getSettings('enrollments')['extra']).toBe('x')
    })
  })

  describe('minimize', () => {
    it('setMinimized updates the minimized flag', () => {
      const store = useDashboardLayoutStore()
      expect(store.isMinimized('stats')).toBe(false)
      store.setMinimized('stats', true)
      expect(store.isMinimized('stats')).toBe(true)
    })
  })

  describe('reorder', () => {
    it('moveCard up swaps order with the previous card', () => {
      const store = useDashboardLayoutStore()
      const originalFirst = store.cards[0]!.id
      const originalSecond = store.cards[1]!.id
      const moved = store.moveCard(originalSecond, 'up')
      expect(moved).not.toBeNull()
      expect(store.cards[0]!.id).toBe(originalSecond)
      expect(store.cards[1]!.id).toBe(originalFirst)
    })

    it('moveCard returns null when at the edge', () => {
      const store = useDashboardLayoutStore()
      const firstId = store.cards[0]!.id
      expect(store.moveCard(firstId, 'up')).toBeNull()
    })

    it('moveCardTo reorders to an arbitrary position', () => {
      const store = useDashboardLayoutStore()
      const fromId = 'quickActions' // last
      store.moveCardTo(fromId, 'stats') // move to front
      expect(store.cards[0]!.id).toBe(fromId)
    })

    it('resetLayout restores the default ordering', () => {
      const store = useDashboardLayoutStore()
      store.moveCardTo('quickActions', 'stats')
      expect(store.cards[0]!.id).toBe('quickActions')
      store.resetLayout()
      expect(store.cards[0]!.id).toBe('stats')
    })
  })

  describe('persistence', () => {
    it('writes visibility changes to localStorage', () => {
      const store = useDashboardLayoutStore()
      store.setCardVisible('stats', false)
      const saved = localStorage.getItem('dashboard-layout')
      expect(saved).not.toBeNull()
      const parsed = JSON.parse(saved!)
      const stats = parsed.find((c: { id: string }) => c.id === 'stats')
      expect(stats.visible).toBe(false)
    })

    it('merges persisted state with new defaults on load', () => {
      // Simulate a pre-existing, partial state in localStorage.
      const partial = [
        { id: 'stats', visible: false, order: 0, size: 'full', minimized: false, settings: {} },
      ]
      localStorage.setItem('dashboard-layout', JSON.stringify(partial))
      setActivePinia(createPinia())
      const store = useDashboardLayoutStore()
      // Persisted: stats hidden.
      expect(store.isVisible('stats')).toBe(false)
      // New defaults still populated for widgets absent from storage.
      expect(store.cards.find(c => c.id === 'quickActions')).toBeDefined()
    })
  })
})
