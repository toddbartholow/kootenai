/**
 * Tests for Pathways API
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { pathwaysApi, mockPathways } from './pathways'
import { enrollmentsApi } from './enrollments'

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

describe('Pathways API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('pathwaysApi', () => {
    describe('list()', () => {
      it('should fetch pathways and return array', async () => {
        const mockData = [
          { id: 'p-1', name: 'Linux Basics', status: 'published' },
          { id: 'p-2', name: 'Security', status: 'draft' },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: { pathways: mockData, count: 2 } })

        const result = await pathwaysApi.list()

        expect(result).toEqual(mockData)
      })

      it('should return empty array when no pathways', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { pathways: null, count: 0 } })

        const result = await pathwaysApi.list()

        expect(result).toEqual([])
      })

      it('should pass filter options as query params', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { pathways: [], count: 0 } })

        await pathwaysApi.list({ status: 'published', difficulty: 'beginner', featured: true })

        expect(mockedAxios.get).toHaveBeenCalledWith('/pathways', {
          params: { status: 'published', difficulty: 'beginner', featured: 'true' },
        })
      })
    })

    describe('get()', () => {
      it('should fetch single pathway by id', async () => {
        const mockPathway = { id: 'p-1', name: 'Linux Basics', modules: [] }
        mockedAxios.get.mockResolvedValueOnce({ data: mockPathway })

        const result = await pathwaysApi.get('p-1')

        expect(result).toEqual(mockPathway)
        expect(mockedAxios.get).toHaveBeenCalledWith('/pathways/p-1')
      })
    })

    describe('getStats()', () => {
      it('should fetch pathway stats', async () => {
        const mockStats = { pathwayId: 'p-1', moduleCount: 5, labCount: 10 }
        mockedAxios.get.mockResolvedValueOnce({ data: mockStats })

        const result = await pathwaysApi.getStats('p-1')

        expect(result).toEqual(mockStats)
        expect(mockedAxios.get).toHaveBeenCalledWith('/pathways/p-1/stats')
      })
    })

    describe('enroll()', () => {
      it('should enroll user in pathway', async () => {
        const mockEnrollment = {
          id: 'enroll-1',
          userId: 'user-1',
          pathwayId: 'p-1',
          status: 'enrolled',
        }
        mockedAxios.post.mockResolvedValueOnce({ data: mockEnrollment })

        const result = await pathwaysApi.enroll('p-1')

        expect(result).toEqual(mockEnrollment)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways/p-1/enroll')
      })
    })

    describe('unenroll()', () => {
      it('should unenroll user from pathway', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(pathwaysApi.unenroll('p-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/pathways/p-1/enroll')
      })
    })

    describe('create()', () => {
      it('should create a new pathway', async () => {
        const newPathway = { id: 'p-new', name: 'New Pathway', status: 'draft' }
        mockedAxios.post.mockResolvedValueOnce({ data: newPathway })

        const result = await pathwaysApi.create({ name: 'New Pathway' })

        expect(result).toEqual(newPathway)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways', { name: 'New Pathway' })
      })
    })

    describe('update()', () => {
      it('should update pathway', async () => {
        const updatedPathway = { id: 'p-1', name: 'Updated Name', status: 'draft' }
        mockedAxios.put.mockResolvedValueOnce({ data: updatedPathway })

        const result = await pathwaysApi.update('p-1', { name: 'Updated Name' })

        expect(result).toEqual(updatedPathway)
        expect(mockedAxios.put).toHaveBeenCalledWith('/pathways/p-1', { name: 'Updated Name' })
      })
    })

    describe('delete()', () => {
      it('should delete pathway', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(pathwaysApi.delete('p-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/pathways/p-1')
      })
    })

    describe('publish()', () => {
      it('should publish pathway', async () => {
        const publishedPathway = { id: 'p-1', name: 'Pathway', status: 'published' }
        mockedAxios.post.mockResolvedValueOnce({ data: publishedPathway })

        const result = await pathwaysApi.publish('p-1')

        expect(result).toEqual(publishedPathway)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways/p-1/publish')
      })
    })

    describe('archive()', () => {
      it('should archive pathway', async () => {
        const archivedPathway = { id: 'p-1', name: 'Pathway', status: 'archived' }
        mockedAxios.post.mockResolvedValueOnce({ data: archivedPathway })

        const result = await pathwaysApi.archive('p-1')

        expect(result).toEqual(archivedPathway)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways/p-1/archive')
      })
    })

    describe('listModules()', () => {
      it('should list modules for pathway', async () => {
        const mockModules = [{ id: 'm-1', name: 'Module 1' }]
        mockedAxios.get.mockResolvedValueOnce({ data: { modules: mockModules } })

        const result = await pathwaysApi.listModules('p-1')

        expect(result).toEqual(mockModules)
        expect(mockedAxios.get).toHaveBeenCalledWith('/pathways/p-1/modules')
      })
    })

    describe('createModule()', () => {
      it('should create a module', async () => {
        const newModule = { id: 'm-new', name: 'New Module', pathwayId: 'p-1' }
        mockedAxios.post.mockResolvedValueOnce({ data: newModule })

        const result = await pathwaysApi.createModule('p-1', { name: 'New Module' })

        expect(result).toEqual(newModule)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways/p-1/modules', { name: 'New Module' })
      })
    })

    describe('updateModule()', () => {
      it('should update a module', async () => {
        const updatedModule = { id: 'm-1', name: 'Updated Module' }
        mockedAxios.put.mockResolvedValueOnce({ data: updatedModule })

        const result = await pathwaysApi.updateModule('m-1', { name: 'Updated Module' })

        expect(result).toEqual(updatedModule)
        expect(mockedAxios.put).toHaveBeenCalledWith('/modules/m-1', { name: 'Updated Module' })
      })
    })

    describe('deleteModule()', () => {
      it('should delete a module', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(pathwaysApi.deleteModule('m-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/modules/m-1')
      })
    })

    describe('addLabToModule()', () => {
      it('should add lab to module', async () => {
        const moduleLab = { id: 'ml-1', moduleId: 'm-1', labTemplateId: 'lab-1' }
        mockedAxios.post.mockResolvedValueOnce({ data: moduleLab })

        const result = await pathwaysApi.addLabToModule('m-1', { labTemplateId: 'lab-1' })

        expect(result).toEqual(moduleLab)
        expect(mockedAxios.post).toHaveBeenCalledWith('/modules/m-1/labs', { labTemplateId: 'lab-1' })
      })
    })

    describe('removeLabFromModule()', () => {
      it('should remove lab from module', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(pathwaysApi.removeLabFromModule('m-1', 'lab-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/modules/m-1/labs/lab-1')
      })
    })
  })

  describe('enrollmentsApi', () => {
    describe('list()', () => {
      it('should fetch enrollments', async () => {
        const mockEnrollments = [
          { id: 'e-1', pathwayId: 'p-1', status: 'in_progress' },
        ]
        mockedAxios.get.mockResolvedValueOnce({ data: { enrollments: mockEnrollments, count: 1 } })

        const result = await enrollmentsApi.list()

        expect(result).toEqual(mockEnrollments)
      })

      it('should filter by status', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { enrollments: [], count: 0 } })

        await enrollmentsApi.list({ status: 'completed' })

        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments', {
          params: { status: 'completed' },
        })
      })
    })

    describe('get()', () => {
      it('should fetch single enrollment', async () => {
        const mockEnrollment = { id: 'e-1', pathwayId: 'p-1', status: 'in_progress' }
        mockedAxios.get.mockResolvedValueOnce({ data: mockEnrollment })

        const result = await enrollmentsApi.get('e-1')

        expect(result).toEqual(mockEnrollment)
        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments/e-1')
      })
    })

    describe('getProgress()', () => {
      it('should fetch enrollment progress', async () => {
        const mockProgress = {
          id: 'e-1',
          pathwayId: 'p-1',
          moduleProgress: [{ id: 'mp-1', status: 'completed' }],
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockProgress })

        const result = await enrollmentsApi.getProgress('e-1')

        expect(result).toEqual(mockProgress)
        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments/e-1/progress')
      })
    })

    describe('getByPathway()', () => {
      it('should fetch enrollment by pathway', async () => {
        const mockEnrollment = { id: 'e-1', pathwayId: 'p-1', status: 'in_progress' }
        mockedAxios.get.mockResolvedValueOnce({ data: { enrollments: [mockEnrollment], count: 1 } })

        const result = await enrollmentsApi.getByPathway('p-1')

        expect(result).toEqual(mockEnrollment)
        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments', {
          params: { pathway_id: 'p-1' },
        })
      })

      it('should return null when no enrollment found', async () => {
        mockedAxios.get.mockResolvedValueOnce({ data: { enrollments: [], count: 0 } })

        const result = await enrollmentsApi.getByPathway('p-1')

        expect(result).toBeNull()
      })
    })

    describe('getUnlockRequirements()', () => {
      it('should fetch unlock requirements for module', async () => {
        const mockRequirements = {
          moduleId: 'm-1',
          moduleName: 'Module 1',
          unlockType: 'sequential',
          currentStatus: 'locked',
          requirements: [],
          message: 'Complete previous module',
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockRequirements })

        const result = await enrollmentsApi.getUnlockRequirements('e-1', 'm-1')

        expect(result).toEqual(mockRequirements)
        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments/e-1/modules/m-1/unlock-requirements')
      })
    })

    describe('manuallyUnlockModule()', () => {
      it('should manually unlock a module', async () => {
        mockedAxios.post.mockResolvedValueOnce({})

        await expect(enrollmentsApi.manuallyUnlockModule('e-1', 'm-1')).resolves.toBeUndefined()
        expect(mockedAxios.post).toHaveBeenCalledWith('/enrollments/e-1/modules/m-1/unlock')
      })
    })

    describe('enroll()', () => {
      it('should enroll in pathway', async () => {
        const mockEnrollment = { id: 'e-new', pathwayId: 'p-1', status: 'enrolled' }
        mockedAxios.post.mockResolvedValueOnce({ data: mockEnrollment })

        const result = await enrollmentsApi.enroll('p-1')

        expect(result).toEqual(mockEnrollment)
        expect(mockedAxios.post).toHaveBeenCalledWith('/pathways/p-1/enroll')
      })
    })

    describe('unenroll()', () => {
      it('should unenroll from pathway', async () => {
        mockedAxios.delete.mockResolvedValueOnce({})

        await expect(enrollmentsApi.unenroll('p-1')).resolves.toBeUndefined()
        expect(mockedAxios.delete).toHaveBeenCalledWith('/pathways/p-1/enroll')
      })
    })

    describe('issueCertificate()', () => {
      it('should issue certificate for completed enrollment', async () => {
        const mockCertificate = {
          id: 'cert-1',
          enrollmentId: 'e-1',
          userId: 'user-1',
          pathwayId: 'p-1',
          pathwayName: 'Linux Fundamentals',
          userName: 'Test User',
          earnedPoints: 1000,
          maxPoints: 1000,
          percentage: 100,
          completedAt: '2024-01-15T10:00:00Z',
          issuedAt: '2024-01-15T10:01:00Z',
          verificationCode: 'CERT-ABC123',
          verificationUrl: 'https://lab.example.com/verify/CERT-ABC123',
        }
        mockedAxios.post.mockResolvedValueOnce({ data: mockCertificate })

        const result = await enrollmentsApi.issueCertificate('e-1')

        expect(result).toEqual(mockCertificate)
        expect(mockedAxios.post).toHaveBeenCalledWith('/enrollments/e-1/certificate')
      })
    })

    describe('getCertificate()', () => {
      it('should get certificate for enrollment', async () => {
        const mockCertificate = {
          id: 'cert-1',
          enrollmentId: 'e-1',
          pathwayName: 'Linux Fundamentals',
          verificationCode: 'CERT-ABC123',
        }
        mockedAxios.get.mockResolvedValueOnce({ data: mockCertificate })

        const result = await enrollmentsApi.getCertificate('e-1')

        expect(result).toEqual(mockCertificate)
        expect(mockedAxios.get).toHaveBeenCalledWith('/enrollments/e-1/certificate')
      })
    })
  })

  describe('Mock Data', () => {
    it('should have mock pathways available for testing', () => {
      expect(mockPathways).toBeDefined()
      expect(Array.isArray(mockPathways)).toBe(true)
      expect(mockPathways.length).toBeGreaterThan(0)
    })

    it('mockPathways should have required fields', () => {
      const pathway = mockPathways[0]
      expect(pathway).toHaveProperty('id')
      expect(pathway).toHaveProperty('name')
      expect(pathway).toHaveProperty('slug')
      expect(pathway).toHaveProperty('status')
      expect(pathway).toHaveProperty('visibility')
    })
  })
})
