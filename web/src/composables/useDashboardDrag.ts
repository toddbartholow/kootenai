import { ref, type Ref } from 'vue'
import type { useDashboardLayout } from './useDashboardLayout'

/**
 * Widget-level bindings returned by useDashboardDrag. These are spread
 * onto each <DashboardWidget> outer div via v-bind so the component
 * itself stays agnostic of the parent view's drag state.
 */
export interface WidgetDragBindings {
  draggable: boolean
  tabindex: number | undefined
  'aria-label': string | undefined
  class: Record<string, boolean>
  onDragstart: (e: DragEvent) => void
  onDragover: (e: DragEvent) => void
  onDragleave: () => void
  onDrop: (e: DragEvent) => void
  onDragend: () => void
  onKeydown: (e: KeyboardEvent) => void
}

export interface UseDashboardDragReturn {
  /** Polite live-region text that announces reorder outcomes. */
  announcement: Ref<string>
  /** Build the v-bind object for a widget with the given id + label. */
  widgetBindings: (id: string, label: string) => WidgetDragBindings
}

type DashboardLayout = ReturnType<typeof useDashboardLayout>

/**
 * Owns the drag + keyboard widget-reorder state that previously lived
 * inline in DashboardView.vue (~60 lines of handlers and two ref()s).
 *
 * `layout` is the return value of useDashboardLayout(); we call
 * `moveCardTo` / `moveCard` through it so the composable has a single
 * source of truth for widget order.
 */
export function useDashboardDrag(layout: DashboardLayout): UseDashboardDragReturn {
  const draggedId = ref<string | null>(null)
  const dragOverId = ref<string | null>(null)
  const announcement = ref('')

  function announce(text: string): void {
    // Clear first so repeated messages still announce — screen readers
    // deduplicate identical aria-live text.
    announcement.value = ''
    requestAnimationFrame(() => {
      announcement.value = text
    })
  }

  function reset(): void {
    draggedId.value = null
    dragOverId.value = null
  }

  function onDragStart(e: DragEvent, id: string): void {
    if (layout.locked.value) return
    draggedId.value = id
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move'
      e.dataTransfer.setData('text/plain', id)
    }
  }

  function onDragOver(e: DragEvent, id: string): void {
    if (layout.locked.value) return
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    dragOverId.value = id
  }

  function onDragLeave(): void {
    dragOverId.value = null
  }

  function onDrop(e: DragEvent, targetId: string): void {
    e.preventDefault()
    const source = draggedId.value
    if (source && source !== targetId) {
      const label = layout.moveCardTo(source, targetId)
      if (label) {
        const newIndex = layout.cards.value.findIndex((c) => c.id === source)
        announce(`${label} moved to position ${newIndex + 1} of ${layout.cards.value.length}`)
      }
    }
    reset()
  }

  /**
   * Keyboard-accessible reorder. Alt+ArrowUp / Alt+ArrowDown moves the
   * focused widget one slot. HTML5 DnD has no keyboard path — this is
   * the primary way screen-reader and keyboard-only users reorder.
   */
  function onWidgetKeydown(e: KeyboardEvent, id: string): void {
    if (layout.locked.value) return
    if (!e.altKey) return
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return

    e.preventDefault()
    const direction = e.key === 'ArrowUp' ? 'up' : 'down'
    const label = layout.moveCard(id, direction)
    if (label) {
      const newIndex = layout.cards.value.findIndex((c) => c.id === id)
      announce(`${label} moved to position ${newIndex + 1} of ${layout.cards.value.length}`)
      // Keep focus on the moved element so a repeat Alt+Arrow continues.
      ;(e.currentTarget as HTMLElement | null)?.focus()
    }
  }

  function widgetBindings(id: string, label: string): WidgetDragBindings {
    const unlocked = !layout.locked.value
    return {
      draggable: unlocked,
      tabindex: unlocked ? 0 : undefined,
      'aria-label': unlocked ? `${label} widget — Alt+Arrow to reorder` : undefined,
      class: {
        'opacity-50': draggedId.value === id,
        'ring-2 ring-primary-400 rounded-xl':
          dragOverId.value === id && draggedId.value !== id,
      },
      onDragstart: (e) => onDragStart(e, id),
      onDragover: (e) => onDragOver(e, id),
      onDragleave: onDragLeave,
      onDrop: (e) => onDrop(e, id),
      onDragend: reset,
      onKeydown: (e) => onWidgetKeydown(e, id),
    }
  }

  return { announcement, widgetBindings }
}
