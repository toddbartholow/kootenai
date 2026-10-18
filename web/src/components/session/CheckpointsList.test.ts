import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import CheckpointsList from './CheckpointsList.vue'
import type { CheckpointProgress } from '@/stores/session'

vi.mock('@volt/Tag.vue', () => ({
  default: {
    template: '<span class="stub-tag" :data-severity="severity">{{ value }}</span>',
    props: ['value', 'severity'],
  },
}))

function makeCheckpoint(overrides: Partial<CheckpointProgress> = {}): CheckpointProgress {
  return {
    id: 'cp-1',
    name: 'Configure Router',
    status: 'pending',
    score: 0,
    maxScore: 10,
    ...overrides,
  }
}

describe('CheckpointsList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders nothing when there are no checkpoints', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [],
        revealedHints: {},
        hasMoreHints: () => false,
        nextHintLevel: () => 0,
        showNextHint: () => {},
      },
    })
    expect(wrapper.find('section').exists()).toBe(false)
  })

  it('renders one row per checkpoint with the name', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [
          makeCheckpoint({ id: 'a', name: 'A' }),
          makeCheckpoint({ id: 'b', name: 'B' }),
          makeCheckpoint({ id: 'c', name: 'C' }),
        ],
        revealedHints: {},
        hasMoreHints: () => false,
        nextHintLevel: () => 0,
        showNextHint: () => {},
      },
    })
    const text = wrapper.text()
    expect(text).toContain('A')
    expect(text).toContain('B')
    expect(text).toContain('C')
    expect(wrapper.findAll('li')).toHaveLength(3)
  })

  it('does not show the hint UI when hintCount is 0', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ hintCount: 0 })],
        revealedHints: {},
        hasMoreHints: () => true,
        nextHintLevel: () => 1,
        showNextHint: () => {},
      },
    })
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders the "Get Hint" button when more hints are available', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ hintCount: 3 })],
        revealedHints: {},
        hasMoreHints: () => true,
        nextHintLevel: () => 1,
        showNextHint: () => {},
      },
    })
    const button = wrapper.find('button')
    expect(button.exists()).toBe(true)
    expect(button.text()).toContain('Get Hint 1/3')
  })

  it('calls showNextHint with the checkpoint id when the button is clicked', async () => {
    const showNext = vi.fn()
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ id: 'cp-x', hintCount: 3 })],
        revealedHints: {},
        hasMoreHints: () => true,
        nextHintLevel: () => 1,
        showNextHint: showNext,
      },
    })
    await wrapper.find('button').trigger('click')
    expect(showNext).toHaveBeenCalledWith('cp-x')
  })

  it('hides the "Get Hint" button when no more hints are available', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ hintCount: 3, hintLevelShown: 3 })],
        revealedHints: {},
        hasMoreHints: () => false,
        nextHintLevel: () => 0,
        showNextHint: () => {},
      },
    })
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.text()).toContain('All hints revealed')
  })

  it('renders revealed hints from revealedHints map', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ id: 'cp-1', hintCount: 3 })],
        revealedHints: {
          'cp-1': [
            { level: 1, text: 'Check your network config' },
            { level: 2, text: 'Ping the gateway' },
          ],
        },
        hasMoreHints: () => true,
        nextHintLevel: () => 3,
        showNextHint: () => {},
      },
    })
    expect(wrapper.text()).toContain('Check your network config')
    expect(wrapper.text()).toContain('Ping the gateway')
    expect(wrapper.text()).toContain('Hint 1/3')
    expect(wrapper.text()).toContain('Hint 2/3')
  })

  it('shows the penalty notice when hintPenaltyApplied > 0', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ hintCount: 3, hintPenaltyApplied: 5 })],
        revealedHints: {},
        hasMoreHints: () => false,
        nextHintLevel: () => 0,
        showNextHint: () => {},
      },
    })
    expect(wrapper.text()).toContain('5 points deducted')
  })

  it('hides the hints UI entirely once the checkpoint passes', () => {
    const wrapper = mount(CheckpointsList, {
      props: {
        checkpoints: [makeCheckpoint({ status: 'passed', hintCount: 3 })],
        revealedHints: {},
        hasMoreHints: () => true,
        nextHintLevel: () => 1,
        showNextHint: () => {},
      },
    })
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
