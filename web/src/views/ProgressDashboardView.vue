<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import Chart from '@volt/Chart.vue'
import Select from '@volt/Select.vue'
import AchievementCard from '../components/progress/AchievementCard.vue'
import type { Achievement } from '@/types/progress'
import LeaderboardCard from '../components/progress/LeaderboardCard.vue'
import { useProgressStore } from '@/stores/progress'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber, formatWeekdayMonthDay, formatMonthDay } = useFormatters()

interface Course {
  id: string
  name: string
  icon: string
  totalLabs: number
  completedLabs: number
  totalPoints: number | null
  earnedPoints: number | null
  percentage: number
}

const progressStore = useProgressStore()

// Time period selector
const timePeriod = ref<'week' | 'month' | 'all'>('month')
const timePeriodOptions = computed(() => [
  { label: t('progressDashboard.timeAnalytics.periods.week'), value: 'week' as const },
  { label: t('progressDashboard.timeAnalytics.periods.month'), value: 'month' as const },
  { label: t('progressDashboard.timeAnalytics.periods.all'), value: 'all' as const },
])

// Fetch data on mount
onMounted(async () => {
  await progressStore.fetchAll()
})

// Refetch analytics when period changes
watch(timePeriod, async newPeriod => {
  await progressStore.fetchTimeAnalytics(newPeriod)
})

// Map store data to component-compatible formats
const userStats = computed(() => progressStore.userStats)
const platformStats = computed(() => progressStore.platformStats)
const courses = computed<Course[]>(() => progressStore.enrolledCourses)

const recentAchievements = computed(() => progressStore.recentAchievements.slice(0, 3))

// Mock upcoming achievements for now (would need API support)
const upcomingAchievements = computed<Achievement[]>(() => [
  {
    id: 'ach-upcoming-1',
    name: 'Security Pro',
    description: 'Complete 10 security-tagged labs',
    icon: 'shield',
    points: 750,
    category: 'skill' as const,
    unlocked: false,
    progress: 8,
    progressMax: 10,
  },
  {
    id: 'ach-upcoming-2',
    name: 'Course Champion',
    description: '100% complete any course',
    icon: 'crown',
    points: 1000,
    category: 'progression' as const,
    unlocked: false,
    progress: courses.value[0]?.completedLabs ?? 0,
    progressMax: courses.value[0]?.totalLabs ?? 10,
  },
])

const leaderboard = computed(() => progressStore.leaderboardEntries)

// overallProgress returns null when maxPoints isn't known (API doesn't
// surface a platform ceiling yet). Callers render "N/A" in that case.
const overallProgress = computed<number | null>(() => {
  const stats = userStats.value
  if (stats.maxPoints == null || stats.maxPoints === 0) return null
  return Math.round((stats.totalPoints / stats.maxPoints) * 100)
})

function getCourseProgress(course: Course): number {
  if (course.totalLabs === 0) return 0
  return Math.round((course.completedLabs / course.totalLabs) * 100)
}

const isLoading = computed(() => progressStore.dashboardLoading || progressStore.leaderboardLoading)

// Chart configuration
const documentStyle = getComputedStyle(document.documentElement)
const textColor = computed(() => documentStyle.getPropertyValue('--p-text-color') || '#495057')
const textColorSecondary = computed(
  () => documentStyle.getPropertyValue('--p-text-muted-color') || '#6c757d',
)
const surfaceBorder = computed(
  () => documentStyle.getPropertyValue('--p-surface-border') || '#dee2e6',
)

// Daily activity chart data
const dailyChartData = computed(() => {
  const analytics = progressStore.timeAnalytics
  if (!analytics?.dailyBreakdown?.length) {
    return { labels: [], datasets: [] }
  }

  const sorted = [...analytics.dailyBreakdown].sort((a, b) => a.date.localeCompare(b.date))
  const labels = sorted.map(d => formatWeekdayMonthDay(d.date))
  const minutes = sorted.map(d => d.timeMinutes)

  return {
    labels,
    datasets: [
      {
        label: t('progressDashboard.charts.dataset.minutesSpent'),
        backgroundColor: 'rgba(59, 130, 246, 0.5)',
        borderColor: 'rgb(59, 130, 246)',
        data: minutes,
        tension: 0.3,
        fill: true,
      },
    ],
  }
})

const dailyChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: false,
    },
  },
  scales: {
    x: {
      ticks: { color: textColorSecondary.value },
      grid: { color: surfaceBorder.value, display: false },
    },
    y: {
      ticks: { color: textColorSecondary.value },
      grid: { color: surfaceBorder.value },
      beginAtZero: true,
      title: {
        display: true,
        text: t('progressDashboard.charts.axis.minutes'),
        color: textColorSecondary.value,
      },
    },
  },
}))

// Lab time breakdown chart
const labChartData = computed(() => {
  const analytics = progressStore.timeAnalytics
  if (!analytics?.labBreakdown?.length) {
    return { labels: [], datasets: [] }
  }

  const sorted = [...analytics.labBreakdown]
    .sort((a, b) => b.totalMinutes - a.totalMinutes)
    .slice(0, 5)
  const labels = sorted.map(l => l.labName || l.labSlug || l.labId)
  const minutes = sorted.map(l => l.totalMinutes)

  return {
    labels,
    datasets: [
      {
        label: t('progressDashboard.charts.dataset.timeSpentMin'),
        backgroundColor: [
          'rgba(59, 130, 246, 0.7)',
          'rgba(16, 185, 129, 0.7)',
          'rgba(245, 158, 11, 0.7)',
          'rgba(239, 68, 68, 0.7)',
          'rgba(139, 92, 246, 0.7)',
        ],
        data: minutes,
      },
    ],
  }
})

const labChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  indexAxis: 'y' as const,
  plugins: {
    legend: {
      display: false,
    },
  },
  scales: {
    x: {
      ticks: { color: textColorSecondary.value },
      grid: { color: surfaceBorder.value },
      title: {
        display: true,
        text: t('progressDashboard.charts.axis.minutes'),
        color: textColorSecondary.value,
      },
    },
    y: {
      ticks: { color: textColorSecondary.value },
      grid: { display: false },
    },
  },
}))

// Weekly trend chart
const weeklyChartData = computed(() => {
  const analytics = progressStore.timeAnalytics
  if (!analytics?.weeklyTrend?.length) {
    return { labels: [], datasets: [] }
  }

  const sorted = [...analytics.weeklyTrend].sort((a, b) => a.weekStart.localeCompare(b.weekStart))
  const labels = sorted.map(w =>
    t('progressDashboard.charts.weekOf', { date: formatMonthDay(w.weekStart) }),
  )
  const minutes = sorted.map(w => w.timeMinutes)
  const sessions = sorted.map(w => w.sessionCount)

  return {
    labels,
    datasets: [
      {
        label: t('progressDashboard.charts.dataset.timeMin'),
        backgroundColor: 'rgba(59, 130, 246, 0.7)',
        data: minutes,
        yAxisID: 'y',
      },
      {
        label: t('progressDashboard.charts.dataset.sessions'),
        backgroundColor: 'rgba(16, 185, 129, 0.7)',
        data: sessions,
        yAxisID: 'y1',
      },
    ],
  }
})

const weeklyChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      labels: { color: textColor.value },
    },
  },
  scales: {
    x: {
      ticks: { color: textColorSecondary.value },
      grid: { display: false },
    },
    y: {
      type: 'linear' as const,
      position: 'left' as const,
      ticks: { color: textColorSecondary.value },
      grid: { color: surfaceBorder.value },
      title: {
        display: true,
        text: t('progressDashboard.charts.axis.minutes'),
        color: textColorSecondary.value,
      },
    },
    y1: {
      type: 'linear' as const,
      position: 'right' as const,
      ticks: { color: textColorSecondary.value },
      grid: { display: false },
      title: {
        display: true,
        text: t('progressDashboard.charts.axis.sessions'),
        color: textColorSecondary.value,
      },
    },
  },
}))

// Time analytics summary
const timeAnalyticsSummary = computed(() => {
  const analytics = progressStore.timeAnalytics
  if (!analytics) return null
  return {
    totalHours: Math.round((analytics.totalTimeMinutes / 60) * 10) / 10,
    sessionCount: analytics.sessionCount,
    avgSession: analytics.averageSessionMinutes,
    longestSession: analytics.longestSessionMinutes,
    pointsPerMinute: analytics.stats.pointsPerMinute?.toFixed(2) || '0',
    mostActiveDay: analytics.stats.mostActiveDay || t('common.notAvailable'),
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
        {{ t('progressDashboard.title') }}
      </h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">
        {{ t('progressDashboard.subtitle') }}
      </p>
    </div>

    <!-- Loading State -->
    <div v-if="isLoading" class="flex items-center justify-center py-12">
      <i class="pi pi-spin pi-spinner text-4xl text-primary-500" />
    </div>

    <template v-else>
      <!-- User Stats Overview -->
      <div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-trophy text-3xl text-amber-500 mb-1" />
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                #{{ userStats.rank || '-' }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{ t('progressDashboard.stats.rankOf', { total: userStats.totalRanked }) }}
              </p>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-star-fill text-3xl text-blue-500 mb-1" />
              <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">
                {{ formatNumber(userStats.totalPoints) }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{ t('progressDashboard.stats.totalPoints') }}
              </p>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-verified text-3xl text-amber-500 mb-1" />
              <p class="text-2xl font-bold text-amber-600 dark:text-amber-400">
                {{ userStats.achievementsUnlocked }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{ t('progressDashboard.stats.badgesOf', { total: userStats.totalAchievements }) }}
              </p>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-book text-3xl text-surface-500 dark:text-surface-400 mb-1" />
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                {{ userStats.coursesCompleted }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{ t('progressDashboard.stats.coursesOf', { total: userStats.totalCourses }) }}
              </p>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-flask text-3xl text-surface-500 dark:text-surface-400 mb-1" />
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                {{ userStats.labsCompleted }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{
                  userStats.totalLabs != null
                    ? t('progressDashboard.stats.labsOf', { total: userStats.totalLabs })
                    : t('progressDashboard.stats.labsCompletedLabel')
                }}
              </p>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="text-center">
              <i class="pi pi-bolt text-3xl text-orange-500 mb-1" />
              <p class="text-2xl font-bold text-orange-600 dark:text-orange-400">
                {{ userStats.currentStreak }}
              </p>
              <p class="text-xs text-surface-500 dark:text-surface-400">
                {{ t('progressDashboard.stats.dayStreak') }}
              </p>
            </div>
          </template>
        </Card>
      </div>

      <!-- Overall Progress — hides the percent + ProgressBar when maxPoints
           is unknown (API doesn't surface a platform ceiling); shows an
           earned-only line instead of a fabricated "X / X" ratio. -->
      <Card>
        <template #content>
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-0">
              {{ t('progressDashboard.overall.heading') }}
            </h2>
            <span class="text-2xl font-bold text-blue-600 dark:text-blue-400">
              {{
                overallProgress != null
                  ? t('progressDashboard.platformStats.percent', { value: overallProgress })
                  : t('progressDashboard.overall.unavailable')
              }}
            </span>
          </div>
          <ProgressBar v-if="overallProgress != null" :value="overallProgress" :showValue="false" />
          <p class="text-sm text-surface-500 dark:text-surface-400 mt-2">
            {{
              userStats.maxPoints != null
                ? t('progressDashboard.overall.pointsEarned', {
                    earned: formatNumber(userStats.totalPoints),
                    max: formatNumber(userStats.maxPoints),
                  })
                : t('progressDashboard.overall.pointsEarnedUnknown', {
                    earned: formatNumber(userStats.totalPoints),
                  })
            }}
          </p>
        </template>
      </Card>

      <!-- Time Analytics Section -->
      <Card>
        <template #title>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <i class="pi pi-clock text-xl" />
              {{ t('progressDashboard.timeAnalytics.heading') }}
            </div>
            <Select
              v-model="timePeriod"
              :options="timePeriodOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('progressDashboard.timeAnalytics.periodAria')"
              class="w-40"
            />
          </div>
        </template>
        <template #content>
          <!-- Analytics Summary -->
          <div
            v-if="timeAnalyticsSummary"
            class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-6"
          >
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">
                {{
                  t('progressDashboard.timeAnalytics.summary.hours', {
                    hours: timeAnalyticsSummary.totalHours,
                  })
                }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.totalTime') }}
              </p>
            </div>
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                {{ timeAnalyticsSummary.sessionCount }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.sessions') }}
              </p>
            </div>
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                {{
                  t('progressDashboard.timeAnalytics.summary.minutesShort', {
                    minutes: timeAnalyticsSummary.avgSession,
                  })
                }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.avgSession') }}
              </p>
            </div>
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-surface-900 dark:text-surface-0">
                {{
                  t('progressDashboard.timeAnalytics.summary.minutesShort', {
                    minutes: timeAnalyticsSummary.longestSession,
                  })
                }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.longest') }}
              </p>
            </div>
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-green-600 dark:text-green-400">
                {{ timeAnalyticsSummary.pointsPerMinute }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.pointsPerMinute') }}
              </p>
            </div>
            <div class="text-center p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <p class="text-2xl font-bold text-accent-600 dark:text-accent-400" data-pseudo-skip>
                {{ timeAnalyticsSummary.mostActiveDay }}
              </p>
              <p class="text-xs text-surface-500">
                {{ t('progressDashboard.timeAnalytics.summary.mostActive') }}
              </p>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Daily Activity Chart -->
            <div>
              <h3 class="text-sm font-medium text-surface-600 dark:text-surface-400 mb-3">
                {{ t('progressDashboard.charts.dailyActivity') }}
              </h3>
              <div class="h-64">
                <Chart
                  v-if="dailyChartData.labels?.length"
                  type="line"
                  :data="dailyChartData"
                  :options="dailyChartOptions"
                />
                <div v-else class="flex items-center justify-center h-full text-surface-400">
                  <p>{{ t('progressDashboard.charts.empty.daily') }}</p>
                </div>
              </div>
            </div>

            <!-- Lab Time Breakdown Chart -->
            <div>
              <h3 class="text-sm font-medium text-surface-600 dark:text-surface-400 mb-3">
                {{ t('progressDashboard.charts.topLabs') }}
              </h3>
              <div class="h-64">
                <Chart
                  v-if="labChartData.labels?.length"
                  type="bar"
                  :data="labChartData"
                  :options="labChartOptions"
                />
                <div v-else class="flex items-center justify-center h-full text-surface-400">
                  <p>{{ t('progressDashboard.charts.empty.lab') }}</p>
                </div>
              </div>
            </div>
          </div>

          <!-- Weekly Trend Chart -->
          <div class="mt-6">
            <h3 class="text-sm font-medium text-surface-600 dark:text-surface-400 mb-3">
              {{ t('progressDashboard.charts.weeklyTrend') }}
            </h3>
            <div class="h-64">
              <Chart
                v-if="weeklyChartData.labels?.length"
                type="bar"
                :data="weeklyChartData"
                :options="weeklyChartOptions"
              />
              <div v-else class="flex items-center justify-center h-full text-surface-400">
                <p>{{ t('progressDashboard.charts.empty.weekly') }}</p>
              </div>
            </div>
          </div>
        </template>
      </Card>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Courses Column -->
        <div class="lg:col-span-2 space-y-6">
          <!-- Courses Progress -->
          <Card>
            <template #title>
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <i class="pi pi-book text-xl" />
                  {{ t('progressDashboard.courses.heading') }}
                </div>
                <RouterLink
                  to="/progress/courses"
                  class="text-sm text-primary-500 hover:underline font-normal"
                >
                  {{ t('progressDashboard.courses.viewAll') }}
                </RouterLink>
              </div>
            </template>
            <template #content>
              <div
                v-if="courses.length === 0"
                class="text-center py-8 text-surface-500 dark:text-surface-400"
              >
                <i class="pi pi-book text-4xl mb-2" />
                <p>{{ t('progressDashboard.courses.empty') }}</p>
                <RouterLink
                  to="/pathways"
                  class="text-primary-500 hover:underline mt-2 inline-block"
                >
                  {{ t('progressDashboard.courses.browsePathways') }}
                </RouterLink>
              </div>
              <!-- Course rows render API names (e.g. "Incident Response"); skip pseudo check. -->
              <div
                v-else
                class="divide-y divide-surface-100 dark:divide-surface-800 -mx-4"
                data-pseudo-skip
              >
                <RouterLink
                  v-for="course in courses"
                  :key="course.id"
                  :to="`/progress/course/${course.id}`"
                  class="flex items-center gap-4 p-4 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
                >
                  <div
                    class="flex-shrink-0 w-12 h-12 rounded-xl bg-surface-100 dark:bg-surface-700 flex items-center justify-center text-2xl"
                  >
                    <i :class="['pi', course.icon]" aria-hidden="true"></i>
                  </div>
                  <div class="flex-1 min-w-0">
                    <h3 class="font-medium text-surface-900 dark:text-surface-0 truncate">
                      {{ course.name }}
                    </h3>
                    <div class="flex items-center gap-4 mt-1">
                      <span class="text-sm text-surface-500 dark:text-surface-400">
                        {{
                          t('progressDashboard.courses.labsProgress', {
                            completed: course.completedLabs,
                            total: course.totalLabs,
                          })
                        }}
                      </span>
                      <!-- earnedPoints is null when the API doesn't return a
                           per-pathway points value; omit the pts chip rather
                           than display a made-up number. -->
                      <span
                        v-if="course.earnedPoints != null"
                        class="text-sm text-blue-600 dark:text-blue-400"
                      >
                        {{
                          t('progressDashboard.courses.pointsShort', {
                            points: formatNumber(course.earnedPoints),
                          })
                        }}
                      </span>
                    </div>
                    <div class="mt-2">
                      <ProgressBar :value="getCourseProgress(course)" :showValue="false" />
                    </div>
                  </div>
                  <div class="flex-shrink-0 text-right">
                    <p class="text-lg font-bold text-surface-900 dark:text-surface-0">
                      {{
                        t('progressDashboard.platformStats.percent', {
                          value: getCourseProgress(course),
                        })
                      }}
                    </p>
                  </div>
                </RouterLink>
              </div>
            </template>
          </Card>

          <!-- Achievements Section -->
          <Card>
            <template #title>
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <i class="pi pi-verified text-xl" />
                  {{ t('progressDashboard.achievements.heading') }}
                </div>
                <RouterLink
                  to="/achievements"
                  class="text-sm text-primary-500 hover:underline font-normal"
                >
                  {{
                    t('progressDashboard.achievements.viewAllCount', {
                      unlocked: userStats.achievementsUnlocked,
                      total: userStats.totalAchievements,
                    })
                  }}
                </RouterLink>
              </div>
            </template>
            <template #content>
              <div
                v-if="recentAchievements.length === 0"
                class="text-center py-8 text-surface-500 dark:text-surface-400"
              >
                <i class="pi pi-verified text-4xl mb-2" />
                <p>{{ t('progressDashboard.achievements.empty') }}</p>
              </div>
              <template v-else>
                <h3 class="text-sm font-medium text-surface-500 mb-3">
                  {{ t('progressDashboard.achievements.recentlyUnlocked') }}
                </h3>
                <!-- Achievement cards render API/mock names + descriptions. -->
                <div class="grid grid-cols-1 md:grid-cols-3 gap-3" data-pseudo-skip>
                  <AchievementCard
                    v-for="achievement in recentAchievements"
                    :key="achievement.id"
                    :achievement="achievement"
                    size="sm"
                  />
                </div>

                <h3 class="text-sm font-medium text-surface-500 mt-6 mb-3">
                  {{ t('progressDashboard.achievements.almostThere') }}
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3" data-pseudo-skip>
                  <AchievementCard
                    v-for="achievement in upcomingAchievements"
                    :key="achievement.id"
                    :achievement="achievement"
                    size="sm"
                  />
                </div>
              </template>
            </template>
          </Card>
        </div>

        <!-- Leaderboard Column -->
        <div class="space-y-6">
          <!-- Leaderboard entries come from the API and are user names; skip pseudo check. -->
          <div data-pseudo-skip>
            <LeaderboardCard
              :entries="leaderboard"
              :title="t('progressDashboard.leaderboardTitle')"
              :show-achievements="true"
            />
          </div>

          <!-- Platform Stats -->
          <Card>
            <template #title>
              <div class="flex items-center gap-2">
                <i class="pi pi-chart-bar text-xl" />
                {{ t('progressDashboard.platformStats.heading') }}
              </div>
            </template>
            <template #content>
              <div class="space-y-3">
                <div class="flex justify-between">
                  <span class="text-surface-500 dark:text-surface-400">{{
                    t('progressDashboard.platformStats.totalUsers')
                  }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-0">{{
                    platformStats.totalUsers
                  }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-surface-500 dark:text-surface-400">{{
                    t('progressDashboard.platformStats.activeLabs')
                  }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-0">{{
                    platformStats.activeLabs
                  }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-surface-500 dark:text-surface-400">{{
                    t('progressDashboard.platformStats.labCompletions')
                  }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-0">{{
                    formatNumber(platformStats.totalCompletions)
                  }}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-surface-500 dark:text-surface-400">{{
                    t('progressDashboard.platformStats.avgScore')
                  }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-0">{{
                    t('progressDashboard.platformStats.percent', {
                      value: platformStats.averageScore,
                    })
                  }}</span>
                </div>
              </div>
            </template>
          </Card>
        </div>
      </div>
    </template>
  </div>
</template>
