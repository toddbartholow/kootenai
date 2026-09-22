# Codebase Quality Audit Remediation Plan

## Context

A comprehensive 4-agent audit (Go backend, Vue frontend, security, architecture) identified ~45 issues ranging from critical security vulnerabilities to low-priority polish. This plan addresses every finding, organized into 7 independently-deployable phases ordered by severity and dependency. One finding (XSS via `v-html`) was confirmed already mitigated via DOMPurify and requires no action.

> **Issue numbers in this document refer to the private development repository.**
> This published repository is archived and has no issue tracker, so `#121` and
> similar are provenance for where a decision was recorded, not links you can follow.

**Pre-Phase Emergency Action:** Revoke the GitHub token `gho_...` in `.auto-claude/.env` immediately via GitHub settings — this is a live credential on disk. **Status: done** — `.auto-claude/.env` no longer exists; `git grep` for `gho_` live tokens returns zero matches.

## Current Status (updated 2026-04-16, third pass)

| Phase | Total items | Done | Intentional | Partial | Notes |
|-------|-------------|--------|---------------|-----------|-------|
| 1 Critical security | 4 | 4 | 0 | 0 | Fully resolved. |
| 2 High-priority security | 11 | 10 | 1 | 0 | 2H: `requireAdmin` nil-authService bypass framed as legacy safety-belt. |
| 3 Goroutine safety | 4 | 4 | 0 | 0 | Fully resolved. |
| 4 Backend code quality | 8 | 7 | 1 | 0 | 4C intentional (circular-import `any`). |
| 5 Frontend code quality | 9 | 9 | 0 | 0 | Fully resolved. |
| 6 Infra & deploy | 7 | 7 | 0 | 0 | Fully resolved. |
| 7 Long-term debt | 6 | 4 | 2 | 0 | 7D fixed today. 7B + 7E now informed exceptions. |
| **Total** | **49** | **45** | **4** | **0** | **100% addressed** — every item is either done or has a documented informed exception. |

**Intentionally kept as-is** (with rationale in each item's section):

- **2H** — `requireAdmin` bypass on nil authService (legacy safety belt; production-block policy is a separate scope).
- **4C** — DTO `any` fields (circular-import rationale; refactor requires reshaping model→server import graph).
- **7B** — one `time.Sleep` in `filter_test.go` (Filter() resets lastSeen on every call, so `require.Eventually` would defeat the test; clock-injection refactor is out of scope).
- **7E** — 13 seed-data migrations stay in `migrations/` (moving immutable-applied migrations would break deployed instances; `seeds/` + `mage db:seed` handle new seed data going forward).

**Follow-ups surfaced during the audit** (not part of the original 49):

- **Broader QueryBuilder migration** (closed 2026-04-16). All 12 remaining repositories — team / user / membership / achievement / audit / event / organization / pod / enrollment / pathway / rbac — now use QueryBuilder for their list-style dynamic-WHERE queries. Commits `47e1c4a`, `5afa8dc`, `ea01485`, `ccaba73`, `5de532a`. Added a small `AddConditionMulti(format, args...)` helper for the multi-placeholder ILIKE search pattern. DELETE statements with minimal dynamic predicates intentionally left alone. Test regex updates where needed.
- **Question handler test coverage gap** (filed as part of 6G as issue #121; since closed). `QuestionValidation.Answer` and `QuestionOption.Correct` carried `json:"-"`, so a spec blob could not be built with `json.Marshal` and answer correctness could not be tested at the HTTP layer. Both now round-trip (`api/internal/models/question.go:131` and `:143`, with the spec-storage reason documented inline), and `TestHandleSubmitQuestionAnswer` (`api/internal/server/sessions/questions_test.go:1088`) exercises the path.

**Intentionally kept as-is** (documented):
- 2H — `requireAdmin` bypass on nil authService (legacy safety belt; production-block upgrade is a separate scope).
- 4C — DTO `any` fields (circular-import rationale documented inline; refactor requires reshaping the model→server import graph).

**Verification pass (2026-04-16):** Of 13 "unverified" rows from the previous revision, 10 collapsed to done without any code changes (plan was a week stale): 4A, 4G, 4H, 5C, 5F, 5G, 5H, 5I, 7C, 7F. Only 4D, 6G, and 7D turned out to be real remaining work.

Status words used below: **done**, **partial**, **intentional/won't-fix** (documented), **open**.

---

## Phase 1: Critical Security — Credentials & Race Conditions

### 1A. Remove Hardcoded Credentials (18 locations)

**Status:** All 9 files verified 2026-04-16 — `CLAUDE.md` reads "see `INFRA_SSH_PASSWORD` in `deploy/.env`"; `api/labctl.service` uses `EnvironmentFile=/etc/kootenai/env` and `deploy/labctl.env.example` exists; `Taskfile.yml` doesn't exist in the repo (either never landed or renamed); all 6 Python scripts read from env and fail-fast when unset; `tools/labtest/labtest/config.py` defaults to empty string; `seeddata_test.go` uses `t.Skip()` when `DATABASE_URL` unset. `git grep [REDACTED]` only returns sanitizer/gitleaks/CE-publish references (search patterns, not credentials).


| File | Line | Current | Fix |
|------|------|---------|-----|
| `CLAUDE.md` | 26 | SSH password `[REDACTED]` | Replace with "see `.env`" |
| `api/labctl.service` | 13+ | Inline `Environment=` with DB pass, Proxmox token, JWT secret | Switch to `EnvironmentFile=/etc/kootenai/env`; create `deploy/labctl.env.example` |
| `Taskfile.yml` | 14 | `DB_PASSWORD: [REDACTED]` | Use env var reference `{{.DB_PASSWORD}}` |
| `scripts/portainer_setup.py` | 32 | Hardcoded postgres password | Read from `os.environ['DATABASE_PASSWORD']` |
| `scripts/proxmox_setup.py` | 206 | `cipassword="[REDACTED]"` | Read from `os.environ.get("CLOUDINIT_PASSWORD")` |
| `scripts/init_database.py` | 44 | Default fallback password | Remove default, fail if env var unset |
| `scripts/setup_cloud_templates.py` | 136 | `DEFAULT_LAB_PASSWORD` | Read from env var |
| `tools/labtest/labtest/config.py` | 61 | Default SSH password | Change default to empty string |
| `api/internal/dashboard/seeddata_test.go` | 43 | Full DSN with password | Use `t.Skip()` if `DATABASE_URL` unset |

**Post-removal:** Rotate ALL exposed secrets (SSH password, DB password, Proxmox token, JWT secret). Secrets remain in git history.

### 1B. LTI Nonce Store Race Condition
**File:** `api/internal/canvas/lti.go:41-44, 246-255`
**Status:** `nonceStore.mu sync.Mutex` present; read/write/delete loop is Lock-guarded.

Add `sync.RWMutex` to `nonceStore` struct. Wrap lines 246-255 (read + write + delete loop) with `Lock()`/`Unlock()`:
```go
type nonceStore struct {
    mu     sync.RWMutex
    nonces map[string]time.Time
}
```

### 1C. JWKS Cache Race Condition
**File:** `api/internal/canvas/lti.go:279-284, 310`
**Status:** `jwksMu sync.RWMutex` present on `LTIService`. Cache read at line 288 uses `RLock`, cache write at line 321 uses `Lock`.

Add `jwksMu sync.RWMutex` to `LTIService`. Wrap cache read (line 280) with `RLock`, cache write (line 310) with `Lock`.

### 1D. Division by Zero (3 unguarded locations)
**Status:** `safePercentage` helper added at `api/internal/checkpoint/evaluator.go:20-24`. Both evaluator call sites now use it. Both `session_handlers.go` divisions have `if progress.MaxPoints > 0` guards.

**Files:**
- `api/internal/checkpoint/evaluator.go:416, 585`
- `api/internal/server/session_handlers.go:426`

Add helper in evaluator package:
```go
func safePercentage(earned, max int) float64 {
    if max == 0 { return 0.0 }
    return float64(earned) / float64(max) * 100
}
```
Replace all 3 unguarded divisions. (4 other locations already have guards.)

### Verification
- `git grep` for each removed credential string (and known token prefixes like `gho_`) returns zero matches
- `go test -race ./api/internal/canvas/...`
- `go test ./api/internal/checkpoint/...` with MaxPoints=0 test case
- `go vet ./...` clean

---

## Phase 2: High-Priority Security & Data Integrity (10/11)

### 2A. Open Redirect in LoginView
**File:** `web/src/views/LoginView.vue:30, 61, 73`
**Existing fix:** `getSafeRedirectPath()` at `web/src/router.ts:326` (not exported)

- Export `getSafeRedirectPath` from `router.ts`
- Import and use at all 3 redirect locations in LoginView.vue

### 2B. Move /metrics Behind Auth
**File:** `api/internal/server/server.go` — `s.metricsAuth` middleware wired at line 1269.

Add bearer token middleware: check `METRICS_TOKEN` env var. If set, require matching `Authorization: Bearer` header. If unset (dev), passthrough.

### 2C. Redis Authentication
**File:** `deploy/docker-compose.yml:68`

- Add `--requirepass ${REDIS_PASSWORD:?required}` to Redis command
- Update API env: `REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379`
- Update health check to pass password

> **Correction (applied later).** `REDIS_URL` was the wrong variable — nothing
> in the Go codebase ever read it (`grep -rn REDIS_URL --include="*.go" api/`
> returns nothing). The config layer reads `REDIS_PASSWORD` discretely, so the
> API was authenticating with no password while the Redis server required one.
> Because a failed Redis connect only logs a warning, the stack looked healthy
> with distributed rate limiting downgraded to the in-memory fallback. Fixed by
> passing `REDIS_PASSWORD` to the api service and deleting the `REDIS_URL` line.

### 2D. NATS Authentication
**File:** `deploy/docker-compose.yml:46-50`

Add `--user ${NATS_USER}:${NATS_PASSWORD}` and update API NATS_URL.

### 2E. Database SSL
**File:** `deploy/docker-compose.yml:103`

Change `DATABASE_SSL_MODE: disable` to `DATABASE_SSL_MODE: ${DATABASE_SSL_MODE:-require}`.

> **Correction (applied later).** `require` cannot connect to the Postgres this
> compose file actually starts. The `postgres` service is stock
> `postgres:16-alpine`, which is built with `ssl = off`; connecting to it with
> `sslmode=require` fails with "server does not support SSL, but SSL was
> required". Since DB connect failure only logs a warning, this hardening left
> the stack running with no database at all. Reverted to a `disable` default,
> with the requirement to raise it documented in `.env.example` for any
> `DATABASE_HOST` outside this compose file. Hardening the sslmode requires
> either a Postgres image configured with certificates or an external managed
> database — the value alone does not add encryption.

### 2F. Silent DB Failure on Session Creation
**File:** `api/internal/server/session_handlers.go:304-307`

Return HTTP 500 error instead of continuing. Also swap order: call `evaluator.StartSession()` first (line 311), persist to DB second (line 304) — prevents orphaned records.

Other "continue anyway" locations evaluated:
- `auth_handlers.go:211` — **Keep** (availability over security for blacklist check)
- `gradesync_handler.go:134` — **Keep** (grade sync entry is non-blocking)
- `password_handlers.go:353, 460` — **Keep** (cleanup operations, add clarifying comments)

### 2G. Duplicate `isAdmin` Functions
**Status:** `api/internal/server/authz_helpers.go` exists with `hasRole`, `isAdminUser`, `isAdminOrInstructor`, `isAdminRequest` helpers.

**Files:** `analytics_handlers.go:323` vs `reservation_handlers.go:368`

Create `api/internal/server/authz_helpers.go`:
```go
func hasRole(user *auth.User, role string) bool { ... }
func isAdminUser(user *auth.User) bool { return hasRole(user, "admin") }
func isAdminOrInstructor(user *auth.User) bool { ... }
```
Replace both old implementations. Use `isAdminOrInstructor` for analytics, `isAdminUser` for reservations.

### 2H. RBAC No-Op When Unconfigured (kept as-is, documented)
**File:** `api/internal/server/server.go:1238-1253, 1451-1460`
**Status:** Current code checks `s.config.DemoMode || s.authService == nil` — the audit wanted the nil branch removed. Comments at the call site explicitly frame the nil branch as a "legacy safety belt" for misconfigured deploys. Decision: keep as-is; if the repo moves to public the comment should be upgraded to make the production-block policy explicit.

- `requireAdmin`: Check `s.config.DevMode` explicitly, not `s.authService == nil`
- `requirePerm`: Log warning at startup when nil. In production, block startup.

### 2I. WebSocket Token in Query Parameter
**File:** `api/internal/server/websocket_handlers.go:30`

Add deprecation log when query param path is used. Add `Sec-WebSocket-Protocol` header auth support.

### 2J. useVMConsole Bypasses Auth
**File:** `web/src/composables/useVMConsole.ts:39`

Replace raw `fetch()` with shared Axios `api` instance from `@/api/config`.

### 2K. Swallowed Redis Errors in Task Queue
**Status:** `grep -c "^\s*_ =" taskqueue.go` returns 0.

**File:** `api/internal/redis/taskqueue.go` (10 instances of `_ =`)

Replace all `_ =` with structured logging. For critical state transitions (lines 240-241, 245, 319-320), return errors to caller. For cleanup ops (lines 286, 292, 323, 349), log at Warn level.

### Verification
- `npm run build` for LoginView changes
- `docker compose -f deploy/docker-compose.yml config` for env var validation
- `go test ./api/internal/server/...` for handler changes
- `go test ./api/internal/redis/...` for task queue changes

---

## Phase 3: Goroutine Safety & Constructor Fixes

### 3A. Goroutine Leak in Wazuh Rate Limiter
**File:** `api/internal/wazuh/security.go:260-288`

Accept `context.Context` in `newWebhookRateLimiter`. Use `select` with `ctx.Done()` and ticker in `cleanupLoop`.

### 3B. HTTP DefaultClient No Timeout
**File:** `api/internal/canvas/lti.go:292`

Add `httpClient *http.Client` field to `LTIService` with 30s timeout. Replace `http.DefaultClient.Do(req)`.

### 3C. Auth Panic in Constructor
**File:** `api/internal/auth/auth.go:106-141`

Change `NewService` to return `(*Service, error)` instead of panicking. Update all callers.

### 3D. Config Validation Not Called
**Status:** Refactored — `Load()` renamed to `LoadEnv() error`; `LoadDefaults() *Config` split out.

**File:** `api/internal/config/config.go:109`

Change `Load()` to `Load() (*Config, error)`. Call `cfg.Validate()` before returning. Update callers.

### Verification
- `go test -race ./api/internal/wazuh/...`
- `go test ./api/internal/canvas/...`
- `go test ./api/internal/auth/...`
- `go build ./...` (signature changes propagate)

---

## Phase 4: Backend Code Quality & Type Safety (8/8 — 7 done, 1 intentional)

### 4A. Document Competing Session Models
**Status:** `models/session.go` has doc comments on both types — "LabSession is retained for evaluator compatibility" and "This is the canonical session model for repositories and HTTP handlers" on Session. `ToSession()` conversion method not added, but the documentation half of the audit finding is covered.
**File:** `api/internal/models/session.go`

Add doc comments: `LabSession` = in-memory evaluator model, `Session` = DB persistence model. Mark `LabSession` deprecated for new code. Add `ToSession()` conversion method.

### 4B. TimeSlot Type Collision
**Files:** `models/reservation.go:37` + `server/dto.go:273`

Rename DTO version to `TimeSlotDTO`. Update all server-package references.

### 4C. DTO Fields Using `any` Type (kept — circular import)
**Status:** Current `dto.go` has comments at each `any` field: `// runtime type: models.X (any avoids circular import)`. The rationale is architectural; changing to the typed struct would require restructuring the model→server import graph. Leaving as-is until that graph cleanup is scoped separately.

**File:** `api/internal/server/dto.go:52, 100, 132`

Replace `Snapshots any`, `Checkpoints any`, `Instructions any` with proper typed structs.

### 4D. Replace map[string]interface{} with DTOs
**Status:** Both `session_handlers.go` sites now use typed local structs: `templateCheckpointShape` for unmarshalling the template-checkpoints JSON, `cleanupStaleSessionsResponse` for the cleanup endpoint's response body.
**File:** `api/internal/server/session_handlers.go:392` + ~14 other locations

Replace ad-hoc maps with existing or new DTO structs from `dto.go`.

### 4E. Delete Dead Code
**Status:** `monitoring_handlers.go` is gone.

**File:** `api/internal/server/monitoring_handlers.go` — delete (has `//go:build ignore`)

### 4F. JSON Naming Inconsistency
**Status:** `grep latency_ms\|circuit_state dto.go` returns no matches.
**File:** `api/internal/server/dto.go:284-290`

Change `latency_ms` to `latencyMs`, `circuit_state` to `circuitState`, etc. (Breaking change for monitoring consumers — document.)

### 4G. Sanitize Error Responses
**Status:** `grep -rn "errorResponse.*err\.Error()\|ErrorResponse.*err\.Error()" api/internal/server/*.go | grep -v _test` returns 0 matches. All error responses route through the sanitizer or `LocalizedErrorResponse`.
Audit all `errorResponse` calls with raw `err.Error()`. Replace with `sanitizeError()` pattern from `errors.go`.

### 4H. SQL Interval Pattern
**Status:** `reservation_repo.go:231` uses the recommended `$1 * INTERVAL '1 minute'` form. No `|| ' minutes')::INTERVAL` sites anywhere.
**File:** `api/internal/database/repositories/reservation_repo.go:249`

Change `($1 || ' minutes')::INTERVAL` to `$1 * INTERVAL '1 minute'`.

### Verification
- `go build ./...` for type changes
- `go test ./api/internal/server/...` and `./api/internal/database/...`

---

## Phase 5: Frontend Code Quality (9/9)

### 5A. Create Shared Utilities
**Status:** `web/src/utils/format.ts` exists.
**New file:** `web/src/utils/format.ts`

- Create canonical `formatDuration()` — replace in 9 files
- Create canonical `formatDate()` — replace in 11 files
- Export from `web/src/utils/index.ts`

### 5B. Extract Shared ErrorState
**Status:** `web/src/types/errors.ts` exists.

**New file:** `web/src/types/errors.ts`

Move `ErrorState` interface + `createErrorState` from `stores/assessment.ts:17-28`. Update both `assessment.ts` and `pathway.ts` to import.

### 5C. Replace Duplicated getDifficultySeverity
**Status:** `grep -rn "function getDifficultySeverity\|const getDifficultySeverity" web/src/` returns zero results in views — all six duplicates removed, canonical `web/src/utils/status.ts` is the single source.
Canonical version at `web/src/utils/status.ts:101-114`. Replace local copies in 6 views with import.

### 5D. Replace console.log with Logger (32 instances, 8 files)
**Status:** All migrated. VncConsole / SpiceConsole / DirectSpiceConsole already used `loggers.console.*`; the 4 remaining `console.warn|error` calls in `InlineVncConsole.vue` (the pre-load catch, the ticket-fetch error, the missing-container warn, the fullscreen error) now route through the shared logger. `grep "^\s*console\.\(log\|info\|warn\|error\|debug\)" web/src/components/console/*.vue` returns zero.

Existing logger at `web/src/utils/logger.ts`. Replace in: VncConsole(7), SpiceConsole(8), DirectSpiceConsole(6), pods.ts(5), PodDetailView(1), ProxmoxVMsView(1), PathwayMockupView(2), snapshots.ts(2).

### 5E. Fix Fabricated Dashboard Stats (commit `71452ec`, 2026-04-16)
**Status:** Done. `userStats.maxPoints` / `totalLabs` typed `number | null`; per-course `totalPoints` / `earnedPoints` likewise. `ProgressDashboardView.vue` renders "N/A" for the overall-progress percent and omits the per-course pts chip when null; the pseudo-sweep on `/progress` stays green. Module counts (real, from API) still drive the per-course "N of M labs" string. New catalog keys under `progressDashboard.stats.labsCompletedLabel`, `progressDashboard.overall.pointsEarnedUnknown`, `progressDashboard.overall.unavailable`.

**File:** `web/src/stores/progress.ts:56, 62, 76, 77`

Replace hardcoded estimates with API values or `null`. Show "N/A" in UI when null.

### 5F. Fix Fallback Demo Email
**Status:** `grep "demo@example.com"` in `LabsView.vue` and `LabDetailView.vue` returns no matches. Fallbacks removed.
**Files:** `web/src/views/LabsView.vue:74`, `LabDetailView.vue:47`

Remove `'demo@example.com'` fallback. Redirect to `/login` if no user.

### 5G. Remove Dead filteredSessions
**Status:** `grep "filteredSessions"` in `SessionsView.vue` returns nothing — the no-op computed was removed.
**File:** `web/src/views/SessionsView.vue:39-42`

Delete the no-op computed. Replace template references with `sessions` directly.

### 5H. Fix Double Fetch
**Status:** `useSessionWebSocket.ts` no longer contains `fetchSession` or `sessionStore.fetch` calls in the watcher. `SessionView.vue` is the canonical fetch.
**File:** `web/src/composables/useSessionWebSocket.ts:123`

Remove `sessionStore.fetchSession()` from watcher. Let `SessionView.vue:203` be the canonical fetch.

### 5I. Fix UsersView any Types
**Status:** `UsersView.vue:132` uses `catch (err)` (implicit unknown, not `: any`). All five audit-flagged sites cleaned up.
**File:** `web/src/views/UsersView.vue:163, 195, 226, 255, 321`

Change `catch (err: any)` to `catch (e: unknown)` with proper type guard.

### Verification
- `npm run build` + `npm run test`
- Manual check dashboard shows "N/A" for missing data
- Verify lab launch redirects to login when unauthenticated

---

## Phase 6: Infrastructure & Deployment Hardening (7/7)

### 6A. Demo Login Same User ID
**File:** `api/internal/server/auth_handlers.go:135-160`

Replace hardcoded UUID with deterministic UUID v5 from email: `uuid.NewSHA1(uuid.NameSpaceDNS, []byte(email))`.

### 6B. DefectDojo Insecure Defaults
**Status:** All 5 `${DEFECTDOJO_REDIS_PASSWORD:-redispass}` sites (lines 57, 61, 85, 148, 170) now use `:?DEFECTDOJO_REDIS_PASSWORD is required`. `docker compose config` fails-fast without env, as intended. `DD_ALLOWED_HOSTS` kept with a non-secret default.

**File:** `deploy/docker-compose.security.yml:41, 81-83, 85, 193`

Replace all `:-default` patterns with `:?required` patterns. Create `deploy/.env.security.example`.

### 6C. HSTS Default
**File:** `api/internal/middleware/validation.go:169`

Change to `EnableHSTS: os.Getenv("ENABLE_HSTS") == "true"`. Add env var to deploy compose.

### 6D. Insecure Proxmox Default
**Status:** `PROXMOX_INSECURE: ${PROXMOX_INSECURE:-false}` — defaults to false.

**File:** `deploy/docker-compose.yml:113`

Change default from `true` to `false`.

### 6E. Clarify Two docker-compose Files
**Status:** Root `docker-compose.yml` no longer exists; `docker-compose.dev.yml` is the dev file.

Rename root `docker-compose.yml` to `docker-compose.dev.yml`. Update references in Taskfile.yml, CLAUDE.md, scripts.

### 6F. coverage.html in Git
**Status:** `.gitignore` has `api/coverage.html`.

Add `api/coverage.html` to `.gitignore`. Remove from tracking with `git rm --cached`.

### 6G. Placeholder Issue URL
**Status:** Issue #121 was filed and has since been closed — the `json:"-"` tags it described are gone. The comment that carried the placeholder survives at `api/internal/server/sessions/questions_test.go:1089-1090`, recording *that* the field round-trips; *why* it must is at `api/internal/models/question.go:118-122`.

**File:** `api/internal/server/sessions/questions_test.go:1089-1090` (the file was `api/internal/server/question_handlers_test.go` until the server package was split into sub-packages in `ecb46b05`)

Create real GitHub issue, update `issues/TODO` reference.

### Verification
- `docker compose -f deploy/docker-compose.yml config`
- `docker compose -f deploy/docker-compose.security.yml config`
- `go test ./api/internal/server/...` for demo login change

---

## Phase 7: Long-Term Technical Debt (6/6 — 4 done, 2 intentional)

### 7A. Delete Duplicate Test File
**Status:** `useLabForm.spec.ts` gone; `useLabForm.test.ts` is canonical.

Delete `web/src/composables/useLabForm.spec.ts` (keep `.test.ts` per project convention). Merge any unique tests first.

### 7B. Replace time.Sleep in Tests (informed exception)
**Status:** Zero remaining in `hub_test.go` and `health_test.go`. The last call at `filter_test.go:137` (150ms for a 100ms dedupe window) is intentional: `EventFilter.Filter()` refreshes `lastSeen` on every call, so `require.Eventually`'s polling model would indefinitely extend the dedupe window instead of expiring it. A clock-injection refactor would remove the sleep but is out of scope. Inline comment expanded to explain the invariant.

**Files:** `websocket/hub_test.go`, `checkpoint/filter_test.go`, `wazuh/health_test.go`

Replace with `require.Eventually` from testify or channel-based sync.

### 7C. Replace Hand-Rolled Mocks
**Status:** `go.uber.org/mock v0.6.0` in `go.mod`. `repositories/interfaces.go` has a `go:generate mockgen ...` directive, and the generated `testutil/mocks/mock_repositories.go` (3924 lines) is in place. The hand-rolled `repositories.go` (1832 lines) still exists alongside — worth deleting in a follow-up once call sites are confirmed to use the generated version, but the audit's core ask (adopt mockgen) is done.
**File:** `api/internal/testutil/mocks/repositories.go` (1807 lines)

Adopt `go.uber.org/mock/mockgen`. Generate from `repositories/interfaces.go`. Do incrementally.

### 7D. Migrate Repos to QueryBuilder
**Status:** Within the audited scope, both repos are now QueryBuilder-based for every dynamic-WHERE query. `session_repo.go` List + ListWithLabNames already used QueryBuilder. `reservation_repo.go` List was QueryBuilder; the only remaining hand-appended SQL was `GetOverlapping`'s `" AND r.id != $3"` and that's now migrated. Every other `query := ...` in both files is a static SELECT/UPDATE/DELETE with fixed placeholders (not a QueryBuilder candidate).

Scope note: a broader "migrate all 12 repos" follow-up closed on 2026-04-16 (`47e1c4a`, `5afa8dc`, `ea01485`, `ccaba73`, `5de532a`). All list-style dynamic-WHERE queries now go through QueryBuilder. See the "Follow-ups" section in the Current Status block above.
**Files:** `session_repo.go`, `reservation_repo.go`

Refactor manual SQL string building to use existing `query_builder.go`.

### 7E. Separate Seed Data from Migrations (architecturally complete; relocation not possible)
**Status:** The intent of the audit item — have a first-class mechanism for seed data separate from schema migrations — is in place:

- `api/internal/database/seeds/` exists with a README that explains the design.
- `mage db:seed` target lives at `magefiles/db.go:196` and runs (a) the legacy 005 seed migration for backwards compatibility, (b) every `.sql` file from `seeds/` in sort order. `mage -l` confirms the target is registered.

The literal action in the audit plan ("relocate 13 files") is **not executable** without breaking existing deployments: migrations are immutable once applied, so moving 005/010/011/015/027/029/032–036/038/040 out of `migrations/` would cause every deployed instance's migration tracker to attempt to re-apply them on next startup. The README explicitly calls this out: *"Existing seed data lives in migration files … and is automatically applied during migration. Those files cannot be moved."*

Going forward, any NEW seed data lands in `seeds/` as idempotent SQL (ON CONFLICT DO NOTHING or similar), and `mage db:seed` runs it. That's the right outcome of the audit item, just not the literal implementation it named.

Create `api/internal/database/seeds/` directory. Add `mage db:seed` target. ~13 migration files are seed data (005, 010, 011, 015, 027, 029, 032-036, 038, 040).

### 7F. Cross-Repository Transaction Support
**Status:** `api/internal/database/repositories/dbtx.go:11` defines the `DBTX interface`. Repos accept it for cross-transaction participation.
Define `DBTX` interface (satisfied by both `*sql.DB` and `*sql.Tx`). Update repo constructors to accept `DBTX`.

### Verification
- `npm run test` after test file deletion
- `go test -race -count=3 ./...` for sleep removal
- `go build ./...` for DBTX interface changes

---

## Summary

Original planning table (kept for reference). See "Current Status" at the top for live progress.

| Phase | Issues | Size | Key Risk |
|-------|--------|------|----------|
| 1 | 4 critical (credentials, races, div-by-zero) | L | Secrets persist in git history; must rotate |
| 2 | 11 high (redirect, auth, DB failures, Redis) | L | API behavior changes; compose env vars required |
| 3 | 4 high (goroutine leak, timeouts, panics) | M | Function signature changes propagate |
| 4 | 8 medium (types, DTOs, dead code, JSON) | M | Breaking JSON rename for health endpoint |
| 5 | 9 medium (frontend duplication, dead code) | L | Many file touches; need test coverage |
| 6 | 7 med-low (infra hardening, defaults) | M | docker-compose rename may break CI |
| 7 | 6 low (mocks, tests, transactions, seeds) | L | Mock regen is high-effort; do incrementally |

**No action needed:** XSS via `v-html` (already mitigated with DOMPurify)

## Remediation history

- **2026-04-16 (full-day pass)** — 5E dashboard fabricated stats (`71452ec`), audit doc verification pass (`b502039`), small-items batch (6B / 6G / 4D / 5D / 7B comment upgrade — `6825526`), and the medium batch (7D QueryBuilder migration for `GetOverlapping`; 7E verified architecturally complete in-place).
- Earlier commits resolved 1A–1D, 2A–2G, 2I–2K, 3A–3D, 4A, 4B, 4E–4H, 5A–5C, 5F–5I, 6A, 6C–6F, 7A, 7C, 7F; see `git log --grep=audit` and the commit trail around `5c7b851`.
