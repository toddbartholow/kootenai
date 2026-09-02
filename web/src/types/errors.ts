import { AxiosError } from 'axios'

/**
 * Error state with rich information for better debugging and user feedback
 */
export interface ErrorState {
  /** Human-readable error message */
  message: string
  /** HTTP status code if applicable */
  code?: number
  /** Error type for programmatic handling */
  type: 'network' | 'validation' | 'server' | 'unknown'
  /** When the error occurred */
  timestamp: Date
  /** Whether the error can be retried */
  retryable: boolean
}

/**
 * Server error body shape. Our Go backend wraps all handler errors through
 * httputil.Responder which emits `{"error": "..."}` (see
 * api/internal/server/httputil/responder.go:ErrorResponse). A minority of
 * third-party components we proxy through may send `.message`, so we fall
 * back to it — but `.error` is the canonical key.
 */
interface ServerErrorBody {
  error?: string
  message?: string
}

// Duck-typed axios-like error. Test mocks and some edge cases (e.g. an
// interceptor that re-throws a plain object) may flow through here without
// being a real AxiosError instance. We recognize the shape rather than
// requiring the class identity so the helper is robust to both.
interface AxiosLikeError {
  response?: {
    status?: number
    data?: ServerErrorBody
  }
  message?: string
}

function isAxiosLike(e: unknown): e is AxiosLikeError {
  if (e === null || typeof e !== 'object') return false
  const o = e as Record<string, unknown>
  return 'response' in o || 'isAxiosError' in o
}

/**
 * Creates an ErrorState from an unknown error
 */
export function createErrorState(e: unknown, defaultMessage: string): ErrorState {
  if (e instanceof AxiosError || isAxiosLike(e)) {
    const axiosLike = e as AxiosLikeError
    const status = axiosLike.response?.status
    const body = axiosLike.response?.data
    const errorState: ErrorState = {
      // Prefer the backend-supplied reason (`.error`) over Axios's generic
      // "Request failed with status code 400" message.
      message: body?.error || body?.message || axiosLike.message || defaultMessage,
      type: status ? (status >= 500 ? 'server' : 'validation') : 'network',
      timestamp: new Date(),
      retryable: !status || status >= 500 || status === 429,
    }
    // Only set code if status exists (satisfies exactOptionalPropertyTypes)
    if (status !== undefined) {
      errorState.code = status
    }
    return errorState
  }

  if (e instanceof Error) {
    return {
      message: e.message || defaultMessage,
      type: 'unknown',
      timestamp: new Date(),
      retryable: true,
    }
  }

  return {
    message: defaultMessage,
    type: 'unknown',
    timestamp: new Date(),
    retryable: true,
  }
}
