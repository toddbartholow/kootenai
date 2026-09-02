import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { podsApi, type Pod, type PodStatus, type PodVM } from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'

// Pod status update from WebSocket
export interface PodStatusUpdate {
  podId: string
  status: PodStatus
  vms?: PodVM[]
  timestamp: string
}

// VM status update from WebSocket
export interface VMStatusUpdate {
  podId: string
  vmName: string
  status: string
  ipAddress?: string
  currentSnapshot?: string
  timestamp: string
}

export const usePodsStore = defineStore('pods', () => {
  const pods = ref<Pod[]>([])
  const currentPod = ref<Pod | null>(null)
  const loading = ref(false)
  const error = ref<ErrorState | null>(null)
  const lastUpdate = ref<Date | null>(null)

  // Track which pod has an action in progress (start/stop/reset/destroy)
  const actionLoading = ref<string | null>(null)

  // Computed for checking if any pod is provisioning
  const hasProvisioningPods = computed(() =>
    pods.value.some(p => p.status === 'provisioning')
  )

  // Computed for active pods (excluding destroyed).
  // Previously this also filtered on `!p.destroyedAt`, a field the backend
  // does not send — pure dead code that the TS second-pass review caught.
  const activePods = computed(() =>
    pods.value.filter(p => p.status !== 'destroyed')
  )

  async function fetchPods(owner?: string) {
    loading.value = true
    error.value = null
    try {
      pods.value = await podsApi.list(owner)
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch pods')
    } finally {
      loading.value = false
    }
  }

  async function fetchPod(id: string) {
    loading.value = true
    error.value = null
    try {
      currentPod.value = await podsApi.get(id)
      // Also update in the list if exists
      const index = pods.value.findIndex(p => p.id === id)
      if (index >= 0) {
        pods.value[index] = currentPod.value
      }
      return currentPod.value
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch pod')
      return null
    } finally {
      loading.value = false
    }
  }

  async function createPod(labTemplate: string, owner: string) {
    loading.value = true
    error.value = null
    try {
      const newPod = await podsApi.create(labTemplate, owner)
      pods.value.push(newPod)
      return newPod
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to create pod')
      throw e
    } finally {
      loading.value = false
    }
  }

  async function destroyPod(id: string) {
    actionLoading.value = id
    error.value = null
    try {
      await podsApi.destroy(id)
      pods.value = pods.value.filter(p => p.id !== id)
      if (currentPod.value?.id === id) {
        currentPod.value = null
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to destroy pod')
      throw e
    } finally {
      actionLoading.value = null
    }
  }

  async function startPod(id: string) {
    actionLoading.value = id
    error.value = null
    try {
      const updatedPod = await podsApi.start(id)
      // Update in pods list
      const index = pods.value.findIndex(p => p.id === id)
      if (index >= 0) {
        pods.value[index] = updatedPod
      }
      // Update current pod if it matches
      if (currentPod.value?.id === id) {
        currentPod.value = updatedPod
      }
      return updatedPod
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to start pod')
      throw e
    } finally {
      actionLoading.value = null
    }
  }

  async function stopPod(id: string) {
    actionLoading.value = id
    error.value = null
    try {
      const updatedPod = await podsApi.stop(id)
      // Update in pods list
      const index = pods.value.findIndex(p => p.id === id)
      if (index >= 0) {
        pods.value[index] = updatedPod
      }
      // Update current pod if it matches
      if (currentPod.value?.id === id) {
        currentPod.value = updatedPod
      }
      return updatedPod
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to stop pod')
      throw e
    } finally {
      actionLoading.value = null
    }
  }

  async function resetPod(id: string, snapshotName = 'initial') {
    actionLoading.value = id
    error.value = null
    try {
      const pod = pods.value.find(p => p.id === id)
      if (!pod?.vms?.length) {
        throw new Error('No VMs to reset')
      }
      // Reset all VMs to the specified snapshot
      for (const vm of pod.vms) {
        await podsApi.resetVM(id, vm.name, snapshotName)
      }
      // Refresh the pod data
      await fetchPod(id)
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to reset pod')
      throw e
    } finally {
      actionLoading.value = null
    }
  }

  async function removePod(id: string) {
    actionLoading.value = id
    error.value = null
    try {
      // Try to destroy, but even if it fails, remove from list
      try {
        await podsApi.destroy(id)
      } catch {
        // Ignore errors - pod may already be gone
      }
      pods.value = pods.value.filter(p => p.id !== id)
      if (currentPod.value?.id === id) {
        currentPod.value = null
      }
    } finally {
      actionLoading.value = null
    }
  }

  /**
   * Handle pod status updates from WebSocket
   */
  function handlePodStatusUpdate(update: PodStatusUpdate): void {
    lastUpdate.value = new Date(update.timestamp)

    // Update in pods list
    const index = pods.value.findIndex(p => p.id === update.podId)
    if (index >= 0) {
      const pod = pods.value[index]
      if (pod) {
        pod.status = update.status
        if (update.vms) {
          pod.vms = update.vms
        }
      }
    }

    // Update current pod if matches
    if (currentPod.value && currentPod.value.id === update.podId) {
      currentPod.value.status = update.status
      if (update.vms) {
        currentPod.value.vms = update.vms
      }
    }
  }

  /**
   * Handle VM status updates from WebSocket
   */
  function handleVMStatusUpdate(update: VMStatusUpdate): void {
    lastUpdate.value = new Date(update.timestamp)

    // Helper to update VM in a pod
    const updateVMInPod = (pod: Pod) => {
      if (!pod.vms) return
      const vmIndex = pod.vms.findIndex(vm => vm.name === update.vmName)
      if (vmIndex >= 0) {
        const vm = pod.vms[vmIndex]
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

    // Update in pods list
    const index = pods.value.findIndex(p => p.id === update.podId)
    if (index >= 0) {
      const pod = pods.value[index]
      if (pod) {
        updateVMInPod(pod)
      }
    }

    // Update current pod if matches
    if (currentPod.value && currentPod.value.id === update.podId) {
      updateVMInPod(currentPod.value)
    }
  }

  /**
   * Reset the store state
   */
  function reset(): void {
    pods.value = []
    currentPod.value = null
    loading.value = false
    error.value = null
    lastUpdate.value = null
    actionLoading.value = null
  }

  return {
    // State
    pods,
    currentPod,
    loading,
    error,
    lastUpdate,
    actionLoading,
    // Computed
    hasProvisioningPods,
    activePods,
    // Actions
    fetchPods,
    fetchPod,
    createPod,
    destroyPod,
    startPod,
    stopPod,
    resetPod,
    removePod,
    handlePodStatusUpdate,
    handleVMStatusUpdate,
    reset,
  }
})
