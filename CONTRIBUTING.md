# Contributing to Kootenai

Thank you for your interest in contributing! This guide will get you started quickly.

## Quick Start

```bash
# Fork and clone
git clone https://github.com/YOUR-USERNAME/kootenai.git
cd kootenai

# Set up development environment
mage dev:setup
mage dev:up

# Verify everything works
mage test:all
```

## Prerequisites

- Go 1.24+
- Node.js 20 LTS
- Docker 24+
- Git 2.30+

## Development Workflow

1. **Create a branch** from `main`:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make changes** following our coding standards

3. **Run tests**:
   ```bash
   mage test:all
   ```

4. **Commit** using conventional commits:
   ```
   feat(api): add team member endpoint
   fix(web): correct session display
   docs: update API reference
   ```

5. **Push and create a Pull Request**

## Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/description` | `feature/add-roles` |
| Bug fix | `fix/description` | `fix/login-error` |
| Docs | `docs/description` | `docs/api-guide` |

## Code Standards

- **Go**: Follow [Effective Go](https://golang.org/doc/effective_go.html), run `go fmt`
- **TypeScript/Vue**: Use Composition API, run type checks
- **SQL**: UPPERCASE keywords, snake_case identifiers

## Adding User-Facing Strings (i18n)

Every string that can end up in a user's browser or API response must live in a
translation catalog — never inline English in a handler or component. The
workflow:

1. **Pick a key path.** Namespace by domain and intent:
   `views.sessions.startButton`, `organization.errors.slugAlreadyExists`.
   Reuse shared keys (`authz.authRequired`, `error.invalidRequestBody`)
   instead of duplicating.
2. **Add to English first.** Frontend: `web/src/locales/en.json`. Backend:
   `api/internal/i18n/translations/active.en.toml` (use `[namespace.key]`
   with an `other = "..."` field).
3. **Use the key, not the literal.** Frontend: `t('views.foo.bar')` via
   `useI18n()`. Backend handlers: `m.responder.LocalizedErrorResponse(ctx, w,
   status, "namespace.errors.key", nil)`. For parameter interpolation, pass
   `{"Field": value}` (Go) or `{ field: value }` (Vue) — the catalog uses
   `{{.Field}}` / `{field}`.
4. **Draft the Spanish translation.** LLM-assisted drafts are fine for the
   initial PR — consult `docs/i18n/glossary.md` for cybersecurity/lab domain
   terms. A human SME reviews periodically before the locale is announced.
5. **Gate with parity checks.**
   - Frontend: `npm run check:i18n-parity` (enforced in CI).
   - Backend: `go test ./api/internal/i18n/...` runs `TestCatalogParity`,
     which fails if `active.es.toml` diverges from `active.en.toml`.
6. **Pseudo-locale sweep.** Run `npm run dev:pseudo` (or set
   `VITE_I18N_PSEUDO=1`) and visit any view you touched. Visible plain-ASCII
   text means a string was hard-coded.

See also: [docs/i18n/glossary.md](docs/i18n/glossary.md) and the i18n
architecture decisions under `docs/architecture/` (ADR-0001, ADR-0003).

## Before Submitting a PR

- [ ] Tests pass (`mage test:all`)
- [ ] Linting passes (`mage test:lint`)
- [ ] Documentation updated if needed
- [ ] Commits follow conventions
- [ ] New user-facing strings are keyed and present in both `en` and `es` catalogs (`npm run check:i18n-parity` is green)

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/toddbartholow/kootenai/discussions)
- **Bugs**: [GitHub Issues](https://github.com/toddbartholow/kootenai/issues)
- **Security**: See [SECURITY.md](SECURITY.md)

## Full Documentation

For detailed guidelines including code examples, testing patterns, and architecture guidelines, see the **[Complete Contributing Guide](docusaurus/docs/development/contributing.md)**.

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 license.
