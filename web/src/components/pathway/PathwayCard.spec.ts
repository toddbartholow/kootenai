import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PathwayCard from './PathwayCard.vue'
import type { Pathway, PathwayEnrollment } from '@/api'

// Mock PrimeVue Button
vi.mock('@volt/Button.vue', () => ({
  default: {
    name: 'Button',
    props: ['label', 'severity', 'outlined', 'size', 'loading', 'disabled', 'icon'],
    template: '<button :disabled="disabled || loading" @click="$emit(\'click\', $event)">{{ label }}</button>',
  },
}))

const createPathway = (overrides: Partial<Pathway> = {}): Pathway => ({
  id: 'test-id',
  name: 'Test Pathway',
  slug: 'test-pathway',
  description: 'A test pathway description',
  shortDescription: 'Short description',
  difficulty: 'intermediate',
  estimatedHours: 10,
  displayOrder: 1,
  status: 'published',
  isFeatured: false,
  visibility: 'global',
  createdAt: '2024-01-01',
  updatedAt: '2024-01-01',
  moduleCount: 5,
  tags: ['security', 'networking', 'advanced'],
  ...overrides,
})

const createEnrollment = (overrides: Partial<PathwayEnrollment> = {}): PathwayEnrollment => ({
  id: 'enrollment-id',
  userId: 'user-id',
  pathwayId: 'test-id',
  status: 'in_progress',
  completedModules: 2,
  totalModules: 5,
  earnedPoints: 100,
  maxPoints: 250,
  percentage: 40,
  enrolledAt: '2024-01-01',
  certificateIssued: false,
  ...overrides,
})

describe('PathwayCard', () => {
  it('renders pathway name and description', () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('Test Pathway')
    expect(wrapper.text()).toContain('Short description')
  })

  it('shows difficulty badge', () => {
    const pathway = createPathway({ difficulty: 'advanced' })
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('Advanced')
  })

  it('shows duration and module count', () => {
    const pathway = createPathway({ estimatedHours: 10, moduleCount: 5 })
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('10h')
    expect(wrapper.text()).toContain('5 modules')
  })

  it('shows Featured badge when featured prop is true', () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway, featured: true },
    })

    expect(wrapper.text()).toContain('Featured')
  })

  it('shows Draft badge when pathway status is draft', () => {
    const pathway = createPathway({ status: 'draft' })
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('Draft')
  })

  it('shows tags in non-featured mode', () => {
    const pathway = createPathway({ tags: ['security', 'networking', 'advanced'] })
    const wrapper = mount(PathwayCard, {
      props: { pathway, featured: false },
    })

    expect(wrapper.text()).toContain('security')
    expect(wrapper.text()).toContain('networking')
    expect(wrapper.text()).toContain('advanced')
  })

  it('hides tags in featured mode', () => {
    const pathway = createPathway({ tags: ['security', 'networking'] })
    const wrapper = mount(PathwayCard, {
      props: { pathway, featured: true },
    })

    // Tags should not be shown in featured mode
    const tagElements = wrapper.findAll('[class*="bg-surface-100"]')
    expect(tagElements.length).toBe(0)
  })

  it('shows progress bar when enrolled', () => {
    const pathway = createPathway()
    const enrollment = createEnrollment({ percentage: 40 })
    const wrapper = mount(PathwayCard, {
      props: { pathway, enrollment },
    })

    expect(wrapper.text()).toContain('Progress')
    expect(wrapper.text()).toContain('40%')
  })

  it('shows Enroll button when not enrolled', () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('Enroll')
  })

  it('shows Continue button when enrolled', () => {
    const pathway = createPathway()
    const enrollment = createEnrollment()
    const wrapper = mount(PathwayCard, {
      props: { pathway, enrollment },
    })

    expect(wrapper.text()).toContain('Continue')
  })

  it('emits view event on click', async () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    await wrapper.trigger('click')
    expect(wrapper.emitted('view')).toBeTruthy()
    expect(wrapper.emitted('view')![0]).toEqual([pathway])
  })

  it('emits view event on enter key', async () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    await wrapper.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('view')).toBeTruthy()
  })

  it('emits enroll event when enroll button clicked', async () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    const enrollButton = wrapper.findAll('button').find(b => b.text() === 'Enroll')
    await enrollButton?.trigger('click')

    expect(wrapper.emitted('enroll')).toBeTruthy()
  })

  it('emits continue event when continue button clicked', async () => {
    const pathway = createPathway()
    const enrollment = createEnrollment()
    const wrapper = mount(PathwayCard, {
      props: { pathway, enrollment },
    })

    const continueButton = wrapper.findAll('button').find(b => b.text() === 'Continue')
    await continueButton?.trigger('click')

    expect(wrapper.emitted('continue')).toBeTruthy()
  })

  it('shows loading state when enrolling', () => {
    const pathway = createPathway()
    const wrapper = mount(PathwayCard, {
      props: { pathway, enrolling: true },
    })

    const enrollButton = wrapper.findAll('button').find(b => b.text() === 'Enroll')
    expect(enrollButton?.attributes('disabled')).toBeDefined()
  })

  it('formats duration correctly for 1 hour', () => {
    const pathway = createPathway({ estimatedHours: 1 })
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('1h')
  })

  it('handles missing duration gracefully', () => {
    const pathway = createPathway()
    // Remove estimatedHours to simulate missing data
    delete (pathway as Partial<Pathway>).estimatedHours
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('N/A')
  })

  it('truncates tags when more than 3', () => {
    const pathway = createPathway({ tags: ['a', 'b', 'c', 'd', 'e'] })
    const wrapper = mount(PathwayCard, {
      props: { pathway },
    })

    expect(wrapper.text()).toContain('+2')
  })
})
