/**
 * Tests for Recommendations API
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { recommendationsApi } from './recommendations'

// Mock axios
vi.mock('axios', () => {
  const mockAxios = {
    create: vi.fn(() => mockAxios),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    defaults: {
      headers: {
        common: {},
      },
    },
    interceptors: {
      request: {
        use: vi.fn(),
      },
      response: {
        use: vi.fn(),
      },
    },
  }
  return { default: mockAxios }
})

const mockedAxios = axios as unknown as {
  create: ReturnType<typeof vi.fn>
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

describe('Recommendations API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('recommendationsApi', () => {
    describe('get()', () => {
      it('should fetch recommendations', async () => {
        const mockData = {
          labs: [
            {
              labTemplateId: 'lab-1',
              labName: 'Test Lab',
              labSlug: 'test-lab',
              type: 'continue_progress',
              reason: 'Continue where you left off',
              priority: 0,
            },
          ],
          pathways: [
            {
              pathwayId: 'p-1',
              pathwayName: 'Test Pathway',
              pathwaySlug: 'test-pathway',
              type: 'new_pathway',
              reason: 'Featured pathway',
              priority: 1,
            },
          ],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        const result = await recommendationsApi.get()

        expect(result).toEqual(mockData)
        expect(mockedAxios.get).toHaveBeenCalledWith('/recommendations', { params: {} })
      })

      it('should pass limit as query param', async () => {
        const mockData = { labs: [], pathways: [] }
        mockedAxios.get.mockResolvedValueOnce({ data: mockData })

        await recommendationsApi.get(3)

        expect(mockedAxios.get).toHaveBeenCalledWith('/recommendations', { params: { limit: 3 } })
      })

      it('should return empty arrays when no recommendations', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { labs: [], pathways: [] } })

        const result = await recommendationsApi.get()

        expect(result.labs).toEqual([])
        expect(result.pathways).toEqual([])
      })
    })

    describe('getNextLabInPathway()', () => {
      it('should fetch next lab for a pathway', async () => {
        const mockLab = {
          labTemplateId: 'lab-firewall',
          labName: 'Firewall Configuration',
          labSlug: 'firewall-config',
          type: 'next_in_pathway',
          reason: 'Next lab in Network Security',
          priority: 0,
          pathwayId: 'pathway-netsec',
          pathwayName: 'Network Security Fundamentals',
          moduleId: 'mod-firewalls',
          moduleName: 'Firewalls & Access Control',
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockLab })

        const result = await recommendationsApi.getNextLabInPathway('pathway-netsec')

        expect(result).toEqual(mockLab)
        expect(mockedAxios.get).toHaveBeenCalledWith('/pathways/pathway-netsec/next-lab')
      })

      it('should return null on 404 (pathway completed or not enrolled)', async () => {
        mockedAxios.get.mockRejectedValueOnce({ response: { status: 404 } })

        const result = await recommendationsApi.getNextLabInPathway('pathway-completed')

        expect(result).toBeNull()
      })

      it('should return null on any error', async () => {
        mockedAxios.get.mockRejectedValueOnce(new Error('Network error'))

        const result = await recommendationsApi.getNextLabInPathway('pathway-1')

        expect(result).toBeNull()
      })
    })
  })

  describe('Types', () => {
    it('should support all recommendation types', async () => {
      const mockData = {
        labs: [
          { labTemplateId: 'l1', labName: 'Lab 1', labSlug: 'lab-1', type: 'next_in_pathway', reason: 'Next', priority: 0 },
          { labTemplateId: 'l2', labName: 'Lab 2', labSlug: 'lab-2', type: 'continue_progress', reason: 'Continue', priority: 1 },
          { labTemplateId: 'l3', labName: 'Lab 3', labSlug: 'lab-3', type: 'similar_difficulty', reason: 'Similar', priority: 2 },
          { labTemplateId: 'l4', labName: 'Lab 4', labSlug: 'lab-4', type: 'popular', reason: 'Popular', priority: 3 },
        ],
        pathways: [
          { pathwayId: 'p1', pathwayName: 'Pathway 1', pathwaySlug: 'pathway-1', type: 'new_pathway', reason: 'New', priority: 0 },
          { pathwayId: 'p2', pathwayName: 'Pathway 2', pathwaySlug: 'pathway-2', type: 'popular', reason: 'Popular', priority: 1 },
        ],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await recommendationsApi.get()

      expect(result.labs).toHaveLength(4)
      expect(result.pathways).toHaveLength(2)
      expect(result.labs[0]!.type).toBe('next_in_pathway')
      expect(result.labs[1]!.type).toBe('continue_progress')
      expect(result.labs[2]!.type).toBe('similar_difficulty')
      expect(result.labs[3]!.type).toBe('popular')
      expect(result.pathways[0]!.type).toBe('new_pathway')
    })

    it('should include optional lab recommendation fields', async () => {
      const mockData = {
        labs: [
          {
            labTemplateId: 'lab-1',
            labName: 'Test Lab',
            labSlug: 'test-lab',
            labDescription: 'A test lab description',
            difficulty: 'intermediate',
            durationMinutes: 60,
            maxPoints: 150,
            type: 'next_in_pathway',
            reason: 'Next lab',
            priority: 0,
            pathwayId: 'p-1',
            pathwayName: 'Test Pathway',
            moduleId: 'm-1',
            moduleName: 'Test Module',
          },
        ],
        pathways: [],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await recommendationsApi.get()
      const lab = result.labs[0]!

      expect(lab.labDescription).toBe('A test lab description')
      expect(lab.difficulty).toBe('intermediate')
      expect(lab.durationMinutes).toBe(60)
      expect(lab.maxPoints).toBe(150)
      expect(lab.pathwayId).toBe('p-1')
      expect(lab.pathwayName).toBe('Test Pathway')
      expect(lab.moduleId).toBe('m-1')
      expect(lab.moduleName).toBe('Test Module')
    })

    it('should include optional pathway recommendation fields', async () => {
      const mockData = {
        labs: [],
        pathways: [
          {
            pathwayId: 'p-1',
            pathwayName: 'Cloud Security',
            pathwaySlug: 'cloud-security',
            description: 'Learn cloud security concepts',
            difficulty: 'intermediate',
            estimatedHours: 16,
            moduleCount: 8,
            labCount: 24,
            type: 'new_pathway',
            reason: 'Featured',
            priority: 0,
            coverImageUrl: '/images/cloud.png',
            icon: 'pi-cloud',
          },
        ],
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockData })

      const result = await recommendationsApi.get()
      const pathway = result.pathways[0]!

      expect(pathway.description).toBe('Learn cloud security concepts')
      expect(pathway.difficulty).toBe('intermediate')
      expect(pathway.estimatedHours).toBe(16)
      expect(pathway.moduleCount).toBe(8)
      expect(pathway.labCount).toBe(24)
      expect(pathway.coverImageUrl).toBe('/images/cloud.png')
      expect(pathway.icon).toBe('pi-cloud')
    })
  })
})
