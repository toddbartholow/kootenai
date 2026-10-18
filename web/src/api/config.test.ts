/**
 * Tests for API Configuration
 */
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { api, setOrganizationContext } from './config'

describe('API Config', () => {
  beforeEach(() => {
    delete api.defaults.headers.common['X-Organization-ID']
  })

  afterEach(() => {
    delete api.defaults.headers.common['X-Organization-ID']
  })

  describe('api instance', () => {
    it('should have correct base URL', () => {
      expect(api.defaults.baseURL).toBe('/api/v1')
    })

    it('should have timeout configured', () => {
      expect(api.defaults.timeout).toBe(30000)
    })

    it('should have withCredentials enabled for cookie auth', () => {
      expect(api.defaults.withCredentials).toBe(true)
    })

    it('should have standard HTTP methods', () => {
      expect(typeof api.get).toBe('function')
      expect(typeof api.post).toBe('function')
      expect(typeof api.put).toBe('function')
      expect(typeof api.delete).toBe('function')
      expect(typeof api.patch).toBe('function')
    })
  })

  describe('setOrganizationContext()', () => {
    it('should set X-Organization-ID header when orgId provided', () => {
      setOrganizationContext('org-123')

      expect(api.defaults.headers.common['X-Organization-ID']).toBe('org-123')
    })

    it('should remove X-Organization-ID header when null provided', () => {
      setOrganizationContext('org-123')
      expect(api.defaults.headers.common['X-Organization-ID']).toBe('org-123')

      setOrganizationContext(null)

      expect(api.defaults.headers.common['X-Organization-ID']).toBeUndefined()
    })

    it('should overwrite existing organization context', () => {
      setOrganizationContext('org-1')
      setOrganizationContext('org-2')

      expect(api.defaults.headers.common['X-Organization-ID']).toBe('org-2')
    })
  })
})
