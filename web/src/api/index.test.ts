/**
 * Tests for API barrel file exports
 * Verifies all domain APIs and types are properly exported
 */
import { describe, it, expect } from 'vitest'
import * as apiExports from './index'

describe('API Index Exports', () => {
  describe('Infrastructure', () => {
    it('should export api instance', () => {
      expect(apiExports.api).toBeDefined()
      expect(typeof apiExports.api.get).toBe('function')
      expect(typeof apiExports.api.post).toBe('function')
    })

    it('should export configuration functions', () => {
      expect(typeof apiExports.setOrganizationContext).toBe('function')
    })

    it('should export mock data utilities', () => {
      expect(typeof apiExports.USE_MOCK_DATA).toBe('boolean')
      expect(typeof apiExports.simulateDelay).toBe('function')
      expect(typeof apiExports.setMockDataEnabled).toBe('function')
      expect(typeof apiExports.isMockDataEnabled).toBe('function')
    })
  })

  describe('Domain APIs', () => {
    const expectedApis = [
      'labsApi',
      'labTemplatesApi',
      'podsApi',
      'snapshotsApi',
      'sessionsApi',
      'questionsApi',
      'assessmentApi',
      'consoleApi',
      'reservationsApi',
      'proxmoxApi',
      'achievementsApi',
      'organizationsApi',
      'teamsApi',
      'featuresApi',
      'auditApi',
      'pathwaysApi',
      'enrollmentsApi',
      'usersApi',
      'passwordApi',
      'dashboardApi',
      'eventsApi',
    ]

    it.each(expectedApis)('should export %s', apiName => {
      expect(apiExports[apiName as keyof typeof apiExports]).toBeDefined()
      expect(typeof apiExports[apiName as keyof typeof apiExports]).toBe('object')
    })
  })

  describe('Mock Data Exports', () => {
    it('should export mockLabs for cross-domain use', () => {
      expect(apiExports.mockLabs).toBeDefined()
      expect(Array.isArray(apiExports.mockLabs)).toBe(true)
    })

    it('should export mockPods for cross-domain use', () => {
      expect(apiExports.mockPods).toBeDefined()
      expect(Array.isArray(apiExports.mockPods)).toBe(true)
    })

    it('should export mockSnapshots for cross-domain use', () => {
      expect(apiExports.mockSnapshots).toBeDefined()
      // mockSnapshots is a nested Record, not an array
      expect(typeof apiExports.mockSnapshots).toBe('object')
    })

    it('should export mockSessions for cross-domain use', () => {
      expect(apiExports.mockSessions).toBeDefined()
      expect(Array.isArray(apiExports.mockSessions)).toBe(true)
    })

    it('should export mockReservations for cross-domain use', () => {
      expect(apiExports.mockReservations).toBeDefined()
      expect(Array.isArray(apiExports.mockReservations)).toBe(true)
    })

    it('should export mockPathways for cross-domain use', () => {
      expect(apiExports.mockPathways).toBeDefined()
      expect(Array.isArray(apiExports.mockPathways)).toBe(true)
    })

    it('should export mockAssessmentResult for testing', () => {
      expect(apiExports.mockAssessmentResult).toBeDefined()
      expect(typeof apiExports.mockAssessmentResult).toBe('object')
    })
  })

  describe('API Method Signatures', () => {
    it('labsApi should have expected methods', () => {
      expect(typeof apiExports.labsApi.list).toBe('function')
      expect(typeof apiExports.labsApi.get).toBe('function')
      expect(typeof apiExports.labsApi.create).toBe('function')
      expect(typeof apiExports.labsApi.update).toBe('function')
      expect(typeof apiExports.labsApi.getWithSpec).toBe('function')
    })

    it('podsApi should have expected methods', () => {
      expect(typeof apiExports.podsApi.list).toBe('function')
      expect(typeof apiExports.podsApi.get).toBe('function')
      expect(typeof apiExports.podsApi.create).toBe('function')
      expect(typeof apiExports.podsApi.destroy).toBe('function')
      expect(typeof apiExports.podsApi.resetVM).toBe('function')
    })

    it('sessionsApi should have expected methods', () => {
      expect(typeof apiExports.sessionsApi.list).toBe('function')
      expect(typeof apiExports.sessionsApi.get).toBe('function')
      expect(typeof apiExports.sessionsApi.create).toBe('function')
      expect(typeof apiExports.sessionsApi.end).toBe('function')
      expect(typeof apiExports.sessionsApi.getProgress).toBe('function')
    })

    it('pathwaysApi should have expected methods', () => {
      expect(typeof apiExports.pathwaysApi.list).toBe('function')
      expect(typeof apiExports.pathwaysApi.get).toBe('function')
      expect(typeof apiExports.pathwaysApi.create).toBe('function')
      expect(typeof apiExports.pathwaysApi.update).toBe('function')
      expect(typeof apiExports.pathwaysApi.delete).toBe('function')
      expect(typeof apiExports.pathwaysApi.enroll).toBe('function')
      expect(typeof apiExports.pathwaysApi.unenroll).toBe('function')
    })

    it('enrollmentsApi should have expected methods', () => {
      expect(typeof apiExports.enrollmentsApi.list).toBe('function')
      expect(typeof apiExports.enrollmentsApi.get).toBe('function')
      expect(typeof apiExports.enrollmentsApi.getProgress).toBe('function')
      expect(typeof apiExports.enrollmentsApi.getByPathway).toBe('function')
      expect(typeof apiExports.enrollmentsApi.enroll).toBe('function')
      expect(typeof apiExports.enrollmentsApi.unenroll).toBe('function')
    })

    it('organizationsApi should have expected methods', () => {
      expect(typeof apiExports.organizationsApi.list).toBe('function')
      expect(typeof apiExports.organizationsApi.get).toBe('function')
      expect(typeof apiExports.organizationsApi.getFeatures).toBe('function')
      expect(typeof apiExports.organizationsApi.getTeams).toBe('function')
      expect(typeof apiExports.organizationsApi.getMembers).toBe('function')
    })

    it('usersApi should have expected methods', () => {
      expect(typeof apiExports.usersApi.list).toBe('function')
      expect(typeof apiExports.usersApi.get).toBe('function')
      expect(typeof apiExports.usersApi.create).toBe('function')
      expect(typeof apiExports.usersApi.update).toBe('function')
      expect(typeof apiExports.usersApi.delete).toBe('function')
    })

    it('dashboardApi should have expected methods', () => {
      expect(typeof apiExports.dashboardApi.get).toBe('function')
      expect(typeof apiExports.dashboardApi.getActivity).toBe('function')
      expect(typeof apiExports.dashboardApi.getLeaderboard).toBe('function')
    })
  })
})
