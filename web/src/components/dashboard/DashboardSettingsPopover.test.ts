import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import DashboardSettingsPopover from './DashboardSettingsPopover.vue'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import { i18n } from '@/locales'

// Stub Popover so we can observe toggle() calls and always render its
// slot content regardless of the overlay's real open/close state.
const popoverToggle = vi.fn()
vi.mock('@volt/Popover.vue', () => ({
  default: {
    name: 'Popover',
    setup(_: unknown, { expose }: { expose: (api: Record<string, unknown>) => void }) {
      expose({ toggle: popoverToggle })
      return { toggle: popoverToggle }
    },
    template: '<div class="stub-popover"><slot /></div>',
  },
}))

// Stub InputNumber to a plain number input we can interact with.
vi.mock('@volt/InputNumber.vue', () => ({
  default: {
    name: 'InputNumber',
    props: ['modelValue', 'id'],
    emits: ['update:modelValue'],
    methods: {
      onInput(e: Event): void {
        const target = e.target as HTMLInputElement
        ;(this as unknown as { $emit: (e: string, v: number) => void }).$emit(
          'update:modelValue',
          Number(target.value),
        )
      },
    },
    template:
      '<input class="stub-number" :id="id" :value="modelValue" @input="onInput" />',
  },
}))

describe('DashboardSettingsPopover', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (typeof localStorage !== 'undefined') localStorage.clear()
    popoverToggle.mockClear()
    i18n.global.locale.value = 'en'
  })

  it('delegates toggle() to the Popover when open() is called', async () => {
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })
    const event = new MouseEvent('click')

    wrapper.vm.open('stats', event)
    await wrapper.vm.$nextTick()

    expect(popoverToggle).toHaveBeenCalledWith(event)
  })

  it('renders size options for the active widget', async () => {
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })
    wrapper.vm.open('stats', new MouseEvent('click'))
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Widget Settings')
    expect(wrapper.find('[aria-label="Set width to 1/3"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="Set width to 2/3"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="Set width to Full"]').exists()).toBe(true)
  })

  it('updates layout size when a size button is clicked', async () => {
    const layout = useDashboardLayout()
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })
    wrapper.vm.open('stats', new MouseEvent('click'))
    await wrapper.vm.$nextTick()

    await wrapper.find('[aria-label="Set width to Full"]').trigger('click')
    expect(layout.getSize('stats')).toBe('full')
  })

  it('shows the item-count control only for widgets that support it', async () => {
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })

    wrapper.vm.open('stats', new MouseEvent('click'))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.stub-number').exists()).toBe(false)

    wrapper.vm.open('enrollments', new MouseEvent('click'))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.stub-number').exists()).toBe(true)
  })

  it('persists the itemCount setting when the control changes', async () => {
    const layout = useDashboardLayout()
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })

    wrapper.vm.open('achievements', new MouseEvent('click'))
    await wrapper.vm.$nextTick()

    const input = wrapper.find<HTMLInputElement>('.stub-number')
    input.element.value = '7'
    await input.trigger('input')

    expect(layout.getSettings('achievements')['itemCount']).toBe(7)
  })

  it('hides the settings panel when no widget is active', () => {
    const wrapper = mount(DashboardSettingsPopover, { global: { plugins: [i18n] } })
    expect(wrapper.text()).not.toContain('Widget Settings')
  })
})
