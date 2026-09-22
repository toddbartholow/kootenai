<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { useFormatters } from '@/composables/useFormatters'

defineProps<{
  stats: {
    totalPoints: number
    earnedAchievements: number
    totalTimeSpentMins: number
    activeSessions: number
    activePods: number
    availableLabs: number
  }
}>()

const { t } = useI18n()
const { formatNumber } = useFormatters()
const themeStore = useThemeStore()

function formatLearningTime(minutes: number): string {
  if (minutes === 0) return '0m'
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours === 0) return `${mins}m`
  if (mins === 0) return `${hours}h`
  return `${hours}h ${mins}m`
}
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
    <!-- Total Points -->
    <RouterLink
      to="/progress"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-amber-600/30 hover:border-amber-500'
          : 'bg-amber-50 border-amber-200 hover:border-amber-400'
      "
    >
      <div class="text-center">
        <i
          class="pi pi-star-fill text-2xl text-amber-500 dark:text-amber-400 mb-2"
          aria-hidden="true"
        />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ formatNumber(stats.totalPoints) }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.totalPoints') }}
        </p>
      </div>
    </RouterLink>

    <!-- Achievements -->
    <RouterLink
      to="/achievements"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-accent-600/30 hover:border-accent-500'
          : 'bg-accent-50 border-accent-200 hover:border-accent-400'
      "
    >
      <div class="text-center">
        <i
          class="pi pi-trophy text-2xl text-accent-500 dark:text-accent-400 mb-2"
          aria-hidden="true"
        />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ stats.earnedAchievements }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.achievements') }}
        </p>
      </div>
    </RouterLink>

    <!-- Learning Time -->
    <RouterLink
      to="/progress"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-teal-600/30 hover:border-teal-500'
          : 'bg-teal-50 border-teal-200 hover:border-teal-400'
      "
    >
      <div class="text-center">
        <i class="pi pi-clock text-2xl text-teal-500 dark:text-teal-400 mb-2" aria-hidden="true" />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ formatLearningTime(stats.totalTimeSpentMins) }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.learningTime') }}
        </p>
      </div>
    </RouterLink>

    <!-- Active Sessions -->
    <RouterLink
      to="/sessions"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-blue-600/30 hover:border-blue-500'
          : 'bg-blue-50 border-blue-200 hover:border-blue-400'
      "
    >
      <div class="text-center">
        <i
          class="pi pi-desktop text-2xl text-blue-500 dark:text-blue-400 mb-2"
          aria-hidden="true"
        />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ stats.activeSessions }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.activeSessions') }}
        </p>
      </div>
    </RouterLink>

    <!-- Active Pods -->
    <RouterLink
      to="/pods"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-green-600/30 hover:border-green-500'
          : 'bg-green-50 border-green-200 hover:border-green-400'
      "
    >
      <div class="text-center">
        <i
          class="pi pi-server text-2xl text-green-500 dark:text-green-400 mb-2"
          aria-hidden="true"
        />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ stats.activePods }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.runningPods') }}
        </p>
      </div>
    </RouterLink>

    <!-- Available Labs -->
    <RouterLink
      to="/labs"
      class="flex flex-col rounded-lg p-5 border cursor-pointer transition-colors"
      :class="
        themeStore.isDark
          ? 'bg-surface-900 border-rose-600/30 hover:border-rose-500'
          : 'bg-rose-50 border-rose-200 hover:border-rose-400'
      "
    >
      <div class="text-center">
        <i class="pi pi-book text-2xl text-rose-500 dark:text-rose-400 mb-2" aria-hidden="true" />
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ stats.availableLabs }}
        </p>
        <p class="text-sm text-surface-500 dark:text-surface-400">
          {{ t('dashboard.stats.availableLabs') }}
        </p>
      </div>
    </RouterLink>
  </div>
</template>
