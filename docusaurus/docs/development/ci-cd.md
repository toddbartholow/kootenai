# CI Checks

The public release ships no GitHub Actions workflows. The repository is archived,
so nothing would run them. While the project was active, every push and pull
request to `main` ran the checks below, and they passed when it was archived.
This page lists them so you can run the same checks locally or rebuild a
pipeline in a fork.

## API (Go)

Run from `api/`:

```bash
go vet ./...
gofmt -l .                     # must print nothing
golangci-lint run --timeout=5m # CI used golangci-lint v2.11.1
go test -race ./...
go build -o bin/labctl ./cmd/labctl
```

Security checks, also from `api/`:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
govulncheck ./...
gosec -fmt=text -severity=high -exclude-generated -exclude=G115,G118,G122,G703 ./...
```

## Web (Vue)

Run from `web/`:

```bash
npm ci
npm run check:api-types        # generated client matches api/docs/swagger.json
npm run check:i18n-parity      # every catalog has the same keys as en.json
npm run typecheck
npm run lint
npm run test:coverage
npm run build
npm audit --audit-level=high
```

The pseudo-locale sweep runs from `tests/e2e/` after `npm ci` in `web/`, and
needs a Chromium download:

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:pseudo
```

## Python

Run from `python/`:

```bash
pip install -e ".[dev]"
ruff check .
pytest -q
```

Then run the tests, without the lint step, from `tools/labtest/` and again
from `tools/sectest/`:

```bash
pip install -e ".[dev]"
pytest -q
```

## Repository-wide

From the repository root:

```bash
gitleaks dir --no-banner --redact --exit-code 1 .
trivy fs --severity CRITICAL,HIGH --exit-code 1 .
docker build -t kootenai-api:test ./api
docker build -t kootenai-web:test ./web
```

CI also scanned both built images with Trivy at the same severity threshold.

## Mage

`mage test:all` runs the API and web test suites in one step. See
[Mage](mage.md) for the other targets.
