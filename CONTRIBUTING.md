# Working on a Fork of Kootenai

> **This repository is archived.** Issues and pull requests are closed, and nothing
> sent here will be reviewed or merged. There is no contribution process to follow.
>
> This guide is kept because the build, test and internationalization instructions in
> it are accurate and hard to reconstruct from the source alone. Read it as
> documentation for **your own fork**, which you may do whatever you like with — it is
> Apache 2.0, and there is no CLA.

## Quick Start

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/kootenai.git
cd kootenai

# Set up development environment
mage dev:setup
mage dev:up

# Verify everything works
mage test:all
```

## Prerequisites

- Go 1.26+
- Node.js 20 LTS
- Docker 24+
- Git 2.30+

## Development Workflow

These are the conventions the project used. Keeping them in a fork is optional; they
are written down because CI and the commit history assume them.

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

5. **Push to your own fork.** There is nowhere upstream to send it.

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
   - Backend: `cd api && go test ./internal/i18n/...` runs `TestCatalogParity`,
     which fails if `active.es.toml` diverges from `active.en.toml`. There is no
     `go.mod` at the repository root — the Go module is `api/`, so the `go` commands
     in this guide must be run from there.
6. **Pseudo-locale sweep.** Run `npm run dev:pseudo` (or set
   `VITE_I18N_PSEUDO=1`) and visit any view you touched. Visible plain-ASCII
   text means a string was hard-coded.

See also: [docs/i18n/glossary.md](docs/i18n/glossary.md) and the i18n
architecture decisions under `docs/architecture/` (ADR-0001, ADR-0003).

## Before You Commit

The checks CI used to enforce. Run them in a fork for the same reason CI did — they
catch the failures that are expensive to find later, particularly the i18n parity gate.

- [ ] Tests pass (`mage test:all`)
- [ ] Linting passes (`mage test:lint`)
- [ ] Documentation updated if needed
- [ ] Commits follow conventions
- [ ] New user-facing strings are keyed and present in both `en` and `es` catalogs (`npm run check:i18n-parity` is green)

## Getting Help

There is none to be had from this repository — Discussions and Issues are closed and
no one is reading them. What exists instead:

- **The docs site**: `cd docusaurus && npm install && npm start`
- **Known gaps and what to expect**: the "State of the project" section of `README.md`
- **Security posture you are inheriting**: [SECURITY.md](SECURITY.md), which explains
  why no vulnerability report will be triaged

## Full Documentation

For code examples, testing patterns and architecture guidelines, see the
**[Complete Contributing Guide](docusaurus/docs/development/contributing.md)** — its
pull-request sections have the same problem this file did, but the technical content
holds.

## License

Apache 2.0. Fork it, modify it, redistribute it, use it commercially — keep the
license and `NOTICE` and state your changes. There is no CLA to sign, and no
contribution to assign.
