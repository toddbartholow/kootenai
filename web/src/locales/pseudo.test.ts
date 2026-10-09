import { describe, it, expect } from 'vitest'
import { pseudoizeString, pseudoizeCatalog } from './pseudo'

describe('pseudoizeString', () => {
  it('wraps the result in detection brackets', () => {
    const out = pseudoizeString('Hello')
    expect(out.startsWith('⟦')).toBe(true)
    expect(out.endsWith('⟧')).toBe(true)
  })

  it('accentifies ASCII letters so unmigrated strings are obvious', () => {
    const out = pseudoizeString('Login')
    // Every letter gets an accent or variant; none of the original chars
    // should survive.
    expect(out).not.toMatch(/[A-Za-z]/)
  })

  it('adds roughly 30% padding characters to stress-test layout', () => {
    // "Login" is 5 chars; 30% = 1.5 → rounds to 2 pad chars.
    const out = pseudoizeString('Login')
    const pads = (out.match(/·/g) ?? []).length
    expect(pads).toBeGreaterThanOrEqual(2)
  })

  it('preserves named interpolation tokens verbatim', () => {
    const out = pseudoizeString('Welcome {userName}!')
    expect(out).toContain('{userName}')
  })

  it('preserves numeric interpolation tokens', () => {
    const out = pseudoizeString('{0} items selected')
    expect(out).toContain('{0}')
  })

  it('keeps plural-branch pipes intact', () => {
    const out = pseudoizeString('no items | one item | {count} items')
    // Two pipes → three branches.
    expect((out.match(/\|/g) ?? []).length).toBe(2)
    expect(out).toContain('{count}')
  })

  it("escapes literal @ so vue-i18n's linked-message parser doesn't trip", () => {
    // Without the escape, "your@email.com" pseudo-localizes to a string
    // where `@` is followed by accented letters and a dot, which vue-i18n
    // tries to parse as `@.modifier:keypath` and rejects.
    const out = pseudoizeString('your@email.com')
    expect(out).not.toMatch(/@[A-Za-zÀ-ž]/)
    expect(out).toContain("{'@'}")
  })

  it('pads each plural branch independently', () => {
    const out = pseudoizeString('short | this is much much longer')
    const branches = out.split('|')
    expect(branches).toHaveLength(2)
    for (const b of branches) {
      // Every branch gets its own brackets + padding.
      expect(b).toMatch(/⟦.*·.*⟧/)
    }
  })
})

describe('pseudoizeCatalog', () => {
  it('walks nested objects and pseudoizes every string leaf', () => {
    const input = {
      login: {
        title: 'Sign in',
        subtitle: 'Please authenticate',
      },
      difficulty: {
        beginner: 'Beginner',
      },
    }
    const out = pseudoizeCatalog(input)
    expect(out.login.title).toMatch(/^⟦.+⟧$/)
    expect(out.login.title).not.toContain('Sign in')
    expect(out.login.subtitle).not.toContain('Please')
    expect(out.difficulty.beginner).not.toContain('Beginner')
  })

  it('preserves non-string leaves unchanged', () => {
    const input = { count: 3, enabled: true, nullValue: null }
    const out = pseudoizeCatalog(input) as typeof input
    expect(out.count).toBe(3)
    expect(out.enabled).toBe(true)
    expect(out.nullValue).toBeNull()
  })

  it('preserves object structure (same keys)', () => {
    const input = { a: { b: { c: 'deep' } } }
    const out = pseudoizeCatalog(input)
    expect(Object.keys(out)).toEqual(['a'])
    expect(Object.keys(out.a)).toEqual(['b'])
    expect(Object.keys(out.a.b)).toEqual(['c'])
  })

  it('handles arrays of strings', () => {
    const out = pseudoizeCatalog(['one', 'two']) as string[]
    expect(out).toHaveLength(2)
    expect(out[0]).toMatch(/^⟦.+⟧$/)
  })
})
