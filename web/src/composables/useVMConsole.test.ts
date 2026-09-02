import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useVMConsole } from './useVMConsole'
import type { PodVM } from '@/api'

// Mock the api module
vi.mock('@/api/config', () => ({
  default: {
    get: vi.fn(),
  },
}))

import api from '@/api/config'
const mockGet = vi.mocked(api.get)

describe('useVMConsole', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  const mockVM: PodVM = {
    name: 'webserver-01',
    platformId: '100',
    status: 'running',
  }

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const { visible, vm, ticket, loading, error } = useVMConsole()

      expect(visible.value).toBe(false)
      expect(vm.value).toBeNull()
      expect(ticket.value).toBeNull()
      expect(loading.value).toBe(false)
      expect(error.value).toBeNull()
    })
  })

  describe('openConsole()', () => {
    it('should open console successfully', async () => {
      const mockTicket = {
        type: 'vnc',
        port: 5900,
        ticket: 'abc123',
        host: 'pve.local',
      }

      mockGet.mockResolvedValueOnce({ data: mockTicket } as any)

      const { openConsole, visible, vm, ticket, loading, error } = useVMConsole()

      const openPromise = openConsole(mockVM)

      // Loading should be true while fetching
      expect(loading.value).toBe(true)
      expect(visible.value).toBe(true)

      await openPromise

      expect(loading.value).toBe(false)
      expect(vm.value).toEqual({ name: 'webserver-01', platformId: '100' })
      expect(ticket.value).toEqual(mockTicket)
      expect(error.value).toBeNull()
      expect(mockGet).toHaveBeenCalledWith('/proxmox/vms/100/console', {
        params: { type: 'vnc' },
      })
    })

    it('should handle fetch error', async () => {
      mockGet.mockRejectedValueOnce(new Error('Request failed with status code 404'))

      const onError = vi.fn()
      const { openConsole, error } = useVMConsole({ onError })

      await openConsole(mockVM)

      expect(error.value).toBe('Request failed with status code 404')
      expect(onError).toHaveBeenCalledWith('Request failed with status code 404')
    })

    it('should handle network error', async () => {
      mockGet.mockRejectedValueOnce(new Error('Network failure'))

      const onError = vi.fn()
      const { openConsole, error, loading } = useVMConsole({ onError })

      await openConsole(mockVM)

      expect(error.value).toBe('Network failure')
      expect(loading.value).toBe(false)
      expect(onError).toHaveBeenCalledWith('Network failure')
    })

    it('should handle non-Error throw', async () => {
      mockGet.mockRejectedValueOnce('string error')

      const { openConsole, error } = useVMConsole()

      await openConsole(mockVM)

      expect(error.value).toBe('Failed to get console access')
    })

    it('should set visible immediately when opening', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc' } } as any)

      const { openConsole, visible } = useVMConsole()

      const promise = openConsole(mockVM)
      expect(visible.value).toBe(true)

      await promise
    })
  })

  describe('closeConsole()', () => {
    it('should reset all state when closing', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc', ticket: 'abc' } } as any)

      const { openConsole, closeConsole, visible, vm, ticket, error } = useVMConsole()

      await openConsole(mockVM)
      expect(visible.value).toBe(true)

      closeConsole()

      expect(visible.value).toBe(false)
      expect(vm.value).toBeNull()
      expect(ticket.value).toBeNull()
      expect(error.value).toBeNull()
    })
  })

  describe('getState()', () => {
    it('should return current console state', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc', ticket: 'xyz' } } as any)

      const { openConsole, getState } = useVMConsole()

      await openConsole(mockVM)
      const state = getState()

      expect(state).toEqual({
        vm: { name: 'webserver-01', platformId: '100' },
        ticket: { type: 'vnc', ticket: 'xyz' },
        loading: false,
        error: null,
        visible: true,
      })
    })

    it('should return initial state when nothing opened', () => {
      const { getState } = useVMConsole()
      const state = getState()

      expect(state).toEqual({
        vm: null,
        ticket: null,
        loading: false,
        error: null,
        visible: false,
      })
    })
  })

  describe('isVNCConsole()', () => {
    it('should return true for VNC ticket', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc', ticket: 'abc' } } as any)

      const { openConsole, isVNCConsole } = useVMConsole()

      await openConsole(mockVM)

      expect(isVNCConsole()).toBe(true)
    })

    it('should return false for non-VNC ticket', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'spice', ticket: 'abc' } } as any)

      const { openConsole, isVNCConsole } = useVMConsole()

      await openConsole(mockVM)

      expect(isVNCConsole()).toBe(false)
    })

    it('should return false when no ticket', () => {
      const { isVNCConsole } = useVMConsole()

      expect(isVNCConsole()).toBe(false)
    })
  })

  describe('getVMID()', () => {
    it('should return VMID as number', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc' } } as any)

      const { openConsole, getVMID } = useVMConsole()

      await openConsole(mockVM)

      expect(getVMID()).toBe(100)
    })

    it('should return 0 when no VM selected', () => {
      const { getVMID } = useVMConsole()

      expect(getVMID()).toBe(0)
    })

    it('should handle non-numeric platform IDs', async () => {
      mockGet.mockResolvedValueOnce({ data: { type: 'vnc' } } as any)

      const { openConsole, getVMID } = useVMConsole()

      const vmWithText: PodVM = {
        name: 'test-vm',
        platformId: 'abc',
        status: 'running',
      }

      await openConsole(vmWithText)

      expect(getVMID()).toBe(NaN)
    })
  })

  describe('error handling with callback', () => {
    it('should call onError callback on failure', async () => {
      mockGet.mockRejectedValueOnce(new Error('Connection refused'))

      const onError = vi.fn()
      const { openConsole } = useVMConsole({ onError })

      await openConsole(mockVM)

      expect(onError).toHaveBeenCalledWith('Connection refused')
    })

    it('should work without onError callback', async () => {
      mockGet.mockRejectedValueOnce(new Error('Connection refused'))

      const { openConsole, error } = useVMConsole()

      await openConsole(mockVM)

      expect(error.value).toBe('Connection refused')
    })
  })
})
