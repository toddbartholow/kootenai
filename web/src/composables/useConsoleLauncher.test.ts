/**
 * Tests for useConsoleLauncher — the composable that owns console modal
 * state + ticket fetch for the pod-detail view.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import type { ConsoleTicket } from '@/api'

// Stub the axios instance so no real HTTP happens.
const mockApiGet = vi.fn()
vi.mock('@/api/config', () => ({
  default: { get: mockApiGet },
}))

const mockToastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({
  useToast: () => ({ add: mockToastAdd }),
}))

// Capture window.open calls for SPICE download verification.
const mockOpen = vi.fn()

let scope: EffectScope

describe('useConsoleLauncher', () => {
  beforeEach(() => {
    mockApiGet.mockReset()
    mockToastAdd.mockReset()
    mockOpen.mockReset()
    vi.stubGlobal('open', mockOpen)
    scope = effectScope()
  })

  afterEach(() => {
    scope.stop()
    vi.unstubAllGlobals()
  })

  it('openConsole fetches a ticket and sets state for a running VM', async () => {
    const ticket: ConsoleTicket = {
      type: 'vnc',
      host: 'example.com',
      port: 5900,
      ticket: 'abc',
      node: 'pve',
      vmid: '100',
    } as ConsoleTicket
    mockApiGet.mockResolvedValueOnce({ data: ticket })

    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })
    await api!.openConsole({ name: 'web-01', platformId: '100' })

    expect(mockApiGet).toHaveBeenCalledWith(
      '/proxmox/vms/100/console',
      expect.objectContaining({ params: expect.objectContaining({ type: 'vnc' }) }),
    )
    expect(api!.visible.value).toBe(true)
    expect(api!.ticket.value).toEqual(ticket)
    expect(api!.loading.value).toBe(false)
    expect(api!.error.value).toBeNull()
  })

  it('closeConsole resets all state', async () => {
    mockApiGet.mockResolvedValueOnce({
      data: { type: 'vnc', host: 'h', port: 1, ticket: 't', node: 'pve', vmid: '100' },
    })
    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })
    await api!.openConsole({ name: 'web-01', platformId: '100' })
    api!.closeConsole()

    expect(api!.visible.value).toBe(false)
    expect(api!.vm.value).toBeNull()
    expect(api!.ticket.value).toBeNull()
    expect(api!.error.value).toBeNull()
  })

  it('sets error and shows a toast when the ticket fetch fails', async () => {
    mockApiGet.mockRejectedValueOnce(new Error('server said no'))
    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })
    await api!.openConsole({ name: 'web-01', platformId: '100' })

    expect(api!.error.value).toBe('server said no')
    expect(api!.visible.value).toBe(true) // still visible so the user can see the error
    expect(mockToastAdd).toHaveBeenCalledWith(
      expect.objectContaining({ severity: 'error', summary: 'Connection Failed' }),
    )
  })

  it('downloadSpiceFile opens a window to the .vv endpoint', async () => {
    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })
    api!.downloadSpiceFile({ name: 'web-01', platformId: '100', node: 'pve2' })

    expect(mockOpen).toHaveBeenCalledWith(
      '/api/v1/proxmox/vms/100/spice.vv?node=pve2',
      '_blank',
    )
  })

  it('retryConsole re-fetches the ticket for the last-opened VM', async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error('transient'))
      .mockResolvedValueOnce({
        data: { type: 'vnc', host: 'h', port: 1, ticket: 't', node: 'pve', vmid: '100' },
      })

    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })
    await api!.openConsole({ name: 'web-01', platformId: '100' })
    expect(api!.error.value).toBe('transient')

    await api!.retryConsole()
    // Wait one more microtask for the retry's promise to resolve.
    await Promise.resolve()

    expect(mockApiGet).toHaveBeenCalledTimes(2)
    expect(api!.ticket.value).toMatchObject({ type: 'vnc' })
    expect(api!.error.value).toBeNull()
  })

  it('retryConsole is a no-op when no VM has been opened', async () => {
    const { useConsoleLauncher } = await import('./useConsoleLauncher')
    let api: ReturnType<typeof useConsoleLauncher> | null = null
    scope.run(() => {
      api = useConsoleLauncher()
    })

    api!.retryConsole()
    expect(mockApiGet).not.toHaveBeenCalled()
  })
})
