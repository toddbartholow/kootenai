/**
 * Tests for useDashboardDrag — drag+keyboard reorder of dashboard widgets.
 * useDashboardLayout wraps a Pinia store; we create a fresh Pinia per
 * test so state and `locked` start clean. requestAnimationFrame is
 * stubbed so the aria-live announcement becomes synchronous.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useDashboardDrag } from './useDashboardDrag'
import { useDashboardLayout } from './useDashboardLayout'

function unlock(layout: ReturnType<typeof useDashboardLayout>): void {
  if (layout.locked.value) layout.toggleLocked()
}
function lock(layout: ReturnType<typeof useDashboardLayout>): void {
  if (!layout.locked.value) layout.toggleLocked()
}

// localStorage stub (happy-dom provides one but we want a fresh start per test).
const storage = new Map<string, string>()
beforeEach(() => {
  setActivePinia(createPinia())
  storage.clear()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => storage.get(k) ?? null,
    setItem: (k: string, v: string) => {
      storage.set(k, v)
    },
    removeItem: (k: string) => {
      storage.delete(k)
    },
    clear: () => storage.clear(),
  })
  // Execute requestAnimationFrame synchronously so announcement
  // transitions are observable in the same tick. happy-dom exposes the
  // function on `window`, not just `globalThis`, so stub both.
  const syncRAF = (cb: FrameRequestCallback): number => {
    cb(0)
    return 0
  }
  vi.stubGlobal('requestAnimationFrame', syncRAF)
  if (typeof window !== 'undefined') {
    window.requestAnimationFrame = syncRAF
  }
})
afterEach(() => {
  vi.unstubAllGlobals()
})

// Helper to build a minimal DataTransfer for drag events.
function fakeDataTransfer(): DataTransfer {
  return {
    effectAllowed: '',
    dropEffect: '',
    setData: vi.fn(),
    getData: vi.fn(),
  } as unknown as DataTransfer
}

function dragEvent(type: string): DragEvent {
  const ev = new Event(type, { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'dataTransfer', { value: fakeDataTransfer() })
  return ev as unknown as DragEvent
}

describe('useDashboardDrag', () => {
  it('widgetBindings marks unlocked widgets draggable and focusable', () => {
    const layout = useDashboardLayout()
    unlock(layout)
    const drag = useDashboardDrag(layout)

    const bindings = drag.widgetBindings('stats', 'Stats Overview')
    expect(bindings.draggable).toBe(true)
    expect(bindings.tabindex).toBe(0)
    expect(bindings['aria-label']).toContain('Stats Overview')
    expect(bindings['aria-label']).toContain('Alt+Arrow')

    lock(layout)
    const lockedBindings = drag.widgetBindings('stats', 'Stats Overview')
    expect(lockedBindings.draggable).toBe(false)
    expect(lockedBindings.tabindex).toBeUndefined()
    expect(lockedBindings['aria-label']).toBeUndefined()
  })

  it('onDragStart does nothing while locked', () => {
    const layout = useDashboardLayout()
    lock(layout)
    const drag = useDashboardDrag(layout)
    const bindings = drag.widgetBindings('stats', 'Stats Overview')

    bindings.onDragstart(dragEvent('dragstart'))
    // No drag state means no "dragging" class is set on any widget.
    expect(drag.widgetBindings('stats', 'Stats Overview').class['opacity-50']).toBe(false)
  })

  it('onDragStart sets "opacity-50" class on the dragged widget when unlocked', () => {
    const layout = useDashboardLayout()
    unlock(layout)
    const drag = useDashboardDrag(layout)
    const bindings = drag.widgetBindings('stats', 'Stats Overview')

    bindings.onDragstart(dragEvent('dragstart'))
    expect(drag.widgetBindings('stats', 'Stats Overview').class['opacity-50']).toBe(true)
  })

  it('onDrop reorders widgets and announces the move', () => {
    const layout = useDashboardLayout()
    unlock(layout)
    layout.resetLayout() // ensure deterministic order
    const drag = useDashboardDrag(layout)

    // Simulate dragging stats over insights, then dropping on insights.
    drag.widgetBindings('stats', 'Stats').onDragstart(dragEvent('dragstart'))
    drag.widgetBindings('insights', 'Insights').onDragover(dragEvent('dragover'))
    drag.widgetBindings('insights', 'Insights').onDrop(dragEvent('drop'))

    // Announcement fires synchronously thanks to the RAF stub.
    expect(drag.announcement.value).toContain('Stats Overview')
    expect(drag.announcement.value).toContain('position')
  })

  it('onWidgetKeydown Alt+ArrowDown moves a widget down when unlocked', () => {
    const layout = useDashboardLayout()
    unlock(layout)
    layout.resetLayout()
    const drag = useDashboardDrag(layout)

    const initialOrder = layout.cards.value.map((c) => c.id)
    const firstId = initialOrder[0]!
    const bindings = drag.widgetBindings(firstId, 'first-label')

    const kbd = new KeyboardEvent('keydown', { key: 'ArrowDown', altKey: true, bubbles: true })
    const target = document.createElement('div')
    target.focus = vi.fn()
    Object.defineProperty(kbd, 'currentTarget', { value: target })
    bindings.onKeydown(kbd)

    const nextOrder = layout.cards.value.map((c) => c.id)
    expect(nextOrder[1]).toBe(firstId) // first card shifted down to index 1
    expect(drag.announcement.value).toContain('position')
  })

  it('onWidgetKeydown ignores unmodified ArrowDown', () => {
    const layout = useDashboardLayout()
    unlock(layout)
    layout.resetLayout()
    const drag = useDashboardDrag(layout)

    const initialOrder = layout.cards.value.map((c) => c.id)
    const firstId = initialOrder[0]!
    const bindings = drag.widgetBindings(firstId, 'first-label')

    const kbd = new KeyboardEvent('keydown', { key: 'ArrowDown', altKey: false })
    bindings.onKeydown(kbd)

    // Order unchanged.
    expect(layout.cards.value.map((c) => c.id)).toEqual(initialOrder)
  })
})
