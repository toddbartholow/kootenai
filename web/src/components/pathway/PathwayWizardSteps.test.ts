import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import PathwayWizardSteps from './PathwayWizardSteps.vue'

function mountSteps(props: Partial<{ currentStep: number; totalSteps: number; canProceed: boolean }> = {}) {
  return mount(PathwayWizardSteps, {
    props: {
      currentStep: props.currentStep ?? 1,
      totalSteps: props.totalSteps ?? 4,
      canProceed: props.canProceed ?? false,
    },
  })
}

describe('PathwayWizardSteps', () => {
  it('renders all 4 default step labels', () => {
    const wrapper = mountSteps()
    expect(wrapper.text()).toContain('Basic Info')
    expect(wrapper.text()).toContain('Modules')
    expect(wrapper.text()).toContain('Labs')
    expect(wrapper.text()).toContain('Review')
  })

  it('emits go-to-step with the clicked step number', async () => {
    const wrapper = mountSteps({ currentStep: 2, canProceed: true })
    // Find the button for step 3 (Labs).
    const stepButtons = wrapper.findAll('button')
    await stepButtons[2]!.trigger('click')
    expect(wrapper.emitted('go-to-step')?.[0]).toEqual([3])
  })

  it('disables forward steps when canProceed is false', () => {
    const wrapper = mountSteps({ currentStep: 1, canProceed: false })
    const stepButtons = wrapper.findAll('button')
    // Step 1 is the current step (enabled).
    expect(stepButtons[0]!.attributes('disabled')).toBeUndefined()
    // Steps 2-4 are forward steps (disabled).
    expect(stepButtons[1]!.attributes('disabled')).toBeDefined()
    expect(stepButtons[2]!.attributes('disabled')).toBeDefined()
    expect(stepButtons[3]!.attributes('disabled')).toBeDefined()
  })

  it('enables all steps when canProceed is true', () => {
    const wrapper = mountSteps({ currentStep: 1, canProceed: true })
    for (const btn of wrapper.findAll('button')) {
      expect(btn.attributes('disabled')).toBeUndefined()
    }
  })

  it('shows a check icon for completed steps', () => {
    const wrapper = mountSteps({ currentStep: 3, canProceed: true })
    const checks = wrapper.findAll('.pi-check')
    // Steps 1 and 2 are completed; step 3 is current; step 4 not yet reached.
    expect(checks).toHaveLength(2)
  })
})
