import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { AxiosError } from 'axios'
import {
  assessmentApi,
  type AssessmentResult,
  type AssessmentUpdate,
  type AssessmentTemplate,
  type CheckUpdatePayload,
  type DeviceUpdatePayload,
  type CheckResult,
} from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'

/**
 * Pinia store for managing assessment state and real-time updates
 */
export const useAssessmentStore = defineStore('assessment', () => {
  const result = ref<AssessmentResult | null>(null)
  const loading = ref(false)
  const error = ref<ErrorState | null>(null)
  const isRunning = ref(false)

  // Expanded state for tree view (using Set for O(1) lookups)
  const expandedDevices = ref<Set<string>>(new Set())
  const expandedInterfaces = ref<Set<string>>(new Set())

  // Computed properties for easy access to result data
  /** Current assessment percentage (0-100) */
  const percentage = computed(() => result.value?.percentage ?? 0)
  /** Current earned score */
  const score = computed(() => result.value?.score ?? 0)
  /** Maximum possible score */
  const maxScore = computed(() => result.value?.maxScore ?? 0)
  /** Number of checks that passed */
  const passedCount = computed(() => result.value?.passedCount ?? 0)
  /** Total number of checks */
  const itemCount = computed(() => result.value?.itemCount ?? 0)
  /** Whether there's a retryable error */
  const canRetry = computed(() => error.value?.retryable ?? false)
  /** Human-readable error message */
  const errorMessage = computed(() => error.value?.message ?? null)

  /**
   * Fetches existing assessment results for a session
   * @param sessionId - The session ID to fetch results for
   */
  async function fetchAssessment(sessionId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const response = await assessmentApi.get(sessionId)
      result.value = response.data
      // Auto-expand all devices initially
      if (result.value?.devices) {
        const newExpanded = new Set(expandedDevices.value)
        result.value.devices.forEach(d => newExpanded.add(d.name))
        expandedDevices.value = newExpanded
      }
    } catch (e: unknown) {
      // Don't treat 404 as error - just means no results yet
      if (e instanceof AxiosError && e.response?.status === 404) {
        return
      }
      error.value = createErrorState(e, 'Failed to fetch assessment')
    } finally {
      loading.value = false
    }
  }

  /**
   * Runs assessment verification against the provided template
   * @param sessionId - The session ID to run assessment for
   * @param template - Assessment template with devices and checks
   * @throws Re-throws the error after storing it for UI display
   */
  async function runAssessment(sessionId: string, template?: AssessmentTemplate): Promise<void> {
    isRunning.value = true
    error.value = null
    try {
      const response = await assessmentApi.run(sessionId, template)
      result.value = response.data
      // Auto-expand all devices
      if (result.value?.devices) {
        const newExpanded = new Set(expandedDevices.value)
        result.value.devices.forEach(d => newExpanded.add(d.name))
        expandedDevices.value = newExpanded
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to run assessment')
      throw e
    } finally {
      isRunning.value = false
    }
  }

  /**
   * Toggles the expanded state of a device in the tree view
   * Uses immutable Set pattern for reliable reactivity
   * @param deviceName - Name of the device to toggle
   */
  function toggleDevice(deviceName: string): void {
    const newSet = new Set(expandedDevices.value)
    if (newSet.has(deviceName)) {
      newSet.delete(deviceName)
    } else {
      newSet.add(deviceName)
    }
    expandedDevices.value = newSet
  }

  /**
   * Toggles the expanded state of an interface in the tree view
   * @param key - Composite key in format "deviceName:interfaceName"
   */
  function toggleInterface(key: string): void {
    const newSet = new Set(expandedInterfaces.value)
    if (newSet.has(key)) {
      newSet.delete(key)
    } else {
      newSet.add(key)
    }
    expandedInterfaces.value = newSet
  }

  /**
   * Checks if a device is expanded in the tree view
   * @param deviceName - Name of the device to check
   */
  function isDeviceExpanded(deviceName: string): boolean {
    return expandedDevices.value.has(deviceName)
  }

  /**
   * Checks if an interface is expanded in the tree view
   * @param deviceName - Name of the parent device
   * @param interfaceName - Name of the interface
   */
  function isInterfaceExpanded(deviceName: string, interfaceName: string): boolean {
    return expandedInterfaces.value.has(`${deviceName}:${interfaceName}`)
  }

  /** Clears the current error state */
  function clearError(): void {
    error.value = null
  }

  /**
   * Handles real-time updates from WebSocket using discriminated union
   * Automatically routes updates to the correct handler based on type
   * @param update - The assessment update from WebSocket
   */
  function handleUpdate(update: AssessmentUpdate): void {
    if (!result.value) return

    switch (update.type) {
      case 'check_update':
        updateCheck(update)
        break
      case 'device_update':
        updateDevice(update)
        break
      case 'component_update': {
        // Update component scores
        const component = result.value.components.find(c => c.id === update.componentId)
        if (component) {
          component.earnedPoints = update.earnedPoints
          component.passedItems = update.passedItems
          component.percentage = update.percentage
        }
        break
      }
      case 'complete':
        // Full assessment complete, refresh totals
        result.value.score = update.totalScore
        result.value.maxScore = update.maxScore
        result.value.passedCount = update.passedItems
        result.value.itemCount = update.totalItems
        result.value.percentage = update.percentage
        result.value.status = 'completed'
        break
    }
  }

  /** Updates a single check result from WebSocket */
  function updateCheck(update: CheckUpdatePayload): void {
    if (!result.value) return

    const device = result.value.devices.find(d => d.name === update.deviceName)
    if (!device) return

    // Check in device-level checks
    if (device.checks) {
      const check = device.checks.find(c => c.id === update.checkId)
      if (check) {
        applyCheckUpdate(check, update)
        return
      }
    }

    // Check in interface checks
    if (device.interfaces && update.interfaceName) {
      const iface = device.interfaces.find(i => i.name === update.interfaceName)
      if (iface) {
        const check = iface.checks.find(c => c.id === update.checkId)
        if (check) {
          applyCheckUpdate(check, update)
        }
      }
    }
  }

  /** Applies check update fields to a CheckResult */
  function applyCheckUpdate(check: CheckResult, update: CheckUpdatePayload): void {
    check.status = update.status
    if (update.actual !== undefined) check.actual = update.actual
    if (update.expected !== undefined) check.expected = update.expected
    if (update.earnedPoints !== undefined) check.earnedPoints = update.earnedPoints
    if (update.feedback !== undefined) check.feedback = update.feedback
  }

  /** Updates a device result from WebSocket */
  function updateDevice(update: DeviceUpdatePayload): void {
    if (!result.value) return

    const device = result.value.devices.find(d => d.name === update.deviceName)
    if (!device) return

    device.status = update.status
    if (update.earnedPoints !== undefined) device.earnedPoints = update.earnedPoints
    if (update.passedItems !== undefined) device.passedItems = update.passedItems
  }

  /**
   * Resets the store to initial state
   * Call when leaving a session or starting fresh
   */
  function reset(): void {
    result.value = null
    loading.value = false
    error.value = null
    isRunning.value = false
    expandedDevices.value = new Set()
    expandedInterfaces.value = new Set()
  }

  return {
    // State
    result,
    loading,
    error,
    isRunning,
    expandedDevices,
    expandedInterfaces,
    // Computed
    percentage,
    score,
    maxScore,
    passedCount,
    itemCount,
    canRetry,
    errorMessage,
    // Actions
    fetchAssessment,
    runAssessment,
    toggleDevice,
    toggleInterface,
    isDeviceExpanded,
    isInterfaceExpanded,
    handleUpdate,
    clearError,
    reset,
  }
})
