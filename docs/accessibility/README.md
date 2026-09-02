# Accessibility

This directory consolidates the accessibility audit documents that previously
lived at the root of `web/`. Nothing here is authoritative policy — it is the
running audit log.

| File | Purpose |
|---|---|
| [summary.md](summary.md) | Short overview of components audited and current status |
| [audit-report.md](audit-report.md) | Original findings from the 2025-12-29 audit |
| [comprehensive-audit.md](comprehensive-audit.md) | Follow-up wider audit across the full app |
| [action-plan.md](action-plan.md) | Prioritized remediation plan |
| [fixes-guide.md](fixes-guide.md) | Concrete code-level fix patterns |

## Status

Accessibility is **not yet gated by tooling**. `eslint-plugin-vuejs-accessibility`
is not installed, and none of the audit items are tracked in CI. Treat the
documents above as aspirational until that changes — the code itself is the
source of truth.

When an a11y rule becomes enforceable, add it to `web/eslint.config.js`
(Vue section) and remove the corresponding item from `action-plan.md`.
