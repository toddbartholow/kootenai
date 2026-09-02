import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { AxiosError } from 'axios'
import {
  pathwaysApi,
  enrollmentsApi,
  type Pathway,
  type PathwayEnrollment,
  type ModuleProgress,
  type ModuleProgressStatus,
  type Certificate,
} from '@/api'
import { createErrorState, type ErrorState } from '@/types/errors'

/**
 * Pinia store for managing pathway state, enrollment, and progress tracking
 */
export const usePathwayStore = defineStore('pathway', () => {
  // State
  const pathway = ref<Pathway | null>(null)
  const enrollment = ref<PathwayEnrollment | null>(null)
  const loading = ref(false)
  const enrolling = ref(false)
  const error = ref<ErrorState | null>(null)
  const certificate = ref<Certificate | null>(null)
  const issuingCertificate = ref(false)

  // Module progress map for quick lookups (moduleId -> progress)
  const moduleProgressMap = computed(() => {
    const map = new Map<string, ModuleProgress>()
    if (enrollment.value?.moduleProgress) {
      for (const progress of enrollment.value.moduleProgress) {
        map.set(progress.moduleId, progress)
      }
    }
    return map
  })

  // Computed properties
  /** Whether user is enrolled in this pathway */
  const isEnrolled = computed(() => enrollment.value !== null)

  /** Overall pathway progress percentage (0-100) */
  const overallProgress = computed(() => enrollment.value?.percentage ?? 0)

  /** Number of completed modules */
  const completedModules = computed(() => enrollment.value?.completedModules ?? 0)

  /** Total number of modules */
  const totalModules = computed(() => enrollment.value?.totalModules ?? pathway.value?.moduleCount ?? 0)

  /** Total earned points */
  const earnedPoints = computed(() => enrollment.value?.earnedPoints ?? 0)

  /** Maximum possible points */
  const maxPoints = computed(() => enrollment.value?.maxPoints ?? pathway.value?.totalPoints ?? 0)

  /** Whether the entire pathway is completed */
  const isPathwayComplete = computed(() => {
    if (!enrollment.value) return false
    return enrollment.value.status === 'completed'
  })

  /** Whether there's a retryable error */
  const canRetry = computed(() => error.value?.retryable ?? false)

  /** Human-readable error message */
  const errorMessage = computed(() => error.value?.message ?? null)

  /**
   * Gets the progress status for a specific module
   * @param moduleId - The module ID to check
   * @returns The module progress status or 'locked' if not found
   */
  function getModuleStatus(moduleId: string): ModuleProgressStatus {
    const progress = moduleProgressMap.value.get(moduleId)
    return progress?.status ?? 'locked'
  }

  /**
   * Gets the progress percentage for a specific module
   * @param moduleId - The module ID to check
   * @returns Progress percentage (0-100)
   */
  function getModuleProgress(moduleId: string): number {
    const progress = moduleProgressMap.value.get(moduleId)
    if (!progress) return 0
    if (progress.totalLabs === 0) return progress.status === 'completed' ? 100 : 0
    return Math.round((progress.completedLabs / progress.totalLabs) * 100)
  }

  /**
   * Gets the full module progress object
   * @param moduleId - The module ID to check
   * @returns The module progress or null if not found
   */
  function getModuleProgressData(moduleId: string): ModuleProgress | null {
    return moduleProgressMap.value.get(moduleId) ?? null
  }

  /**
   * Checks if a module is clickable (not locked)
   * @param moduleId - The module ID to check
   */
  function isModuleClickable(moduleId: string): boolean {
    const status = getModuleStatus(moduleId)
    return status !== 'locked'
  }

  /**
   * Checks if a specific module is completed
   * @param moduleId - The module ID to check
   */
  function isModuleComplete(moduleId: string): boolean {
    return getModuleStatus(moduleId) === 'completed'
  }

  /**
   * Fetches pathway details by ID or slug
   * @param idOrSlug - The pathway ID or slug
   */
  async function fetchPathway(idOrSlug: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      pathway.value = await pathwaysApi.get(idOrSlug)
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch pathway')
    } finally {
      loading.value = false
    }
  }

  /**
   * Fetches enrollment status and progress for the current pathway
   * @param pathwayId - The pathway ID to check enrollment for
   */
  async function fetchEnrollment(pathwayId: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const result = await enrollmentsApi.getByPathway(pathwayId)
      enrollment.value = result
      // If enrolled, fetch full progress
      if (result) {
        const progressData = await enrollmentsApi.getProgress(result.id)
        enrollment.value = progressData
      }
    } catch (e: unknown) {
      // 404 means not enrolled - not an error
      if (e instanceof AxiosError && e.response?.status === 404) {
        enrollment.value = null
        return
      }
      error.value = createErrorState(e, 'Failed to fetch enrollment')
    } finally {
      loading.value = false
    }
  }

  /**
   * Enrolls the current user in a pathway
   * @param pathwayId - The pathway ID to enroll in
   */
  async function enroll(pathwayId: string): Promise<void> {
    enrolling.value = true
    error.value = null
    try {
      enrollment.value = await pathwaysApi.enroll(pathwayId)
      // Fetch full progress data after enrollment
      if (enrollment.value) {
        const progressData = await enrollmentsApi.getProgress(enrollment.value.id)
        enrollment.value = progressData
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to enroll in pathway')
      throw e
    } finally {
      enrolling.value = false
    }
  }

  /**
   * Unenrolls the current user from a pathway
   * @param pathwayId - The pathway ID to unenroll from
   */
  async function unenroll(pathwayId: string): Promise<void> {
    enrolling.value = true
    error.value = null
    try {
      await pathwaysApi.unenroll(pathwayId)
      enrollment.value = null
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to unenroll from pathway')
      throw e
    } finally {
      enrolling.value = false
    }
  }

  /**
   * Updates local module progress when a module is completed
   * This is used for optimistic updates before server confirmation
   * @param moduleId - The module that was completed
   * @param earnedPoints - Points earned in this module
   */
  /**
   * Marks a module as completed and unlocks the next module
   * @param moduleId - The module being completed
   * @param earnedPts - Points earned for completing the module
   * @returns Object with unlockedModuleName if a module was unlocked, and pathwayCompleted if pathway is done
   */
  function completeModule(moduleId: string, earnedPts: number): { unlockedModuleName: string | null; pathwayCompleted: boolean } {
    const result = { unlockedModuleName: null as string | null, pathwayCompleted: false }

    if (!enrollment.value) return result

    // Find existing progress or create new one
    const progressIndex = enrollment.value.moduleProgress?.findIndex(p => p.moduleId === moduleId) ?? -1

    if (progressIndex >= 0 && enrollment.value.moduleProgress) {
      const progress = enrollment.value.moduleProgress[progressIndex]
      if (progress) {
        progress.status = 'completed'
        progress.earnedPoints = earnedPts
        progress.completedLabs = progress.totalLabs
        progress.completedAt = new Date().toISOString()
      }
    }

    // Update enrollment totals
    enrollment.value.completedModules = (enrollment.value.completedModules || 0) + 1
    enrollment.value.earnedPoints = (enrollment.value.earnedPoints || 0) + earnedPts

    // Recalculate percentage
    if (enrollment.value.totalModules > 0) {
      enrollment.value.percentage = Math.round(
        (enrollment.value.completedModules / enrollment.value.totalModules) * 100
      )
    }

    // Check if pathway is complete
    if (enrollment.value.completedModules >= enrollment.value.totalModules) {
      enrollment.value.status = 'completed'
      enrollment.value.completedAt = new Date().toISOString()
      result.pathwayCompleted = true
    }

    // Unlock next module (sequential unlock)
    result.unlockedModuleName = unlockNextModule(moduleId)

    return result
  }

  /**
   * Unlocks the next module in sequence after completing a module
   * @param completedModuleId - The module that was just completed
   * @returns The name of the unlocked module, or null if no module was unlocked
   */
  function unlockNextModule(completedModuleId: string): string | null {
    if (!enrollment.value?.moduleProgress || !pathway.value?.modules) return null

    // Find the completed module's display order
    const completedModule = pathway.value.modules.find(m => m.id === completedModuleId)
    if (!completedModule) return null

    // Find the next module by display order
    const nextModule = pathway.value.modules
      .filter(m => m.displayOrder > completedModule.displayOrder)
      .sort((a, b) => a.displayOrder - b.displayOrder)[0]

    if (!nextModule) return null

    // Find or create progress for next module
    const nextProgress = enrollment.value.moduleProgress.find(p => p.moduleId === nextModule.id)
    if (nextProgress && nextProgress.status === 'locked') {
      nextProgress.status = 'unlocked'
      nextProgress.unlockedAt = new Date().toISOString()
      return nextModule.name
    }

    return null
  }

  /**
   * Issues a certificate for the completed pathway
   * Called automatically when pathway is completed
   * @returns The issued certificate, or null if already issued or error
   */
  async function issueCertificateOnCompletion(): Promise<Certificate | null> {
    if (!enrollment.value?.id) return null

    // Don't re-issue if already issued
    if (enrollment.value.certificateIssued && certificate.value) {
      return certificate.value
    }

    issuingCertificate.value = true
    try {
      const issuedCert = await enrollmentsApi.issueCertificate(enrollment.value.id)
      certificate.value = issuedCert

      // Update enrollment to reflect certificate issued
      if (enrollment.value) {
        enrollment.value.certificateIssued = true
      }

      return issuedCert
    } catch (e: unknown) {
      // Log error but don't set store error - certificate is non-critical
      console.error('Failed to auto-issue certificate:', e)
      return null
    } finally {
      issuingCertificate.value = false
    }
  }

  /**
   * Gets the certificate for the current enrollment
   * @returns The certificate or null
   */
  async function fetchCertificate(): Promise<Certificate | null> {
    if (!enrollment.value?.id) return null

    // Return cached certificate if available
    if (certificate.value && certificate.value.enrollmentId === enrollment.value.id) {
      return certificate.value
    }

    try {
      const cert = await enrollmentsApi.getCertificate(enrollment.value.id)
      certificate.value = cert
      return cert
    } catch (e: unknown) {
      // 404 means no certificate yet
      if (e instanceof AxiosError && e.response?.status === 404) {
        return null
      }
      console.error('Failed to fetch certificate:', e)
      return null
    }
  }

  /**
   * Refreshes enrollment progress from the server
   */
  async function refreshProgress(): Promise<void> {
    if (!enrollment.value?.id) return

    try {
      const progressData = await enrollmentsApi.getProgress(enrollment.value.id)
      enrollment.value = progressData
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to refresh progress')
    }
  }

  /** Clears the current error state */
  function clearError(): void {
    error.value = null
  }

  /**
   * Resets the store to initial state
   * Call when leaving a pathway or starting fresh
   */
  function reset(): void {
    pathway.value = null
    enrollment.value = null
    loading.value = false
    enrolling.value = false
    error.value = null
    certificate.value = null
    issuingCertificate.value = false
  }

  return {
    // State
    pathway,
    enrollment,
    loading,
    enrolling,
    error,
    certificate,
    issuingCertificate,
    // Computed
    isEnrolled,
    overallProgress,
    completedModules,
    totalModules,
    earnedPoints,
    maxPoints,
    isPathwayComplete,
    canRetry,
    errorMessage,
    moduleProgressMap,
    // Actions
    fetchPathway,
    fetchEnrollment,
    enroll,
    unenroll,
    getModuleStatus,
    getModuleProgress,
    getModuleProgressData,
    isModuleClickable,
    isModuleComplete,
    completeModule,
    unlockNextModule,
    issueCertificateOnCompletion,
    fetchCertificate,
    refreshProgress,
    clearError,
    reset,
  }
})
