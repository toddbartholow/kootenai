/**
 * CSRF Utilities for HttpOnly Cookie Authentication (ADR-0002)
 *
 * The auth token lives in an HttpOnly cookie — invisible to JS. The browser
 * attaches it automatically via `withCredentials: true` on the axios instance.
 *
 * CSRF protection uses the double-submit cookie pattern: the backend sets a
 * non-HttpOnly `csrf_token` cookie on login/refresh, and this module reads it
 * so the axios request interceptor can echo it as `X-CSRF-Token` on mutating
 * requests.
 */

/** Name of the JS-readable CSRF cookie set by the backend. */
export const CSRF_COOKIE_NAME = 'csrf_token'

/** Header name the backend's CSRF middleware expects. */
export const CSRF_HEADER_NAME = 'X-CSRF-Token'

/** HTTP methods that require CSRF protection. */
const CSRF_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

/** Whether a request method requires a CSRF token. */
export function requiresCSRF(method: string): boolean {
  return CSRF_METHODS.has(method.toUpperCase())
}

/** Escape regex metacharacters so a string can be safely interpolated into a RegExp. */
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Read a cookie value by name. Returns null if the cookie doesn't exist.
 * Only used for the non-HttpOnly CSRF cookie — the auth cookie is invisible to JS.
 */
export function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(?:^|; )${escapeRegExp(name)}=([^;]*)`))
  if (match?.[1] == null) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return null
  }
}
