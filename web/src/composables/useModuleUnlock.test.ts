import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref, computed } from 'vue'
import { useModuleUnlock } from './useModuleUnlock'
import { enrollmentsApi, type PathwayModule, type ModuleProgress, type UnlockType } from '@/api'

// Mock the API client
vi.mock('@/api', () => ({
  enrollmentsApi: {
    getUnlockRequirements: vi.fn(),
    manuallyUnlockModule: vi.fn(),
  },
}))

// Helper to create mock modules
function createMockModule(
  id: string,
  name: string,
  displayOrder: number,
  unlockType: UnlockType = 'sequential'
): PathwayModule {
  return {
    id,
    name,
    slug: id,
    displayOrder,
    unlockType,
    pathwayId: 'pathway-1',
    description: `Description for ${name}`,
    isActive: true,
    createdAt: '2024-01-01',
  }
}

// Helper to create mock module progress
function createMockProgress(
  moduleId: string,
  status: 'locked' | 'unlocked' | 'in_progress' | 'completed'
): ModuleProgress {
  return {
    id: `progress-${moduleId}`,
    enrollmentId: 'enroll-1',
    moduleId,
    status,
    completedLabs: status === 'completed' ? 1 : 0,
    totalLabs: 1,
    earnedPoints: status === 'completed' ? 100 : 0,
    maxPoints: 100,
  }
}

describe('useModuleUnlock', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('getModuleStatus', () => {
    it('should return locked for modules without progress', () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1)])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getModuleStatus } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getModuleStatus('mod-1')).toBe('locked')
    })

    it('should return status from progress map', () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1)])
      const progress = createMockProgress('mod-1', 'completed')
      const moduleProgressMap = computed(() => new Map([['mod-1', progress]]))

      const { getModuleStatus } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getModuleStatus('mod-1')).toBe('completed')
    })
  })

  describe('sortedModules', () => {
    it('should sort modules by displayOrder', () => {
      const modules = ref([
        createMockModule('mod-3', 'Module 3', 3),
        createMockModule('mod-1', 'Module 1', 1),
        createMockModule('mod-2', 'Module 2', 2),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { sortedModules } = useModuleUnlock({ modules, moduleProgressMap })

      expect(sortedModules.value.map(m => m.id)).toEqual(['mod-1', 'mod-2', 'mod-3'])
    })
  })

  describe('getPreviousModules', () => {
    it('should return empty array for first module', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1),
        createMockModule('mod-2', 'Module 2', 2),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getPreviousModules } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getPreviousModules('mod-1')).toEqual([])
    })

    it('should return all previous modules', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1),
        createMockModule('mod-2', 'Module 2', 2),
        createMockModule('mod-3', 'Module 3', 3),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getPreviousModules } = useModuleUnlock({ modules, moduleProgressMap })

      const previous = getPreviousModules('mod-3')
      expect(previous.map(m => m.id)).toEqual(['mod-1', 'mod-2'])
    })
  })

  describe('getImmediatePreviousModule', () => {
    it('should return null for first module', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1),
        createMockModule('mod-2', 'Module 2', 2),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getImmediatePreviousModule } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getImmediatePreviousModule('mod-1')).toBeNull()
    })

    it('should return the immediate previous module', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1),
        createMockModule('mod-2', 'Module 2', 2),
        createMockModule('mod-3', 'Module 3', 3),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getImmediatePreviousModule } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getImmediatePreviousModule('mod-3')?.id).toBe('mod-2')
    })
  })

  describe('getUnlockState - always unlock type', () => {
    it('should return unlocked for always type', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'always'),
        createMockModule('mod-2', 'Module 2', 2, 'always'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-2')
      expect(state.isLocked).toBe(false)
      expect(state.canUnlock).toBe(true)
      expect(state.message).toBe('Available')
    })
  })

  describe('getUnlockState - sequential unlock type', () => {
    it('should unlock first module', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-1')
      expect(state.isLocked).toBe(false)
      expect(state.canUnlock).toBe(true)
    })

    it('should lock second module when first is not complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-2')
      expect(state.isLocked).toBe(true)
      expect(state.canUnlock).toBe(false)
      expect(state.message).toContain('Module 1')
      expect(state.prerequisiteModuleIds).toEqual(['mod-1'])
    })

    it('should unlock second module when first is complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-2')
      // Prerequisites met, so module is available (not locked)
      expect(state.isLocked).toBe(false)
      expect(state.canUnlock).toBe(true)
      expect(state.message).toBe('Available')
    })
  })

  describe('getUnlockState - all_previous unlock type', () => {
    it('should unlock first module', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'all_previous'),
        createMockModule('mod-2', 'Module 2', 2, 'all_previous'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-1')
      expect(state.isLocked).toBe(false)
      expect(state.canUnlock).toBe(true)
    })

    it('should lock third module when only first is complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'all_previous'),
        createMockModule('mod-2', 'Module 2', 2, 'all_previous'),
        createMockModule('mod-3', 'Module 3', 3, 'all_previous'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-3')
      expect(state.isLocked).toBe(true)
      expect(state.canUnlock).toBe(false)
      expect(state.prerequisiteModuleIds).toEqual(['mod-2'])
      expect(state.prerequisiteProgress).toBe(50) // 1 of 2 complete
    })

    it('should unlock third module when all previous complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'all_previous'),
        createMockModule('mod-2', 'Module 2', 2, 'all_previous'),
        createMockModule('mod-3', 'Module 3', 3, 'all_previous'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
        ['mod-2', createMockProgress('mod-2', 'completed')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-3')
      // All prerequisites complete, so module is available
      expect(state.isLocked).toBe(false)
      expect(state.canUnlock).toBe(true)
      expect(state.prerequisiteProgress).toBe(100)
    })
  })

  describe('getUnlockState - manual unlock type', () => {
    it('should show requires approval message', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'manual'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-2')
      expect(state.isLocked).toBe(true)
      expect(state.canUnlock).toBe(false)
      expect(state.message).toContain('instructor approval')
    })
  })

  describe('getUnlockState - already unlocked modules', () => {
    it('should return not locked for unlocked modules', () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'sequential')])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'unlocked')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-1')
      expect(state.isLocked).toBe(false)
      expect(state.message).toBe('Available')
    })

    it('should return not locked for in_progress modules', () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'sequential')])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'in_progress')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-1')
      expect(state.isLocked).toBe(false)
      expect(state.message).toBe('Available')
    })

    it('should return not locked for completed modules', () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'sequential')])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
      ]))

      const { getUnlockState } = useModuleUnlock({ modules, moduleProgressMap })

      const state = getUnlockState('mod-1')
      expect(state.isLocked).toBe(false)
      expect(state.message).toBe('Completed')
    })
  })

  describe('getModulesToUnlockAfterCompletion', () => {
    it('should return next module for sequential unlock', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
        createMockModule('mod-3', 'Module 3', 3, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
        ['mod-2', createMockProgress('mod-2', 'locked')],
        ['mod-3', createMockProgress('mod-3', 'locked')],
      ]))

      const { getModulesToUnlockAfterCompletion } = useModuleUnlock({ modules, moduleProgressMap })

      const toUnlock = getModulesToUnlockAfterCompletion('mod-1')
      expect(toUnlock.map(m => m.id)).toEqual(['mod-2'])
    })

    it('should return module for all_previous when all are complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'all_previous'),
        createMockModule('mod-2', 'Module 2', 2, 'all_previous'),
        createMockModule('mod-3', 'Module 3', 3, 'all_previous'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
        ['mod-2', createMockProgress('mod-2', 'completed')],
        ['mod-3', createMockProgress('mod-3', 'locked')],
      ]))

      const { getModulesToUnlockAfterCompletion } = useModuleUnlock({ modules, moduleProgressMap })

      // Completing mod-2 should unlock mod-3 since mod-1 is already complete
      const toUnlock = getModulesToUnlockAfterCompletion('mod-2')
      expect(toUnlock.map(m => m.id)).toEqual(['mod-3'])
    })

    it('should not unlock for all_previous when not all are complete', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'all_previous'),
        createMockModule('mod-2', 'Module 2', 2, 'all_previous'),
        createMockModule('mod-3', 'Module 3', 3, 'all_previous'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
        ['mod-2', createMockProgress('mod-2', 'locked')],
        ['mod-3', createMockProgress('mod-3', 'locked')],
      ]))

      const { getModulesToUnlockAfterCompletion } = useModuleUnlock({ modules, moduleProgressMap })

      // Completing mod-1 should not unlock mod-3 (mod-2 still incomplete)
      const toUnlock = getModulesToUnlockAfterCompletion('mod-1')
      // mod-2 should be unlocked since it's sequential from mod-1
      expect(toUnlock.map(m => m.id)).toEqual(['mod-2'])
    })
  })

  describe('fetchUnlockRequirements', () => {
    it('should call API with correct parameters', async () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1)])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())
      const enrollmentId = ref('enroll-123')

      vi.mocked(enrollmentsApi.getUnlockRequirements).mockResolvedValue({
        moduleId: 'mod-1',
        moduleName: 'Module 1',
        unlockType: 'sequential',
        currentStatus: 'locked',
        requirements: [],
        message: 'Complete previous module',
      })

      const { fetchUnlockRequirements } = useModuleUnlock({
        modules,
        moduleProgressMap,
        enrollmentId,
      })

      const result = await fetchUnlockRequirements('mod-1')

      expect(enrollmentsApi.getUnlockRequirements).toHaveBeenCalledWith('enroll-123', 'mod-1')
      expect(result).toBeDefined()
      expect(result?.moduleName).toBe('Module 1')
    })

    it('should return null when not enrolled', async () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1)])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())
      const enrollmentId = ref<string | null>(null)

      const { fetchUnlockRequirements } = useModuleUnlock({
        modules,
        moduleProgressMap,
        enrollmentId,
      })

      const result = await fetchUnlockRequirements('mod-1')
      expect(result).toBeNull()
    })
  })

  describe('requestManualUnlock', () => {
    it('should call API and return true on success', async () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'manual')])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())
      const enrollmentId = ref('enroll-123')

      vi.mocked(enrollmentsApi.manuallyUnlockModule).mockResolvedValue(undefined)

      const { requestManualUnlock, loadingUnlock, unlockError } = useModuleUnlock({
        modules,
        moduleProgressMap,
        enrollmentId,
      })

      const result = await requestManualUnlock('mod-1')

      expect(result).toBe(true)
      expect(enrollmentsApi.manuallyUnlockModule).toHaveBeenCalledWith('enroll-123', 'mod-1')
      expect(loadingUnlock.value).toBeNull()
      expect(unlockError.value).toBeNull()
    })

    it('should return false and set error on failure', async () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'manual')])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())
      const enrollmentId = ref('enroll-123')

      vi.mocked(enrollmentsApi.manuallyUnlockModule).mockRejectedValue(new Error('Not authorized'))

      const { requestManualUnlock, unlockError } = useModuleUnlock({
        modules,
        moduleProgressMap,
        enrollmentId,
      })

      const result = await requestManualUnlock('mod-1')

      expect(result).toBe(false)
      expect(unlockError.value).toBe('Not authorized')
    })

    it('should return false when not enrolled', async () => {
      const modules = ref([createMockModule('mod-1', 'Module 1', 1, 'manual')])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())
      const enrollmentId = ref<string | null>(null)

      const { requestManualUnlock, unlockError } = useModuleUnlock({
        modules,
        moduleProgressMap,
        enrollmentId,
      })

      const result = await requestManualUnlock('mod-1')

      expect(result).toBe(false)
      expect(unlockError.value).toBe('Not enrolled')
    })
  })

  describe('hasUnlockableModules', () => {
    it('should return false when all modules with met prerequisites are already unlocked', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
        createMockModule('mod-3', 'Module 3', 3, 'sequential'),
      ])
      // When prerequisites are met, isLocked returns false
      // so hasUnlockableModules (which checks isLocked && canUnlock) returns false
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'completed')],
        ['mod-2', createMockProgress('mod-2', 'locked')],
        ['mod-3', createMockProgress('mod-3', 'locked')],
      ]))

      const { hasUnlockableModules } = useModuleUnlock({ modules, moduleProgressMap })

      // Prerequisites are met for mod-2, so it's not considered "locked"
      // mod-3 prerequisites not met (mod-2 not complete), but canUnlock is false
      expect(hasUnlockableModules.value).toBe(false)
    })

    it('should return false when no locked modules can be unlocked', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
        createMockModule('mod-3', 'Module 3', 3, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map([
        ['mod-1', createMockProgress('mod-1', 'in_progress')],
        ['mod-2', createMockProgress('mod-2', 'locked')],
        ['mod-3', createMockProgress('mod-3', 'locked')],
      ]))

      const { hasUnlockableModules } = useModuleUnlock({ modules, moduleProgressMap })

      // mod-1 is not completed, so mod-2 cannot be unlocked
      expect(hasUnlockableModules.value).toBe(false)
    })
  })

  describe('getUnlockMessage', () => {
    it('should return the message from unlock state', () => {
      const modules = ref([
        createMockModule('mod-1', 'Module 1', 1, 'sequential'),
        createMockModule('mod-2', 'Module 2', 2, 'sequential'),
      ])
      const moduleProgressMap = computed(() => new Map<string, ModuleProgress>())

      const { getUnlockMessage } = useModuleUnlock({ modules, moduleProgressMap })

      expect(getUnlockMessage('mod-2')).toContain('Module 1')
    })
  })
})
