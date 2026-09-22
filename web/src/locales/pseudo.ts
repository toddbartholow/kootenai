/**
 * Pseudo-locale generator (`en-XA`).
 *
 * Transforms an English catalog into a "loud" variant that exposes three
 * classes of i18n defects *without* needing a second translator to be
 * involved:
 *
 * 1. **Unmigrated strings** — anything still hardcoded in a template shows
 *    up as plain English next to brackets-and-accents; it's instantly
 *    visible during a visual sweep.
 * 2. **Layout breakage from longer translations** — real translations
 *    often run 30% longer than English (German is the usual worst case).
 *    We pad every string by ~30% so CSS overflow, truncation, and
 *    fixed-width containers fail early instead of shipping to customers.
 * 3. **Interpolation bugs** — ICU-style `{named}` placeholders and the
 *    vue-i18n plural separator (`|`) must survive transformation, or
 *    runtime formatting falls apart. We preserve both.
 *
 * The output is not meant to be readable prose; it's a diagnostic.
 */

const ACCENT_MAP: Record<string, string> = {
  a: 'á', b: 'ƀ', c: 'ç', d: 'đ', e: 'é', f: 'ƒ', g: 'ĝ', h: 'ĥ', i: 'ḯ',
  j: 'ĵ', k: 'ķ', l: 'ĺ', m: 'ɱ', n: 'ñ', o: 'ó', p: 'ṗ', q: 'ǫ', r: 'ŕ',
  s: 'ṩ', t: 'ţ', u: 'ú', v: 'ṽ', w: 'ŵ', x: 'ẋ', y: 'ý', z: 'ž',
  A: 'Á', B: 'Ɓ', C: 'Ç', D: 'Đ', E: 'É', F: 'Ƒ', G: 'Ĝ', H: 'Ĥ', I: 'Ḯ',
  J: 'Ĵ', K: 'Ķ', L: 'Ĺ', M: 'Ṁ', N: 'Ñ', O: 'Ó', P: 'Ṗ', Q: 'Ǫ', R: 'Ŕ',
  S: 'Ṩ', T: 'Ţ', U: 'Ú', V: 'Ṽ', W: 'Ŵ', X: 'Ẋ', Y: 'Ý', Z: 'Ž',
}

/** Matches any `{name}` or `{0}`-style vue-i18n interpolation token. */
const INTERPOLATION = /\{[^}]+\}/g
/** vue-i18n plural separator; must remain unescaped. */
const PLURAL_PIPE = '|'
/**
 * vue-i18n's message compiler treats `@` as the prefix for linked-message
 * syntax (`@:key`, `@.modifier:key`). A literal `@` followed by accented
 * letters (e.g. `your@email.com` → `ýóúŕ@éɱáḯĺ.çóɱ`) makes the parser try
 * to resolve a link and throw "Invalid linked format". Replace each literal
 * `@` with the vue-i18n literal-interpolation form so it round-trips back
 * to `@` at render time without invoking the linked-message lexer.
 */
const VUE_I18N_AT_ESCAPE = "{'@'}"
/** Pad ratio: 30% extra visible characters, chosen to catch most overflow
 *  bugs without making copy illegible in screenshots. */
const PAD_RATIO = 0.3
const PAD_CHAR = '·'
/** Brackets make the pseudo-locale visually distinct at a glance. */
const OPEN = '⟦'
const CLOSE = '⟧'

function accentify(text: string): string {
  let out = ''
  for (const ch of text) {
    out += ACCENT_MAP[ch] ?? ch
  }
  return out
}

/**
 * Transform a single string: accentify each letter, pad proportionally,
 * preserve interpolation placeholders verbatim, and keep plural-branch
 * pipes intact.
 */
export function pseudoizeString(value: string): string {
  // Split on the plural separator and recurse so each branch is padded on
  // its own (otherwise a long first branch swallows the `|`).
  if (value.includes(PLURAL_PIPE)) {
    return value.split(PLURAL_PIPE).map(pseudoizeString).join(PLURAL_PIPE)
  }

  // Mask interpolation tokens before accenting; restore them afterwards so
  // `{count}` doesn't become `{çóúñţ}`.
  const tokens: string[] = []
  const masked = value.replace(INTERPOLATION, (match) => {
    tokens.push(match)
    return `\u0000${tokens.length - 1}\u0000`
  })

  const accented = accentify(masked)

  // eslint-disable-next-line no-control-regex -- NUL-byte sentinel is a deliberate placeholder that no legitimate input can contain; used to mask interpolation tokens from the accent-mapper.
  const unmasked = accented.replace(/\u0000(\d+)\u0000/g, (_, idx) => {
    const i = Number(idx)
    return tokens[i] ?? ''
  })

  // Escape literal `@` so vue-i18n's linked-message lexer doesn't try to
  // parse it. Done after accenting so the escape sequence itself isn't
  // accented, and after token restoration so we don't touch placeholders.
  const escaped = unmasked.replace(/@/g, VUE_I18N_AT_ESCAPE)

  // Pad AFTER restoring tokens so padding never ends up inside a token.
  // Visible length excludes interpolations and the @-escape since neither
  // is rendered as on-screen characters.
  const visibleLength = escaped.replace(INTERPOLATION, '').length
  const padLength = Math.max(1, Math.round(visibleLength * PAD_RATIO))
  const padding = PAD_CHAR.repeat(padLength)

  return `${OPEN}${escaped}${padding}${CLOSE}`
}

/**
 * Walk a catalog recursively and pseudoize every string leaf. Preserves
 * object structure and non-string leaves (numbers, booleans) unchanged so
 * the transformed catalog remains type-compatible with the source.
 */
export function pseudoizeCatalog<T>(catalog: T): T {
  if (typeof catalog === 'string') {
    return pseudoizeString(catalog) as unknown as T
  }
  if (Array.isArray(catalog)) {
    return catalog.map((item) => pseudoizeCatalog(item)) as unknown as T
  }
  if (catalog !== null && typeof catalog === 'object') {
    const out: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(catalog)) {
      out[key] = pseudoizeCatalog(value)
    }
    return out as T
  }
  return catalog
}
