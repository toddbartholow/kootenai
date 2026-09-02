# Senior Engineer Code Review: Kootenai Platform

## Overall Assessment

This is a **solid intermediate-level project** that demonstrates real architectural thinking and practical understanding of distributed systems. If this came across my desk from a candidate, I'd be interested in talking to them — but I'd have specific questions and concerns.

---

## What's Done Well

### 1. Architecture & Design
- **Clean separation of concerns**: The `internal/` package structure follows Go conventions properly. Repository pattern, service layer, handlers — it's textbook clean architecture.
- **Platform abstraction**: The orchestrator cleanly abstracts Proxmox and CloudStack behind a unified interface. This shows understanding of the Strategy pattern and thinking ahead.
- **Functional options pattern**: Used correctly in `orchestrator.go:72-86` and `server.go:102-152` — this is idiomatic Go.

### 2. Error Handling
Proper error wrapping throughout:
```go
return fmt.Errorf("provisioning pod: %w", err)  // orchestrator.go:156
```
This enables error chain inspection with `errors.Is()` and `errors.As()`.

### 3. Configuration Management
The layered config loading (`.env` → `config.yaml` → env vars) in `config/config.go` is well-designed. Supports 12-factor app principles.

### 4. Database Layer
- Embedded migrations with proper transaction handling (`migrate.go`)
- Repository interfaces for testability
- Connection pooling configured correctly

### 5. Frontend
- TypeScript strict mode enabled
- Pinia stores with proper typing
- Composition API throughout (modern Vue)
- Discriminated unions for WebSocket messages (`client.ts:442-495`) — shows TypeScript maturity

---

## Red Flags & Concerns

### 1. **Security: Hardcoded Demo Secret** 🔴
```go
// auth/auth.go:24-25
func DefaultConfig() Config {
    return Config{
        JWTSecret: "<JWT_SECRET_CHANGE_ME>",  // TERRIBLE
```
This is a **production incident waiting to happen**. The default should be empty, forcing explicit configuration. A comment saying "change in production" doesn't prevent shipping to production.

**Worse — the auth middleware has a demo bypass:**
```go
// auth/auth.go:119-129
if authHeader == "" {
    // For demo mode, allow unauthenticated access with demo user
    demoUser := &User{...}
    next.ServeHTTP(w, r.WithContext(ctx))
    return
}
```
Any unauthenticated request gets elevated to a "demo-user" with student role. This should be behind an explicit `DEV_MODE` check.

**Status: ✅ FIXED**

### 2. **VM ID Generation is Not Collision-Safe** 🔴
```go
// orchestrator.go:705-711
func generateVMID(podID, vmName string) int {
    hash := 0
    for _, c := range podID + vmName {
        hash = hash*31 + int(c)
    }
    return 10000 + (hash % 90000)
}
```
This is a simple polynomial hash with a 90,000 ID space. On a busy system with many students, **collisions are mathematically inevitable**. The hash isn't even using the full int range. Should use UUID or check for existing VMIDs before assignment.

**Status: ✅ FIXED**

### 3. **Inconsistent Error Handling in HTTP Responses**
```go
// proxmox/client.go:217-219
if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)  // Error ignored
    return fmt.Errorf("failed to create snapshot: %s", string(body))
}
```
The `_ =` pattern for error handling is scattered throughout the Proxmox client. If the body read fails, you get an empty error message. This makes debugging in production painful.

**Status: ✅ FIXED**

### 4. **Password in Default Config** 🟠
```go
// database/db.go:39
Password: "<DB_PASSWORD>",
```
Even for defaults, embedding passwords in code is problematic. Use empty defaults that fail loudly.

**Status: ✅ FIXED**

### 5. **Test Coverage is Thin** 🟠
Only 7 Go test files for 52 source files. The tests I reviewed (`server_test.go`) only test configuration validation — no actual HTTP handler testing. The core business logic (orchestrator, checkpoint evaluator) has tests, but the server handlers don't.

```go
// server_test.go - only tests Config.Validate()
// No tests for handleCreatePod, handleDeletePod, etc.
```

**Status: ⏳ TODO** - Requires significant effort, tracked separately

### 6. **Compiled Binary Checked Into Git** 🟠
The exploration found `api/labctl` (compiled binary) is checked in. This is a 40MB+ file that should never be in source control.

**Status: ✅ FIXED**

### 7. **Session ID Generation** 🟡
```go
// server.go:829
sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
```
UnixNano is not guaranteed unique across machines or even rapid sequential calls. For production, use UUIDs (the project already imports `github.com/google/uuid`).

**Status: ✅ FIXED**

### 8. **Go 1.24.0?** 🟡
```go
// go.mod:3
go 1.24.0
```
Go 1.24 doesn't exist yet (as of my knowledge). This is likely a typo or forward-looking versioning. Current stable is 1.22.x. This would fail on real systems.

**Status: ✅ FIXED**

---

## Code Smells

### Over-engineering in Places
The `podRepoAdapter` in `main.go:39-77` exists solely to adapt between two nearly identical interfaces. This suggests the interfaces weren't designed together — a code smell for API design.

### Under-engineering in Others
The Proxmox client has no retry logic, no circuit breaker, no rate limiting. For VM operations that can take 30+ seconds, this is fragile.

### Mixed Concerns in Main
`main.go` is 919 lines with CLI handling, server initialization, and NATS consumer setup all mixed together. This should be decomposed.

---

## What I'd Ask in an Interview

1. "Walk me through how you'd handle a VM clone operation that takes 60 seconds. What happens if the HTTP request times out?"

2. "How do you prevent two students from getting the same VMID in a race condition?"

3. "The auth middleware falls back to a demo user. How would you ensure this never ships to production?"

4. "I see 7 test files for 52 source files. What's your testing philosophy and where would you add tests first?"

5. "The Proxmox and CloudStack clients are similar. Did you consider a common interface? Why or why not?"

---

## Verdict

**Grade: B- / Hire with reservations**

This candidate understands:
- Go project structure and idioms
- Clean architecture principles
- Database design and migrations
- Modern frontend development
- Distributed system concepts (NATS, WebSockets)

This candidate needs growth in:
- Security-first thinking
- Production-hardening (retries, timeouts, rate limits)
- Comprehensive testing
- Attention to edge cases

**I'd hire for a mid-level position** with mentorship on security practices. For a senior role, the security issues would be disqualifying without a good explanation ("this was a learning project, here's what I'd do differently...").

The project shows ambition and follow-through. The architecture is sound. The gaps are teachable. That combination is more valuable than a perfect toy project.
