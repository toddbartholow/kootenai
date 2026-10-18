import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAssessmentStore } from './assessment'
import { assessmentApi, type AssessmentResult, type CheckUpdatePayload, type DeviceUpdatePayload, type ComponentUpdatePayload, type AssessmentCompletePayload } from '@/api'
import { AxiosError } from 'axios'
import { createMockAxiosResponse } from '@/test-utils'

// Mock the assessment API
vi.mock('@/api', () => ({
  assessmentApi: {
    get: vi.fn(),
    run: vi.fn(),
    getStatus: vi.fn(),
    getComponents: vi.fn(),
    getDevice: vi.fn(),
  },
  AxiosError: class MockAxiosError extends Error {
    response: { status?: number; data?: { message?: string } } | null
    constructor(message: string, response?: { status?: number; data?: { message?: string } }) {
      super(message)
      this.response = response ?? null
    }
  },
}))

describe('Assessment Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const store = useAssessmentStore()

      expect(store.result).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.isRunning).toBe(false)
      expect(store.expandedDevices.size).toBe(0)
      expect(store.expandedInterfaces.size).toBe(0)
    })

    it('should have correct computed initial values', () => {
      const store = useAssessmentStore()

      expect(store.percentage).toBe(0)
      expect(store.score).toBe(0)
      expect(store.maxScore).toBe(0)
      expect(store.passedCount).toBe(0)
      expect(store.itemCount).toBe(0)
      expect(store.canRetry).toBe(false)
      expect(store.errorMessage).toBeNull()
    })
  })

  describe('fetchAssessment()', () => {
    it('should fetch assessment results successfully', async () => {
      const mockResult: AssessmentResult = {
        sessionId: 'session-1',
        score: 85,
        maxScore: 100,
        percentage: 85,
        itemCount: 10,
        passedCount: 8,
        status: 'completed',
        startedAt: '2024-01-01T00:00:00Z',
        lastChecked: '2024-01-01T01:00:00Z',
        components: [],
        devices: [
          { name: 'R1', type: 'router', status: 'correct', totalItems: 5, passedItems: 5, earnedPoints: 50, maxPoints: 50 },
        ],
        timeElapsed: '1h 0m',
      }
      vi.mocked(assessmentApi.get).mockResolvedValueOnce(createMockAxiosResponse(mockResult))

      const store = useAssessmentStore()
      await store.fetchAssessment('session-1')

      expect(store.result).toEqual(mockResult)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      // Devices should be auto-expanded
      expect(store.expandedDevices.has('R1')).toBe(true)
    })

    it('should handle 404 gracefully (no results yet)', async () => {
      const error = new AxiosError('Not found')
      ;(error as any).response = { status: 404 }
      vi.mocked(assessmentApi.get).mockRejectedValueOnce(error)

      const store = useAssessmentStore()
      await store.fetchAssessment('session-1')

      expect(store.result).toBeNull()
      expect(store.error).toBeNull() // 404 should not be treated as error
    })

    it('should set error on API failure', async () => {
      const error = new AxiosError('Server error')
      ;(error as any).response = { status: 500, data: { message: 'Internal server error' } }
      vi.mocked(assessmentApi.get).mockRejectedValueOnce(error)

      const store = useAssessmentStore()
      await store.fetchAssessment('session-1')

      expect(store.error).not.toBeNull()
      expect(store.error?.type).toBe('server')
      expect(store.error?.retryable).toBe(true)
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise: (value: unknown) => void
      const pendingPromise = new Promise(resolve => {
        resolvePromise = resolve
      })
      vi.mocked(assessmentApi.get).mockReturnValueOnce(pendingPromise as never)

      const store = useAssessmentStore()
      const fetchPromise = store.fetchAssessment('session-1')

      expect(store.loading).toBe(true)

      resolvePromise!({ data: { sessionId: 'session-1', devices: [] } })
      await fetchPromise

      expect(store.loading).toBe(false)
    })
  })

  describe('runAssessment()', () => {
    it('should run assessment successfully', async () => {
      const mockResult: AssessmentResult = {
        sessionId: 'session-1',
        score: 75,
        maxScore: 100,
        percentage: 75,
        itemCount: 10,
        passedCount: 7,
        status: 'completed',
        startedAt: '2024-01-01T00:00:00Z',
        lastChecked: '2024-01-01T01:00:00Z',
        components: [],
        devices: [],
        timeElapsed: '1h 0m',
      }
      vi.mocked(assessmentApi.run).mockResolvedValueOnce(createMockAxiosResponse(mockResult))

      const store = useAssessmentStore()
      await store.runAssessment('session-1')

      expect(store.result).toEqual(mockResult)
      expect(store.isRunning).toBe(false)
    })

    it('should set isRunning during assessment', async () => {
      let resolvePromise: (value: unknown) => void
      const pendingPromise = new Promise(resolve => {
        resolvePromise = resolve
      })
      vi.mocked(assessmentApi.run).mockReturnValueOnce(pendingPromise as never)

      const store = useAssessmentStore()
      const runPromise = store.runAssessment('session-1')

      expect(store.isRunning).toBe(true)

      resolvePromise!({ data: { sessionId: 'session-1', devices: [] } })
      await runPromise

      expect(store.isRunning).toBe(false)
    })

    it('should throw and set error on failure', async () => {
      const error = new Error('Assessment failed')
      vi.mocked(assessmentApi.run).mockRejectedValueOnce(error)

      const store = useAssessmentStore()

      await expect(store.runAssessment('session-1')).rejects.toThrow('Assessment failed')
      expect(store.error).not.toBeNull()
    })
  })

  describe('toggleDevice()', () => {
    it('should expand collapsed device', () => {
      const store = useAssessmentStore()

      store.toggleDevice('R1')

      expect(store.expandedDevices.has('R1')).toBe(true)
    })

    it('should collapse expanded device', () => {
      const store = useAssessmentStore()
      store.expandedDevices = new Set(['R1'])

      store.toggleDevice('R1')

      expect(store.expandedDevices.has('R1')).toBe(false)
    })
  })

  describe('toggleInterface()', () => {
    it('should expand collapsed interface', () => {
      const store = useAssessmentStore()

      store.toggleInterface('R1:GigabitEthernet0/0')

      expect(store.expandedInterfaces.has('R1:GigabitEthernet0/0')).toBe(true)
    })

    it('should collapse expanded interface', () => {
      const store = useAssessmentStore()
      store.expandedInterfaces = new Set(['R1:GigabitEthernet0/0'])

      store.toggleInterface('R1:GigabitEthernet0/0')

      expect(store.expandedInterfaces.has('R1:GigabitEthernet0/0')).toBe(false)
    })
  })

  describe('isDeviceExpanded()', () => {
    it('should return true for expanded device', () => {
      const store = useAssessmentStore()
      store.expandedDevices = new Set(['R1'])

      expect(store.isDeviceExpanded('R1')).toBe(true)
    })

    it('should return false for collapsed device', () => {
      const store = useAssessmentStore()

      expect(store.isDeviceExpanded('R1')).toBe(false)
    })
  })

  describe('isInterfaceExpanded()', () => {
    it('should return true for expanded interface', () => {
      const store = useAssessmentStore()
      store.expandedInterfaces = new Set(['R1:GigabitEthernet0/0'])

      expect(store.isInterfaceExpanded('R1', 'GigabitEthernet0/0')).toBe(true)
    })

    it('should return false for collapsed interface', () => {
      const store = useAssessmentStore()

      expect(store.isInterfaceExpanded('R1', 'GigabitEthernet0/0')).toBe(false)
    })
  })

  describe('handleUpdate()', () => {
    beforeEach(() => {
      const store = useAssessmentStore()
      store.result = {
        sessionId: 'session-1',
        score: 50,
        maxScore: 100,
        percentage: 50,
        itemCount: 10,
        passedCount: 5,
        status: 'running',
        startedAt: '2024-01-01T00:00:00Z',
        lastChecked: '2024-01-01T01:00:00Z',
        components: [{ id: 'vlsm', description: 'VLSM', totalItems: 5, passedItems: 2, maxPoints: 50, earnedPoints: 20, percentage: 40 }],
        devices: [
          {
            name: 'R1',
            type: 'router',
            status: 'pending',
            checks: [{ id: 'check-1', description: 'Check 1', component: 'vlsm', status: 'pending', expected: '192.168.1.1', actual: '', points: 10, earnedPoints: 0, feedback: '' }],
            interfaces: [
              {
                name: 'GigabitEthernet0/0',
                status: 'pending',
                checks: [{ id: 'check-2', description: 'Check 2', component: 'vlsm', status: 'pending', expected: 'up', actual: '', points: 5, earnedPoints: 0, feedback: '' }],
                totalItems: 1,
                passedItems: 0,
                earnedPoints: 0,
                maxPoints: 5,
              },
            ],
            totalItems: 2,
            passedItems: 0,
            earnedPoints: 0,
            maxPoints: 15,
          },
        ],
        timeElapsed: '0m',
      }
    })

    it('should handle check_update for device-level check', () => {
      const store = useAssessmentStore()
      const update: CheckUpdatePayload = {
        type: 'check_update',
        sessionId: 'session-1',
        deviceName: 'R1',
        checkId: 'check-1',
        status: 'correct',
        actual: '192.168.1.1',
        earnedPoints: 10,
        feedback: 'Correct!',
        timestamp: new Date().toISOString(),
      }

      store.handleUpdate(update)

      const device = store.result?.devices[0]
      const check = device?.checks?.[0]
      expect(check?.status).toBe('correct')
      expect(check?.actual).toBe('192.168.1.1')
      expect(check?.earnedPoints).toBe(10)
      expect(check?.feedback).toBe('Correct!')
    })

    it('should handle check_update for interface-level check', () => {
      const store = useAssessmentStore()
      const update: CheckUpdatePayload = {
        type: 'check_update',
        sessionId: 'session-1',
        deviceName: 'R1',
        interfaceName: 'GigabitEthernet0/0',
        checkId: 'check-2',
        status: 'correct',
        actual: 'up',
        earnedPoints: 5,
        timestamp: new Date().toISOString(),
      }

      store.handleUpdate(update)

      const device = store.result?.devices[0]
      const iface = device?.interfaces?.[0]
      const check = iface?.checks[0]
      expect(check?.status).toBe('correct')
      expect(check?.actual).toBe('up')
    })

    it('should handle device_update', () => {
      const store = useAssessmentStore()
      const update: DeviceUpdatePayload = {
        type: 'device_update',
        sessionId: 'session-1',
        deviceName: 'R1',
        status: 'correct',
        earnedPoints: 15,
        passedItems: 2,
        timestamp: new Date().toISOString(),
      }

      store.handleUpdate(update)

      const device = store.result?.devices[0]
      expect(device?.status).toBe('correct')
      expect(device?.earnedPoints).toBe(15)
      expect(device?.passedItems).toBe(2)
    })

    it('should handle component_update', () => {
      const store = useAssessmentStore()
      const update: ComponentUpdatePayload = {
        type: 'component_update',
        sessionId: 'session-1',
        componentId: 'vlsm',
        earnedPoints: 40,
        maxPoints: 50,
        passedItems: 4,
        totalItems: 5,
        percentage: 80,
        timestamp: new Date().toISOString(),
      }

      store.handleUpdate(update)

      const component = store.result?.components[0]
      expect(component?.earnedPoints).toBe(40)
      expect(component?.passedItems).toBe(4)
      expect(component?.percentage).toBe(80)
    })

    it('should handle complete update', () => {
      const store = useAssessmentStore()
      const update: AssessmentCompletePayload = {
        type: 'complete',
        sessionId: 'session-1',
        totalScore: 90,
        maxScore: 100,
        passedItems: 9,
        totalItems: 10,
        percentage: 90,
        timestamp: new Date().toISOString(),
      }

      store.handleUpdate(update)

      expect(store.result?.score).toBe(90)
      expect(store.result?.maxScore).toBe(100)
      expect(store.result?.passedCount).toBe(9)
      expect(store.result?.itemCount).toBe(10)
      expect(store.result?.percentage).toBe(90)
      expect(store.result?.status).toBe('completed')
    })

    it('should ignore updates when result is null', () => {
      const store = useAssessmentStore()
      store.result = null

      const update: CheckUpdatePayload = {
        type: 'check_update',
        sessionId: 'session-1',
        deviceName: 'R1',
        checkId: 'check-1',
        status: 'correct',
        timestamp: new Date().toISOString(),
      }

      // Should not throw
      expect(() => store.handleUpdate(update)).not.toThrow()
    })
  })

  describe('clearError()', () => {
    it('should clear error state', () => {
      const store = useAssessmentStore()
      store.error = {
        message: 'Test error',
        type: 'unknown',
        timestamp: new Date(),
        retryable: true,
      }

      store.clearError()

      expect(store.error).toBeNull()
    })
  })

  describe('reset()', () => {
    it('should reset all state to initial values', () => {
      const store = useAssessmentStore()
      store.result = { sessionId: 'test' } as AssessmentResult
      store.loading = true
      store.error = { message: 'error', type: 'unknown', timestamp: new Date(), retryable: false }
      store.isRunning = true
      store.expandedDevices = new Set(['R1', 'R2'])
      store.expandedInterfaces = new Set(['R1:Gi0/0'])

      store.reset()

      expect(store.result).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.isRunning).toBe(false)
      expect(store.expandedDevices.size).toBe(0)
      expect(store.expandedInterfaces.size).toBe(0)
    })
  })
})
