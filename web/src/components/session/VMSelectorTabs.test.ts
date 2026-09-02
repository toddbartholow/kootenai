import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import VMSelectorTabs from './VMSelectorTabs.vue'
import type { PodVM } from '@/api'

function makeVM(overrides: Partial<PodVM> = {}): PodVM {
  return { name: 'web-01', platformId: '100', status: 'running', ...overrides } as PodVM
}

describe('VMSelectorTabs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders nothing when the vm list is empty', () => {
    const wrapper = mount(VMSelectorTabs, {
      props: { vms: [], selectedName: null, active: false },
    })
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders one button per VM and the count badge', () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [makeVM({ name: 'a' }), makeVM({ name: 'b' }), makeVM({ name: 'c' })],
        selectedName: null,
        active: false,
      },
    })
    expect(wrapper.findAll('button')).toHaveLength(3)
    expect(wrapper.text()).toContain('3 VMs')
  })

  it('disables buttons for non-running VMs', () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [
          makeVM({ name: 'r', status: 'running' }),
          makeVM({ name: 's', status: 'stopped' }),
        ],
        selectedName: null,
        active: false,
      },
    })
    const buttons = wrapper.findAll('button')
    expect(buttons[0]!.attributes('disabled')).toBeUndefined()
    expect(buttons[1]!.attributes('disabled')).toBeDefined()
  })

  it('emits "select" when a running VM is clicked', async () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [makeVM({ name: 'a', status: 'running' })],
        selectedName: null,
        active: false,
      },
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('select')).toBeTruthy()
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({ name: 'a' })
  })

  it('does NOT emit when a stopped VM is clicked', async () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [makeVM({ name: 'a', status: 'stopped' })],
        selectedName: null,
        active: false,
      },
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('select')).toBeFalsy()
  })

  it('marks aria-pressed on the selected VM when active', () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [makeVM({ name: 'a' }), makeVM({ name: 'b' })],
        selectedName: 'b',
        active: true,
      },
    })
    const buttons = wrapper.findAll('button')
    expect(buttons[0]!.attributes('aria-pressed')).toBe('false')
    expect(buttons[1]!.attributes('aria-pressed')).toBe('true')
  })

  it('does NOT mark aria-pressed when selectedName matches but active=false', () => {
    const wrapper = mount(VMSelectorTabs, {
      props: {
        vms: [makeVM({ name: 'a' })],
        selectedName: 'a',
        active: false,
      },
    })
    expect(wrapper.find('button').attributes('aria-pressed')).toBe('false')
  })
})
