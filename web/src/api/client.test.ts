import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import { labsApi, podsApi, snapshotsApi, sessionsApi, assessmentApi } from './index'
import { labsApi as labsApiDirect } from './domains/labs'

// Mock axios
vi.mock('axios', () => {
  const mockAxios = {
    create: vi.fn(() => mockAxios),
    get: vi.fn(),
    post: vi.fn(),
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
  delete: ReturnType<typeof vi.fn>
}

describe('API Client', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('labsApi', () => {
    it('list() should fetch labs and return response with pagination', async () => {
      const mockLabs = [
        { id: 'lab-1', name: 'Network Basics', difficulty: 'beginner' },
        { id: 'lab-2', name: 'Advanced Routing', difficulty: 'advanced' },
      ]
      const mockResponse = {
        labs: mockLabs,
        count: 2,
        total: 2,
        pagination: { total: 2, limit: 50, offset: 0, hasMore: false },
      }
      mockedAxios.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await labsApi.list()

      expect(result.labs).toEqual(mockLabs)
      expect(result.count).toBe(2)
    })

    it('list() should return response with empty labs array when no labs', async () => {
      const mockResponse = { labs: [], count: 0 }
      mockedAxios.get.mockResolvedValueOnce({ data: mockResponse })

      const result = await labsApi.list()

      expect(result.labs).toEqual([])
    })

    it('get() should fetch single lab by id', async () => {
      const mockLab = { id: 'lab-1', name: 'Network Basics', difficulty: 'beginner' }
      mockedAxios.get.mockResolvedValueOnce({ data: mockLab })

      const result = await labsApi.get('lab-1')

      expect(result).toEqual(mockLab)
    })
  })

  describe('podsApi', () => {
    it('list() should fetch all pods', async () => {
      const mockPods = [
        { id: 'pod-1', labTemplate: 'lab-1', status: 'running' },
        { id: 'pod-2', labTemplate: 'lab-2', status: 'stopped' },
      ]
      mockedAxios.get.mockResolvedValueOnce({ data: { pods: mockPods } })

      const result = await podsApi.list()

      expect(result).toEqual(mockPods)
    })

    it('list() should filter by owner', async () => {
      const mockPods = [{ id: 'pod-1', labTemplate: 'lab-1', owner: 'user@test.com' }]
      mockedAxios.get.mockResolvedValueOnce({ data: { pods: mockPods } })

      const result = await podsApi.list('user@test.com')

      expect(result).toEqual(mockPods)
    })

    it('list() should return empty array when no pods', async () => {
      mockedAxios.get.mockResolvedValueOnce({ data: { pods: null } })

      const result = await podsApi.list()

      expect(result).toEqual([])
    })

    it('get() should fetch single pod by id', async () => {
      const mockPod = { id: 'pod-1', labTemplate: 'lab-1', status: 'running' }
      mockedAxios.get.mockResolvedValueOnce({ data: mockPod })

      const result = await podsApi.get('pod-1')

      expect(result).toEqual(mockPod)
    })

    it('create() should create a new pod', async () => {
      const mockPod = { id: 'pod-new', labTemplate: 'lab-1', owner: 'user@test.com', status: 'provisioning' }
      mockedAxios.post.mockResolvedValueOnce({ data: mockPod })

      const result = await podsApi.create('lab-1', 'user@test.com')

      expect(result).toEqual(mockPod)
    })

    it('destroy() should delete a pod', async () => {
      mockedAxios.delete.mockResolvedValueOnce({})

      await expect(podsApi.destroy('pod-1')).resolves.toBeUndefined()
    })

    it('resetVM() should reset VM to snapshot', async () => {
      mockedAxios.post.mockResolvedValueOnce({})

      await expect(podsApi.resetVM('pod-1', 'vm-1', 'initial')).resolves.toBeUndefined()
    })
  })

  describe('snapshotsApi', () => {
    it('list() should fetch snapshots for a VM', async () => {
      const mockSnapshots = [
        { name: 'initial', description: 'Initial state' },
        { name: 'checkpoint-1', description: 'After config' },
      ]
      mockedAxios.get.mockResolvedValueOnce({
        data: { podId: 'pod-1', vmName: 'vm-1', snapshots: mockSnapshots, count: 2 },
      })

      const result = await snapshotsApi.list('pod-1', 'vm-1')

      expect(result).toEqual(mockSnapshots)
    })

    it('list() should return empty array when no snapshots', async () => {
      mockedAxios.get.mockResolvedValueOnce({
        data: { podId: 'pod-1', vmName: 'vm-1', snapshots: null, count: 0 },
      })

      const result = await snapshotsApi.list('pod-1', 'vm-1')

      expect(result).toEqual([])
    })

    it('create() should create a snapshot', async () => {
      mockedAxios.post.mockResolvedValueOnce({})

      await expect(
        snapshotsApi.create('pod-1', 'vm-1', 'checkpoint-1', 'Test description', true)
      ).resolves.toBeUndefined()
    })

    it('delete() should delete a snapshot', async () => {
      mockedAxios.delete.mockResolvedValueOnce({})

      await expect(snapshotsApi.delete('pod-1', 'vm-1', 'checkpoint-1')).resolves.toBeUndefined()
    })
  })

  describe('sessionsApi', () => {
    it('list() should fetch all sessions', async () => {
      const mockSessions = [
        { id: 'session-1', podId: 'pod-1', status: 'active' },
        { id: 'session-2', podId: 'pod-2', status: 'completed' },
      ]
      mockedAxios.get.mockResolvedValueOnce({ data: { sessions: mockSessions, count: 2 } })

      const result = await sessionsApi.list()

      expect(result).toEqual(mockSessions)
    })

    it('list() should filter by userId and active', async () => {
      const mockSessions = [{ id: 'session-1', podId: 'pod-1', userId: 'user@test.com', status: 'active' }]
      mockedAxios.get.mockResolvedValueOnce({ data: { sessions: mockSessions, count: 1 } })

      const result = await sessionsApi.list('user@test.com', true)

      expect(result).toEqual(mockSessions)
    })

    it('list() should return empty array when no sessions', async () => {
      mockedAxios.get.mockResolvedValueOnce({ data: { sessions: null, count: 0 } })

      const result = await sessionsApi.list()

      expect(result).toEqual([])
    })

    it('get() should fetch single session', async () => {
      const mockSession = { id: 'session-1', podId: 'pod-1', status: 'active' }
      mockedAxios.get.mockResolvedValueOnce({ data: mockSession })

      const result = await sessionsApi.get('session-1')

      expect(result).toEqual(mockSession)
    })

    it('create() should create a new session', async () => {
      const mockResponse = { sessionId: 'session-new', status: 'active', maxPoints: 100 }
      mockedAxios.post.mockResolvedValueOnce({ data: mockResponse })

      const result = await sessionsApi.create({
        podId: 'pod-1',
        userId: 'user@test.com',
        labTemplate: 'lab-1',
      })

      expect(result).toEqual(mockResponse)
    })

    it('end() should end a session', async () => {
      const mockResult = { status: 'completed', earnedPoints: 85, passed: true }
      mockedAxios.post.mockResolvedValueOnce({ data: mockResult })

      const result = await sessionsApi.end('session-1')

      expect(result).toEqual(mockResult)
    })

    it('getProgress() should get session progress', async () => {
      const mockProgress = { sessionId: 'session-1', earnedPoints: 50, maxPoints: 100, checkpoints: [] }
      mockedAxios.get.mockResolvedValueOnce({ data: mockProgress })

      const result = await sessionsApi.getProgress('session-1')

      expect(result).toEqual(mockProgress)
    })
  })

  describe('assessmentApi', () => {
    it('get() should fetch assessment results', async () => {
      const mockResult = { sessionId: 'session-1', score: 85, maxScore: 100 }
      mockedAxios.get.mockResolvedValueOnce({ data: mockResult })

      const result = await assessmentApi.get('session-1')

      expect(result.data).toEqual(mockResult)
    })

    it('run() should run assessment verification', async () => {
      const mockResult = { sessionId: 'session-1', score: 85, maxScore: 100, status: 'completed' }
      mockedAxios.post.mockResolvedValueOnce({ data: mockResult })

      const result = await assessmentApi.run('session-1')

      expect(result.data).toEqual(mockResult)
    })

    it('getStatus() should get assessment status', async () => {
      const mockStatus = { sessionId: 'session-1', status: 'running' }
      mockedAxios.get.mockResolvedValueOnce({ data: mockStatus })

      const result = await assessmentApi.getStatus('session-1')

      expect(result.data).toEqual(mockStatus)
    })

    it('getComponents() should get component results', async () => {
      const mockComponents = { sessionId: 'session-1', components: [{ id: 'vlsm', earnedPoints: 10 }] }
      mockedAxios.get.mockResolvedValueOnce({ data: mockComponents })

      const result = await assessmentApi.getComponents('session-1')

      expect(result.data).toEqual(mockComponents)
    })

    it('getDevice() should get device results', async () => {
      const mockDevice = { name: 'R1', type: 'router', status: 'correct' }
      mockedAxios.get.mockResolvedValueOnce({ data: mockDevice })

      const result = await assessmentApi.getDevice('session-1', 'R1')

      expect(result.data).toEqual(mockDevice)
    })
  })

  describe('Barrel Exports', () => {
    it('should export same API from index.ts and direct domain import', () => {
      // Verify barrel file exports match direct imports
      expect(labsApi).toBe(labsApiDirect)
    })
  })
})
