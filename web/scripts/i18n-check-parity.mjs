#!/usr/bin/env node
/**
 * Key-parity gate for locale catalogs. Every non-English catalog must
 * have the exact same set of leaf keys as `en.json`:
 *   - missing keys would silently fall back to English (user sees mixed
 *     languages with no warning in prod builds)
 *   - extra keys are dead weight and usually point at a copy-paste bug
 *     from an older catalog
 *   - type mismatches (string where en.json has an object, or vice
 *     versa) break vue-i18n's `$t()` resolution
 *
 * Called from `npm run check:i18n-parity` and wired in CI before lint.
 *
 * Exit 0 on parity; exit 1 on any mismatch.
 */
import { readFile, readdir } from 'node:fs/promises'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = fileURLToPath(new URL('.', import.meta.url))
const LOCALES_DIR = join(HERE, '..', 'src', 'locales')
const BASELINE = 'en.json'

/** Flatten a nested object into `{ 'a.b.c': <leafType> }` entries.
 *  Leaf type is either 'string' or 'array'; objects recurse. */
function flattenKeys(obj, prefix = '') {
  const out = {}
  for (const [key, value] of Object.entries(obj)) {
    const full = prefix ? `${prefix}.${key}` : key
    if (value === null || value === undefined) {
      out[full] = 'null'
    } else if (Array.isArray(value)) {
      out[full] = 'array'
    } else if (typeof value === 'object') {
      Object.assign(out, flattenKeys(value, full))
    } else {
      out[full] = typeof value
    }
  }
  return out
}

async function loadCatalog(file) {
  const raw = await readFile(join(LOCALES_DIR, file), 'utf8')
  return JSON.parse(raw)
}

async function main() {
  const entries = await readdir(LOCALES_DIR)
  const catalogs = entries.filter((f) => f.endsWith('.json'))
  if (!catalogs.includes(BASELINE)) {
    console.error(`[i18n-parity] Missing baseline catalog: ${BASELINE}`)
    process.exit(1)
  }

  const baselineFlat = flattenKeys(await loadCatalog(BASELINE))
  const baselineKeys = new Set(Object.keys(baselineFlat))

  let hadError = false

  for (const file of catalogs) {
    if (file === BASELINE) continue
    const locale = file.replace(/\.json$/, '')
    const flat = flattenKeys(await loadCatalog(file))
    const keys = new Set(Object.keys(flat))

    const missing = [...baselineKeys].filter((k) => !keys.has(k))
    const extra = [...keys].filter((k) => !baselineKeys.has(k))
    const wrongType = [...keys].filter(
      (k) => baselineFlat[k] !== undefined && flat[k] !== baselineFlat[k],
    )

    if (missing.length === 0 && extra.length === 0 && wrongType.length === 0) {
      console.log(`[i18n-parity] ${locale}: ${keys.size} keys — OK`)
      continue
    }

    hadError = true
    console.error(`[i18n-parity] ${locale}: FAIL`)
    if (missing.length) {
      console.error(`  missing ${missing.length} key(s) (will silently fall back to English):`)
      for (const k of missing.slice(0, 20)) console.error(`    - ${k}`)
      if (missing.length > 20) console.error(`    … and ${missing.length - 20} more`)
    }
    if (extra.length) {
      console.error(`  extra ${extra.length} key(s) (dead weight, likely a stale merge):`)
      for (const k of extra.slice(0, 20)) console.error(`    + ${k}`)
      if (extra.length > 20) console.error(`    … and ${extra.length - 20} more`)
    }
    if (wrongType.length) {
      console.error(`  ${wrongType.length} type mismatch(es) (will break $t() resolution):`)
      for (const k of wrongType) {
        console.error(`    ! ${k}: expected ${baselineFlat[k]}, got ${flat[k]}`)
      }
    }
  }

  if (hadError) {
    console.error('\n[i18n-parity] One or more catalogs are out of sync with en.json.')
    console.error('Fix by adding missing keys (translate to match English intent) or')
    console.error('removing stale keys. Do not commit mismatched catalogs.')
    process.exit(1)
  }

  console.log('\n[i18n-parity] All catalogs in sync with en.json.')
}

main().catch((e) => {
  console.error('[i18n-parity] failed:', e)
  process.exit(1)
})
