/**
 * Tests for InstructorDashboardView component
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { InstructorDashboardResponse } from '@/api'

// Mock instructorApi
const mockGetDashboard = vi.fn()
const mockExportReport = vi.fn()

vi.mock('@/api', () => ({
  instructorApi: {
    getDashboard: () => mockGetDashboard(),
    exportReport: (format: string) => mockExportReport(format),
  },
}))

// Mock PrimeVue useToast
const mockToastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({
  useToast: () => ({
    add: mockToastAdd,
  }),
}))

// Import component after mocks
import InstructorDashboardView from './InstructorDashboardView.vue'

// Sample test data
const sampleDashboard: InstructorDashboardResponse = {
  classOverview: {
    totalStudents: 25,
    activeStudents: 18,
    inactiveStudents: 3,
    atRiskStudents: 4,
    excellingStudents: 6,
    averageCompletion: 72,
    averageScore: 78,
    totalLabsCompleted: 150,
  },
  students: [
    {
      id: 'student-1',
      name: 'Alice Johnson',
      email: 'alice@example.com',
      enrolledAt: '2024-09-01T00:00:00Z',
      labsCompleted: 8,
      labsAttempted: 10,
      averageScore: 92,
      status: 'excelling',
      currentStreak: 5,
      lastActiveAt: new Date().toISOString(),
      totalPoints: 1500,
    },
    {
      id: 'student-2',
      name: 'Bob Smith',
      email: 'bob@example.com',
      enrolledAt: '2024-09-01T00:00:00Z',
      labsCompleted: 5,
      labsAttempted: 8,
      averageScore: 65,
      status: 'active',
      currentStreak: 2,
      lastActiveAt: new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(),
      totalPoints: 800,
    },
    {
      id: 'student-3',
      name: 'Charlie Brown',
      email: 'charlie@example.com',
      enrolledAt: '2024-09-01T00:00:00Z',
      labsCompleted: 2,
      labsAttempted: 6,
      averageScore: 45,
      status: 'at-risk',
      currentStreak: 0,
      lastActiveAt: new Date(Date.now() - 14 * 24 * 60 * 60 * 1000).toISOString(),
      totalPoints: 300,
    },
    {
      id: 'student-4',
      name: 'Diana Prince',
      email: 'diana@example.com',
      enrolledAt: '2024-10-01T00:00:00Z',
      labsCompleted: 1,
      labsAttempted: 2,
      averageScore: 55,
      status: 'inactive',
      currentStreak: 0,
      lastActiveAt: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
      totalPoints: 150,
    },
  ],
  labStats: [
    {
      labId: 'lab-1',
      labName: 'Firewall Configuration',
      totalAttempts: 50,
      completions: 45,
      passRate: 90,
      averageScore: 85,
      averageTimeMinutes: 45,
      failurePoints: [],
    },
    {
      labId: 'lab-2',
      labName: 'Intrusion Detection',
      totalAttempts: 40,
      completions: 25,
      passRate: 62,
      averageScore: 68,
      averageTimeMinutes: 60,
      failurePoints: [
        { checkpointId: 'cp-1', checkpointName: 'Configure Snort Rules', failureCount: 12, failureRate: 30 },
        { checkpointId: 'cp-2', checkpointName: 'Analyze Alert Logs', failureCount: 8, failureRate: 20 },
      ],
    },
    {
      labId: 'lab-3',
      labName: 'Incident Response',
      totalAttempts: 30,
      completions: 15,
      passRate: 50,
      averageScore: 55,
      averageTimeMinutes: 90,
      failurePoints: [
        { checkpointId: 'cp-3', checkpointName: 'Identify Attack Vector', failureCount: 10, failureRate: 33 },
      ],
    },
  ],
  activityHeatmap: [
    { date: '2025-01-10', hour: 10, count: 15 },
    { date: '2025-01-10', hour: 14, count: 20 },
    { date: '2025-01-11', hour: 9, count: 8 },
    { date: '2025-01-11', hour: 15, count: 12 },
    { date: '2025-01-12', hour: 10, count: 25 },
  ],
  recentProgress: [
    {
      studentId: 'student-1',
      studentName: 'Alice Johnson',
      pathwayId: 'pathway-1',
      pathwayName: 'Security Fundamentals',
      modulesCompleted: 6,
      totalModules: 8,
      labsCompleted: 17,
      totalLabs: 20,
      percentage: 85,
      lastActivityAt: new Date().toISOString(),
    },
    {
      studentId: 'student-2',
      studentName: 'Bob Smith',
      pathwayId: 'pathway-2',
      pathwayName: 'Network Defense',
      modulesCompleted: 4,
      totalModules: 8,
      labsCompleted: 12,
      totalLabs: 20,
      percentage: 60,
      lastActivityAt: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
    },
  ],
}

// Global stubs for PrimeVue components
const globalStubs = {
  Card: {
    template: '<div class="card"><slot name="title" /><slot name="content" /></div>',
  },
  Button: {
    template: '<button @click="$emit(\'click\')" :disabled="loading"><slot />{{ label }}</button>',
    props: ['label', 'icon', 'severity', 'size', 'loading'],
  },
  Tag: {
    template: '<span class="tag" :class="severity">{{ value }}</span>',
    props: ['value', 'severity'],
  },
  Message: {
    template: '<div class="message" :class="severity"><slot /></div>',
    props: ['severity', 'closable'],
  },
  ProgressSpinner: {
    template: '<div class="progress-spinner">Loading...</div>',
  },
  ProgressBar: {
    template: '<div class="progress-bar" :style="{ width: value + \'%\' }">{{ value }}%</div>',
    props: ['value'],
  },
  DataTable: {
    template: '<table class="data-table"><tbody></tbody></table>',
    props: ['value', 'paginator', 'rows', 'rowsPerPageOptions', 'sortable'],
  },
  Column: {
    template: '<th>{{ header }}</th>',
    props: ['field', 'header', 'sortable'],
  },
  Tabs: {
    template: '<div class="tabs"><slot /></div>',
    props: ['value'],
  },
  TabList: {
    template: '<div class="tab-list"><slot /></div>',
  },
  Tab: {
    template: '<button class="tab" @click="$emit(\'click\')"><slot /></button>',
    props: ['value'],
  },
  TabPanels: {
    template: '<div class="tab-panels"><slot /></div>',
  },
  TabPanel: {
    template: '<div class="tab-panel"><slot /></div>',
    props: ['value'],
  },
  Dialog: {
    template: '<div v-if="visible" class="dialog"><slot /><slot name="footer" /></div>',
    props: ['visible', 'header', 'modal', 'style'],
  },
  Select: {
    template: '<select v-model="modelValue"><option v-for="opt in options" :key="opt.value" :value="opt.value">{{ opt.label }}</option></select>',
    props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
  },
  Toast: {
    template: '<div class="toast"></div>',
  },
}

describe('InstructorDashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockGetDashboard.mockResolvedValue(sampleDashboard)
    mockExportReport.mockResolvedValue(new Blob(['test'], { type: 'text/csv' }))
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('mounting and loading', () => {
    it('mounts successfully', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()
      expect(wrapper.exists()).toBe(true)
    })

    it('shows loading spinner initially', async () => {
      // Make the API call hang
      mockGetDashboard.mockImplementation(() => new Promise(() => {}))

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      expect(wrapper.find('.progress-spinner').exists()).toBe(true)
    })

    it('calls getDashboard on mount', async () => {
      mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()
      expect(mockGetDashboard).toHaveBeenCalledTimes(1)
    })

    it('displays dashboard content after loading', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      // Loading spinner should be gone
      expect(wrapper.find('.progress-spinner').exists()).toBe(false)
      // Dashboard content should be visible
      expect(wrapper.text()).toContain('Instructor Dashboard')
    })
  })

  describe('error handling', () => {
    it('displays error message when API fails', async () => {
      mockGetDashboard.mockRejectedValue(new Error('Network error'))

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const errorMessage = wrapper.find('.message.error')
      expect(errorMessage.exists()).toBe(true)
      expect(errorMessage.text()).toContain('Failed to load dashboard data')
    })
  })

  describe('class overview display', () => {
    it('displays total students count', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('25')
      expect(wrapper.text()).toContain('Total Students')
    })

    it('displays average completion percentage', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('72%')
      expect(wrapper.text()).toContain('Avg Completion')
    })

    it('displays at-risk students count', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('4')
      expect(wrapper.text()).toContain('At-Risk Students')
    })

    it('displays average score', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('78%')
      expect(wrapper.text()).toContain('Average Score')
    })
  })

  describe('computed properties', () => {
    it('computes filteredStudents correctly for all filter', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.filteredStudents.length).toBe(4)
    })

    it('computes strugglingStudents correctly', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      // Should include at-risk and inactive students (Charlie and Diana)
      expect(vm.strugglingStudents.length).toBe(2)
      expect(vm.strugglingStudents.every(s => s.status === 'at-risk' || s.status === 'inactive')).toBe(true)
    })

    it('computes labsWithHighFailure correctly', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      // Labs with pass rate < 70: Intrusion Detection (62%) and Incident Response (50%)
      expect(vm.labsWithHighFailure.length).toBe(2)
      expect(vm.labsWithHighFailure[0]!.passRate).toBe(50) // Sorted by pass rate ascending
      expect(vm.labsWithHighFailure[1]!.passRate).toBe(62)
    })

    it('computes heatmapByDay correctly', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      // Should group by day and sum counts
      expect(vm.heatmapByDay.length).toBe(3)
      expect(vm.heatmapByDay.find(d => d.date === '2025-01-10')?.count).toBe(35) // 15 + 20
      expect(vm.heatmapByDay.find(d => d.date === '2025-01-11')?.count).toBe(20) // 8 + 12
      expect(vm.heatmapByDay.find(d => d.date === '2025-01-12')?.count).toBe(25)
    })

    it('computes maxActivityCount correctly', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.maxActivityCount).toBe(35)
    })
  })

  describe('helper functions', () => {
    it('getStatusSeverity returns correct values', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.getStatusSeverity('excelling')).toBe('success')
      expect(vm.getStatusSeverity('active')).toBe('info')
      expect(vm.getStatusSeverity('at-risk')).toBe('warn')
      expect(vm.getStatusSeverity('inactive')).toBe('danger')
      expect(vm.getStatusSeverity('unknown')).toBe('secondary')
    })

    it('getStatusLabel returns correct labels', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.getStatusLabel('excelling')).toBe('Excelling')
      expect(vm.getStatusLabel('active')).toBe('Active')
      expect(vm.getStatusLabel('at-risk')).toBe('At Risk')
      expect(vm.getStatusLabel('inactive')).toBe('Inactive')
      expect(vm.getStatusLabel('unknown')).toBe('unknown')
    })

    it('formatDate returns correct relative dates', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.formatDate(undefined)).toBe('Never')
      expect(vm.formatDate(new Date().toISOString())).toBe('Today')
      expect(vm.formatDate(new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString())).toBe('Yesterday')
      expect(vm.formatDate(new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString())).toBe('3 days ago')
    })

    it('getHeatmapColor returns correct classes', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.getHeatmapColor(0)).toContain('bg-surface')
      expect(vm.getHeatmapColor(Math.floor(vm.maxActivityCount * 0.1))).toContain('bg-green-200')
      expect(vm.getHeatmapColor(Math.floor(vm.maxActivityCount * 0.3))).toContain('bg-green-400')
      expect(vm.getHeatmapColor(Math.floor(vm.maxActivityCount * 0.6))).toContain('bg-green-500')
      expect(vm.getHeatmapColor(vm.maxActivityCount)).toContain('bg-green-600')
    })

    it('getPassRateColor returns correct severity', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.getPassRateColor(90)).toBe('success')
      expect(vm.getPassRateColor(80)).toBe('success')
      expect(vm.getPassRateColor(70)).toBe('warn')
      expect(vm.getPassRateColor(60)).toBe('warn')
      expect(vm.getPassRateColor(50)).toBe('danger')
    })
  })

  describe('refresh functionality', () => {
    it('calls getDashboard when refresh is triggered', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()
      expect(mockGetDashboard).toHaveBeenCalledTimes(1)

      const vm = wrapper.vm

      await vm.refresh()
      expect(mockGetDashboard).toHaveBeenCalledTimes(2)
    })
  })

  describe('export functionality', () => {
    it('calls exportReport with correct format', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.exportFormat = 'json'
      await vm.exportReport()

      expect(mockExportReport).toHaveBeenCalledWith('json')
    })

    it('shows success toast on successful export', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      await vm.exportReport()

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'success',
          summary: 'Export Complete',
        })
      )
    })

    it('shows error toast on export failure', async () => {
      mockExportReport.mockRejectedValue(new Error('Export failed'))

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      await vm.exportReport()

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'error',
          summary: 'Export Failed',
        })
      )
    })

    it('sets exporting state during export', async () => {
      let resolveExport: (value: Blob) => void
      mockExportReport.mockImplementation(() => new Promise(resolve => {
        resolveExport = resolve
      }))

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.exporting).toBe(false)

      const exportPromise = vm.exportReport()
      await flushPromises()
      expect(vm.exporting).toBe(true)

      resolveExport!(new Blob(['test'], { type: 'text/csv' }))
      await exportPromise
      await flushPromises()
      expect(vm.exporting).toBe(false)
    })
  })

  describe('student filtering', () => {
    it('filters students by active status', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.studentFilter = 'active'
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents.length).toBe(1)
      expect(vm.filteredStudents[0]!.name).toBe('Bob Smith')
    })

    it('filters students by excelling status', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.studentFilter = 'excelling'
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents.length).toBe(1)
      expect(vm.filteredStudents[0]!.name).toBe('Alice Johnson')
    })

    it('filters students by at-risk status', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.studentFilter = 'at-risk'
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents.length).toBe(1)
      expect(vm.filteredStudents[0]!.name).toBe('Charlie Brown')
    })

    it('filters students by inactive status', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.studentFilter = 'inactive'
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents.length).toBe(1)
      expect(vm.filteredStudents[0]!.name).toBe('Diana Prince')
    })
  })

  describe('student sorting', () => {
    it('sorts students by name ascending', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.sortField = 'name'
      vm.sortOrder = 1
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents[0]!.name).toBe('Alice Johnson')
      expect(vm.filteredStudents[3]!.name).toBe('Diana Prince')
    })

    it('sorts students by name descending', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.sortField = 'name'
      vm.sortOrder = -1
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents[0]!.name).toBe('Diana Prince')
      expect(vm.filteredStudents[3]!.name).toBe('Alice Johnson')
    })

    it('sorts students by averageScore', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      vm.sortField = 'averageScore'
      vm.sortOrder = -1 // Descending
      await wrapper.vm.$nextTick()

      expect(vm.filteredStudents[0]!.averageScore).toBe(92)
      expect(vm.filteredStudents[3]!.averageScore).toBe(45)
    })
  })

  describe('empty state handling', () => {
    it('handles empty students list', async () => {
      mockGetDashboard.mockResolvedValue({
        ...sampleDashboard,
        students: [],
      })

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.filteredStudents.length).toBe(0)
      expect(vm.strugglingStudents.length).toBe(0)
    })

    it('handles no struggling students', async () => {
      mockGetDashboard.mockResolvedValue({
        ...sampleDashboard,
        students: [sampleDashboard.students[0]], // Only excelling student
      })

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.strugglingStudents.length).toBe(0)
      expect(wrapper.text()).toContain('All students are on track')
    })

    it('handles all labs with good pass rates', async () => {
      mockGetDashboard.mockResolvedValue({
        ...sampleDashboard,
        labStats: [sampleDashboard.labStats[0]], // Only the one with 90% pass rate
      })

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.labsWithHighFailure.length).toBe(0)
    })

    it('handles empty heatmap data', async () => {
      mockGetDashboard.mockResolvedValue({
        ...sampleDashboard,
        activityHeatmap: [],
      })

      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      const vm = wrapper.vm

      expect(vm.heatmapByDay.length).toBe(0)
      expect(vm.maxActivityCount).toBe(1) // Default to 1 to avoid division by zero
    })
  })

  describe('header elements', () => {
    it('displays page title', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('Instructor Dashboard')
    })

    it('displays page description', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('Monitor student progress')
    })

    it('has export and refresh buttons', async () => {
      const wrapper = mount(InstructorDashboardView, {
        global: {
          stubs: globalStubs,
          directives: { tooltip: {} },
        },
      })

      await flushPromises()

      expect(wrapper.text()).toContain('Export Report')
      // Refresh button doesn't have a label, it's just an icon
    })
  })
})
