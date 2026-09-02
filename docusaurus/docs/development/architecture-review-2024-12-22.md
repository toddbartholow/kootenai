# Kootenai Platform - Architecture Review Report

**Review Date:** 2024-12-22
**Codebase Version:** Commit d46019f
**Reviewer:** Automated Architecture Analysis

---

## Executive Summary

This architecture review evaluates a Go-based virtual lab orchestration platform for cybersecurity education that manages lab environments across Proxmox and Apache CloudStack. The codebase demonstrates **solid architectural fundamentals** with well-structured packages, comprehensive resilience patterns, and thoughtful separation of concerns. The platform consists of ~98 Go files across 27 packages, totaling approximately 15,000-20,000 lines of code.

**Overall Assessment: SOLID (B+) - 85/100**
- Strong resilience patterns and error handling
- Well-defined repository interfaces with dependency injection
- Good package structure following Go standards
- Some architectural concerns around dual storage patterns and missing abstractions

---

## 1. Overall Architecture Assessment

### Package Structure - EXCELLENT

The codebase follows **Go standard project layout** effectively:

```
/Users/todd/Downloads/Claude/kootenai/api/
├── cmd/labctl/              # Entry point (CLI + server)
├── internal/                # Private packages
│   ├── server/             # HTTP server & handlers
│   ├── orchestrator/       # Core business logic
│   ├── database/           # Data layer
│   │   └── repositories/   # Repository pattern
│   ├── proxmox/           # Infrastructure client
│   ├── cloudstack/        # Infrastructure client
│   ├── redis/             # Caching & session layer
│   ├── wazuh/             # SIEM integration
│   ├── resilience/        # Circuit breakers & retry
│   ├── checkpoint/        # Assessment logic
│   ├── auth/              # Authentication
│   ├── canvas/            # LTI integration
│   ├── nats/              # Event streaming
│   ├── websocket/         # Real-time communication
│   └── ...                # Supporting packages
```

**Strengths:**
- Clear separation of concerns with focused packages
- Infrastructure clients isolated in dedicated packages
- Business logic (orchestrator) separated from HTTP concerns (server)
- Cross-cutting concerns (resilience, auth) properly abstracted

---

## 2. Design Patterns Analysis

### 2.1 Repository Pattern - WELL-IMPLEMENTED

**Location:** `internal/database/repositories/interfaces.go`

The codebase demonstrates excellent use of the Repository pattern:

```go
// Interface-based repositories for all entities
type LabTemplateRepository interface {
    Create(ctx context.Context, record *models.LabTemplateRecord) error
    GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error)
    // ... full CRUD operations
}

type PodRepository interface { ... }
type SessionRepository interface { ... }
type CheckpointProgressRepository interface { ... }
```

**Strengths:**
- Well-defined interfaces separate from implementations
- Context-first design for cancellation support
- Filter structs for flexible querying
- Comprehensive coverage (10+ repository interfaces)

### 2.2 Dependency Injection - STRONG

**Location:** `internal/server/server.go` (lines 120-177)

**Functional Options Pattern** used effectively:

```go
type ServerOption func(*Server)

func WithLabTemplateRepo(repo repositories.LabTemplateRepository) ServerOption {
    return func(s *Server) { s.labTemplateRepo = repo }
}

func WithAuthService(authSvc *auth.Service) ServerOption {
    return func(s *Server) { s.authService = authSvc }
}
```

**Strengths:**
- Non-breaking API evolution (add options without breaking existing code)
- Optional dependencies handled gracefully
- Used consistently across Server, Orchestrator, infrastructure clients
- Supports partial initialization (e.g., running without database)

### 2.3 Circuit Breaker Pattern - EXCELLENT

**Location:** `internal/resilience/resilience.go`

**State Machine Implementation:**

```go
type CircuitState int
const (
    StateClosed   CircuitState = iota // Normal operation
    StateOpen                         // Rejecting requests
    StateHalfOpen                     // Testing recovery
)
```

**Integration:** All external service clients (Proxmox, CloudStack, Wazuh) wrap their HTTP calls with circuit breakers.

**Strengths:**
- Prevents cascading failures to external services
- Configurable thresholds per service
- State change callbacks for monitoring
- Combined with exponential backoff retry
- Thread-safe implementation (sync.RWMutex)

### 2.4 Adapter Pattern - PRAGMATIC BUT DUPLICATIVE

**Location:** `cmd/labctl/main.go` (lines 39-78)

```go
type podRepoAdapter struct {
    repo *repositories.PodRepo
}
```

**Issue:** The adapter exists to bridge two nearly-identical interfaces. Creates maintenance overhead.

**Recommendation:** Consolidate interfaces or define shared interface in `models` package.

---

## 3. API Design Review

### 3.1 HTTP Server Architecture - WELL-STRUCTURED

**Location:** `internal/server/server.go`

**Middleware Stack** (lines 279-321):
```go
// Layered middleware (correct order)
s.router.Use(custommiddleware.SecureHeaders)      // Security first
s.router.Use(s.metrics.Middleware)                // Capture full duration
s.router.Use(middleware.RequestID)
s.router.Use(middleware.RealIP)
s.router.Use(middleware.Logger)
s.router.Use(middleware.Recoverer)                // Panic recovery
s.router.Use(middleware.Timeout(60 * time.Second))
s.router.Use(custommiddleware.MaxBodySize(1MB))
s.router.Use(cors.Handler(...))
s.router.Use(rateLimiter.PerIPMiddleware(...))    // Rate limiting last
```

**Strengths:**
- Correct middleware ordering (security -> metrics -> logging -> recovery)
- Conditional middleware (rate limiting only when Redis available)
- CORS properly configured with credential support
- Request timeouts prevent resource leaks

### 3.2 API Response Patterns - BASIC (Needs Improvement)

**Location:** `internal/server/server.go` (lines 475-487)

**Issues:**
1. No standardized response envelope - Inconsistent error formats
2. No error codes - Clients can't distinguish error types programmatically
3. No request ID in errors - Difficult to correlate logs with client errors
4. No pagination envelope - List endpoints lack pagination metadata

**Recommendation:** Implement standardized response envelopes:
```go
type APIResponse struct {
    Success   bool        `json:"success"`
    Data      interface{} `json:"data,omitempty"`
    Error     *APIError   `json:"error,omitempty"`
    RequestID string      `json:"request_id"`
}

type APIError struct {
    Code    string `json:"code"`     // e.g., "POD_NOT_FOUND"
    Message string `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}
```

---

## 4. Data Flow Analysis

### 4.1 Pod Lifecycle Flow - CLEAR

**Location:** `internal/orchestrator/orchestrator.go`

```
HTTP Request -> Handler -> Orchestrator -> Infrastructure Client -> External API
                                |
                         PodRepository (DB)
                                |
                         Models (Domain)
```

**CreatePod Flow:**
1. Generate unique Pod ID
2. Create pod record with `Provisioning` status
3. Database-first persistence - authoritative store when configured
4. Platform-specific provisioning (Proxmox/CloudStack)
5. Update pod with VM details and `Running` status
6. Rollback on error (sets status to `Error`)

### 4.2 Dual Storage Pattern - ARCHITECTURAL CONCERN

**Issue:** Orchestrator maintains **both** in-memory map AND database persistence:

```go
type Orchestrator struct {
    podRepo    PodRepository          // Database persistence
    pods       map[string]*models.Pod // In-memory cache
    podsMu     sync.RWMutex
}
```

**Problems:**
1. Confusion about single source of truth
2. Cache invalidation complexity in multi-instance deployments
3. Memory leaks - In-memory map never cleaned
4. Inconsistent patterns - GetPod reads from DB, never uses in-memory cache

**Recommendation:**
- **Option 1 (Preferred):** Remove in-memory map entirely. Use Redis for caching if needed.
- **Option 2:** Implement proper cache-aside pattern with TTL and invalidation
- **Option 3:** Make in-memory-only mode explicit (different orchestrator implementation)

### 4.3 Event Flow - EVENT-DRIVEN ARCHITECTURE

**Location:** `cmd/labctl/main.go` (lines 448-542)

```
External Event -> Webhook/NATS -> NATS Stream -> Consumers -> Actions
                                                     |
                                    [EventStoreConsumer] -> Database
                                    [CheckpointConsumer] -> Evaluator -> WebSocket
                                    [WebSocketConsumer] -> Broadcast
```

**Strengths:**
- NATS JetStream for durable event streaming
- Multiple consumers for different concerns
- Decoupled event producers from consumers
- Checkpoint evaluation triggered by events (reactive system)

---

## 5. Resilience Patterns Assessment - EXEMPLARY

### 5.1 Circuit Breaker Implementation - Grade: A+

**Features:**
- Three-state machine (Closed -> Open -> Half-Open)
- Configurable thresholds per service
- Automatic recovery attempts
- State change callbacks for monitoring
- Thread-safe with minimal lock contention

**Integration in all infrastructure clients:**
- Proxmox: `internal/proxmox/client.go` (lines 183-232)
- CloudStack: `internal/cloudstack/client.go` (lines 226-273)
- Wazuh: `internal/wazuh/client.go` (lines 186-252)

### 5.2 Retry Logic - COMPREHENSIVE

**Exponential Backoff** with configurable parameters:
```go
type RetryConfig struct {
    MaxAttempts     int           // Default: 3
    InitialDelay    time.Duration // Default: 100ms
    MaxDelay        time.Duration // Default: 5s
    Multiplier      float64       // Default: 2.0
    RetryableErrors func(error) bool
}
```

**Smart Retryable Detection:**
- Network timeouts
- DNS errors (temporary)
- Connection refused
- HTTP 429, 502, 503, 504
- Does NOT retry: Circuit breaker open, context cancelled

### 5.3 Rate Limiting - MULTI-TIER

**Three Layers:**
1. Global rate limiting - Per-IP, all endpoints (100/min default)
2. Pod creation limiting - Expensive operation (10/hour default)
3. Task queue rate limiting - Background job throttling

**Implementation:** Redis-based using sliding window algorithm (distributed rate limiting safe for multi-instance deployments)

---

## 6. Integration Points Assessment

### 6.1 Proxmox Integration - ROBUST

**Location:** `internal/proxmox/client.go`

**Strengths:**
- Token-based authentication (more secure than user/pass)
- Complete API coverage (VMs, snapshots, cloning)
- Resilient HTTP client with circuit breaker
- Configurable retry and timeout settings

### 6.2 CloudStack Integration - WELL-DESIGNED

**Location:** `internal/cloudstack/client.go`

**Strengths:**
- HMAC-SHA1 request signing (proper CloudStack auth)
- Async job polling with context cancellation
- Network isolation per pod
- Sentinel errors for specific conditions

### 6.3 Wazuh SIEM Integration - PRODUCTION-READY

**Location:** `internal/wazuh/client.go`

**Strengths:**
- JWT token management with automatic refresh
- Agent lifecycle management
- Group management for pod-based organization
- Circuit breaker protection

---

## 7. Potential Architectural Issues

### 7.1 CRITICAL: Dual Storage Pattern

**Severity: HIGH**
**Location:** `internal/orchestrator/orchestrator.go`

**Problem:** Orchestrator maintains both database and in-memory pod storage.

**Impact:**
- Multi-instance deployments will have divergent in-memory state
- Memory leaks (pods never removed from memory)
- Unclear source of truth

**Solution:** Remove in-memory storage entirely; use database + Redis cache.

### 7.2 HIGH: Missing Platform Abstraction

**Severity: MEDIUM-HIGH**

**Problem:** Orchestrator has direct coupling to both Proxmox and CloudStack clients.

**Recommendation:** Introduce platform abstraction:
```go
type PlatformProvider interface {
    DeployVM(ctx context.Context, params VMDeployParams) (*VM, error)
    DestroyVM(ctx context.Context, vmID string) error
    CreateSnapshot(ctx context.Context, vmID string, params SnapshotParams) error
    RevertSnapshot(ctx context.Context, vmID string, snapshotID string) error
}
```

### 7.3 MEDIUM: Adapter Pattern Duplication

**Severity: MEDIUM**
**Location:** `cmd/labctl/main.go` (lines 39-78)

**Solution:** Define shared interface in `models` or `common` package.

### 7.4 MEDIUM: Configuration Complexity

**Severity: MEDIUM**
**Location:** `internal/config/config.go`

**Issues:**
- 278-line `loadFromEnv()` function
- Manual environment variable parsing
- No validation until runtime

**Recommendation:** Use struct tags (`github.com/caarlos0/env`) and validation library.

### 7.5 LOW: Missing Request Context Propagation

**Severity: LOW**

**Recommendation:** Propagate user ID, request ID via context for audit logging.

---

## 8. Scalability Considerations

### 8.1 Horizontal Scaling - LIMITED

**Current State:**
- Stateless HTTP handlers - Can run multiple instances
- Redis for sessions - Shared session state
- NATS for events - Distributed messaging
- PostgreSQL for persistence - Centralized data

**Blockers:**
1. In-memory pod cache - Different instances have different views
2. Background cleanup - Multiple lifecycle managers compete (no leader election)
3. WebSocket hub - Local only, no inter-instance pub/sub bridging

**Recommendations:**
1. Remove in-memory storage - Use Redis or database only
2. Leader election - Use etcd, Consul, or Redis-based leader election
3. Distributed WebSocket - Use Redis pub/sub to bridge local hubs

### 8.2 Resource Management - NEEDS ATTENTION

**VM ID Generation** (orchestrator.go):
```go
func generateVMID() int {
    // Random VMID in range [100000, 999999]
    // 900,000 possible IDs
}
```

**Problem:** No collision detection. At scale, birthday paradox means collisions likely after ~1,000 VMs.

**Recommendation:** Check for existence before using VMID or use database sequence.

---

## 9. Security Considerations

### 9.1 Authentication - DEMO MODE RISK

**Location:** `internal/config/config.go`

**Risk:** Demo mode can be accidentally enabled in production via environment variable.

**Recommendation:** Remove demo mode for production builds or add runtime check.

### 9.2 Secrets Management - ADEQUATE

**Strengths:**
- Credentials from environment variables
- TLS for external API clients
- Token-based authentication
- API key signing (CloudStack HMAC)

### 9.3 Input Validation - INCONSISTENT

**Recommendation:** Use struct tags validation consistently across all input structs.

---

## 10. Code Quality Assessment

### 10.1 Error Handling - EXCELLENT

**Strengths:**
- Errors wrapped with context: `fmt.Errorf("context: %w", err)`
- Sentinel errors for specific conditions
- Errors don't expose internal details to clients
- Proper error propagation through layers

### 10.2 Concurrency Safety - GOOD

**Strengths:**
- Mutexes protect shared state
- Context cancellation used throughout
- Goroutine leaks prevented

### 10.3 Documentation - ADEQUATE

**Strengths:**
- Package-level comments present
- Key functions documented

**Gaps:**
- ADRs not found
- OpenAPI documentation not generated
- Deployment diagrams missing

---

## 11. Architecture Diagram

```
+------------------------------------------------------------------+
|                         HTTP Clients                              |
+--------------------------------+---------------------------------+
                                 |
+--------------------------------v---------------------------------+
|                      API Server (chi)                            |
|  +------------------------------------------------------------+  |
|  | Middleware: Auth, CORS, Rate Limit, Metrics, Recovery      |  |
|  +------------------------------------------------------------+  |
|  +------------------------------------------------------------+  |
|  | Handlers: Pods, Sessions, Labs, Checkpoints, Events        |  |
|  +------------------------------------------------------------+  |
+-----+----------------+----------------+----------------+---------+
      |                |                |                |
+-----v-----+  +-------v-------+  +-----v------+  +------v---------+
|Orchestrator|  |Checkpoint    |  |Assessment  |  |WebSocket Hub   |
|            |  |Evaluator     |  |Manager     |  |                |
+-----+------+  +-------+------+  +-----+------+  +----------------+
      |                 |               |
+-----v-----------------v---------------v--------------------------+
|              Repository Layer (Interfaces)                       |
|  PodRepo, SessionRepo, CheckpointRepo, LabTemplateRepo, ...     |
+-----+-------------------------------------------+----------------+
      |                                           |
+-----v--------------+              +-------------v----------------+
|   PostgreSQL       |              |     Redis (Cache)            |
|   (Primary)        |              |  Sessions, Pub/Sub, Locks    |
+--------------------+              +------------------------------+

+------------------------------------------------------------------+
|              Infrastructure Clients (Resilience)                  |
+----------------+----------------+----------------+----------------+
|   Proxmox      |  CloudStack    |    Wazuh       |  Canvas LTI   |
|  (Circuit)     |  (Circuit)     |  (Circuit)     |  (OAuth)      |
+-------+--------+-------+--------+-------+--------+-------+-------+
        |                |                |                |
+-------v------+ +-------v------+ +-------v------+ +-------v------+
| Proxmox VE   | | CloudStack   | | Wazuh SIEM   | | Canvas LMS   |
| Cluster      | | Cloud        | | Manager      | |              |
+--------------+ +--------------+ +--------------+ +--------------+

+------------------------------------------------------------------+
|                    Event Streaming (NATS)                         |
|  +----------------+  +----------------+  +---------------------+  |
|  |Event Store    |  |  Checkpoint    |  |  WebSocket Broadcast|  |
|  | Consumer      |  |  Consumer      |  |     Consumer        |  |
|  +----------------+  +----------------+  +---------------------+  |
+------------------------------------------------------------------+
```

---

## 12. Recommendations Summary

### Critical (Address Immediately)

| # | Issue | Location | Action |
|---|-------|----------|--------|
| 1 | Dual Storage Pattern | `internal/orchestrator/orchestrator.go` | Remove in-memory pod map; use database + Redis cache |
| 2 | No Leader Election | `internal/server/server.go` | Use Redis or etcd for leader election |

### High Priority

| # | Issue | Location | Action |
|---|-------|----------|--------|
| 3 | Missing Platform Abstraction | `internal/orchestrator/orchestrator.go` | Introduce `PlatformProvider` interface |
| 4 | VMID Collision Risk | `internal/orchestrator/orchestrator.go` | Implement collision detection |
| 5 | Basic API Responses | `internal/server/server.go` | Implement response envelope with error codes |

### Medium Priority

| # | Issue | Location | Action |
|---|-------|----------|--------|
| 6 | Configuration Validation | `internal/config/config.go` | Use struct tags and validation library |
| 7 | Pod Adapter Duplication | `cmd/labctl/main.go` | Consolidate repository interfaces |
| 8 | Local WebSocket | `internal/websocket/hub.go` | Use Redis pub/sub to bridge hubs |

### Low Priority

| # | Issue | Action |
|---|-------|--------|
| 9 | Context Value Propagation | Propagate user ID, request ID via context |
| 10 | Distributed Tracing | Integrate OpenTelemetry |

---

## 13. Next Steps

### Immediate Actions (Week 1)
1. Review and validate dual storage pattern behavior in production
2. Document scaling limitations (in-memory state, no leader election)
3. Implement configuration validation

### Short-Term (Month 1)
1. Remove in-memory pod storage (use database + Redis)
2. Implement leader election for background jobs
3. Add platform provider abstraction layer
4. Standardize API response envelopes

### Long-Term (Quarter 1)
1. Add distributed tracing (OpenTelemetry)
2. Implement comprehensive integration tests
3. Architecture decision records (ADRs)
4. OpenAPI documentation generation
5. Load testing and capacity planning

---

## 14. Final Verdict

### Strengths
1. Exceptional resilience patterns - Circuit breakers, retry logic, rate limiting
2. Clean package structure - Well-organized with clear separation of concerns
3. Strong infrastructure abstractions - Proxmox and CloudStack clients are production-ready
4. Event-driven design - NATS integration enables scalable, decoupled event processing
5. Dependency injection - Functional options pattern used consistently
6. Repository pattern - Well-defined interfaces separate from implementations

### Weaknesses
1. Dual storage confusion - In-memory cache + database creates ambiguity
2. Missing platform abstraction - Direct coupling limits extensibility
3. No leader election - Background jobs run on all instances
4. Basic API responses - No standardized error codes or pagination
5. Scalability gaps - In-memory state prevents true horizontal scaling

### Code Maturity Level: Production-Ready with Caveats

This platform is **suitable for production deployment** with the following conditions:
- Single-instance deployments only (until dual storage and leader election resolved)
- Small-to-medium scale (< 100 concurrent pods due to VMID collision risk)
- Monitoring configured (circuit breaker states, metrics endpoints)

For large-scale or multi-instance deployments, address critical recommendations first.
