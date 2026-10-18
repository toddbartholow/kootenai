/**
 * Enrollments API
 * Pathway enrollment and progress tracking
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import {
  mockPathways,
  type Pathway,
  type PathwayEnrollment,
  type UnlockRequirements,
  type EnrollmentStatus,
} from './pathways'

// Re-export types from pathways for backwards compatibility
export type {
  EnrollmentStatus,
  PathwayEnrollment,
  ModuleProgress,
  LabProgress,
  UnlockRequirement,
  UnlockRequirements,
} from './pathways'

// ============================================================================
// Certificate Types
// ============================================================================
export interface Certificate {
  id: string
  enrollmentId: string
  userId: string
  pathwayId: string
  pathwayName: string
  userName: string
  userEmail?: string
  earnedPoints: number
  maxPoints: number
  percentage: number
  completedAt: string
  issuedAt: string
  verificationCode: string
  verificationUrl: string
  certificateUrl?: string
  expiresAt?: string
}

// ============================================================================
// Mock Data
// ============================================================================
const mockCertificates: Certificate[] = []

const mockEnrollments: PathwayEnrollment[] = [
  {
    id: 'enroll-1',
    userId: '00000000-0000-0000-0000-000000000001',
    pathwayId: 'pathway-linux',
    status: 'in_progress',
    completedModules: 2,
    totalModules: 8,
    earnedPoints: 200,
    maxPoints: 1030,
    percentage: 19.4,
    enrolledAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
    startedAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(),
    lastActivityAt: new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(),
    certificateIssued: false,
    pathway: mockPathways[0] as Pathway,
  },
]

// ============================================================================
// API
// ============================================================================
export const enrollmentsApi = {
  list: async (options?: {
    status?: EnrollmentStatus
  }): Promise<PathwayEnrollment[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      let enrollments = [...mockEnrollments]
      if (options?.status) {
        enrollments = enrollments.filter(e => e.status === options.status)
      }
      return enrollments
    }
    const params: Record<string, string> = {}
    if (options?.status) params['status'] = options.status
    const response = await api.get<{ enrollments: PathwayEnrollment[]; count: number }>('/enrollments', { params })
    return response.data.enrollments || []
  },

  get: async (enrollmentId: string): Promise<PathwayEnrollment> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const enrollment = mockEnrollments.find(e => e.id === enrollmentId)
      if (!enrollment) throw new Error(`Enrollment not found: ${enrollmentId}`)
      return enrollment
    }
    const response = await api.get<PathwayEnrollment>(`/enrollments/${enrollmentId}`)
    return response.data
  },

  getProgress: async (enrollmentId: string): Promise<PathwayEnrollment> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const enrollment = mockEnrollments.find(e => e.id === enrollmentId)
      if (!enrollment) throw new Error(`Enrollment not found: ${enrollmentId}`)
      // Return with mock module progress
      return {
        ...enrollment,
        moduleProgress: [
          {
            id: 'mp-1',
            enrollmentId,
            moduleId: 'mod-1',
            status: 'completed',
            completedLabs: 1,
            totalLabs: 1,
            earnedPoints: 100,
            maxPoints: 100,
            completedAt: new Date().toISOString(),
          },
          {
            id: 'mp-2',
            enrollmentId,
            moduleId: 'mod-2',
            status: 'in_progress',
            completedLabs: 0,
            totalLabs: 1,
            earnedPoints: 50,
            maxPoints: 100,
          },
        ],
      }
    }
    const response = await api.get<PathwayEnrollment>(`/enrollments/${enrollmentId}/progress`)
    return response.data
  },

  getByPathway: async (pathwayId: string): Promise<PathwayEnrollment | null> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockEnrollments.find(e => e.pathwayId === pathwayId) || null
    }
    // List enrollments filtered by pathway
    const response = await api.get<{ enrollments: PathwayEnrollment[]; count: number }>(
      '/enrollments',
      { params: { pathway_id: pathwayId } }
    )
    return response.data.enrollments?.[0] || null
  },

  getUnlockRequirements: async (enrollmentId: string, moduleId: string): Promise<UnlockRequirements> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return {
        moduleId,
        moduleName: 'Module',
        unlockType: 'sequential',
        currentStatus: 'locked',
        requirements: [],
        message: 'Complete the previous module to unlock this one.',
      }
    }
    const response = await api.get<UnlockRequirements>(
      `/enrollments/${enrollmentId}/modules/${moduleId}/unlock-requirements`
    )
    return response.data
  },

  manuallyUnlockModule: async (enrollmentId: string, moduleId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return
    }
    await api.post(`/enrollments/${enrollmentId}/modules/${moduleId}/unlock`)
  },

  // Enroll in a pathway (called from pathwaysApi but kept here for direct access)
  enroll: async (pathwayId: string): Promise<PathwayEnrollment> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const pathway = mockPathways.find(p => p.id === pathwayId)
      const enrollment: PathwayEnrollment = {
        id: `enroll-${Date.now()}`,
        userId: '00000000-0000-0000-0000-000000000001',
        pathwayId,
        status: 'enrolled',
        completedModules: 0,
        totalModules: pathway?.moduleCount || 0,
        earnedPoints: 0,
        maxPoints: pathway?.totalPoints || 0,
        percentage: 0,
        enrolledAt: new Date().toISOString(),
        certificateIssued: false,
        ...(pathway && { pathway }),
      }
      mockEnrollments.push(enrollment)
      return enrollment
    }
    const response = await api.post<PathwayEnrollment>(`/pathways/${pathwayId}/enroll`)
    return response.data
  },

  unenroll: async (pathwayId: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const index = mockEnrollments.findIndex(e => e.pathwayId === pathwayId)
      if (index !== -1) mockEnrollments.splice(index, 1)
      return
    }
    await api.delete(`/pathways/${pathwayId}/enroll`)
  },

  // ============================================================================
  // Certificate Methods
  // ============================================================================

  /**
   * Issue a certificate for a completed pathway enrollment
   */
  issueCertificate: async (enrollmentId: string): Promise<Certificate> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const enrollment = mockEnrollments.find(e => e.id === enrollmentId)
      if (!enrollment) throw new Error(`Enrollment not found: ${enrollmentId}`)
      if (enrollment.status !== 'completed') throw new Error('Pathway not completed')
      if (enrollment.certificateIssued) {
        const existing = mockCertificates.find(c => c.enrollmentId === enrollmentId)
        if (existing) return existing
      }

      const certificate: Certificate = {
        id: `cert-${Date.now()}`,
        enrollmentId,
        userId: enrollment.userId,
        pathwayId: enrollment.pathwayId,
        pathwayName: enrollment.pathway?.name || 'Unknown Pathway',
        userName: 'Demo User',
        earnedPoints: enrollment.earnedPoints,
        maxPoints: enrollment.maxPoints,
        percentage: enrollment.percentage,
        completedAt: enrollment.completedAt || new Date().toISOString(),
        issuedAt: new Date().toISOString(),
        verificationCode: `CERT-${Math.random().toString(36).substring(2, 10).toUpperCase()}`,
        verificationUrl: `https://lab.example.com/verify/CERT-${Math.random().toString(36).substring(2, 10).toUpperCase()}`,
      }

      enrollment.certificateIssued = true
      enrollment.certificateUrl = certificate.verificationUrl
      mockCertificates.push(certificate)
      return certificate
    }
    const response = await api.post<Certificate>(`/enrollments/${enrollmentId}/certificate`)
    return response.data
  },

  /**
   * Get certificate for an enrollment
   */
  getCertificate: async (enrollmentId: string): Promise<Certificate> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const certificate = mockCertificates.find(c => c.enrollmentId === enrollmentId)
      if (!certificate) throw new Error('Certificate not found')
      return certificate
    }
    const response = await api.get<Certificate>(`/enrollments/${enrollmentId}/certificate`)
    return response.data
  },

  /**
   * List all certificates for the current user
   */
  listCertificates: async (): Promise<Certificate[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return [...mockCertificates]
    }
    const response = await api.get<Certificate[]>('/certificates')
    return response.data
  },

  /**
   * Verify a certificate by its verification code (public endpoint)
   */
  verifyCertificate: async (code: string): Promise<Certificate> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const certificate = mockCertificates.find(c => c.verificationCode === code)
      if (!certificate) throw new Error('Invalid verification code')
      return certificate
    }
    const response = await api.get<Certificate>(`/certificates/verify/${code}`)
    return response.data
  },
}
