import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import VMSnapshotManager from './VMSnapshotManager.vue'
import type { PodVM, Snapshot } from '@/api'

const mockListResult: Snapshot[] = [
  { name: 'initial', description: 'First', parent: '' } as Snapshot,
  { name: 'checkpoint-1', description: 'Mid-lab', parent: 'initial' } as Snapshot,
]

// Mock the API module. Each test reaches through vi.mocked(…) to customize.
vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return {
    ...actual,
    snapshotsApi: {
      list: vi.fn(() => Promise.resolve(mockListResult)),
      create: vi.fn(() => Promise.resolve()),
      delete: vi.fn(() => Promise.resolve()),
    },
    podsApi: {
      resetVM: vi.fn(() => Promise.resolve()),
    },
  }
})

const mockToastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({
  useToast: () => ({ add: mockToastAdd }),
}))

const mockConfirmRequire = vi.fn()
vi.mock('primevue/useconfirm', () => ({
  useConfirm: () => ({ require: mockConfirmRequire }),
}))

// Stub all the Volt components we use so we don't need PrimeVue runtime.
function stubComponent(name: string): { template: string; props: string[] } {
  return {
    template: `<div class="stub-${name}"><slot></slot><slot name="footer"></slot></div>`,
    props: [],
  }
}
vi.mock('@volt/Button.vue', () => ({
  default: {
    props: ['icon', 'label', 'severity', 'size', 'disabled', 'loading'],
    emits: ['click'],
    template:
      '<button :disabled="disabled" :data-label="label" @click="$emit(\'click\', $event)"><slot>{{ label }}</slot></button>',
  },
}))
vi.mock('@volt/Dialog.vue', () => ({
  default: {
    props: ['visible', 'modal', 'header'],
    emits: ['update:visible'],
    template:
      '<div v-if="visible" class="stub-dialog" :data-header="header"><slot></slot><footer><slot name="footer"></slot></footer></div>',
  },
}))
vi.mock('@volt/InputText.vue', () => ({
  default: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
vi.mock('@volt/Checkbox.vue', () => ({
  default: {
    props: ['modelValue', 'binary', 'inputId'],
    emits: ['update:modelValue'],
    template:
      '<input type="checkbox" :id="inputId" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
  },
}))
vi.mock('@volt/RadioButton.vue', () => ({
  default: {
    props: ['modelValue', 'value', 'inputId'],
    emits: ['update:modelValue'],
    template:
      '<input type="radio" :id="inputId" :checked="modelValue === value" @change="$emit(\'update:modelValue\', value)" />',
  },
}))
vi.mock('@volt/ProgressSpinner.vue', () => ({
  default: stubComponent('ProgressSpinner'),
}))

// Stub useFocusRestore — it attaches watchers we don't want leaking.
vi.mock('@/composables', async () => {
  const actual = await vi.importActual<typeof import('@/composables')>('@/composables')
  return {
    ...actual,
    useFocusRestore: () => ({ savedFocus: { value: null }, restore: () => {} }),
  }
})

function makeVM(overrides: Partial<PodVM> = {}): PodVM {
  return { name: 'web-01', platformId: '100', status: 'running', ...overrides } as PodVM
}

describe('VMSnapshotManager', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockToastAdd.mockReset()
    mockConfirmRequire.mockReset()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  it('loads the snapshot list on mount for the given VM', async () => {
    const { snapshotsApi } = await import('@/api')
    mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    expect(snapshotsApi.list).toHaveBeenCalledWith('pod-1', 'web-01')
  })

  it('renders snapshot rows after load', async () => {
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('initial')
    expect(text).toContain('checkpoint-1')
  })

  it('creates a snapshot and refreshes the list', async () => {
    const { snapshotsApi } = await import('@/api')
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    // Open create modal
    await wrapper.find('button[data-label="Create Snapshot"]').trigger('click')
    await wrapper.vm.$nextTick()

    // Fill in name and submit via the Create button inside the modal.
    const textInput = wrapper.find('.stub-dialog input:not([type="checkbox"]):not([type="radio"])')
    await textInput.setValue('new-snap')

    const createButtons = wrapper.findAll('button[data-label="Create Snapshot"]')
    // There are two buttons with the same label (trigger + modal submit).
    // The modal one is the second.
    await createButtons[createButtons.length - 1]!.trigger('click')
    await flushPromises()

    expect(snapshotsApi.create).toHaveBeenCalledWith(
      'pod-1',
      'web-01',
      'new-snap',
      undefined,
      false,
    )
    // List reloaded (call count: 1 on mount + 1 after create = 2).
    expect(vi.mocked(snapshotsApi.list).mock.calls.length).toBeGreaterThanOrEqual(2)
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('asks for confirmation before deleting a snapshot', async () => {
    const { snapshotsApi } = await import('@/api')
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    await wrapper.find('button[data-label="Delete"]').trigger('click')
    // The delete MUST NOT have fired until the user confirms.
    expect(snapshotsApi.delete).not.toHaveBeenCalled()
    expect(mockConfirmRequire).toHaveBeenCalledTimes(1)

    // Simulate accept.
    const acceptFn = mockConfirmRequire.mock.calls[0]?.[0]?.accept as () => void
    acceptFn()
    await flushPromises()

    expect(snapshotsApi.delete).toHaveBeenCalledWith('pod-1', 'web-01', 'initial')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('reverts to a snapshot via the inline Revert button', async () => {
    const { podsApi } = await import('@/api')
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    const revertButtons = wrapper.findAll('button[data-label="Revert"]')
    // There are inline Revert buttons per row (2 snapshots → 2 buttons).
    expect(revertButtons.length).toBeGreaterThan(0)
    await revertButtons[0]!.trigger('click')
    await flushPromises()

    expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'web-01', 'initial')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('resets to initial from the "Reset to Initial" button', async () => {
    const { podsApi } = await import('@/api')
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    await wrapper.find('button[data-label="Reset to Initial"]').trigger('click')
    await flushPromises()

    expect(podsApi.resetVM).toHaveBeenCalledWith('pod-1', 'web-01', 'initial')
    expect(wrapper.emitted('changed')).toBeTruthy()
  })

  it('emits error when snapshot creation fails', async () => {
    const { snapshotsApi } = await import('@/api')
    vi.mocked(snapshotsApi.create).mockRejectedValueOnce(new Error('boom'))
    const wrapper = mount(VMSnapshotManager, { props: { vm: makeVM(), podId: 'pod-1' } })
    await flushPromises()

    await wrapper.find('button[data-label="Create Snapshot"]').trigger('click')
    await wrapper.vm.$nextTick()
    await wrapper.find('.stub-dialog input:not([type="checkbox"]):not([type="radio"])').setValue('will-fail')
    const createButtons = wrapper.findAll('button[data-label="Create Snapshot"]')
    await createButtons[createButtons.length - 1]!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('error')).toBeTruthy()
    expect(wrapper.emitted('error')?.[0]?.[0]).toContain('Failed to create snapshot')
    expect(wrapper.emitted('changed')).toBeFalsy()
  })
})
