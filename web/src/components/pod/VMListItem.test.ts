import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import VMListItem from './VMListItem.vue'
import type { PodVM } from '@/api'

// Stub child components so we don't bring up the full tree. The point of
// this test is VMListItem's event-bubbling and toggle contract, not
// child behavior (which has its own tests).
vi.mock('./VMPowerControls.vue', () => ({
  default: { template: '<div class="stub-power" />', props: ['vm', 'podId'], emits: ['changed', 'error'] },
}))
vi.mock('./VMSnapshotManager.vue', () => ({
  default: { template: '<div class="stub-snapshots" />', props: ['vm', 'podId'], emits: ['changed', 'error'] },
}))
vi.mock('@volt/Tag.vue', () => ({
  default: { template: '<span class="stub-tag"><slot>{{ value }}</slot></span>', props: ['value', 'severity'] },
}))
vi.mock('@volt/Button.vue', () => ({
  default: {
    template:
      '<button :disabled="disabled" :data-label="label" @click="$emit(\'click\', $event)"><slot>{{ label }}</slot></button>',
    props: ['icon', 'label', 'severity', 'size', 'disabled'],
    emits: ['click'],
  },
}))
vi.mock('@volt/Menu.vue', () => ({
  default: {
    template: '<div class="stub-menu" />',
    props: ['model', 'popup'],
    methods: { toggle() {} },
  },
}))

function makeVM(overrides: Partial<PodVM> = {}): PodVM {
  return {
    name: 'web-01',
    platformId: '100',
    status: 'running',
    ...overrides,
  } as PodVM
}

describe('VMListItem', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  it('emits toggle when the header is clicked', async () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM(), podId: 'p1', expanded: false },
    })
    // The header is the first top-level div with cursor-pointer.
    const header = wrapper.find('.cursor-pointer')
    await header.trigger('click')
    expect(wrapper.emitted('toggle')).toBeTruthy()
  })

  it('renders the VM metadata in the header', () => {
    const wrapper = mount(VMListItem, {
      props: {
        vm: makeVM({ name: 'db-02', platformId: '101', ipAddress: '10.0.0.5' }),
        podId: 'p1',
        expanded: false,
      },
    })
    const text = wrapper.text()
    expect(text).toContain('db-02')
    expect(text).toContain('101')
    expect(text).toContain('10.0.0.5')
  })

  it('does NOT render the expanded body when expanded=false', () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM(), podId: 'p1', expanded: false },
    })
    expect(wrapper.find('.stub-power').exists()).toBe(false)
    expect(wrapper.find('.stub-snapshots').exists()).toBe(false)
  })

  it('renders VMPowerControls + VMSnapshotManager when expanded=true', () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM(), podId: 'p1', expanded: true },
    })
    expect(wrapper.find('.stub-power').exists()).toBe(true)
    expect(wrapper.find('.stub-snapshots').exists()).toBe(true)
  })

  it('bubbles "changed" events from power controls child', async () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM(), podId: 'p1', expanded: true },
    })
    // Find the power-controls stub and emit from it. Global component
    // lookup by tag ('.stub-power').findComponent isn't reliable for
    // module-mocked stubs, so we grab the first component that emitted
    // the props we set up.
    // Cast to VueWrapper because findComponent() with a CSS selector
    // returns the generic WrapperLike.
    const powerStub = wrapper.findComponent<{ $emit: (e: string) => void }>('.stub-power')
    await powerStub.vm.$emit('changed')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('disables Console button when VM is not running or created', () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM({ status: 'stopped' }), podId: 'p1', expanded: true },
    })
    const consoleBtn = wrapper.find('button[data-label="Console"]')
    expect(consoleBtn.attributes('disabled')).toBeDefined()
  })

  it('enables Console button when VM is running', () => {
    const wrapper = mount(VMListItem, {
      props: { vm: makeVM({ status: 'running' }), podId: 'p1', expanded: true },
    })
    const consoleBtn = wrapper.find('button[data-label="Console"]')
    expect(consoleBtn.attributes('disabled')).toBeUndefined()
  })
})
