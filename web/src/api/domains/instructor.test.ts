/**
 * Tests for Instructor API module
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { instructorApi } from './instructor'
import { api } from '../config'
import * as mockModule from '../shared/mock'

// Mock the api module
vi.mock('../config', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('instructorApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getDashboard', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock dashboard data', async () => {
        const result = await instructorApi.getDashboard()

        expect(result).toBeDefined()
        expect(result.classOverview).toBeDefined()
        expect(result.students).toBeDefined()
        expect(result.labStats).toBeDefined()
        expect(result.activityHeatmap).toBeDefined()
        expect(result.recentProgress).toBeDefined()
      })

      it('returns class overview with correct structure', async () => {
        const result = await instructorApi.getDashboard()

        expect(result.classOverview.totalStudents).toBeGreaterThan(0)
        expect(result.classOverview.activeStudents).toBeDefined()
        expect(result.classOverview.inactiveStudents).toBeDefined()
        expect(result.classOverview.atRiskStudents).toBeDefined()
        expect(result.classOverview.excellingStudents).toBeDefined()
        expect(result.classOverview.averageCompletion).toBeDefined()
        expect(result.classOverview.averageScore).toBeDefined()
      })

      it('does not call actual API in mock mode', async () => {
        await instructorApi.getDashboard()

        expect(api.get).not.toHaveBeenCalled()
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint', async () => {
        const mockResponse = {
          data: {
            classOverview: { totalStudents: 10 },
            students: [],
            labStats: [],
            activityHeatmap: [],
            recentProgress: [],
          },
        }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getDashboard()

        expect(api.get).toHaveBeenCalledWith('/instructor/dashboard', { params: {} })
      })

      it('passes organizationId when provided', async () => {
        const mockResponse = { data: {} }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getDashboard('org-123')

        expect(api.get).toHaveBeenCalledWith('/instructor/dashboard', {
          params: { organizationId: 'org-123' },
        })
      })
    })
  })

  describe('getStudents', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock students list', async () => {
        const result = await instructorApi.getStudents()

        expect(result).toBeDefined()
        expect(Array.isArray(result)).toBe(true)
        expect(result.length).toBeGreaterThan(0)
      })

      it('returns students with correct structure', async () => {
        const result = await instructorApi.getStudents()

        const student = result[0]!
        expect(student.id).toBeDefined()
        expect(student.name).toBeDefined()
        expect(student.email).toBeDefined()
        expect(student.labsCompleted).toBeDefined()
        expect(student.averageScore).toBeDefined()
        expect(student.status).toBeDefined()
      })

      it('includes students with various statuses', async () => {
        const result = await instructorApi.getStudents()

        const statuses = new Set(result.map(s => s.status))
        expect(statuses.size).toBeGreaterThan(1)
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint', async () => {
        const mockResponse = { data: { students: [] } }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getStudents()

        expect(api.get).toHaveBeenCalledWith('/instructor/students', { params: {} })
      })
    })
  })

  describe('getLabStats', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock lab statistics', async () => {
        const result = await instructorApi.getLabStats()

        expect(result).toBeDefined()
        expect(Array.isArray(result)).toBe(true)
        expect(result.length).toBeGreaterThan(0)
      })

      it('returns lab stats with failure points', async () => {
        const result = await instructorApi.getLabStats()

        const labWithFailures = result.find(l => l.failurePoints.length > 0)
        expect(labWithFailures).toBeDefined()
        expect(labWithFailures!.failurePoints[0]).toHaveProperty('checkpointId')
        expect(labWithFailures!.failurePoints[0]).toHaveProperty('failureCount')
        expect(labWithFailures!.failurePoints[0]).toHaveProperty('failureRate')
      })

      it('returns lab stats with completion metrics', async () => {
        const result = await instructorApi.getLabStats()

        const lab = result[0]!
        expect(lab.totalAttempts).toBeDefined()
        expect(lab.completions).toBeDefined()
        expect(lab.passRate).toBeDefined()
        expect(lab.averageScore).toBeDefined()
        expect(lab.averageTimeMinutes).toBeDefined()
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint', async () => {
        const mockResponse = { data: { labs: [] } }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getLabStats()

        expect(api.get).toHaveBeenCalledWith('/instructor/lab-stats', { params: {} })
      })
    })
  })

  describe('getActivityHeatmap', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock heatmap data', async () => {
        const result = await instructorApi.getActivityHeatmap()

        expect(result).toBeDefined()
        expect(Array.isArray(result)).toBe(true)
        expect(result.length).toBeGreaterThan(0)
      })

      it('returns heatmap data with correct structure', async () => {
        const result = await instructorApi.getActivityHeatmap()

        const entry = result[0]!
        expect(entry.date).toBeDefined()
        expect(entry.hour).toBeDefined()
        expect(entry.count).toBeDefined()
        expect(typeof entry.hour).toBe('number')
        expect(entry.hour).toBeGreaterThanOrEqual(0)
        expect(entry.hour).toBeLessThan(24)
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint', async () => {
        const mockResponse = { data: { data: [] } }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getActivityHeatmap()

        expect(api.get).toHaveBeenCalledWith('/instructor/activity-heatmap', { params: {} })
      })

      it('passes days parameter when provided', async () => {
        const mockResponse = { data: { data: [] } }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getActivityHeatmap(14)

        expect(api.get).toHaveBeenCalledWith('/instructor/activity-heatmap', { params: { days: 14 } })
      })
    })
  })

  describe('exportReport', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns CSV blob for csv format', async () => {
        const result = await instructorApi.exportReport('csv')

        expect(result).toBeInstanceOf(Blob)
        expect(result.type).toBe('text/csv')
      })

      it('returns JSON blob for json format', async () => {
        const result = await instructorApi.exportReport('json')

        expect(result).toBeInstanceOf(Blob)
        expect(result.type).toBe('application/json')
      })

      it('throws error for pdf format in mock mode', async () => {
        await expect(instructorApi.exportReport('pdf')).rejects.toThrow(
          'PDF export requires backend support'
        )
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint with format', async () => {
        const mockBlob = new Blob(['test'], { type: 'text/csv' })
        vi.mocked(api.get).mockResolvedValue({ data: mockBlob })

        await instructorApi.exportReport('csv')

        expect(api.get).toHaveBeenCalledWith('/instructor/export', {
          params: { format: 'csv' },
          responseType: 'blob',
        })
      })

      it('includes organizationId in params when provided', async () => {
        const mockBlob = new Blob(['test'], { type: 'application/json' })
        vi.mocked(api.get).mockResolvedValue({ data: mockBlob })

        await instructorApi.exportReport('json', 'org-456')

        expect(api.get).toHaveBeenCalledWith('/instructor/export', {
          params: { format: 'json', organizationId: 'org-456' },
          responseType: 'blob',
        })
      })
    })
  })

  describe('getStrugglingStudents', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns students with at-risk or inactive status', async () => {
        const result = await instructorApi.getStrugglingStudents()

        expect(result).toBeDefined()
        expect(Array.isArray(result)).toBe(true)

        // All returned students should be at-risk or inactive
        result.forEach(student => {
          expect(['at-risk', 'inactive']).toContain(student.status)
        })
      })

      it('returns fewer students than total', async () => {
        const allStudents = await instructorApi.getStudents()
        const strugglingStudents = await instructorApi.getStrugglingStudents()

        expect(strugglingStudents.length).toBeLessThan(allStudents.length)
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls correct endpoint', async () => {
        const mockResponse = { data: { students: [] } }
        vi.mocked(api.get).mockResolvedValue(mockResponse)

        await instructorApi.getStrugglingStudents()

        expect(api.get).toHaveBeenCalledWith('/instructor/struggling-students', { params: {} })
      })
    })
  })
})
