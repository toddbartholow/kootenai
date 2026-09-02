import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LabTemplateDeleteDialog from './LabTemplateDeleteDialog.vue'

const stubs = {
  Dialog: {
    props: ['visible', 'header', 'closable'],
    emits: ['update:visible'],
    template:
      '<div class="stub-dialog" v-if="visible"><header>{{ header }}</header><main><slot /></main><footer><slot name="footer" /></footer></div>',
  },
  Button: {
    props: ['label', 'loading', 'disabled'],
    emits: ['click'],
    template:
      '<button class="stub-button" :disabled="disabled || loading" @click="$emit(\'click\')">{{ label }}</button>',
  },
}

function mountDialog(overrides: Partial<{ visible: boolean; saving: boolean; templateName: string }> = {}) {
  return mount(LabTemplateDeleteDialog, {
    props: {
      visible: overrides.visible ?? true,
      saving: overrides.saving ?? false,
      templateName: overrides.templateName ?? 'My Template',
    },
    global: { stubs },
  })
}

describe('LabTemplateDeleteDialog', () => {
  it('shows the template name in the confirmation copy', () => {
    const wrapper = mountDialog({ templateName: 'Network Basics' })
    expect(wrapper.text()).toContain('Network Basics')
  })

  it('emits confirm when Delete is clicked', async () => {
    const wrapper = mountDialog()
    await wrapper.findAll('button').find(b => b.text() === 'Delete')!.trigger('click')
    expect(wrapper.emitted('confirm')).toHaveLength(1)
  })

  it('emits update:visible(false) on Cancel', async () => {
    const wrapper = mountDialog()
    await wrapper.findAll('button').find(b => b.text() === 'Cancel')!.trigger('click')
    expect(wrapper.emitted('update:visible')![0]).toEqual([false])
  })

  it('disables Cancel while saving', () => {
    const wrapper = mountDialog({ saving: true })
    const cancel = wrapper.findAll('button').find(b => b.text() === 'Cancel')!
    expect(cancel.attributes('disabled')).toBeDefined()
  })

  it('does not render when visible=false', () => {
    const wrapper = mountDialog({ visible: false })
    expect(wrapper.find('.stub-dialog').exists()).toBe(false)
  })
})
