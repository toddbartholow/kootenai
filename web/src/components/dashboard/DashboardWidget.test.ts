import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import DashboardWidget from './DashboardWidget.vue'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import { useDashboardDrag } from '@/composables'

// Stub child components to keep tests focused on the widget's own
// contract: visibility gate, slot rendering, settings emit, drag bindings.
vi.mock('@volt/Card.vue', () => ({
  default: {
    template:
      '<div class="stub-card"><header><slot name="title" /></header><section><slot name="content" /></section></div>',
  },
}))
vi.mock('./DashboardWidgetHeader.vue', () => ({
  default: {
    template:
      '<div class="stub-header" :data-label="label" :data-locked="locked" :data-minimized="minimized"><slot name="actions" /><button class="stub-minimize" @click="$emit(\'minimize\')" /><button class="stub-remove" @click="$emit(\'remove\')" /><button class="stub-configure" @click="$emit(\'configure\', $event)" /></div>',
    props: ['id', 'label', 'icon', 'locked', 'minimized', 'hasSettings'],
    emits: ['minimize', 'remove', 'configure'],
  },
}))

function unlock(layout: ReturnType<typeof useDashboardLayout>): void {
  if (layout.locked.value) layout.toggleLocked()
}
function lock(layout: ReturnType<typeof useDashboardLayout>): void {
  if (!layout.locked.value) layout.toggleLocked()
}

describe('DashboardWidget', () => {
  beforeEach(() => {
    // Fresh Pinia + cleared localStorage → each test starts with the
    // default widget layout (stats visible, locked=true).
    setActivePinia(createPinia())
    if (typeof localStorage !== 'undefined') localStorage.clear()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  it('renders default slot content when the widget is visible', () => {
    const layout = useDashboardLayout()
    const drag = useDashboardDrag(layout)

    const wrapper = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div class="payload">hello</div>' },
    })
    expect(wrapper.find('.payload').exists()).toBe(true)
  })

  it('does NOT render anything when the widget is hidden', () => {
    const layout = useDashboardLayout()
    layout.setCardVisible('stats', false)
    const drag = useDashboardDrag(layout)

    const wrapper = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div class="payload">hello</div>' },
    })
    expect(wrapper.find('.payload').exists()).toBe(false)
  })

  it('forwards configure event from the header', async () => {
    const layout = useDashboardLayout()
    const drag = useDashboardDrag(layout)

    const wrapper = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div />' },
    })
    await wrapper.find('.stub-configure').trigger('click')
    expect(wrapper.emitted('configure')).toBeTruthy()
  })

  it('toggles minimized via the header', async () => {
    const layout = useDashboardLayout()
    layout.setMinimized('stats', false) // ensure known starting state
    const drag = useDashboardDrag(layout)

    const wrapper = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div />' },
    })
    expect(layout.isMinimized('stats')).toBe(false)

    await wrapper.find('.stub-minimize').trigger('click')
    expect(layout.isMinimized('stats')).toBe(true)
  })

  it('renders the actions slot alongside the header', () => {
    const layout = useDashboardLayout()
    const drag = useDashboardDrag(layout)

    const wrapper = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: {
        default: '<div />',
        actions: '<a class="actions-link" href="/more">View all</a>',
      },
    })
    expect(wrapper.find('.actions-link').exists()).toBe(true)
  })

  it('applies draggable=true when unlocked and false when locked', async () => {
    const layout = useDashboardLayout()
    const drag = useDashboardDrag(layout)

    lock(layout)
    const locked = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div />' },
    })
    expect(locked.element.getAttribute('draggable')).toBe('false')

    unlock(layout)
    const unlocked = mount(DashboardWidget, {
      props: { id: 'stats', label: 'Stats Overview', icon: 'pi-chart-bar', layout, drag },
      slots: { default: '<div />' },
    })
    expect(unlocked.element.getAttribute('draggable')).toBe('true')

    lock(layout) // reset for later tests
  })
})
