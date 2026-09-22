#!/usr/bin/env node
/**
 * i18n audit — heuristic scanner that flags probable hardcoded user-facing
 * strings inside Vue SFC templates. Complements, not replaces, a manual
 * review: every match needs a human decision (is it UI copy? is it an
 * internal label that doesn't ship to users? is it a `data-testid`?).
 *
 * What it catches:
 *   - `<h1>Capitalized Text</h1>` and friends (block-level headings/paras
 *     typically contain copy)
 *   - `<label>Capitalized Text</label>`
 *   - `placeholder="Capitalized text..."` attributes
 *   - `label="Create Pod"` / `header="..."` / `summary="..."` attributes
 *     on PrimeVue components
 *
 * What it deliberately ignores:
 *   - `{{ $t(...) }}` — already migrated
 *   - `{{ computed.value }}` — dynamic, not copy
 *   - `<script>` blocks — covered by grepping `toast.add({ summary: ... })`
 *     manually (see `audit-script.mjs` TODO)
 *   - Single-word strings (mostly class names, test IDs, field values)
 *   - ALL_CAPS / camelCase / URL-looking strings
 *
 * Exit code is always 0; this is an audit, not a gate. CI prints the
 * report as a build artifact for review.
 */
import { readdir, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const SRC = join(HERE, '..', 'src')

/** Heuristic for a "likely copy" string — has at least two words, starts
 *  with a capital letter, and doesn't look like an identifier or URL. */
const LIKELY_COPY = /^[A-Z][a-zA-Z0-9]*(?:\s+[A-Za-z0-9][a-zA-Z0-9'&.]*){1,}[!?.]?$/
const IGNORES = [
  /^[A-Z_]+$/, // ALL_CAPS (constants)
  /^https?:\/\//, // URLs
  /\//, // path-like
]

/** Attribute patterns on Vue/PrimeVue components that almost always carry
 *  user-visible copy. */
const ATTR_PATTERNS = [
  /\bplaceholder="([^"]+)"/g,
  /\blabel="([^"]+)"/g,
  /\bheader="([^"]+)"/g,
  /\bsummary="([^"]+)"/g,
  /\btitle="([^"]+)"/g,
  /\baria-label="([^"]+)"/g,
]

/** Plain text between block-level tags (the opening tag cannot itself be
 *  a `v-text`/`v-html` binding). */
const BLOCK_TEXT = /<(h[1-6]|p|label|span|div|a|button)(?:\s[^>]*)?>([^<{]*)</g

function isLikelyCopy(s) {
  const t = s.trim()
  if (t.length < 4) return false
  if (IGNORES.some((r) => r.test(t))) return false
  return LIKELY_COPY.test(t)
}

async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  const out = []
  for (const e of entries) {
    const full = join(dir, e.name)
    if (e.isDirectory()) {
      out.push(...(await walk(full)))
    } else if (e.name.endsWith('.vue')) {
      out.push(full)
    }
  }
  return out
}

function extractTemplate(src) {
  const m = src.match(/<template[^>]*>([\s\S]*?)<\/template>/)
  return m ? m[1] : ''
}

async function auditFile(path) {
  const src = await readFile(path, 'utf8')
  const tpl = extractTemplate(src)
  const hits = []

  for (const pattern of ATTR_PATTERNS) {
    let m
    pattern.lastIndex = 0
    while ((m = pattern.exec(tpl)) !== null) {
      const value = m[1]
      if (isLikelyCopy(value)) {
        const line = tpl.slice(0, m.index).split('\n').length
        hits.push({ kind: 'attr', line, text: value })
      }
    }
  }

  let m
  BLOCK_TEXT.lastIndex = 0
  while ((m = BLOCK_TEXT.exec(tpl)) !== null) {
    const value = m[2].trim()
    if (!value) continue
    // Skip anything that's a vue-i18n expression or a Vue binding.
    if (value.includes('$t(') || value.includes('{{')) continue
    if (isLikelyCopy(value)) {
      const line = tpl.slice(0, m.index).split('\n').length
      hits.push({ kind: 'text', line, text: value })
    }
  }

  return hits
}

async function main() {
  const files = await walk(SRC)
  const report = []
  for (const f of files) {
    const hits = await auditFile(f)
    if (hits.length) report.push({ file: f, hits })
  }

  if (report.length === 0) {
    console.log('[i18n-audit] No likely-unmigrated strings found.')
    return
  }

  const total = report.reduce((acc, r) => acc + r.hits.length, 0)
  console.log(`[i18n-audit] ${total} probable string(s) across ${report.length} file(s):\n`)
  for (const { file, hits } of report) {
    const rel = file.replace(SRC, 'src')
    console.log(rel)
    for (const h of hits) {
      console.log(`  ${String(h.line).padStart(4)}  ${h.kind.padEnd(4)}  ${h.text}`)
    }
    console.log()
  }
}

main().catch((e) => {
  console.error('[i18n-audit] failed:', e)
  process.exit(1)
})
