import { ref, type Ref } from 'vue'
import api from '@/api/config'
import type { ConsoleTicket } from '@/api'
import { useToast } from 'primevue/usetoast'
import { loggers } from '@/utils/logger'

export interface ConsoleVMRef {
  name: string
  platformId: string
  node?: string
}

export interface UseConsoleLauncherReturn {
  visible: Ref<boolean>
  vm: Ref<{ name: string; id: string; node: string } | null>
  ticket: Ref<ConsoleTicket | null>
  loading: Ref<boolean>
  error: Ref<string | null>
  openConsole: (vm: ConsoleVMRef) => Promise<void>
  closeConsole: () => void
  downloadSpiceFile: (vm: ConsoleVMRef) => void
  retryConsole: () => void
}

/**
 * Per-caller VM-console launcher. Owns the modal-visibility ref, the
 * ticket fetch (via the authed axios instance so 401-refresh applies),
 * and the supporting loading/error state.
 *
 * Previously this logic lived inline in PodDetailView.vue as ~60 lines
 * of handlers and refs. Extracting makes it reusable by the session
 * view and any future VM-detail page, and gives us a single testable
 * unit for the console ticket flow.
 *
 * NOTE: This is different from the older `useVMConsole` composable,
 * which is tied to the session-view ConsoleTicket + PodVM shape. Both
 * will converge in a follow-up once the session view is also
 * refactored to the launcher shape.
 */
export function useConsoleLauncher(): UseConsoleLauncherReturn {
  const toast = useToast()

  const visible = ref(false)
  const vm = ref<{ name: string; id: string; node: string } | null>(null)
  const ticket = ref<ConsoleTicket | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function openConsole(target: ConsoleVMRef): Promise<void> {
    const vmid = parseInt(target.platformId, 10)
    const node = target.node || 'pve'

    vm.value = { name: target.name, id: target.platformId, node }
    loading.value = true
    error.value = null
    visible.value = true

    try {
      // Use the shared axios instance so the Authorization header and any
      // 401-refresh retry logic apply here too.
      const response = await api.get<ConsoleTicket>(`/proxmox/vms/${vmid}/console`, {
        params: { node, type: 'vnc' },
      })
      ticket.value = response.data
      loggers.console.debug('Got VNC console ticket', {}, ticket.value)
    } catch (err) {
      console.error('Failed to get console ticket:', err)
      error.value = err instanceof Error ? err.message : 'Failed to get console access'
      toast.add({
        severity: 'error',
        summary: 'Connection Failed',
        detail: error.value,
        life: 5000,
      })
    } finally {
      loading.value = false
    }
  }

  function closeConsole(): void {
    visible.value = false
    vm.value = null
    ticket.value = null
    error.value = null
  }

  function downloadSpiceFile(target: ConsoleVMRef): void {
    const node = target.node || 'pve'
    const url = `/api/v1/proxmox/vms/${target.platformId}/spice.vv?node=${node}`
    window.open(url, '_blank')
  }

  function retryConsole(): void {
    const current = vm.value
    if (!current) return
    void openConsole({ name: current.name, platformId: current.id, node: current.node })
  }

  return {
    visible,
    vm,
    ticket,
    loading,
    error,
    openConsole,
    closeConsole,
    downloadSpiceFile,
    retryConsole,
  }
}
