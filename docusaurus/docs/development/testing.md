---
title: Testing Guide
description: Guide to testing Kootenai components
tags:
  - development
  - testing
  - quality
---

# Testing Guide

This guide covers testing strategies, tools, and best practices for Kootenai development.

## Testing Stack

| Component | Framework | Purpose |
|-----------|-----------|---------|
| API (Go) | go test | Unit and integration tests |
| Web (Vue) | Vitest | Unit tests |
| E2E | labtest | End-to-end testing |
| Linting | golangci-lint, ESLint | Code quality |

## Running Tests

### All Tests

```bash
# Using Mage
mage test:all

# Manual
cd api && go test ./...
cd web && npm test
```

### API Tests

```bash
# All API tests
mage test:api

# With coverage
mage test:apiCoverage

# With race detection
mage test:apiRace

# Specific package
cd api && go test ./internal/server/...

# Verbose output
cd api && go test -v ./internal/server/...

# Specific test
cd api && go test -v -run TestHealthHandler ./internal/server/...
```

### Web Tests

```bash
# All web tests
mage test:web

# Manual
cd web && npm test

# Watch mode
cd web && npm run test:watch

# Coverage
cd web && npm run test:coverage
```

### E2E Tests

```bash
# Install labtest
cd tools/labtest && pip install -e .

# Quick health check
labtest quick

# Full test suite
labtest all

# Specific test
labtest pods
labtest sessions
labtest checkpoints
```

## Writing Tests

### Go Unit Tests

#### Test File Structure

```go
// server/handlers_test.go
package server

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestHealthHandler(t *testing.T) {
    // Arrange
    server := NewTestServer(t)

    // Act
    req := httptest.NewRequest("GET", "/health", nil)
    rec := httptest.NewRecorder()
    server.ServeHTTP(rec, req)

    // Assert
    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}
```

#### Table-Driven Tests

```go
func TestValidateLabTemplate(t *testing.T) {
    tests := []struct {
        name    string
        input   LabTemplate
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid template",
            input: LabTemplate{
                Name:     "Test Lab",
                Platform: "proxmox",
            },
            wantErr: false,
        },
        {
            name: "missing name",
            input: LabTemplate{
                Platform: "proxmox",
            },
            wantErr: true,
            errMsg:  "name is required",
        },
        {
            name: "invalid platform",
            input: LabTemplate{
                Name:     "Test Lab",
                Platform: "invalid",
            },
            wantErr: true,
            errMsg:  "invalid platform",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateLabTemplate(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

#### Mocking

```go
// Define interface
type LabRepository interface {
    GetByID(id string) (*Lab, error)
    List() ([]*Lab, error)
}

// Mock implementation
type MockLabRepository struct {
    mock.Mock
}

func (m *MockLabRepository) GetByID(id string) (*Lab, error) {
    args := m.Called(id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*Lab), args.Error(1)
}

// Use in test
func TestGetLab(t *testing.T) {
    mockRepo := new(MockLabRepository)
    mockRepo.On("GetByID", "test-id").Return(&Lab{Name: "Test"}, nil)

    service := NewLabService(mockRepo)
    lab, err := service.GetLab("test-id")

    require.NoError(t, err)
    assert.Equal(t, "Test", lab.Name)
    mockRepo.AssertExpectations(t)
}
```

### Vue Component Tests

#### Basic Component Test

```typescript
// LabCard.test.ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LabCard from './LabCard.vue'

describe('LabCard', () => {
  it('renders lab name', () => {
    const wrapper = mount(LabCard, {
      props: {
        lab: {
          id: '1',
          name: 'Linux Foundations',
          difficulty: 'beginner'
        }
      }
    })

    expect(wrapper.text()).toContain('Linux Foundations')
    expect(wrapper.text()).toContain('beginner')
  })

  it('emits launch event when button clicked', async () => {
    const wrapper = mount(LabCard, {
      props: { lab: { id: '1', name: 'Test' } }
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('launch')).toBeTruthy()
    expect(wrapper.emitted('launch')![0]).toEqual(['1'])
  })
})
```

#### Testing with Pinia Store

```typescript
import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { useLabStore } from '@/stores/lab'
import LabList from './LabList.vue'

describe('LabList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('displays labs from store', async () => {
    const store = useLabStore()
    store.labs = [
      { id: '1', name: 'Lab 1' },
      { id: '2', name: 'Lab 2' }
    ]

    const wrapper = mount(LabList)

    expect(wrapper.findAll('.lab-card')).toHaveLength(2)
  })
})
```

#### Testing Async Operations

```typescript
import { describe, it, expect, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { mount } from '@vue/test-utils'
import PodView from './PodView.vue'
import { podsApi } from '@/api/client'

vi.mock('@/api/client', () => ({
  podsApi: {
    list: vi.fn(),
    create: vi.fn()
  }
}))

describe('PodView', () => {
  it('loads pods on mount', async () => {
    vi.mocked(podsApi.list).mockResolvedValue([
      { id: '1', name: 'pod-1', status: 'running' }
    ])

    const wrapper = mount(PodView)
    await flushPromises()

    expect(podsApi.list).toHaveBeenCalled()
    expect(wrapper.text()).toContain('pod-1')
  })
})
```

### E2E Tests with labtest

#### Test Configuration

```yaml
# labtest.yaml
api_base_url: "http://<INFRA_VM_IP>:8080"
db_host: "<INFRA_VM_IP>"
db_port: 5432
db_user: "labadmin"
db_name: "kootenai"
verbose: false
timeout: 300
```

#### Custom Test Script

```python
# tests/custom_test.py
from labtest import LabTestClient

def test_lab_workflow():
    client = LabTestClient()

    # Create pod
    pod = client.create_pod("linux-foundations")
    assert pod.status == "provisioning"

    # Wait for ready
    client.wait_for_pod(pod.id, timeout=120)
    pod = client.get_pod(pod.id)
    assert pod.status == "running"

    # Start session
    session = client.start_session(pod.id)
    assert session.status == "active"

    # Verify checkpoints exist
    progress = client.get_progress(session.id)
    assert len(progress.checkpoints) > 0

    # Cleanup
    client.destroy_pod(pod.id)
```

## Test Coverage

### API Coverage

```bash
# Generate coverage
cd api && go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# View in browser
go tool cover -html=coverage.out

# Check specific package
go test -coverprofile=coverage.out ./internal/server/...
go tool cover -func=coverage.out | grep -E "total:|server"
```

### Web Coverage

```bash
cd web && npm run test:coverage
```

Coverage report in `web/coverage/index.html`.

### Coverage Targets

These are design intent — the bar the project aimed at, not a record of what was reached.
A "Current" column used to sit alongside them; every figure in it overstated the real
number, and two of them inverted the comparison (`internal/proxmox` was documented as
clearing its target when it did not, and so were the web components). Rather than freeze a
fresh set of numbers that will rot the same way, measure your own:

```bash
cd api && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
cd web && npm run test:coverage
```

| Package | Target |
|---------|--------|
| `internal/server` | 70% |
| `internal/orchestrator` | 60% |
| `internal/proxmox` | 80% |
| `internal/redis` | 70% |
| Web components | 60% |

## Integration Testing

### Database Tests

```go
func TestUserRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test database
    db := setupTestDB(t)
    defer db.Close()

    repo := NewUserRepository(db)

    // Test CRUD
    user := &User{Email: "test@example.com", Name: "Test"}
    err := repo.Create(user)
    require.NoError(t, err)

    found, err := repo.GetByEmail("test@example.com")
    require.NoError(t, err)
    assert.Equal(t, "Test", found.Name)

    err = repo.Delete(user.ID)
    require.NoError(t, err)
}
```

### API Integration Tests

```go
func TestLabEndpoints_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    server := setupIntegrationServer(t)

    t.Run("create and get lab", func(t *testing.T) {
        // Create
        resp := server.Post("/api/v1/labs", labJSON)
        require.Equal(t, 201, resp.Code)

        var created Lab
        json.Unmarshal(resp.Body.Bytes(), &created)

        // Get
        resp = server.Get("/api/v1/labs/" + created.ID)
        require.Equal(t, 200, resp.Code)

        var fetched Lab
        json.Unmarshal(resp.Body.Bytes(), &fetched)
        assert.Equal(t, created.Name, fetched.Name)
    })
}
```

## Linting

### Go Linting

```bash
# Run golangci-lint
mage test:lint

# Or directly
cd api && golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

Configuration in `api/.golangci.yml`.

### Web Linting

```bash
cd web && npm run lint

# Auto-fix
npm run lint:fix
```

## CI/CD Testing

Tests run automatically in GitHub Actions:

```yaml
# .github/workflows/ci.yml
jobs:
  test-api:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: cd api && go test -race ./...

  test-web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: cd web && npm ci && npm test
```

## Best Practices

### Test Naming

```go
// Good: describes behavior
func TestCreatePod_WithInvalidLab_ReturnsError(t *testing.T)
func TestHealthHandler_WhenDBDown_ReturnsUnhealthy(t *testing.T)

// Bad: unclear
func TestCreatePod(t *testing.T)
func TestHandler(t *testing.T)
```

### Test Isolation

```go
// Good: each test is independent
func TestA(t *testing.T) {
    db := setupCleanDB(t)
    // Test with clean state
}

func TestB(t *testing.T) {
    db := setupCleanDB(t)
    // Test with clean state
}
```

### Assertion Helpers

```go
// Custom assertion helpers
func assertValidPod(t *testing.T, pod *Pod) {
    t.Helper()
    assert.NotEmpty(t, pod.ID)
    assert.NotEmpty(t, pod.Name)
    assert.NotZero(t, pod.CreatedAt)
}

func TestCreatePod(t *testing.T) {
    pod := createTestPod(t)
    assertValidPod(t, pod)
}
```

## Related Documentation

- [CI/CD](ci-cd.md) - Continuous integration setup
- [Mage Build System](mage.md) - Build and test commands
- [Contributing](contributing.md) - Contribution guidelines
