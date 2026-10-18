import type { AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import type { DOMWrapper } from '@vue/test-utils'

/**
 * Creates a mock AxiosResponse with all required properties
 */
export function createMockAxiosResponse<T>(
  data: T,
  status = 200,
  statusText = 'OK'
): AxiosResponse<T> {
  return {
    data,
    status,
    statusText,
    headers: {},
    config: {
      headers: {},
    } as InternalAxiosRequestConfig,
  }
}

/**
 * Type guard to assert an element exists before interacting with it
 */
export function assertElementExists<T extends Element>(
  wrapper: DOMWrapper<T> | undefined,
  message = 'Element should exist'
): asserts wrapper is DOMWrapper<T> {
  if (!wrapper || !wrapper.exists()) {
    throw new Error(message)
  }
}

/**
 * Safely get an element at index from a wrapper array
 * Returns the element or throws if not found
 */
export function getElementAt<T extends Element>(
  wrappers: DOMWrapper<T>[],
  index: number,
  message = `Element at index ${index} should exist`
): DOMWrapper<T> {
  const element = wrappers[index]
  if (!element) {
    throw new Error(message)
  }
  return element
}
