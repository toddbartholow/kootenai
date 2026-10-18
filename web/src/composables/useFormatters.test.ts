import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { useFormatters } from './useFormatters'
import { i18n } from '@/locales'

type Formatters = ReturnType<typeof useFormatters>

/**
 * Mount a stub component that exposes the composable's return value so tests
 * can call the formatter functions inside a valid setup context.
 */
function harness(): Formatters {
  let api: Formatters | undefined
  const Harness = defineComponent({
    setup() {
      api = useFormatters()
      return () => h('div')
    },
  })
  mount(Harness)
  if (!api) throw new Error('harness did not initialise formatters')
  return api
}

describe('useFormatters', () => {
  beforeEach(() => {
    i18n.global.locale.value = 'en'
  })

  it('formatDate returns N/A for missing input', () => {
    const { formatDate } = harness()
    expect(formatDate(undefined)).toBe('N/A')
  })

  it('formatDate renders a date via Intl for valid input', () => {
    const { formatDate } = harness()
    const formatted = formatDate('2026-04-14T12:00:00Z')
    expect(formatted).toMatch(/2026|4|14/)
  })

  it('formatDuration handles minutes, hours, and mixed', () => {
    const { formatDuration } = harness()
    expect(formatDuration(undefined)).toBe('N/A')
    expect(formatDuration(0)).toBe('N/A')
    expect(formatDuration(1)).toBe('1m')
    expect(formatDuration(30)).toBe('30m')
    expect(formatDuration(60)).toBe('1h')
    expect(formatDuration(90)).toBe('1h 30m')
    expect(formatDuration(150)).toBe('2h 30m')
  })

  it('formatNumber uses Intl for the current locale', () => {
    const { formatNumber } = harness()
    expect(formatNumber(1234)).toBe('1,234')
  })
})
