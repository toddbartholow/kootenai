/**
 * Module Unlock Composable
 *
 * Provides logic for determining module unlock status based on unlock types:
 * - sequential: Must complete the previous module
 * - all_previous: Must complete all previous modules
 * - manual: Requires instructor/admin approval
 * - always: Always unlocked (no prerequisites)
 */
import { computed, ref, type Ref, type ComputedRef } from 'vue'
import { enrollmentsApi, type PathwayModule, type ModuleProgress, type UnlockType, type ModuleProgressStatus, type UnlockRequirements } from '@/api'

export interface ModuleUnlockState {
  /** Whether the module is currently locked */
  isLocked: boolean
  /** Whether the module can be unlocked (requirements met) */
  canUnlock: boolean
  /** Human-readable message about unlock status */
  message: string
  /** IDs of prerequisite modules that need to be completed */
  prerequisiteModuleIds: string[]
  /** Names of prerequisite modules that need to be completed */
  prerequisiteModuleNames: string[]
  /** Percentage of prerequisites completed */
  prerequisiteProgress: number
}

export interface UseModuleUnlockOptions {
  /** All modules in the pathway (sorted by displayOrder) */
  modules: Ref<PathwayModule[]>
  /** Module progress map (moduleId -> ModuleProgress) */
  moduleProgressMap: Ref<Map<string, ModuleProgress>> | ComputedRef<Map<string, ModuleProgress>>
  /** Enrollment ID for API calls */
  enrollmentId?: Ref<string | null>
}

/**
 * Composable for managing module unlock logic
 */
export function useModuleUnlock(options: UseModuleUnlockOptions) {
  const { modules, moduleProgressMap, enrollmentId } = options

  const loadingUnlock = ref<string | null>(null)
  const unlockError = ref<string | null>(null)

  /**
   * Gets the current status of a module
   */
  function getModuleStatus(moduleId: string): ModuleProgressStatus {
    const map = moduleProgressMap.value
    if (!map) return 'locked'
    const progress = map.get(moduleId)
    return progress?.status ?? 'locked'
  }

  /**
   * Gets sorted modules by display order
   */
  const sortedModules = computed(() =>
    [...modules.value].sort((a, b) => a.displayOrder - b.displayOrder)
  )

  /**
   * Gets the index of a module in the sorted list
   */
  function getModuleIndex(moduleId: string): number {
    return sortedModules.value.findIndex(m => m.id === moduleId)
  }

  /**
   * Gets all modules before a given module (by display order)
   */
  function getPreviousModules(moduleId: string): PathwayModule[] {
    const index = getModuleIndex(moduleId)
    if (index <= 0) return []
    return sortedModules.value.slice(0, index)
  }

  /**
   * Gets the immediately previous module
   */
  function getImmediatePreviousModule(moduleId: string): PathwayModule | null {
    const index = getModuleIndex(moduleId)
    if (index <= 0) return null
    return sortedModules.value[index - 1] ?? null
  }

  /**
   * Checks if a module is completed
   */
  function isModuleCompleted(moduleId: string): boolean {
    return getModuleStatus(moduleId) === 'completed'
  }

  /**
   * Determines unlock state for a module based on its unlock type
   */
  function getUnlockState(moduleId: string): ModuleUnlockState {
    const module = modules.value.find(m => m.id === moduleId)
    if (!module) {
      return {
        isLocked: true,
        canUnlock: false,
        message: 'Module not found',
        prerequisiteModuleIds: [],
        prerequisiteModuleNames: [],
        prerequisiteProgress: 0,
      }
    }

    const currentStatus = getModuleStatus(moduleId)

    // If already unlocked or completed, not locked
    if (currentStatus !== 'locked') {
      return {
        isLocked: false,
        canUnlock: true,
        message: currentStatus === 'completed' ? 'Completed' : 'Available',
        prerequisiteModuleIds: [],
        prerequisiteModuleNames: [],
        prerequisiteProgress: 100,
      }
    }

    const unlockType = module.unlockType as UnlockType

    switch (unlockType) {
      case 'always':
        return getAlwaysUnlockState()

      case 'sequential':
        return getSequentialUnlockState(moduleId)

      case 'all_previous':
        return getAllPreviousUnlockState(moduleId)

      case 'manual':
        return getManualUnlockState()

      default:
        return {
          isLocked: true,
          canUnlock: false,
          message: 'Unknown unlock type',
          prerequisiteModuleIds: [],
          prerequisiteModuleNames: [],
          prerequisiteProgress: 0,
        }
    }
  }

  /**
   * Always unlock - module is always available
   */
  function getAlwaysUnlockState(): ModuleUnlockState {
    return {
      isLocked: false,
      canUnlock: true,
      message: 'Available',
      prerequisiteModuleIds: [],
      prerequisiteModuleNames: [],
      prerequisiteProgress: 100,
    }
  }

  /**
   * Sequential unlock - must complete the immediately previous module
   */
  function getSequentialUnlockState(moduleId: string): ModuleUnlockState {
    const previousModule = getImmediatePreviousModule(moduleId)

    // First module is always available
    if (!previousModule) {
      return {
        isLocked: false,
        canUnlock: true,
        message: 'First module - available',
        prerequisiteModuleIds: [],
        prerequisiteModuleNames: [],
        prerequisiteProgress: 100,
      }
    }

    const previousCompleted = isModuleCompleted(previousModule.id)

    return {
      isLocked: !previousCompleted,
      canUnlock: previousCompleted,
      message: previousCompleted
        ? 'Available'
        : `Complete "${previousModule.name}" to unlock`,
      prerequisiteModuleIds: previousCompleted ? [] : [previousModule.id],
      prerequisiteModuleNames: previousCompleted ? [] : [previousModule.name],
      prerequisiteProgress: previousCompleted ? 100 : 0,
    }
  }

  /**
   * All previous unlock - must complete all modules before this one
   */
  function getAllPreviousUnlockState(moduleId: string): ModuleUnlockState {
    const previousModules = getPreviousModules(moduleId)

    // First module is always available
    if (previousModules.length === 0) {
      return {
        isLocked: false,
        canUnlock: true,
        message: 'First module - available',
        prerequisiteModuleIds: [],
        prerequisiteModuleNames: [],
        prerequisiteProgress: 100,
      }
    }

    const completedModules = previousModules.filter(m => isModuleCompleted(m.id))
    const incompleteModules = previousModules.filter(m => !isModuleCompleted(m.id))
    const allCompleted = incompleteModules.length === 0
    const progress = Math.round((completedModules.length / previousModules.length) * 100)

    if (allCompleted) {
      return {
        isLocked: false,
        canUnlock: true,
        message: 'Available',
        prerequisiteModuleIds: [],
        prerequisiteModuleNames: [],
        prerequisiteProgress: 100,
      }
    }

    const remaining = incompleteModules.length
    return {
      isLocked: true,
      canUnlock: false,
      message: `Complete ${remaining} more module${remaining > 1 ? 's' : ''} to unlock`,
      prerequisiteModuleIds: incompleteModules.map(m => m.id),
      prerequisiteModuleNames: incompleteModules.map(m => m.name),
      prerequisiteProgress: progress,
    }
  }

  /**
   * Manual unlock - requires instructor approval
   */
  function getManualUnlockState(): ModuleUnlockState {
    return {
      isLocked: true,
      canUnlock: false,
      message: 'Requires instructor approval to unlock',
      prerequisiteModuleIds: [],
      prerequisiteModuleNames: [],
      prerequisiteProgress: 0,
    }
  }

  /**
   * Fetches detailed unlock requirements from the API
   */
  async function fetchUnlockRequirements(moduleId: string): Promise<UnlockRequirements | null> {
    if (!enrollmentId?.value) return null

    try {
      return await enrollmentsApi.getUnlockRequirements(enrollmentId.value, moduleId)
    } catch (e) {
      console.error('Failed to fetch unlock requirements:', e)
      return null
    }
  }

  /**
   * Requests manual unlock of a module (for 'manual' unlock type)
   */
  async function requestManualUnlock(moduleId: string): Promise<boolean> {
    if (!enrollmentId?.value) {
      unlockError.value = 'Not enrolled'
      return false
    }

    loadingUnlock.value = moduleId
    unlockError.value = null

    try {
      await enrollmentsApi.manuallyUnlockModule(enrollmentId.value, moduleId)
      return true
    } catch (e) {
      unlockError.value = e instanceof Error ? e.message : 'Failed to unlock module'
      return false
    } finally {
      loadingUnlock.value = null
    }
  }

  /**
   * Checks if a module should be unlocked after completing another module
   * Returns the list of modules that should be unlocked
   */
  function getModulesToUnlockAfterCompletion(completedModuleId: string): PathwayModule[] {
    const toUnlock: PathwayModule[] = []
    const _completedIndex = getModuleIndex(completedModuleId)

    for (const module of sortedModules.value) {
      const currentStatus = getModuleStatus(module.id)
      if (currentStatus !== 'locked') continue

      const _unlockState = getUnlockState(module.id)

      // Recheck after the completion - we need to temporarily mark the module as completed
      // For sequential: check if the completed module is the immediate previous
      if (module.unlockType === 'sequential') {
        const prevModule = getImmediatePreviousModule(module.id)
        if (prevModule?.id === completedModuleId) {
          toUnlock.push(module)
        }
      }

      // For all_previous: check if all previous are now completed
      if (module.unlockType === 'all_previous') {
        const previousModules = getPreviousModules(module.id)
        // Check if all are completed (including the one just completed)
        const allCompleted = previousModules.every(
          m => m.id === completedModuleId || isModuleCompleted(m.id)
        )
        if (allCompleted) {
          toUnlock.push(module)
        }
      }
    }

    return toUnlock
  }

  /**
   * Gets a formatted message for unlock requirements
   */
  function getUnlockMessage(moduleId: string): string {
    const state = getUnlockState(moduleId)
    return state.message
  }

  /**
   * Checks if any modules can be unlocked
   */
  const hasUnlockableModules = computed(() =>
    sortedModules.value.some(m => {
      const state = getUnlockState(m.id)
      return state.isLocked && state.canUnlock
    })
  )

  return {
    // State
    loadingUnlock,
    unlockError,

    // Computed
    sortedModules,
    hasUnlockableModules,

    // Functions
    getModuleStatus,
    getModuleIndex,
    getPreviousModules,
    getImmediatePreviousModule,
    isModuleCompleted,
    getUnlockState,
    getUnlockMessage,
    getModulesToUnlockAfterCompletion,
    fetchUnlockRequirements,
    requestManualUnlock,
  }
}
