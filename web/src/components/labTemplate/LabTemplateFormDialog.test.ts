import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LabTemplateFormDialog, {
  type LabTemplateFormData,
} from './LabTemplateFormDialog.vue'

// Stub PrimeVue form controls to simple native inputs so tests can
// interact with them without pulling in full PrimeVue behavior.
const stubs = {
  Dialog: {
    props: ['visible', 'header', 'closable'],
    emits: ['update:visible'],
    template:
      '<div class="stub-dialog" v-if="visible"><header>{{ header }}</header><main><slot /></main><footer><slot name="footer" /></footer></div>',
  },
  InputText: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input class="stub-input" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Textarea: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<textarea class="stub-textarea" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Select: {
    props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
    emits: ['update:modelValue'],
    template:
      '<select class="stub-select" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="o in options" :key="o[optionValue]" :value="o[optionValue]">{{ o[optionLabel] }}</option></select>',
  },
  InputSwitch: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input type="checkbox" class="stub-switch" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
  },
  InputNumber: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input type="number" class="stub-number" :value="modelValue" @input="$emit(\'update:modelValue\', Number($event.target.value))" />',
  },
  Button: {
    props: ['label', 'loading', 'disabled'],
    emits: ['click'],
    template:
      '<button class="stub-button" :disabled="disabled || loading" @click="$emit(\'click\')">{{ label }}</button>',
  },
}

function blankFormData(): LabTemplateFormData {
  return {
    name: 'Test Lab',
    description: 'Desc',
    version: '1.0.0',
    platform: 'proxmox',
    durationMinutes: 60,
    difficulty: 'beginner',
    category: '',
    tags: [],
    maxPoints: 100,
    passThreshold: 70,
    spec: 'spec: yaml',
    isActive: true,
    visibility: 'global',
  }
}

function mountDialog(overrides: Partial<{ mode: 'create' | 'edit'; visible: boolean; saving: boolean }> = {}) {
  return mount(LabTemplateFormDialog, {
    props: {
      visible: overrides.visible ?? true,
      mode: overrides.mode ?? 'create',
      formData: blankFormData(),
      saving: overrides.saving ?? false,
    },
    global: { stubs },
  })
}

describe('LabTemplateFormDialog', () => {
  it('shows the "Create Lab Template" header in create mode', () => {
    const wrapper = mountDialog({ mode: 'create' })
    expect(wrapper.text()).toContain('Create Lab Template')
  })

  it('shows the "Edit Lab Template" header in edit mode', () => {
    const wrapper = mountDialog({ mode: 'edit' })
    expect(wrapper.text()).toContain('Edit Lab Template')
  })

  it('labels the submit button "Create" in create mode', () => {
    const wrapper = mountDialog({ mode: 'create' })
    const submit = wrapper.findAll('button').find(b => b.text() === 'Create')
    expect(submit).toBeDefined()
  })

  it('labels the submit button "Save Changes" in edit mode', () => {
    const wrapper = mountDialog({ mode: 'edit' })
    const submit = wrapper.findAll('button').find(b => b.text() === 'Save Changes')
    expect(submit).toBeDefined()
  })

  it('emits update:formData when a text field is edited', async () => {
    const wrapper = mountDialog()
    const nameInput = wrapper.find<HTMLInputElement>('input[aria-label="Name"]')
    nameInput.element.value = 'Renamed'
    await nameInput.trigger('input')

    const emitted = wrapper.emitted('update:formData')
    expect(emitted).toBeDefined()
    const lastPayload = emitted![emitted!.length - 1]![0] as LabTemplateFormData
    expect(lastPayload.name).toBe('Renamed')
    // Other fields preserved from the original formData.
    expect(lastPayload.version).toBe('1.0.0')
  })

  it('emits save when the submit button is clicked', async () => {
    const wrapper = mountDialog({ mode: 'create' })
    const submit = wrapper.findAll('button').find(b => b.text() === 'Create')!
    await submit.trigger('click')
    expect(wrapper.emitted('save')).toHaveLength(1)
  })

  it('emits update:visible(false) when cancel is clicked', async () => {
    const wrapper = mountDialog()
    const cancel = wrapper.findAll('button').find(b => b.text() === 'Cancel')!
    await cancel.trigger('click')
    const emitted = wrapper.emitted('update:visible')
    expect(emitted).toBeDefined()
    expect(emitted![0]).toEqual([false])
  })

  it('disables cancel and makes submit show loading while saving', () => {
    const wrapper = mountDialog({ saving: true })
    const buttons = wrapper.findAll('button')
    const cancel = buttons.find(b => b.text() === 'Cancel')!
    const submit = buttons.find(b => b.text() === 'Create')!
    expect(cancel.attributes('disabled')).toBeDefined()
    expect(submit.attributes('disabled')).toBeDefined()
  })

  it('does not render when visible=false', () => {
    const wrapper = mountDialog({ visible: false })
    expect(wrapper.find('.stub-dialog').exists()).toBe(false)
  })
})
