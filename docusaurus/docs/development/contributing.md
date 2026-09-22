---
title: Contributing Guide
description: Guidelines for contributing to Kootenai
tags:
  - development
  - contributing
  - guidelines
---

# Contributing Guide

:::warning Archived repository
Issues and pull requests are closed. Nothing sent to this repository will be reviewed
or merged, so the pull-request and review sections below describe a process that no
longer runs. The setup, build, testing and code-standards material is accurate and is
why the page is kept — read it as guidance for working in **your own fork**.
:::

This guide covers the development environment, the test suites and the conventions the
project used.

## Getting Started

### Prerequisites

- Go 1.25+
- Node.js 20 LTS
- Docker 24+
- Git 2.30+

### Setup

1. **Fork the repository**

   Fork [toddbartholow/kootenai](https://github.com/toddbartholow/kootenai) on GitHub.

2. **Clone your fork**

   ```bash
   git clone https://github.com/YOUR-USERNAME/kootenai.git
   cd kootenai
   ```

3. **Add upstream remote**

   ```bash
   git remote add upstream https://github.com/toddbartholow/kootenai.git
   ```

4. **Set up development environment**

   ```bash
   mage dev:setup
   mage dev:up
   ```

5. **Verify setup**

   ```bash
   mage test:all
   ```

## Workflow

### 1. Create a Branch

```bash
# Update main
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/my-feature
```

### Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/description` | `feature/add-team-roles` |
| Bug fix | `fix/description` | `fix/session-timeout` |
| Documentation | `docs/description` | `docs/api-reference` |
| Refactor | `refactor/description` | `refactor/auth-middleware` |

### 2. Make Changes

Follow the coding standards below. Commit often with clear messages.

### 3. Test Your Changes

```bash
# Run all tests
mage test:all

# Run specific tests
mage test:api
mage test:web

# Check linting
mage test:lint
```

### 4. Commit

Use conventional commit messages:

```
type(scope): description

[optional body]

[optional footer]
```

**Types:**

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `style` | Formatting (no code change) |
| `refactor` | Code restructure |
| `test` | Adding tests |
| `chore` | Maintenance |

**Examples:**

```
feat(api): add team member invitation endpoint
fix(web): correct session expiration display
docs(admin): add Proxmox troubleshooting section
test(server): add handler tests for auth endpoints
```

### 5. Push and Create PR

```bash
git push origin feature/my-feature
```

Then create a Pull Request on GitHub.

## Coding Standards

### Go Code

#### Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Run `go fmt` before committing
- Use `golangci-lint` for linting

#### Error Handling

```go
// Good: wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create pod: %w", err)
}

// Good: use structured logging
slog.Error("pod creation failed", "error", err, "labID", labID)
```

#### Naming

```go
// Good: clear, descriptive names
func CreatePodFromTemplate(ctx context.Context, tmpl *LabTemplate) (*Pod, error)

// Bad: abbreviated, unclear
func CrtPod(ctx context.Context, t *LT) (*P, error)
```

#### Testing

```go
// Good: table-driven tests
func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        wantErr bool
    }{
        {"valid input", validInput, false},
        {"missing name", noNameInput, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### TypeScript/Vue Code

#### Style

- Use TypeScript for type safety
- Follow Vue 3 Composition API patterns
- Run ESLint before committing

#### Components

```vue
<!-- Good: clear props and types -->
<script setup lang="ts">
interface Props {
  lab: Lab
  isLoading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  isLoading: false
})

const emit = defineEmits<{
  launch: [labId: string]
  delete: [labId: string]
}>()
</script>
```

#### Composables

```typescript
// Good: reusable composable
export function usePodActions(podId: Ref<string>) {
  const { toast } = useToast()

  const startPod = async () => {
    try {
      await podsApi.start(podId.value)
      toast.success('Pod started')
    } catch (error) {
      toast.error('Failed to start pod')
    }
  }

  return { startPod }
}
```

### SQL Code

- Use UPPERCASE for SQL keywords
- Use snake_case for identifiers
- Include comments for complex queries

```sql
-- Get user's active sessions with lab details
SELECT
    s.id,
    s.started_at,
    s.earned_points,
    lt.name AS lab_name
FROM lab_sessions s
JOIN lab_templates lt ON lt.id = s.lab_template_id
WHERE s.user_id = $1
    AND s.ended_at IS NULL
ORDER BY s.started_at DESC;
```

## Pull Request Process

### Before Submitting

- [ ] Tests pass (`mage test:all`)
- [ ] Linting passes (`mage test:lint`)
- [ ] Documentation updated (if needed)
- [ ] Commit messages follow conventions
- [ ] Branch is up to date with main

### PR Description

Include:

1. **What** - Summary of changes
2. **Why** - Motivation/context
3. **How** - Implementation approach
4. **Testing** - How you tested it

### Review Process

1. Automated checks run (CI)
2. Maintainer reviews code
3. Address feedback
4. Merge when approved

## Development Tips

### Hot Reload

**API:**

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with hot reload
cd api && air
```

**Web:**

```bash
cd web && npm run dev
```

### Database Access

```bash
# Open psql shell
mage db:psql

# Reset database
mage db:reset
mage db:migrate
mage db:seed
```

### View Logs

```bash
# Development logs
mage dev:logs

# Deployment logs
mage deploy:logs
```

### Debug API

```go
// Add debug logging
slog.Debug("processing request",
    "method", r.Method,
    "path", r.URL.Path,
    "user", user.ID,
)
```

Run with:

```bash
LOG_LEVEL=debug ./bin/labctl serve
```

## Architecture Guidelines

### Adding a New Endpoint

1. Add handler in `api/internal/server/`
2. Add route in router
3. Add tests
4. Update OpenAPI spec

### Adding a New Component

1. Create in `web/src/components/`
2. Add tests in same directory
3. Export from index if shared

### Adding a Migration

1. Create file in `api/internal/database/migrations/`
2. Follow naming convention: `{version}_{description}.sql`
3. Test locally before committing

## If something does not work

There is no help channel. Issues and pull requests are closed, Discussions were never
enabled, and nothing sent to this repository is read — see the banner at the top of this
page. Working on a fork means the debugging is yours.

- [Common issues](../troubleshooting/common-issues.md) — the documented failure modes
- `SECURITY.md` in the source tree describes the posture at archiving. It is a
  description, not an intake address: no report will be triaged.

## License

By contributing, you agree that your contributions will be licensed under the project's license.

## Recognition

Contributors are recognized in:

- README.md contributors section
- Release notes
- GitHub contributor graph

Thank you for contributing!
