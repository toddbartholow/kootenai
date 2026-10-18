<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { instructorApi, type InstructorDashboardResponse, type StudentSummary } from '@/api'
import { useFocusRestore } from '@/composables'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import Tabs from '@volt/Tabs.vue'
import TabList from '@volt/TabList.vue'
import Tab from '@volt/Tab.vue'
import TabPanels from '@volt/TabPanels.vue'
import TabPanel from '@volt/TabPanel.vue'
import Dialog from '@volt/Dialog.vue'
import Select from '@volt/Select.vue'
import { useToast } from 'primevue/usetoast'
import Toast from '@volt/Toast.vue'
import { useFormatters } from '@/composables/useFormatters'
import { StudentRoster, StrugglingStudents, LabAnalyticsPanel } from '@/components/instructor'

const { t } = useI18n()
const toast = useToast()
const { formatDate: fmtDate, formatWeekdayShort } = useFormatters()

function formatDate(dateStr?: string): string {
  if (!dateStr) return t('instructorDashboard.formatDate.never')
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (24 * 60 * 60 * 1000))
  if (diffDays === 0) return t('instructorDashboard.formatDate.today')
  if (diffDays === 1) return t('instructorDashboard.formatDate.yesterday')
  if (diffDays < 30) return t('instructorDashboard.formatDate.daysAgo', { count: diffDays })
  return fmtDate(date)
}

// State
const loading = ref(true)
const error = ref<string | null>(null)
const dashboard = ref<InstructorDashboardResponse | null>(null)
const activeTab = ref('0')
const exportDialogVisible = ref(false)
useFocusRestore(exportDialogVisible)
const exportFormat = ref<'csv' | 'json' | 'pdf'>('csv')
const exporting = ref(false)

// Filters
const studentFilter = ref<'all' | 'active' | 'at-risk' | 'inactive' | 'excelling'>('all')
const sortField = ref('name')
const sortOrder = ref(1)

// Load data
onMounted(async () => {
  await loadDashboard()
})

async function loadDashboard() {
  try {
    loading.value = true
    error.value = null
    dashboard.value = await instructorApi.getDashboard()
  } catch (err) {
    console.error('Failed to load instructor dashboard:', err)
    error.value = t('instructorDashboard.loadFailed')
  } finally {
    loading.value = false
  }
}

async function refresh() {
  await loadDashboard()
}

// Computed
const filteredStudents = computed(() => {
  if (!dashboard.value) return []
  let students = [...dashboard.value.students]

  if (studentFilter.value !== 'all') {
    students = students.filter(s => s.status === studentFilter.value)
  }

  return students.sort((a, b) => {
    const aVal = a[sortField.value as keyof StudentSummary]
    const bVal = b[sortField.value as keyof StudentSummary]
    if (typeof aVal === 'string' && typeof bVal === 'string') {
      return sortOrder.value * aVal.localeCompare(bVal)
    }
    if (typeof aVal === 'number' && typeof bVal === 'number') {
      return sortOrder.value * (aVal - bVal)
    }
    return 0
  })
})

const strugglingStudents = computed(() => {
  if (!dashboard.value) return []
  return dashboard.value.students.filter(s => s.status === 'at-risk' || s.status === 'inactive')
})

const labsWithHighFailure = computed(() => {
  if (!dashboard.value) return []
  return dashboard.value.labStats
    .filter(lab => lab.passRate < 70)
    .sort((a, b) => a.passRate - b.passRate)
})

// Activity heatmap processing
const heatmapByDay = computed(() => {
  if (!dashboard.value) return []

  const grouped = new Map<string, number>()
  dashboard.value.activityHeatmap.forEach(item => {
    const current = grouped.get(item.date) || 0
    grouped.set(item.date, current + item.count)
  })

  return Array.from(grouped.entries())
    .map(([date, count]) => ({ date, count }))
    .sort((a, b) => a.date.localeCompare(b.date))
    .slice(-14)
})

const maxActivityCount = computed(() => {
  if (!heatmapByDay.value.length) return 1
  return Math.max(...heatmapByDay.value.map(d => d.count))
})

// Helpers
function getStatusSeverity(status: string): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
  switch (status) {
    case 'excelling':
      return 'success'
    case 'active':
      return 'info'
    case 'at-risk':
      return 'warn'
    case 'inactive':
      return 'danger'
    default:
      return 'secondary'
  }
}

const STATUS_LABEL_KEYS: Record<string, string> = {
  excelling: 'instructorDashboard.statusLabels.excelling',
  active: 'instructorDashboard.statusLabels.active',
  'at-risk': 'instructorDashboard.statusLabels.atRisk',
  inactive: 'instructorDashboard.statusLabels.inactive',
}

function getStatusLabel(status: string): string {
  const key = STATUS_LABEL_KEYS[status]
  return key ? t(key) : status
}

function getPassRateColor(rate: number): 'success' | 'warn' | 'danger' {
  if (rate >= 80) return 'success'
  if (rate >= 60) return 'warn'
  return 'danger'
}

function getHeatmapColor(count: number): string {
  if (count === 0) return 'bg-surface-100 dark:bg-surface-800'
  const intensity = Math.min(count / maxActivityCount.value, 1)
  if (intensity < 0.25) return 'bg-green-200 dark:bg-green-900'
  if (intensity < 0.5) return 'bg-green-400 dark:bg-green-700'
  if (intensity < 0.75) return 'bg-green-500 dark:bg-green-600'
  return 'bg-green-600 dark:bg-green-500'
}

// Export functionality
async function exportReport() {
  try {
    exporting.value = true
    const blob = await instructorApi.exportReport(exportFormat.value)

    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `student-progress-report.${exportFormat.value}`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    toast.add({
      severity: 'success',
      summary: t('instructorDashboard.exportToast.successSummary'),
      detail: t('instructorDashboard.exportToast.successDetail'),
      life: 3000,
    })
    exportDialogVisible.value = false
  } catch (err) {
    console.error('Export failed:', err)
    toast.add({
      severity: 'error',
      summary: t('instructorDashboard.exportToast.failedSummary'),
      detail: t('instructorDashboard.exportToast.failedDetail'),
      life: 5000,
    })
  } finally {
    exporting.value = false
  }
}

const exportFormatOptions = computed(() => [
  { label: t('instructorDashboard.export.formatCsv'), value: 'csv' },
  { label: t('instructorDashboard.export.formatJson'), value: 'json' },
])

defineExpose({
  studentFilter,
  sortField,
  sortOrder,
  exportFormat,
  exporting,
  filteredStudents,
  strugglingStudents,
  labsWithHighFailure,
  heatmapByDay,
  maxActivityCount,
  getStatusSeverity,
  getStatusLabel,
  formatDate,
  getHeatmapColor,
  getPassRateColor,
  refresh,
  exportReport,
})
</script>

<template>
  <Toast />
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ t('instructorDashboard.title') }}
        </h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ t('instructorDashboard.subtitle') }}
        </p>
      </div>
      <div class="flex gap-2">
        <Button
          @click="exportDialogVisible = true"
          icon="pi pi-download"
          :label="t('instructorDashboard.exportAction')"
          severity="secondary"
        />
        <Button @click="refresh" :loading="loading" icon="pi pi-refresh" severity="secondary" />
      </div>
    </div>

    <!-- Error Message -->
    <Message v-if="error" severity="error" :closable="false">
      {{ error }}
    </Message>

    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <template v-else-if="dashboard">
      <!-- Class Overview Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div
                class="w-12 h-12 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center"
              >
                <i class="pi pi-users text-xl text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ dashboard.classOverview.totalStudents }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">
                  {{ t('instructorDashboard.overview.totalStudents') }}
                </div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div
                class="w-12 h-12 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center"
              >
                <i class="pi pi-check-circle text-xl text-green-600 dark:text-green-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ dashboard.classOverview.averageCompletion }}%
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">
                  {{ t('instructorDashboard.overview.avgCompletion') }}
                </div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div
                class="w-12 h-12 rounded-lg bg-yellow-100 dark:bg-yellow-900/30 flex items-center justify-center"
              >
                <i
                  class="pi pi-exclamation-triangle text-xl text-yellow-600 dark:text-yellow-400"
                />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ dashboard.classOverview.atRiskStudents }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">
                  {{ t('instructorDashboard.overview.atRiskStudents') }}
                </div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div
                class="w-12 h-12 rounded-lg bg-accent-100 dark:bg-accent-900/30 flex items-center justify-center"
              >
                <i class="pi pi-star text-xl text-accent-600 dark:text-accent-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ dashboard.classOverview.averageScore }}%
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">
                  {{ t('instructorDashboard.overview.averageScore') }}
                </div>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Main Content Tabs -->
      <Tabs v-model:value="activeTab">
        <TabList>
          <Tab value="0">
            <i class="pi pi-users mr-2" />
            {{ t('instructorDashboard.tabs.students') }}
          </Tab>
          <Tab value="1">
            <i class="pi pi-exclamation-circle mr-2" />
            {{ t('instructorDashboard.tabs.struggling') }}
            <Tag
              v-if="strugglingStudents.length > 0"
              :value="strugglingStudents.length.toString()"
              severity="warn"
              class="ml-2"
            />
          </Tab>
          <Tab value="2">
            <i class="pi pi-chart-bar mr-2" />
            {{ t('instructorDashboard.tabs.labAnalytics') }}
          </Tab>
          <Tab value="3">
            <i class="pi pi-calendar mr-2" />
            {{ t('instructorDashboard.tabs.activity') }}
          </Tab>
        </TabList>

        <TabPanels class="mt-4">
          <!-- Students Tab -->
          <TabPanel value="0">
            <StudentRoster
              :students="filteredStudents"
              :student-filter="studentFilter"
              :get-status-severity="getStatusSeverity"
              :get-status-label="getStatusLabel"
              :format-date="formatDate"
              @update:student-filter="studentFilter = $event"
            />
          </TabPanel>

          <!-- Struggling Students Tab -->
          <TabPanel value="1">
            <StrugglingStudents
              :students="strugglingStudents"
              :get-status-severity="getStatusSeverity"
              :get-status-label="getStatusLabel"
              :format-date="formatDate"
            />
          </TabPanel>

          <!-- Lab Analytics Tab -->
          <TabPanel value="2">
            <LabAnalyticsPanel
              :lab-stats="dashboard.labStats"
              :labs-with-high-failure="labsWithHighFailure"
            />
          </TabPanel>

          <!-- Activity Tab -->
          <TabPanel value="3">
            <Card>
              <template #title>
                <div class="flex items-center gap-2">
                  <i class="pi pi-calendar text-blue-500" />
                  {{ t('instructorDashboard.activity.heatmapHeading') }}
                </div>
              </template>
              <template #content>
                <div class="flex flex-col gap-4">
                  <div class="flex items-end gap-1">
                    <div
                      v-for="day in heatmapByDay"
                      :key="day.date"
                      class="flex flex-col items-center gap-1"
                    >
                      <div
                        class="w-8 h-8 rounded"
                        :class="getHeatmapColor(day.count)"
                        :title="
                          t('instructorDashboard.activity.heatmapTooltip', {
                            date: day.date,
                            count: day.count,
                          })
                        "
                      ></div>
                      <span class="text-[10px] text-surface-500">
                        {{ formatWeekdayShort(day.date) }}
                      </span>
                    </div>
                  </div>

                  <div class="flex items-center gap-2 text-xs text-surface-500">
                    <span>{{ t('instructorDashboard.activity.less') }}</span>
                    <div class="w-4 h-4 rounded bg-surface-100 dark:bg-surface-800"></div>
                    <div class="w-4 h-4 rounded bg-green-200 dark:bg-green-900"></div>
                    <div class="w-4 h-4 rounded bg-green-400 dark:bg-green-700"></div>
                    <div class="w-4 h-4 rounded bg-green-500 dark:bg-green-600"></div>
                    <div class="w-4 h-4 rounded bg-green-600 dark:bg-green-500"></div>
                    <span>{{ t('instructorDashboard.activity.more') }}</span>
                  </div>
                </div>

                <div class="mt-8">
                  <h2 class="font-semibold text-surface-900 dark:text-surface-100 mb-4 text-lg">
                    {{ t('instructorDashboard.activity.recentHeading') }}
                  </h2>
                  <div class="space-y-3">
                    <div
                      v-for="progress in dashboard.recentProgress"
                      :key="`${progress.studentId}-${progress.pathwayId}`"
                      class="flex items-center gap-4 p-3 rounded-lg bg-surface-50 dark:bg-surface-800"
                    >
                      <div class="flex-1 min-w-0">
                        <div class="font-medium text-surface-900 dark:text-surface-100 truncate">
                          {{ progress.studentName }}
                        </div>
                        <div class="text-sm text-surface-500 truncate">
                          {{ progress.pathwayName }}
                        </div>
                      </div>
                      <div class="flex items-center gap-3">
                        <ProgressBar :value="progress.percentage" class="w-24 h-2" />
                        <span class="text-sm font-medium w-10 text-right">
                          {{ progress.percentage }}%
                        </span>
                      </div>
                      <div class="text-xs text-surface-500">
                        {{ formatDate(progress.lastActivityAt) }}
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </Card>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </template>

    <!-- Export Dialog -->
    <Dialog
      v-model:visible="exportDialogVisible"
      :header="t('instructorDashboard.export.header')"
      modal
      :style="{ width: '400px' }"
    >
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
            {{ t('instructorDashboard.export.formatLabel') }}
          </label>
          <Select
            v-model="exportFormat"
            :options="exportFormatOptions"
            option-label="label"
            option-value="value"
            :aria-label="t('instructorDashboard.export.formatAria')"
            class="w-full"
          />
        </div>
        <p class="text-sm text-surface-500">
          {{ t('instructorDashboard.export.description') }}
        </p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button
            :label="t('instructorDashboard.export.cancel')"
            severity="secondary"
            @click="exportDialogVisible = false"
          />
          <Button
            :label="t('instructorDashboard.export.submit')"
            icon="pi pi-download"
            :loading="exporting"
            @click="exportReport"
          />
        </div>
      </template>
    </Dialog>
  </div>
</template>
