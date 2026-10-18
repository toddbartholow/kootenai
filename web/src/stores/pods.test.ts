import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePodsStore, type PodStatusUpdate, type VMStatusUpdate } from './pods'
import { podsApi, type Pod, type PodVM } from '@/api'
import type { ErrorState } from '@/types/errors'

const testError: ErrorState = {
  message: 'Some error',
  type: 'unknown',
  timestamp: new Date(),
  retryable: true,
}

// Mock the API client
vi.mock('@/api', () => ({
  podsApi: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    destroy: vi.fn(),
    start: vi.fn(),
    stop: vi.fn(),
  },
}))

// Mock pod data
const mockVM: PodVM = {
  name: 'vm-1',
  status: 'running',
  ipAddress: '10.0.0.1',
  platformId: 'pve-100',
  currentSnapshot: 'initial',
}

const mockPod: Pod = {
  id: 'pod-1',
  labTemplate: 'linux-basics',
  platform: 'proxmox',
  owner: 'test@example.com',
  status: 'running',
  vms: [mockVM],
  createdAt: '2024-01-01T00:00:00Z',
}

const mockPod2: Pod = {
  id: 'pod-2',
  labTemplate: 'network-lab',
  platform: 'proxmox',
  owner: 'test@example.com',
  status: 'provisioning',
  vms: [],
  createdAt: '2024-01-01T01:00:00Z',
}

describe('Pods Store', () => {
  let store: ReturnType<typeof usePodsStore>

  beforeEach(() => {
    setActivePinia(createPinia())
    store = usePodsStore()
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have empty initial state', () => {
      expect(store.pods).toEqual([])
      expect(store.currentPod).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.lastUpdate).toBeNull()
      expect(store.actionLoading).toBeNull()
    })
  })

  describe('computed: hasProvisioningPods', () => {
    it('should return false when no pods are provisioning', () => {
      store.pods = [mockPod]
      expect(store.hasProvisioningPods).toBe(false)
    })

    it('should return true when a pod is provisioning', () => {
      store.pods = [mockPod, mockPod2]
      expect(store.hasProvisioningPods).toBe(true)
    })
  })

  describe('fetchPods()', () => {
    it('should fetch all pods successfully', async () => {
      vi.mocked(podsApi.list).mockResolvedValue([mockPod, mockPod2])

      await store.fetchPods()

      expect(store.pods).toHaveLength(2)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should fetch pods filtered by owner', async () => {
      vi.mocked(podsApi.list).mockResolvedValue([mockPod])

      await store.fetchPods('test@example.com')

      expect(podsApi.list).toHaveBeenCalledWith('test@example.com')
      expect(store.pods).toHaveLength(1)
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise: (value: Pod[]) => void
      vi.mocked(podsApi.list).mockImplementation(
        () => new Promise(resolve => { resolvePromise = resolve })
      )

      const fetchPromise = store.fetchPods()
      expect(store.loading).toBe(true)

      resolvePromise!([mockPod])
      await fetchPromise

      expect(store.loading).toBe(false)
    })

    it('should handle fetch error', async () => {
      vi.mocked(podsApi.list).mockRejectedValue(new Error('Network error'))

      await store.fetchPods()

      expect(store.error?.message).toBe('Network error')
      expect(store.loading).toBe(false)
    })
  })

  describe('fetchPod()', () => {
    it('should fetch a single pod', async () => {
      vi.mocked(podsApi.get).mockResolvedValue(mockPod)

      const result = await store.fetchPod('pod-1')

      expect(result).toEqual(mockPod)
      expect(store.currentPod).toEqual(mockPod)
    })

    it('should update pod in list if exists', async () => {
      const updatedPod = { ...mockPod, status: 'stopped' as const }
      store.pods = [mockPod, mockPod2]
      vi.mocked(podsApi.get).mockResolvedValue(updatedPod)

      await store.fetchPod('pod-1')

      expect(store.pods[0]!.status).toBe('stopped')
    })

    it('should handle fetch error', async () => {
      vi.mocked(podsApi.get).mockRejectedValue(new Error('Pod not found'))

      const result = await store.fetchPod('pod-999')

      expect(result).toBeNull()
      expect(store.error?.message).toBe('Pod not found')
    })
  })

  describe('createPod()', () => {
    it('should create a new pod', async () => {
      const newPod: Pod = { ...mockPod, id: 'pod-new', status: 'provisioning' as const }
      vi.mocked(podsApi.create).mockResolvedValue(newPod)

      const result = await store.createPod('linux-basics', 'test@example.com')

      expect(result).toEqual(newPod)
      expect(store.pods).toContainEqual(newPod)
      expect(podsApi.create).toHaveBeenCalledWith('linux-basics', 'test@example.com')
    })

    it('should throw on create error', async () => {
      vi.mocked(podsApi.create).mockRejectedValue(new Error('Create failed'))

      await expect(store.createPod('invalid', 'user')).rejects.toThrow('Create failed')
      expect(store.error?.message).toBe('Create failed')
    })
  })

  describe('destroyPod()', () => {
    it('should destroy a pod and remove from list', async () => {
      store.pods = [mockPod, mockPod2]
      vi.mocked(podsApi.destroy).mockResolvedValue(undefined)

      await store.destroyPod('pod-1')

      expect(store.pods).toHaveLength(1)
      expect(store.pods[0]!.id).toBe('pod-2')
      expect(store.actionLoading).toBeNull()
    })

    it('should set actionLoading during destroy', async () => {
      let resolvePromise: () => void
      store.pods = [mockPod]
      vi.mocked(podsApi.destroy).mockImplementation(
        () => new Promise(resolve => { resolvePromise = resolve })
      )

      const destroyPromise = store.destroyPod('pod-1')
      expect(store.actionLoading).toBe('pod-1')

      resolvePromise!()
      await destroyPromise

      expect(store.actionLoading).toBeNull()
    })

    it('should clear currentPod if it was destroyed', async () => {
      store.pods = [mockPod]
      store.currentPod = mockPod
      vi.mocked(podsApi.destroy).mockResolvedValue(undefined)

      await store.destroyPod('pod-1')

      expect(store.currentPod).toBeNull()
    })

    it('should throw on destroy error', async () => {
      store.pods = [mockPod]
      vi.mocked(podsApi.destroy).mockRejectedValue(new Error('Destroy failed'))

      await expect(store.destroyPod('pod-1')).rejects.toThrow('Destroy failed')
      expect(store.error?.message).toBe('Destroy failed')
    })
  })

  describe('startPod()', () => {
    it('should start a pod and update list', async () => {
      const startedPod = { ...mockPod, status: 'running' as const }
      store.pods = [{ ...mockPod, status: 'stopped' as const }]
      vi.mocked(podsApi.start).mockResolvedValue(startedPod)

      const result = await store.startPod('pod-1')

      expect(result.status).toBe('running')
      expect(store.pods[0]!.status).toBe('running')
      expect(store.actionLoading).toBeNull()
    })

    it('should set actionLoading during start', async () => {
      let resolvePromise: (value: Pod) => void
      const startedPod = { ...mockPod, status: 'running' as const }
      store.pods = [{ ...mockPod, status: 'stopped' as const }]
      vi.mocked(podsApi.start).mockImplementation(
        () => new Promise(resolve => { resolvePromise = resolve })
      )

      const startPromise = store.startPod('pod-1')
      expect(store.actionLoading).toBe('pod-1')

      resolvePromise!(startedPod)
      await startPromise

      expect(store.actionLoading).toBeNull()
    })

    it('should update currentPod if it matches', async () => {
      const startedPod = { ...mockPod, status: 'running' as const }
      store.pods = [{ ...mockPod, status: 'stopped' as const }]
      store.currentPod = { ...mockPod, status: 'stopped' as const }
      vi.mocked(podsApi.start).mockResolvedValue(startedPod)

      await store.startPod('pod-1')

      expect(store.currentPod?.status).toBe('running')
    })

    it('should throw on start error', async () => {
      store.pods = [mockPod]
      vi.mocked(podsApi.start).mockRejectedValue(new Error('Start failed'))

      await expect(store.startPod('pod-1')).rejects.toThrow('Start failed')
    })
  })

  describe('stopPod()', () => {
    it('should stop a pod and update list', async () => {
      const stoppedPod = { ...mockPod, status: 'stopped' as const }
      store.pods = [mockPod]
      vi.mocked(podsApi.stop).mockResolvedValue(stoppedPod)

      const result = await store.stopPod('pod-1')

      expect(result.status).toBe('stopped')
      expect(store.pods[0]!.status).toBe('stopped')
      expect(store.actionLoading).toBeNull()
    })

    it('should set actionLoading during stop', async () => {
      let resolvePromise: (value: Pod) => void
      const stoppedPod = { ...mockPod, status: 'stopped' as const }
      store.pods = [mockPod]
      vi.mocked(podsApi.stop).mockImplementation(
        () => new Promise(resolve => { resolvePromise = resolve })
      )

      const stopPromise = store.stopPod('pod-1')
      expect(store.actionLoading).toBe('pod-1')

      resolvePromise!(stoppedPod)
      await stopPromise

      expect(store.actionLoading).toBeNull()
    })

    it('should update currentPod if it matches', async () => {
      const stoppedPod = { ...mockPod, status: 'stopped' as const }
      store.pods = [mockPod]
      store.currentPod = mockPod
      vi.mocked(podsApi.stop).mockResolvedValue(stoppedPod)

      await store.stopPod('pod-1')

      expect(store.currentPod?.status).toBe('stopped')
    })

    it('should throw on stop error', async () => {
      store.pods = [mockPod]
      vi.mocked(podsApi.stop).mockRejectedValue(new Error('Stop failed'))

      await expect(store.stopPod('pod-1')).rejects.toThrow('Stop failed')
    })
  })

  describe('handlePodStatusUpdate()', () => {
    it('should update pod status from WebSocket event', () => {
      store.pods = [mockPod, mockPod2]
      const update: PodStatusUpdate = {
        podId: 'pod-1',
        status: 'stopped',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handlePodStatusUpdate(update)

      expect(store.pods[0]!.status).toBe('stopped')
      expect(store.lastUpdate).toEqual(new Date('2024-01-01T12:00:00Z'))
    })

    it('should update pod VMs if provided', () => {
      store.pods = [mockPod]
      const newVMs: PodVM[] = [{ ...mockVM, status: 'stopped' as const }]
      const update: PodStatusUpdate = {
        podId: 'pod-1',
        status: 'stopped',
        vms: newVMs,
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handlePodStatusUpdate(update)

      expect(store.pods[0]!.vms).toEqual(newVMs)
    })

    it('should update currentPod if it matches', () => {
      store.pods = [mockPod]
      store.currentPod = { ...mockPod }
      const update: PodStatusUpdate = {
        podId: 'pod-1',
        status: 'error',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handlePodStatusUpdate(update)

      expect(store.currentPod?.status).toBe('error')
    })

    it('should not crash if pod not found', () => {
      store.pods = [mockPod]
      const update: PodStatusUpdate = {
        podId: 'pod-unknown',
        status: 'stopped',
        timestamp: '2024-01-01T12:00:00Z',
      }

      expect(() => store.handlePodStatusUpdate(update)).not.toThrow()
    })
  })

  describe('handleVMStatusUpdate()', () => {
    it('should update VM status from WebSocket event', () => {
      store.pods = [mockPod]
      const update: VMStatusUpdate = {
        podId: 'pod-1',
        vmName: 'vm-1',
        status: 'stopped',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handleVMStatusUpdate(update)

      expect(store.pods[0]!.vms[0]!.status).toBe('stopped')
      expect(store.lastUpdate).toEqual(new Date('2024-01-01T12:00:00Z'))
    })

    it('should update VM IP address', () => {
      store.pods = [mockPod]
      const update: VMStatusUpdate = {
        podId: 'pod-1',
        vmName: 'vm-1',
        status: 'running',
        ipAddress: '10.0.0.100',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handleVMStatusUpdate(update)

      expect(store.pods[0]!.vms[0]!.ipAddress).toBe('10.0.0.100')
    })

    it('should update VM snapshot', () => {
      store.pods = [mockPod]
      const update: VMStatusUpdate = {
        podId: 'pod-1',
        vmName: 'vm-1',
        status: 'running',
        currentSnapshot: 'checkpoint-1',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handleVMStatusUpdate(update)

      expect(store.pods[0]!.vms[0]!.currentSnapshot).toBe('checkpoint-1')
    })

    it('should update VM in currentPod if matches', () => {
      store.pods = [mockPod]
      store.currentPod = { ...mockPod, vms: [{ ...mockVM }] }
      const update: VMStatusUpdate = {
        podId: 'pod-1',
        vmName: 'vm-1',
        status: 'stopped',
        timestamp: '2024-01-01T12:00:00Z',
      }

      store.handleVMStatusUpdate(update)

      expect(store.currentPod?.vms[0]!.status).toBe('stopped')
    })

    it('should not crash if VM not found', () => {
      store.pods = [mockPod]
      const update: VMStatusUpdate = {
        podId: 'pod-1',
        vmName: 'vm-unknown',
        status: 'stopped',
        timestamp: '2024-01-01T12:00:00Z',
      }

      expect(() => store.handleVMStatusUpdate(update)).not.toThrow()
    })
  })

  describe('reset()', () => {
    it('should reset all state to initial values', () => {
      store.pods = [mockPod, mockPod2]
      store.currentPod = mockPod
      store.loading = true
      store.error = testError
      store.lastUpdate = new Date()
      store.actionLoading = 'pod-1'

      store.reset()

      expect(store.pods).toEqual([])
      expect(store.currentPod).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.lastUpdate).toBeNull()
      expect(store.actionLoading).toBeNull()
    })
  })
})
