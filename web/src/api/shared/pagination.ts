/**
 * Pagination Types and Utilities
 *
 * Shared types for paginated API responses.
 */

// ============================================================================
// Types
// ============================================================================

/**
 * Pagination metadata returned with list responses
 */
export interface PaginationMeta {
  /** Total number of items across all pages */
  total: number
  /** Maximum items per page */
  limit: number
  /** Number of items to skip */
  offset: number
  /** Whether there are more items after this page */
  hasMore: boolean
  /** Total number of pages (optional) */
  totalPages?: number
  /** Current page number, 1-indexed (optional) */
  page?: number
}

/**
 * Generic paginated response wrapper
 */
export interface PaginatedResponse<T> {
  items: T[]
  pagination: PaginationMeta
}

/**
 * Parameters for paginated list requests
 */
export interface PaginationParams {
  /** Maximum items to return (default: 50, max: 500) */
  limit?: number
  /** Number of items to skip (default: 0) */
  offset?: number
}

// ============================================================================
// Utilities
// ============================================================================

/**
 * Default pagination parameters
 */
export const DEFAULT_PAGINATION: Required<PaginationParams> = {
  limit: 50,
  offset: 0,
}

/**
 * Calculate the current page from offset and limit
 */
export function calculatePage(offset: number, limit: number): number {
  if (limit <= 0) return 1
  return Math.floor(offset / limit) + 1
}

/**
 * Calculate offset from page number and limit
 */
export function calculateOffset(page: number, limit: number): number {
  if (page <= 1) return 0
  return (page - 1) * limit
}

/**
 * Create pagination params for a specific page
 */
export function pageParams(page: number, limit: number = DEFAULT_PAGINATION.limit): PaginationParams {
  return {
    limit,
    offset: calculateOffset(page, limit),
  }
}

/**
 * Check if there's a next page based on current state
 */
export function hasNextPage(pagination: PaginationMeta): boolean {
  return pagination.hasMore
}

/**
 * Check if there's a previous page based on current state
 */
export function hasPrevPage(pagination: PaginationMeta): boolean {
  return pagination.offset > 0
}

/**
 * Build query string params from pagination params
 */
export function paginationToQueryParams(params: PaginationParams): Record<string, string> {
  const result: Record<string, string> = {}
  if (params.limit !== undefined) {
    result['limit'] = String(params.limit)
  }
  if (params.offset !== undefined) {
    result['offset'] = String(params.offset)
  }
  return result
}
