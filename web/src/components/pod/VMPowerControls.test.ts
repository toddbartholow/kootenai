import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import VMPowerControls from './VMPowerControls.vue'
import type { PodVM } from '@/api'

// Mock podsApi so the test doesn't hit a real axios instance.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return {
    ...actual,
    podsApi: {
      startVM: vi.fn(() => Promise.resolve()),
      stopVM: vi.fn(() => Promise.resolve()),
      suspendVM: vi.fn(() => Promise.resolve()),
      resumeVM: vi.fn(() => Promise.resolve()),
    },
  }
})

// Minimal stand-ins for PrimeVue's injected services. The component only
// uses `add()` and `require()`, so we only need those methods.
const mockToastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({
  useToast: () => ({ add: mockToastAdd }),
}))
const mockConfirmRequire = vi.fn()
vi.mock('primevue/useconfirm', () => ({
  useConfirm: () => ({ require: mockConfirmRequire }),
}))

// Stub the Volt Button with a plain <button> so test queries can match it.
vi.mock('@volt/Button.vue', () => ({
  default: {
    props: ['icon', 'label', 'severity', 'size', 'disabled', 'loading'],
    emits: ['click'],
    template:
      '<button :disabled="disabled" :data-label="label" @click="$emit(\'click\', $event)"><slot>{{ label }}</slot></button>',
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

describe('VMPowerControls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockToastAdd.mockReset()
    mockConfirmRequire.mockReset()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  it('shows Stop and Suspend for a running VM', () => {
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'running' }), podId: 'p1' },
    })
    const labels = wrapper.findAll('button').map((b) => b.attributes('data-label'))
    expect(labels).toContain('Stop')
    expect(labels).toContain('Suspend')
    expect(labels).not.toContain('Start')
    expect(labels).not.toContain('Resume')
  })

  it('shows Start only for a stopped VM', () => {
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'stopped' }), podId: 'p1' },
    })
    const labels = wrapper.findAll('button').map((b) => b.attributes('data-label'))
    expect(labels).toEqual(['Start'])
  })

  it('shows Resume only for a suspended VM', () => {
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'suspended' }), podId: 'p1' },
    })
    const labels = wrapper.findAll('button').map((b) => b.attributes('data-label'))
    expect(labels).toEqual(['Resume'])
  })

  it('calls podsApi.startVM and emits changed on Start click', async () => {
    const { podsApi } = await import('@/api')
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'stopped' }), podId: 'p1' },
    })
    await wrapper.find('button[data-label="Start"]').trigger('click')
    // Give the in-component promise a tick to resolve.
    await wrapper.vm.$nextTick()
    await Promise.resolve()

    expect(podsApi.startVM).toHaveBeenCalledWith('p1', 'web-01')
    expect(wrapper.emitted('changed')).toBeTruthy()
    expect(mockToastAdd).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'success', summary: 'VM Started' }),
    )
  })

  it('requires confirmation before stopping and only calls stopVM on accept', async () => {
    const { podsApi } = await import('@/api')
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'running' }), podId: 'p1' },
    })
    await wrapper.find('button[data-label="Stop"]').trigger('click')

    // Stop MUST NOT have fired yet — we go through the confirm dialog.
    expect(podsApi.stopVM).not.toHaveBeenCalled()
    expect(mockConfirmRequire).toHaveBeenCalledTimes(1)

    // Simulate the user clicking "Stop" in the confirm dialog.
    const acceptFn = mockConfirmRequire.mock.calls[0]?.[0]?.accept as () => void
    acceptFn()
    await wrapper.vm.$nextTick()
    await Promise.resolve()

    expect(podsApi.stopVM).toHaveBeenCalledWith('p1', 'web-01')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('emits error when the API rejects', async () => {
    const { podsApi } = await import('@/api')
    vi.mocked(podsApi.startVM).mockRejectedValueOnce(new Error('nope'))
    const wrapper = mount(VMPowerControls, {
      props: { vm: makeVM({ status: 'stopped' }), podId: 'p1' },
    })
    await wrapper.find('button[data-label="Start"]').trigger('click')
    await wrapper.vm.$nextTick()
    await Promise.resolve()
    await Promise.resolve()

    expect(wrapper.emitted('error')).toBeTruthy()
    expect(wrapper.emitted('error')?.[0]?.[0]).toContain('Failed to start VM')
    expect(wrapper.emitted('changed')).toBeFalsy()
  })
})
