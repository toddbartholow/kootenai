import { ref } from 'vue'
import api from '@/api/config'
import type { ConsoleTicket, PodVM } from '@/api'

export interface ConsoleState {
  vm: { name: string; platformId: string; node?: string } | null
  ticket: ConsoleTicket | null
  loading: boolean
  error: string | null
  visible: boolean
}

export interface UseVMConsoleOptions {
  onError?: (error: string) => void
}

/**
 * Composable for managing VM console connections
 */
export function useVMConsole(options: UseVMConsoleOptions = {}) {
  const { onError } = options

  const visible = ref(false)
  const vm = ref<{ name: string; platformId: string; node?: string } | null>(null)
  const ticket = ref<ConsoleTicket | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  /**
   * Opens a VNC console for a VM.
   *
   * Accepts either a full PodVM (session-view path) or a lightweight
   * `{ name, platformId, node? }` descriptor (pod-detail-view path). The
   * previous pod-detail code path used raw `fetch()` with no Authorization
   * header — after a token rotation mid-session it would 401 while the
   * composable-based session-view path succeeded. Consolidating here ensures
   * both call sites go through the same auth-aware axios instance.
   */
  async function openConsole(
    targetVM: PodVM | { name: string; platformId: string; node?: string },
  ): Promise<void> {
    const node = 'node' in targetVM ? targetVM.node : undefined
    vm.value = { name: targetVM.name, platformId: targetVM.platformId, ...(node ? { node } : {}) }
    loading.value = true
    error.value = null
    visible.value = true

    try {
      const vmid = parseInt(targetVM.platformId, 10)
      const params: Record<string, string> = { type: 'vnc' }
      if (node) {
        params['node'] = node
      }
      const response = await api.get<ConsoleTicket>(`/proxmox/vms/${vmid}/console`, { params })

      ticket.value = response.data
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to get console access'
      error.value = errorMessage
      console.error('Failed to get console ticket:', err)

      if (onError) {
        onError(errorMessage)
      }
    } finally {
      loading.value = false
    }
  }

  /**
   * Closes the console connection
   */
  function closeConsole(): void {
    visible.value = false
    vm.value = null
    ticket.value = null
    error.value = null
  }

  /**
   * Returns the current console state
   */
  function getState(): ConsoleState {
    return {
      vm: vm.value,
      ticket: ticket.value,
      loading: loading.value,
      error: error.value,
      visible: visible.value,
    }
  }

  /**
   * Check if the console is VNC type
   */
  function isVNCConsole(): boolean {
    return ticket.value?.type === 'vnc'
  }

  /**
   * Get the VMID as a number for the VNC component
   */
  function getVMID(): number {
    return vm.value ? parseInt(vm.value.platformId, 10) : 0
  }

  return {
    // Reactive state
    visible,
    vm,
    ticket,
    loading,
    error,

    // Methods
    openConsole,
    closeConsole,
    getState,
    isVNCConsole,
    getVMID,
  }
}
