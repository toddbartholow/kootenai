import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface DashboardCardConfig {
  id: string
  label: string
  icon: string
  size: 'full' | 'large' | 'small'
  visible: boolean
  order: number
  minimized: boolean
  settings: Record<string, unknown>
}

const STORAGE_KEY = 'dashboard-layout'

const DEFAULT_CARDS: Omit<DashboardCardConfig, 'order'>[] = [
  { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', size: 'full', visible: true, minimized: false, settings: {} },
  { id: 'insights', label: 'Learning Insights', icon: 'pi-chart-pie', size: 'full', visible: true, minimized: false, settings: {} },
  { id: 'enrollments', label: 'My Learning Pathways', icon: 'pi-graduation-cap', size: 'large', visible: true, minimized: false, settings: { itemCount: 3 } },
  { id: 'achievements', label: 'Recent Achievements', icon: 'pi-trophy', size: 'small', visible: true, minimized: false, settings: { itemCount: 4 } },
  { id: 'sessions', label: 'Active Lab Sessions', icon: 'pi-desktop', size: 'large', visible: true, minimized: false, settings: { itemCount: 3 } },
  { id: 'recommendations', label: 'Recommendations', icon: 'pi-lightbulb', size: 'small', visible: true, minimized: false, settings: { itemCount: 3 } },
  { id: 'quickActions', label: 'Quick Actions', icon: 'pi-bolt', size: 'small', visible: true, minimized: false, settings: {} },
]

function makeDefaults(): DashboardCardConfig[] {
  return DEFAULT_CARDS.map((c, i) => ({ ...c, order: i }))
}

function loadLayout(): DashboardCardConfig[] {
  try {
    const stored = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null
    if (stored) {
      const parsed = JSON.parse(stored) as DashboardCardConfig[]
      return makeDefaults().map(defaultCard => {
        const saved = parsed.find(c => c.id === defaultCard.id)
        return saved
          ? {
              ...defaultCard,
              visible: saved.visible,
              order: saved.order,
              size: saved.size ?? defaultCard.size,
              minimized: saved.minimized ?? defaultCard.minimized,
              settings: { ...defaultCard.settings, ...saved.settings },
            }
          : defaultCard
      }).sort((a, b) => a.order - b.order)
    }
  } catch {
    // ignore corrupt data
  }
  return makeDefaults()
}

export const useDashboardLayoutStore = defineStore('dashboardLayout', () => {
  const cards = ref<DashboardCardConfig[]>(loadLayout())
  const locked = ref(true)

  const sortedCards = computed(() =>
    [...cards.value].sort((a, b) => a.order - b.order)
  )

  function saveLayout(): void {
    if (typeof localStorage === 'undefined') return
    localStorage.setItem(STORAGE_KEY, JSON.stringify(cards.value))
  }

  function isVisible(id: string): boolean {
    return cards.value.find(c => c.id === id)?.visible ?? true
  }

  function getOrder(id: string): number {
    return cards.value.find(c => c.id === id)?.order ?? 0
  }

  function setCardVisible(id: string, visible: boolean): void {
    cards.value = cards.value.map(c =>
      c.id === id ? { ...c, visible } : { ...c }
    )
    saveLayout()
  }

  function moveCard(id: string, direction: 'up' | 'down'): string | null {
    const sorted = [...cards.value].sort((a, b) => a.order - b.order)
    const index = sorted.findIndex(c => c.id === id)
    if (index < 0) return null
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= sorted.length) return null
    const current = sorted[index]
    const target = sorted[targetIndex]
    if (!current || !target) return null
    cards.value = sorted.map((card, i) => {
      if (i === index) return { ...card, order: target.order }
      if (i === targetIndex) return { ...card, order: current.order }
      return { ...card }
    })
    saveLayout()
    return current.label
  }

  function moveCardTo(fromId: string, toId: string): string | null {
    const sorted = [...cards.value].sort((a, b) => a.order - b.order)
    const fromIndex = sorted.findIndex(c => c.id === fromId)
    const toIndex = sorted.findIndex(c => c.id === toId)
    if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) return null
    const label = sorted[fromIndex]?.label ?? null
    const reordered = [...sorted]
    const [moved] = reordered.splice(fromIndex, 1)
    reordered.splice(toIndex, 0, moved!)
    cards.value = reordered.map((card, i) => ({ ...card, order: i }))
    saveLayout()
    return label
  }

  function resetLayout(): void {
    cards.value = makeDefaults()
    saveLayout()
  }

  function toggleLocked(): void {
    locked.value = !locked.value
  }

  function isMinimized(id: string): boolean {
    return cards.value.find(c => c.id === id)?.minimized ?? false
  }

  function setMinimized(id: string, minimized: boolean): void {
    cards.value = cards.value.map(c =>
      c.id === id ? { ...c, minimized } : { ...c }
    )
    saveLayout()
  }

  function getSettings(id: string): Record<string, unknown> {
    return cards.value.find(c => c.id === id)?.settings ?? {}
  }

  function updateSettings(id: string, settings: Record<string, unknown>): void {
    cards.value = cards.value.map(c =>
      c.id === id ? { ...c, settings: { ...c.settings, ...settings } } : { ...c }
    )
    saveLayout()
  }

  function getSize(id: string): 'full' | 'large' | 'small' {
    return cards.value.find(c => c.id === id)?.size ?? 'small'
  }

  function setSize(id: string, size: 'full' | 'large' | 'small'): void {
    cards.value = cards.value.map(c =>
      c.id === id ? { ...c, size } : { ...c }
    )
    saveLayout()
  }

  function addWidget(id: string): void {
    setCardVisible(id, true)
  }

  function removeWidget(id: string): void {
    setCardVisible(id, false)
  }

  return {
    // Exposed as `cards` (the sorted view) to match the prior composable contract.
    cards: sortedCards,
    locked,
    isVisible,
    getOrder,
    setCardVisible,
    moveCard,
    moveCardTo,
    resetLayout,
    toggleLocked,
    isMinimized,
    setMinimized,
    getSize,
    setSize,
    getSettings,
    updateSettings,
    addWidget,
    removeWidget,
  }
})
