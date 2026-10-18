import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePathwayStore } from './pathway'
import {
  pathwaysApi,
  enrollmentsApi,
  type Pathway,
  type PathwayEnrollment,
  type PathwayModule,
  type ModuleProgress,
} from '@/api'
import { AxiosError } from 'axios'

// Mock the APIs
vi.mock('@/api', () => ({
  pathwaysApi: {
    get: vi.fn(),
    enroll: vi.fn(),
    unenroll: vi.fn(),
  },
  enrollmentsApi: {
    getByPathway: vi.fn(),
    getProgress: vi.fn(),
    issueCertificate: vi.fn(),
    getCertificate: vi.fn(),
  },
}))

// Helper to create mock pathway
function createMockPathway(overrides: Partial<Pathway> = {}): Pathway {
  return {
    id: 'pathway-1',
    name: 'Linux Fundamentals',
    slug: 'linux-fundamentals',
    description: 'Learn Linux basics',
    difficulty: 'beginner',
    estimatedHours: 9,
    displayOrder: 0,
    status: 'published',
    isFeatured: true,
    visibility: 'global',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    moduleCount: 8,
    labCount: 12,
    totalPoints: 1030,
    modules: [
      createMockModule({ id: 'mod-1', name: 'Module 1', displayOrder: 0 }),
      createMockModule({ id: 'mod-2', name: 'Module 2', displayOrder: 1 }),
      createMockModule({ id: 'mod-3', name: 'Module 3', displayOrder: 2 }),
      createMockModule({ id: 'mod-4', name: 'Module 4', displayOrder: 3 }),
    ],
    ...overrides,
  }
}

// Helper to create mock module
function createMockModule(overrides: Partial<PathwayModule> = {}): PathwayModule {
  return {
    id: 'mod-1',
    pathwayId: 'pathway-1',
    name: 'Test Module',
    slug: 'test-module',
    displayOrder: 0,
    unlockType: 'sequential',
    isActive: true,
    createdAt: new Date().toISOString(),
    labCount: 2,
    totalPoints: 100,
    ...overrides,
  }
}

// Helper to create mock enrollment
function createMockEnrollment(overrides: Partial<PathwayEnrollment> = {}): PathwayEnrollment {
  return {
    id: 'enroll-1',
    userId: 'user-1',
    pathwayId: 'pathway-1',
    status: 'in_progress',
    completedModules: 2,
    totalModules: 8,
    earnedPoints: 200,
    maxPoints: 1030,
    percentage: 25,
    enrolledAt: new Date().toISOString(),
    certificateIssued: false,
    moduleProgress: [
      createMockModuleProgress({ moduleId: 'mod-1', status: 'completed' }),
      createMockModuleProgress({ moduleId: 'mod-2', status: 'completed' }),
      createMockModuleProgress({ moduleId: 'mod-3', status: 'in_progress' }),
      createMockModuleProgress({ moduleId: 'mod-4', status: 'locked' }),
    ],
    ...overrides,
  }
}

// Helper to create mock module progress
function createMockModuleProgress(overrides: Partial<ModuleProgress> = {}): ModuleProgress {
  return {
    id: 'mp-1',
    enrollmentId: 'enroll-1',
    moduleId: 'mod-1',
    status: 'locked',
    completedLabs: 0,
    totalLabs: 2,
    earnedPoints: 0,
    maxPoints: 100,
    ...overrides,
  }
}

// Helper to create mock certificate
function createMockCertificate(overrides: Record<string, unknown> = {}) {
  return {
    id: 'cert-1',
    enrollmentId: 'enroll-1',
    userId: 'user-1',
    pathwayId: 'pathway-1',
    pathwayName: 'Linux Fundamentals',
    userName: 'Test User',
    earnedPoints: 1030,
    maxPoints: 1030,
    percentage: 100,
    completedAt: new Date().toISOString(),
    issuedAt: new Date().toISOString(),
    verificationCode: 'CERT-ABC123',
    verificationUrl: 'https://lab.example.com/verify/CERT-ABC123',
    ...overrides,
  }
}

describe('Pathway Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  describe('initial state', () => {
    it('should have correct initial state', () => {
      const store = usePathwayStore()

      expect(store.pathway).toBeNull()
      expect(store.enrollment).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.enrolling).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should have correct computed initial values', () => {
      const store = usePathwayStore()

      expect(store.isEnrolled).toBe(false)
      expect(store.overallProgress).toBe(0)
      expect(store.completedModules).toBe(0)
      expect(store.totalModules).toBe(0)
      expect(store.earnedPoints).toBe(0)
      expect(store.maxPoints).toBe(0)
      expect(store.isPathwayComplete).toBe(false)
      expect(store.canRetry).toBe(false)
      expect(store.errorMessage).toBeNull()
    })
  })

  describe('fetchPathway()', () => {
    it('should fetch pathway successfully', async () => {
      const mockPathway = createMockPathway()
      vi.mocked(pathwaysApi.get).mockResolvedValueOnce(mockPathway)

      const store = usePathwayStore()
      await store.fetchPathway('linux-fundamentals')

      expect(store.pathway).toEqual(mockPathway)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should set error on API failure', async () => {
      const error = new AxiosError('Not found')
      ;(error as unknown as { response: { status: number } }).response = { status: 404 }
      vi.mocked(pathwaysApi.get).mockRejectedValueOnce(error)

      const store = usePathwayStore()
      await store.fetchPathway('invalid-slug')

      expect(store.pathway).toBeNull()
      expect(store.error).not.toBeNull()
      expect(store.error?.type).toBe('validation')
    })

    it('should set loading state during fetch', async () => {
      let resolvePromise: (value: Pathway) => void
      const pendingPromise = new Promise<Pathway>(resolve => {
        resolvePromise = resolve
      })
      vi.mocked(pathwaysApi.get).mockReturnValueOnce(pendingPromise)

      const store = usePathwayStore()
      const fetchPromise = store.fetchPathway('linux-fundamentals')

      expect(store.loading).toBe(true)

      resolvePromise!(createMockPathway())
      await fetchPromise

      expect(store.loading).toBe(false)
    })
  })

  describe('fetchEnrollment()', () => {
    it('should fetch enrollment successfully', async () => {
      const mockEnrollment = createMockEnrollment()
      vi.mocked(enrollmentsApi.getByPathway).mockResolvedValueOnce(mockEnrollment)
      vi.mocked(enrollmentsApi.getProgress).mockResolvedValueOnce(mockEnrollment)

      const store = usePathwayStore()
      await store.fetchEnrollment('pathway-1')

      expect(store.enrollment).toEqual(mockEnrollment)
      expect(store.isEnrolled).toBe(true)
    })

    it('should handle not enrolled (404) gracefully', async () => {
      const error = new AxiosError('Not found')
      ;(error as unknown as { response: { status: number } }).response = { status: 404 }
      vi.mocked(enrollmentsApi.getByPathway).mockRejectedValueOnce(error)

      const store = usePathwayStore()
      await store.fetchEnrollment('pathway-1')

      expect(store.enrollment).toBeNull()
      expect(store.isEnrolled).toBe(false)
      expect(store.error).toBeNull() // 404 should not be treated as error
    })

    it('should handle null response (not enrolled)', async () => {
      vi.mocked(enrollmentsApi.getByPathway).mockResolvedValueOnce(null)

      const store = usePathwayStore()
      await store.fetchEnrollment('pathway-1')

      expect(store.enrollment).toBeNull()
      expect(store.isEnrolled).toBe(false)
    })
  })

  describe('enroll()', () => {
    it('should enroll successfully', async () => {
      const mockEnrollment = createMockEnrollment({ status: 'enrolled', completedModules: 0 })
      vi.mocked(pathwaysApi.enroll).mockResolvedValueOnce(mockEnrollment)
      vi.mocked(enrollmentsApi.getProgress).mockResolvedValueOnce(mockEnrollment)

      const store = usePathwayStore()
      await store.enroll('pathway-1')

      expect(store.enrollment).toEqual(mockEnrollment)
      expect(store.isEnrolled).toBe(true)
      expect(store.enrolling).toBe(false)
    })

    it('should set error and throw on failure', async () => {
      const error = new Error('Enrollment failed')
      vi.mocked(pathwaysApi.enroll).mockRejectedValueOnce(error)

      const store = usePathwayStore()

      await expect(store.enroll('pathway-1')).rejects.toThrow('Enrollment failed')
      expect(store.error).not.toBeNull()
      expect(store.enrolling).toBe(false)
    })

    it('should set enrolling state during enrollment', async () => {
      let resolvePromise: (value: PathwayEnrollment) => void
      const pendingPromise = new Promise<PathwayEnrollment>(resolve => {
        resolvePromise = resolve
      })
      vi.mocked(pathwaysApi.enroll).mockReturnValueOnce(pendingPromise)

      const store = usePathwayStore()
      const enrollPromise = store.enroll('pathway-1')

      expect(store.enrolling).toBe(true)

      const mockEnrollment = createMockEnrollment()
      vi.mocked(enrollmentsApi.getProgress).mockResolvedValueOnce(mockEnrollment)
      resolvePromise!(mockEnrollment)
      await enrollPromise

      expect(store.enrolling).toBe(false)
    })
  })

  describe('unenroll()', () => {
    it('should unenroll successfully', async () => {
      vi.mocked(pathwaysApi.unenroll).mockResolvedValueOnce(undefined)

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      await store.unenroll('pathway-1')

      expect(store.enrollment).toBeNull()
      expect(store.isEnrolled).toBe(false)
    })

    it('should set error and throw on failure', async () => {
      const error = new Error('Unenroll failed')
      vi.mocked(pathwaysApi.unenroll).mockRejectedValueOnce(error)

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      await expect(store.unenroll('pathway-1')).rejects.toThrow('Unenroll failed')
      expect(store.error).not.toBeNull()
    })
  })

  describe('getModuleStatus()', () => {
    it('should return correct status for enrolled user', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.getModuleStatus('mod-1')).toBe('completed')
      expect(store.getModuleStatus('mod-2')).toBe('completed')
      expect(store.getModuleStatus('mod-3')).toBe('in_progress')
      expect(store.getModuleStatus('mod-4')).toBe('locked')
    })

    it('should return locked for unknown module', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.getModuleStatus('unknown-module')).toBe('locked')
    })

    it('should return locked when not enrolled', () => {
      const store = usePathwayStore()

      expect(store.getModuleStatus('mod-1')).toBe('locked')
    })
  })

  describe('getModuleProgress()', () => {
    it('should return correct progress percentage', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed', completedLabs: 2, totalLabs: 2 }),
          createMockModuleProgress({ moduleId: 'mod-2', status: 'in_progress', completedLabs: 1, totalLabs: 2 }),
        ],
      })

      expect(store.getModuleProgress('mod-1')).toBe(100)
      expect(store.getModuleProgress('mod-2')).toBe(50)
    })

    it('should return 0 for unknown module', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.getModuleProgress('unknown-module')).toBe(0)
    })

    it('should handle zero total labs', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed', completedLabs: 0, totalLabs: 0 }),
        ],
      })

      expect(store.getModuleProgress('mod-1')).toBe(100) // Completed with 0 labs = 100%
    })
  })

  describe('isModuleClickable()', () => {
    it('should return true for non-locked modules', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.isModuleClickable('mod-1')).toBe(true) // completed
      expect(store.isModuleClickable('mod-2')).toBe(true) // completed
      expect(store.isModuleClickable('mod-3')).toBe(true) // in_progress
    })

    it('should return false for locked modules', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.isModuleClickable('mod-4')).toBe(false) // locked
    })
  })

  describe('isModuleComplete()', () => {
    it('should return true only for completed modules', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment()

      expect(store.isModuleComplete('mod-1')).toBe(true)
      expect(store.isModuleComplete('mod-2')).toBe(true)
      expect(store.isModuleComplete('mod-3')).toBe(false) // in_progress
      expect(store.isModuleComplete('mod-4')).toBe(false) // locked
    })
  })

  describe('completeModule()', () => {
    it('should mark module as completed and update totals', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway()
      store.enrollment = createMockEnrollment({
        completedModules: 2,
        totalModules: 4,
        earnedPoints: 200,
        maxPoints: 400,
        percentage: 50,
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed', earnedPoints: 100 }),
          createMockModuleProgress({ moduleId: 'mod-2', status: 'completed', earnedPoints: 100 }),
          createMockModuleProgress({
            moduleId: 'mod-3',
            status: 'in_progress',
            earnedPoints: 0,
            completedLabs: 0,
            totalLabs: 2,
          }),
          createMockModuleProgress({ moduleId: 'mod-4', status: 'locked' }),
        ],
      })

      const result = store.completeModule('mod-3', 100)

      // Check return value
      expect(result.unlockedModuleName).toBe('Module 4')
      expect(result.pathwayCompleted).toBe(false)

      // Check module was marked complete
      const mod3Progress = store.getModuleProgressData('mod-3')
      expect(mod3Progress?.status).toBe('completed')
      expect(mod3Progress?.earnedPoints).toBe(100)

      // Check enrollment totals updated
      expect(store.enrollment?.completedModules).toBe(3)
      expect(store.enrollment?.earnedPoints).toBe(300)
      expect(store.enrollment?.percentage).toBe(75)
    })

    it('should unlock next module after completion', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway()
      store.enrollment = createMockEnrollment({
        completedModules: 2,
        totalModules: 4,
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed' }),
          createMockModuleProgress({ moduleId: 'mod-2', status: 'completed' }),
          createMockModuleProgress({ moduleId: 'mod-3', status: 'in_progress', totalLabs: 2 }),
          createMockModuleProgress({ moduleId: 'mod-4', status: 'locked' }),
        ],
      })

      const result = store.completeModule('mod-3', 100)

      // Check return value includes unlocked module
      expect(result.unlockedModuleName).toBe('Module 4')

      // Check mod-4 was unlocked
      const mod4Progress = store.getModuleProgressData('mod-4')
      expect(mod4Progress?.status).toBe('unlocked')
      expect(mod4Progress?.unlockedAt).toBeDefined()
    })

    it('should mark pathway as complete when all modules done', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway({
        modules: [
          createMockModule({ id: 'mod-1', displayOrder: 0 }),
          createMockModule({ id: 'mod-2', displayOrder: 1 }),
        ],
      })
      store.enrollment = createMockEnrollment({
        status: 'in_progress',
        completedModules: 1,
        totalModules: 2,
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed' }),
          createMockModuleProgress({ moduleId: 'mod-2', status: 'in_progress', totalLabs: 1 }),
        ],
      })

      const result = store.completeModule('mod-2', 100)

      // Check return value indicates pathway completed
      expect(result.pathwayCompleted).toBe(true)
      expect(result.unlockedModuleName).toBeNull() // No more modules to unlock

      expect(store.enrollment?.status).toBe('completed')
      expect(store.enrollment?.completedAt).toBeDefined()
      expect(store.isPathwayComplete).toBe(true)
    })

    it('should do nothing when not enrolled', () => {
      const store = usePathwayStore()

      // Should not throw and return empty result
      const result = store.completeModule('mod-1', 100)

      expect(result.unlockedModuleName).toBeNull()
      expect(result.pathwayCompleted).toBe(false)
      expect(store.enrollment).toBeNull()
    })
  })

  describe('unlockNextModule()', () => {
    it('should unlock the next module in sequence', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway()
      store.enrollment = createMockEnrollment({
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-1', status: 'completed' }),
          createMockModuleProgress({ moduleId: 'mod-2', status: 'locked' }),
        ],
      })

      const result = store.unlockNextModule('mod-1')

      // Check return value
      expect(result).toBe('Module 2')

      const mod2Progress = store.getModuleProgressData('mod-2')
      expect(mod2Progress?.status).toBe('unlocked')
    })

    it('should not unlock if there is no next module', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway({
        modules: [createMockModule({ id: 'mod-1', displayOrder: 0 })],
      })
      store.enrollment = createMockEnrollment({
        moduleProgress: [createMockModuleProgress({ moduleId: 'mod-1', status: 'completed' })],
      })

      // Should return null when no more modules
      const result = store.unlockNextModule('mod-1')
      expect(result).toBeNull()
    })

    it('should handle non-sequential display orders', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway({
        modules: [
          createMockModule({ id: 'mod-a', name: 'Module A', displayOrder: 5 }),
          createMockModule({ id: 'mod-b', name: 'Module B', displayOrder: 10 }),
          createMockModule({ id: 'mod-c', name: 'Module C', displayOrder: 15 }),
        ],
      })
      store.enrollment = createMockEnrollment({
        moduleProgress: [
          createMockModuleProgress({ moduleId: 'mod-a', status: 'completed' }),
          createMockModuleProgress({ moduleId: 'mod-b', status: 'locked' }),
          createMockModuleProgress({ moduleId: 'mod-c', status: 'locked' }),
        ],
      })

      const result = store.unlockNextModule('mod-a')

      // Check return value
      expect(result).toBe('Module B')

      // mod-b should be unlocked (next by display order)
      expect(store.getModuleProgressData('mod-b')?.status).toBe('unlocked')
      // mod-c should still be locked
      expect(store.getModuleProgressData('mod-c')?.status).toBe('locked')
    })
  })

  describe('isPathwayComplete computed', () => {
    it('should return true when enrollment status is completed', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({ status: 'completed' })

      expect(store.isPathwayComplete).toBe(true)
    })

    it('should return false when enrollment status is in_progress', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({ status: 'in_progress' })

      expect(store.isPathwayComplete).toBe(false)
    })

    it('should return false when not enrolled', () => {
      const store = usePathwayStore()

      expect(store.isPathwayComplete).toBe(false)
    })
  })

  describe('computed values with enrollment', () => {
    it('should compute correct values from enrollment', () => {
      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        completedModules: 3,
        totalModules: 8,
        earnedPoints: 350,
        maxPoints: 1030,
        percentage: 34,
      })

      expect(store.overallProgress).toBe(34)
      expect(store.completedModules).toBe(3)
      expect(store.totalModules).toBe(8)
      expect(store.earnedPoints).toBe(350)
      expect(store.maxPoints).toBe(1030)
    })

    it('should fall back to pathway values when not enrolled', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway({
        moduleCount: 8,
        totalPoints: 1030,
      })

      expect(store.totalModules).toBe(8)
      expect(store.maxPoints).toBe(1030)
    })
  })

  describe('clearError()', () => {
    it('should clear error state', () => {
      const store = usePathwayStore()
      store.error = {
        message: 'Test error',
        type: 'unknown',
        timestamp: new Date(),
        retryable: true,
      }

      store.clearError()

      expect(store.error).toBeNull()
      expect(store.errorMessage).toBeNull()
    })
  })

  describe('reset()', () => {
    it('should reset all state to initial values', () => {
      const store = usePathwayStore()
      store.pathway = createMockPathway()
      store.enrollment = createMockEnrollment()
      store.loading = true
      store.enrolling = true
      store.error = {
        message: 'error',
        type: 'unknown',
        timestamp: new Date(),
        retryable: false,
      }

      store.reset()

      expect(store.pathway).toBeNull()
      expect(store.enrollment).toBeNull()
      expect(store.loading).toBe(false)
      expect(store.enrolling).toBe(false)
      expect(store.error).toBeNull()
    })
  })

  describe('issueCertificateOnCompletion()', () => {
    it('should issue certificate successfully', async () => {
      const mockCertificate = createMockCertificate()
      vi.mocked(enrollmentsApi.issueCertificate).mockResolvedValueOnce(mockCertificate)

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        status: 'completed',
        certificateIssued: false,
      })

      const result = await store.issueCertificateOnCompletion()

      expect(result).toEqual(mockCertificate)
      expect(store.enrollment?.certificateIssued).toBe(true)
      expect(enrollmentsApi.issueCertificate).toHaveBeenCalledWith('enroll-1')
    })

    it('should return existing certificate if already issued', async () => {
      const mockCertificate = createMockCertificate()

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        status: 'completed',
        certificateIssued: true,
      })
      store.certificate = mockCertificate

      const result = await store.issueCertificateOnCompletion()

      expect(result).toEqual(mockCertificate)
      expect(enrollmentsApi.issueCertificate).not.toHaveBeenCalled()
    })

    it('should return null when not enrolled', async () => {
      const store = usePathwayStore()

      const result = await store.issueCertificateOnCompletion()

      expect(result).toBeNull()
      expect(enrollmentsApi.issueCertificate).not.toHaveBeenCalled()
    })

    it('should handle API errors gracefully', async () => {
      vi.mocked(enrollmentsApi.issueCertificate).mockRejectedValueOnce(new Error('API Error'))

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({
        status: 'completed',
        certificateIssued: false,
      })

      const result = await store.issueCertificateOnCompletion()

      expect(result).toBeNull()
      // Error should not be set in store (certificate is non-critical)
      expect(store.error).toBeNull()
    })
  })

  describe('fetchCertificate()', () => {
    it('should fetch certificate from API', async () => {
      const mockCertificate = createMockCertificate()
      vi.mocked(enrollmentsApi.getCertificate).mockResolvedValueOnce(mockCertificate)

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({ status: 'completed' })

      const result = await store.fetchCertificate()

      expect(result).toEqual(mockCertificate)
      expect(store.certificate).toEqual(mockCertificate)
      expect(enrollmentsApi.getCertificate).toHaveBeenCalledWith('enroll-1')
    })

    it('should return cached certificate if available', async () => {
      const mockCertificate = createMockCertificate()

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({ status: 'completed' })
      store.certificate = mockCertificate

      const result = await store.fetchCertificate()

      expect(result).toEqual(mockCertificate)
      expect(enrollmentsApi.getCertificate).not.toHaveBeenCalled()
    })

    it('should return null when not enrolled', async () => {
      const store = usePathwayStore()

      const result = await store.fetchCertificate()

      expect(result).toBeNull()
      expect(enrollmentsApi.getCertificate).not.toHaveBeenCalled()
    })

    it('should handle 404 (no certificate) gracefully', async () => {
      const error = new AxiosError('Not found')
      ;(error as unknown as { response: { status: number } }).response = { status: 404 }
      vi.mocked(enrollmentsApi.getCertificate).mockRejectedValueOnce(error)

      const store = usePathwayStore()
      store.enrollment = createMockEnrollment({ status: 'completed' })

      const result = await store.fetchCertificate()

      expect(result).toBeNull()
      expect(store.error).toBeNull()
    })
  })
})
