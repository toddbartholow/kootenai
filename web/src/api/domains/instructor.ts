/**
 * Instructor API
 * Instructor dashboard, class management, and analytics
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================

export interface StudentSummary {
  id: string
  name: string
  email: string
  enrolledAt: string
  lastActiveAt?: string
  labsCompleted: number
  labsAttempted: number
  averageScore: number
  totalPoints: number
  currentStreak: number
  status: 'active' | 'inactive' | 'at-risk' | 'excelling'
}

export interface ClassOverview {
  totalStudents: number
  activeStudents: number
  inactiveStudents: number
  atRiskStudents: number
  excellingStudents: number
  averageCompletion: number
  averageScore: number
  totalLabsCompleted: number
}

export interface LabCompletionStats {
  labId: string
  labName: string
  totalAttempts: number
  completions: number
  passRate: number
  averageScore: number
  averageTimeMinutes: number
  failurePoints: FailurePoint[]
}

export interface FailurePoint {
  checkpointId: string
  checkpointName: string
  failureCount: number
  failureRate: number
}

export interface ActivityHeatmapData {
  date: string
  hour: number
  count: number
}

export interface StudentProgress {
  studentId: string
  studentName: string
  pathwayId: string
  pathwayName: string
  modulesCompleted: number
  totalModules: number
  labsCompleted: number
  totalLabs: number
  percentage: number
  lastActivityAt?: string
}

export interface InstructorDashboardResponse {
  classOverview: ClassOverview
  students: StudentSummary[]
  labStats: LabCompletionStats[]
  activityHeatmap: ActivityHeatmapData[]
  recentProgress: StudentProgress[]
}

export interface ExportFormat {
  format: 'csv' | 'json' | 'pdf'
}

// ============================================================================
// Mock Data
// ============================================================================

const mockStudents: StudentSummary[] = [
  {
    id: 'student-1',
    name: 'Alice Johnson',
    email: 'alice@example.com',
    enrolledAt: '2024-09-01T00:00:00Z',
    lastActiveAt: '2025-01-19T10:30:00Z',
    labsCompleted: 18,
    labsAttempted: 20,
    averageScore: 92,
    totalPoints: 18500,
    currentStreak: 5,
    status: 'excelling'
  },
  {
    id: 'student-2',
    name: 'Bob Smith',
    email: 'bob@example.com',
    enrolledAt: '2024-09-01T00:00:00Z',
    lastActiveAt: '2025-01-18T15:45:00Z',
    labsCompleted: 15,
    labsAttempted: 18,
    averageScore: 85,
    totalPoints: 14200,
    currentStreak: 3,
    status: 'active'
  },
  {
    id: 'student-3',
    name: 'Carol Martinez',
    email: 'carol@example.com',
    enrolledAt: '2024-09-01T00:00:00Z',
    lastActiveAt: '2025-01-10T08:20:00Z',
    labsCompleted: 8,
    labsAttempted: 15,
    averageScore: 62,
    totalPoints: 6400,
    currentStreak: 0,
    status: 'at-risk'
  },
  {
    id: 'student-4',
    name: 'David Kim',
    email: 'david@example.com',
    enrolledAt: '2024-09-15T00:00:00Z',
    lastActiveAt: '2025-01-19T09:15:00Z',
    labsCompleted: 12,
    labsAttempted: 14,
    averageScore: 88,
    totalPoints: 11800,
    currentStreak: 7,
    status: 'active'
  },
  {
    id: 'student-5',
    name: 'Eva Chen',
    email: 'eva@example.com',
    enrolledAt: '2024-10-01T00:00:00Z',
    lastActiveAt: '2024-12-15T14:00:00Z',
    labsCompleted: 3,
    labsAttempted: 5,
    averageScore: 58,
    totalPoints: 2100,
    currentStreak: 0,
    status: 'inactive'
  },
  {
    id: 'student-6',
    name: 'Frank Williams',
    email: 'frank@example.com',
    enrolledAt: '2024-09-01T00:00:00Z',
    lastActiveAt: '2025-01-19T11:00:00Z',
    labsCompleted: 20,
    labsAttempted: 20,
    averageScore: 95,
    totalPoints: 21000,
    currentStreak: 12,
    status: 'excelling'
  }
]

const mockLabStats: LabCompletionStats[] = [
  {
    labId: 'lab-1',
    labName: 'Linux Foundations',
    totalAttempts: 45,
    completions: 42,
    passRate: 93,
    averageScore: 88,
    averageTimeMinutes: 35,
    failurePoints: [
      { checkpointId: 'cp-1', checkpointName: 'Create user account', failureCount: 3, failureRate: 7 }
    ]
  },
  {
    labId: 'lab-2',
    labName: 'Network Configuration',
    totalAttempts: 38,
    completions: 28,
    passRate: 74,
    averageScore: 72,
    averageTimeMinutes: 55,
    failurePoints: [
      { checkpointId: 'cp-2', checkpointName: 'Configure static IP', failureCount: 8, failureRate: 21 },
      { checkpointId: 'cp-3', checkpointName: 'Enable IP forwarding', failureCount: 6, failureRate: 16 }
    ]
  },
  {
    labId: 'lab-3',
    labName: 'Firewall Configuration',
    totalAttempts: 32,
    completions: 20,
    passRate: 63,
    averageScore: 65,
    averageTimeMinutes: 70,
    failurePoints: [
      { checkpointId: 'cp-4', checkpointName: 'Create allow rule', failureCount: 10, failureRate: 31 },
      { checkpointId: 'cp-5', checkpointName: 'Configure NAT', failureCount: 8, failureRate: 25 },
      { checkpointId: 'cp-6', checkpointName: 'Test connectivity', failureCount: 5, failureRate: 16 }
    ]
  },
  {
    labId: 'lab-4',
    labName: 'Web Server Setup',
    totalAttempts: 28,
    completions: 25,
    passRate: 89,
    averageScore: 85,
    averageTimeMinutes: 40,
    failurePoints: [
      { checkpointId: 'cp-7', checkpointName: 'Configure virtual host', failureCount: 3, failureRate: 11 }
    ]
  }
]

// Generate heatmap data for the last 30 days
function generateHeatmapData(): ActivityHeatmapData[] {
  const data: ActivityHeatmapData[] = []
  const now = new Date()

  for (let d = 0; d < 30; d++) {
    const date = new Date(now)
    date.setDate(date.getDate() - d)
    const dateStr = date.toISOString().split('T')[0]

    // Generate activity for each hour (higher during class hours)
    for (let h = 0; h < 24; h++) {
      let count = 0
      if (h >= 9 && h <= 17) {
        // Peak hours
        count = Math.floor(Math.random() * 15) + 5
      } else if (h >= 18 && h <= 22) {
        // Evening hours
        count = Math.floor(Math.random() * 8) + 2
      } else if (h >= 6 && h <= 8) {
        // Morning hours
        count = Math.floor(Math.random() * 5)
      } else {
        // Night hours
        count = Math.floor(Math.random() * 2)
      }

      if (count > 0) {
        data.push({ date: dateStr ?? '', hour: h, count })
      }
    }
  }

  return data
}

const mockDashboardResponse: InstructorDashboardResponse = {
  classOverview: {
    totalStudents: 6,
    activeStudents: 4,
    inactiveStudents: 1,
    atRiskStudents: 1,
    excellingStudents: 2,
    averageCompletion: 73,
    averageScore: 80,
    totalLabsCompleted: 76
  },
  students: mockStudents,
  labStats: mockLabStats,
  activityHeatmap: generateHeatmapData(),
  recentProgress: [
    {
      studentId: 'student-1',
      studentName: 'Alice Johnson',
      pathwayId: 'pathway-1',
      pathwayName: 'Network Security Fundamentals',
      modulesCompleted: 7,
      totalModules: 8,
      labsCompleted: 18,
      totalLabs: 20,
      percentage: 90,
      lastActivityAt: '2025-01-19T10:30:00Z'
    },
    {
      studentId: 'student-6',
      studentName: 'Frank Williams',
      pathwayId: 'pathway-1',
      pathwayName: 'Network Security Fundamentals',
      modulesCompleted: 8,
      totalModules: 8,
      labsCompleted: 20,
      totalLabs: 20,
      percentage: 100,
      lastActivityAt: '2025-01-19T11:00:00Z'
    },
    {
      studentId: 'student-4',
      studentName: 'David Kim',
      pathwayId: 'pathway-2',
      pathwayName: 'Incident Response',
      modulesCompleted: 5,
      totalModules: 8,
      labsCompleted: 12,
      totalLabs: 16,
      percentage: 75,
      lastActivityAt: '2025-01-19T09:15:00Z'
    }
  ]
}

// ============================================================================
// API
// ============================================================================

export const instructorApi = {
  /**
   * Get instructor dashboard data
   */
  getDashboard: async (organizationId?: string): Promise<InstructorDashboardResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockDashboardResponse
    }
    const params = organizationId ? { organizationId } : {}
    const response = await api.get<InstructorDashboardResponse>('/instructor/dashboard', { params })
    return response.data
  },

  /**
   * Get list of students with detailed stats
   */
  getStudents: async (organizationId?: string): Promise<StudentSummary[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockStudents
    }
    const params = organizationId ? { organizationId } : {}
    const response = await api.get<{ students: StudentSummary[] }>('/instructor/students', { params })
    return response.data.students
  },

  /**
   * Get lab completion statistics
   */
  getLabStats: async (organizationId?: string): Promise<LabCompletionStats[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockLabStats
    }
    const params = organizationId ? { organizationId } : {}
    const response = await api.get<{ labs: LabCompletionStats[] }>('/instructor/lab-stats', { params })
    return response.data.labs
  },

  /**
   * Get activity heatmap data
   */
  getActivityHeatmap: async (days?: number): Promise<ActivityHeatmapData[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return generateHeatmapData()
    }
    const params = days ? { days } : {}
    const response = await api.get<{ data: ActivityHeatmapData[] }>('/instructor/activity-heatmap', { params })
    return response.data.data
  },

  /**
   * Export progress report
   */
  exportReport: async (format: 'csv' | 'json' | 'pdf', organizationId?: string): Promise<Blob> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(1000)
      // Generate mock CSV data
      if (format === 'csv') {
        const csv = [
          'Name,Email,Labs Completed,Average Score,Status',
          ...mockStudents.map(s => `${s.name},${s.email},${s.labsCompleted},${s.averageScore},${s.status}`)
        ].join('\n')
        return new Blob([csv], { type: 'text/csv' })
      } else if (format === 'json') {
        return new Blob([JSON.stringify(mockStudents, null, 2)], { type: 'application/json' })
      }
      // PDF would need a real backend
      throw new Error('PDF export requires backend support')
    }
    const params: Record<string, string> = { format }
    if (organizationId) params['organizationId'] = organizationId
    const response = await api.get<Blob>('/instructor/export', {
      params,
      responseType: 'blob'
    })
    return response.data
  },

  /**
   * Get struggling students (at-risk)
   */
  getStrugglingStudents: async (organizationId?: string): Promise<StudentSummary[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockStudents.filter(s => s.status === 'at-risk' || s.status === 'inactive')
    }
    const params = organizationId ? { organizationId } : {}
    const response = await api.get<{ students: StudentSummary[] }>('/instructor/struggling-students', { params })
    return response.data.students
  }
}
