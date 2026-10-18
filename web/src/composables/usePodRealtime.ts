import { ref, watch, onBeforeUnmount, type Ref } from 'vue'
import { useWebSocket } from './useWebSocket'
import { usePodsStore } from '@/stores/pods'
import type { Pod } from '@/api'

export interface UsePodRealtimeReturn {
  /** Live WebSocket connection state. */
  wsConnected: Ref<boolean>
  /** Timestamp of the most recent pod or VM update received over the socket. */
  lastUpdate: Ref<Date | null>
  /** Tear down the socket early (otherwise cleaned up on unmount). */
  disconnect: () => void
}

/**
 * Subscribes to WebSocket pod_status / vm_status events for a single pod
 * and merges them into the supplied pod ref. Previously this lived inline
 * in PodDetailView.vue at ~60 lines of boilerplate; extracting it lets the
 * same logic serve the session view and any future pod-detail variants
 * without copy-paste.
 *
 * Usage:
 *   const pod = ref<Pod | null>(null)
 *   // ...load pod
 *   const { wsConnected, lastUpdate } = usePodRealtime(route.params.podId as string, pod)
 *
 * The composable auto-disconnects on component unmount. The pod ref is
 * mutated in place when matching updates arrive, so the view doesn't need
 * to wire up watchers itself.
 */
export function usePodRealtime(podId: string, pod: Ref<Pod | null>): UsePodRealtimeReturn {
  const podsStore = usePodsStore()
  const wsConnected = ref(false)
  const lastUpdate = ref<Date | null>(null)

  const ws = useWebSocket({
    podId,
    autoConnect: true,
    autoReconnect: true,
    onPodStatusUpdate: (update) => {
      lastUpdate.value = new Date(update.timestamp)
      if (pod.value && update.podId === pod.value.id) {
        pod.value.status = update.status
        if (update.vms) {
          pod.value.vms = update.vms
        }
      }
      podsStore.handlePodStatusUpdate(update)
    },
    onVMStatusUpdate: (update) => {
      lastUpdate.value = new Date(update.timestamp)
      if (pod.value && update.podId === pod.value.id) {
        const vmIndex = pod.value.vms?.findIndex((vm) => vm.name === update.vmName) ?? -1
        if (vmIndex >= 0 && pod.value.vms) {
          const vm = pod.value.vms[vmIndex]
          if (vm) {
            vm.status = update.status
            if (update.ipAddress !== undefined) {
              vm.ipAddress = update.ipAddress
            }
            if (update.currentSnapshot !== undefined) {
              vm.currentSnapshot = update.currentSnapshot
            }
          }
        }
      }
      podsStore.handleVMStatusUpdate(update)
    },
  })

  // Mirror the ws.connected ref into a local so consumers don't have to
  // reach through `ws` (which is an internal implementation detail).
  const stopConnectedWatch = watch(
    () => ws.connected.value,
    (connected) => {
      wsConnected.value = connected
    },
    { immediate: true },
  )

  function disconnect() {
    stopConnectedWatch()
    ws.disconnect()
  }

  onBeforeUnmount(disconnect)

  return {
    wsConnected,
    lastUpdate,
    disconnect,
  }
}
