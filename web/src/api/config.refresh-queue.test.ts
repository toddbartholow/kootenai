/**
 * Tests the 401-refresh queue in api/config.ts — the most load-bearing
 * piece of TypeScript in this repo. Verifies:
 *
 *   1) Concurrent 401s coalesce into a single /auth/refresh call.
 *   2) Both queued requests retry after the refresh.
 *   3) A failed refresh rejects every queued caller (so they each see
 *      the error rather than waiting forever).
 *   4) The refresh path is not triggered for /auth/refresh or /auth/login
 *      themselves (avoids infinite loops).
 *
 * In cookie mode the browser attaches the auth cookie automatically, so
 * there is no Bearer token to thread through — the queue just retries the
 * original request and the rotated cookie is sent automatically.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { api } from './config'

// Shared mock auth store — the interceptor lazy-imports this.
const mockAuthStoreState = {
  refreshToken: vi.fn(async () => {
    /* overridden per test */
  }),
  logout: vi.fn(async () => {}),
}

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => mockAuthStoreState,
}))

// Per-test URL dispatcher. Each key = request URL; each value = a function
// that receives the adapter config and returns either a resolved response
// (object) or throws an error (to simulate an axios error).
type AdapterHandler = (config: InternalAxiosRequestConfig) => Promise<AxiosResponse>
let urlHandlers: Map<string, AdapterHandler>
let callLog: string[]

function installTestAdapter(): () => void {
  const original = api.defaults.adapter
  const adapter: AxiosAdapter = async config => {
    const url = config.url ?? ''
    callLog.push(`${config.method?.toUpperCase() ?? 'GET'} ${url}`)
    const handler = urlHandlers.get(url)
    if (!handler) {
      throw Object.assign(new Error(`unexpected request: ${url}`), {
        config,
        response: { status: 500, data: {}, headers: {}, config, statusText: 'Error' },
      })
    }
    return handler(config as InternalAxiosRequestConfig)
  }
  api.defaults.adapter = adapter
  return () => {
    if (original) {
      api.defaults.adapter = original
    } else {
      delete api.defaults.adapter
    }
  }
}

// Helper: build an AxiosResponse shell.
function ok<T>(data: T, config: InternalAxiosRequestConfig): AxiosResponse<T> {
  return {
    data,
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  }
}

// Helper: build an axios-style error object with a .response we can mock.
function axiosLikeError(status: number, body: unknown, config: InternalAxiosRequestConfig) {
  const err = new Error(`Request failed with status code ${status}`) as Error & {
    isAxiosError: boolean
    response: AxiosResponse
    config: InternalAxiosRequestConfig
  }
  err.isAxiosError = true
  err.config = config
  err.response = {
    data: body,
    status,
    statusText: 'Unauthorized',
    headers: {},
    config,
  }
  return err
}

describe('API 401 refresh queue (api/config.ts)', () => {
  let restoreAdapter: () => void

  beforeEach(() => {
    urlHandlers = new Map()
    callLog = []
    mockAuthStoreState.refreshToken.mockReset()
    mockAuthStoreState.logout.mockReset()
    restoreAdapter = installTestAdapter()
  })

  afterEach(() => {
    restoreAdapter()
  })

  it('coalesces two concurrent 401s into ONE refresh call', async () => {
    let firstCount = 0
    let secondCount = 0

    urlHandlers.set('/first', async config => {
      firstCount++
      if (firstCount === 1) throw axiosLikeError(401, { error: 'stale' }, config)
      return ok({ who: 'first' }, config)
    })
    urlHandlers.set('/second', async config => {
      secondCount++
      if (secondCount === 1) throw axiosLikeError(401, { error: 'stale' }, config)
      return ok({ who: 'second' }, config)
    })

    mockAuthStoreState.refreshToken.mockImplementation(async () => {})

    const [first, second] = await Promise.all([
      api.get('/first').then(r => r.data),
      api.get('/second').then(r => r.data),
    ])

    expect(first).toEqual({ who: 'first' })
    expect(second).toEqual({ who: 'second' })
    expect(mockAuthStoreState.refreshToken).toHaveBeenCalledTimes(1)
    expect(firstCount).toBe(2)
    expect(secondCount).toBe(2)
  })

  it('rejects every queued caller when the refresh itself fails', async () => {
    urlHandlers.set('/a', async c => {
      throw axiosLikeError(401, { error: 'stale' }, c)
    })
    urlHandlers.set('/b', async c => {
      throw axiosLikeError(401, { error: 'stale' }, c)
    })
    urlHandlers.set('/c', async c => {
      throw axiosLikeError(401, { error: 'stale' }, c)
    })

    const refreshErr = new Error('refresh blew up')
    mockAuthStoreState.refreshToken.mockRejectedValue(refreshErr)

    const results = await Promise.allSettled([api.get('/a'), api.get('/b'), api.get('/c')])

    expect(results.every(r => r.status === 'rejected')).toBe(true)
    expect(mockAuthStoreState.refreshToken).toHaveBeenCalledTimes(1)
  })

  it('does NOT attempt refresh for /auth/refresh itself (no infinite loop)', async () => {
    urlHandlers.set('/auth/refresh', async c => {
      throw axiosLikeError(401, { error: 'invalid' }, c)
    })

    await expect(api.post('/auth/refresh', {})).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(mockAuthStoreState.refreshToken).not.toHaveBeenCalled()
  })

  it('does NOT attempt refresh for /auth/login itself', async () => {
    urlHandlers.set('/auth/login', async c => {
      throw axiosLikeError(401, { error: 'bad creds' }, c)
    })

    await expect(api.post('/auth/login', {})).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(mockAuthStoreState.refreshToken).not.toHaveBeenCalled()
  })
})
