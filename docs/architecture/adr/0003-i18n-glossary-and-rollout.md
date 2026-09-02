# ADR 0003: i18n Glossary and Language Rollout Strategy

- **Status**: Accepted
- **Date**: 2026-04-14
- **Deciders**: Kootenai maintainers
- **Supersedes**: Extends ADR-0001 (i18n strategy)

## Context

ADR-0001 established the tooling (vue-i18n, go-i18n, `Accept-Language` routing). It deliberately left three things unanswered:

1. **Which domain terms do we translate, and which stay English?** Security and networking vocabulary is full of English borrowings that native speakers already use in their working environment ("snapshot", "VLAN", "pod"). Translating them for the sake of translation confuses domain experts; leaving everything English undermines the point of localizing.
2. **Which language ships first, and in what order?**
3. **How do we prevent regressions once the catalog grows beyond one developer's memory?**

This ADR records the answers so a future contributor adding a locale has a clear playbook.

## Decisions

### Glossary — what we do and don't translate

The project-wide glossary lives at `docs/i18n/glossary.md` and is authoritative for all translators. Summary of the core policy:

| Term | Translate? | Notes |
| --- | --- | --- |
| Lab | Yes | "Laboratorio" in `es`, etc. — pedagogical term, universal |
| Lab Template | Yes | — |
| Pod | **No** | Virtualization term of art; translating creates ambiguity with "legumes" in several Latin languages |
| Session | Yes | — |
| Snapshot | **No** | Industry-standard term; every vendor's UI uses it untranslated |
| Pathway | Yes | "Ruta de aprendizaje" etc. |
| Module | Yes | — |
| Enrollment | Yes | — |
| Objective | Yes | Pedagogical use, not military/project-management |
| VM / Virtual Machine | **No** | Universal abbreviation |
| VLAN / Subnet / Network Segment | **No** | Networking terms of art |
| Proxmox / CloudStack | **No** | Product names |
| Instructor / Admin / Owner / Member | Yes | Role titles |
| Beginner / Intermediate / Advanced / Expert | Yes | |
| Pass Threshold | Yes | |
| Active / Inactive / Draft / Published | Yes | |
| "Created", "Updated", "Last seen" etc. | Yes | UI chrome |

The full glossary also covers product-specific noun-verb pairs ("provision a pod", "revert to snapshot") with recommended renderings per locale, and a "do not translate" list for brand/product names.

### Language rollout order

1. **English (`en`)** — baseline, already in place.
2. **`en-XA` pseudo-locale** — not a real language. Enabled locally via `npm run dev:pseudo` (sets `VITE_I18N_PSEUDO=1`). Transforms the English catalog into `⟦Ṩḯĝñ ḯñ··············⟧`-style output to expose unmigrated strings and layout overflow. Used for QA before shipping any new language.
3. **Spanish (`es`)** — first real non-English locale. Largest cyber-ed user segment after English; no script/directionality complications; easy to source reviewers.
4. **Portuguese (Brazil) (`pt-BR`)**.
5. **French (`fr`)**.
6. **German (`de`)** — worth-catching layout stress (German strings run 30-50% longer than English; the pseudo-locale's 30% padding approximates but doesn't fully cover this).
7. **Japanese (`ja`)** — first non-Latin locale. Introduces CJK font stacks, different line-breaking behavior, word-count-based copy issues.
8. **Chinese Simplified (`zh-CN`)**.
9. **Arabic (`ar`)** — only add after RTL infrastructure is in place (ADR-TBD).

Each locale must pass the same gate before shipping: (a) 100% key parity with `en.json`, (b) SME review against the glossary, (c) pseudo-locale + target-locale smoke test of login / dashboard / one lab.

### Sourcing translations

1. **LLM draft + native-speaker SME review.** The first draft for each locale is machine-translated using the glossary as context. A native-speaker reviewer with domain background goes through every key and fixes it. Cheap, fast, and the only practical option for a project of this size.
2. **Agency or community (Crowdin/Weblate)** — tracked as future work; not worth the overhead for the first 3-4 locales.

Translators always work in the JSON catalog directly (not through a TMS) until we cross ~1000 keys.

### Anti-regression tooling

All gated in CI (see `.github/workflows/ci.yml`):

1. **Key parity.** `npm run check:i18n-parity` fails if any non-English catalog has missing, extra, or type-mismatched keys relative to `en.json`. Runs before typecheck and lint.
2. **Pseudo-locale build.** `npm run dev:pseudo` is available locally. Not gated in CI yet — gating requires a Playwright snapshot run, tracked separately.
3. **Hardcoded-string audit.** `npm run i18n:audit` heuristically flags probable user-facing strings still in templates. Not gated (too many false positives) — printed as an informational build artifact and reviewed manually when touching a file.
4. **Missing-key telemetry.** vue-i18n's `missing` handler and go-i18n's `DefaultMessage` callback log every cache miss. A production alarm fires when miss rate for any locale exceeds 1% of requests (wired once we have the first non-English locale live).

## Consequences

### Positive

- Translators have an unambiguous yes/no decision for every domain term before they start.
- CI catches the most common failure mode (locale drift) before it reaches users.
- The pseudo-locale lets any developer spot unmigrated UI without waiting for a native speaker.
- Rollout is predictable: each locale goes through the same four-step gate.

### Negative

- The glossary will drift unless reviewed each time we add a domain concept. Owner: whoever adds the concept. Reviewed quarterly alongside strategy review.
- LLM-drafted translations still require an SME pass; we do not yet have an SME on retainer for any specific language. Onboarding the first SME for each locale is a blocker until solved.
- Pseudo-locale padding (30%) catches most overflow cases but won't catch German's worst case (50%+) or Japanese's narrow-but-tall issues. Belt-and-suspenders: the pseudo pass is not a substitute for a target-locale smoke test.

### Neutral

- The rollout order is a recommendation, not a gate. If a major customer asks for `ko` before `es`, we reorder.

## References

- ADR-0001: i18n/l10n strategy (the baseline)
- `web/src/locales/pseudo.ts` — pseudo-locale implementation
- `web/scripts/i18n-check-parity.mjs` — parity gate
- `web/scripts/i18n-audit.mjs` — hardcoded-string heuristic
- `docs/i18n/glossary.md` — authoritative term list
