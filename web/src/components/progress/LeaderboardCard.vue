<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { LeaderboardEntry } from '@/types/progress'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber } = useFormatters()

defineProps<{
  entries: LeaderboardEntry[]
  title?: string
  showAchievements?: boolean
}>()
</script>

<template>
  <div
    class="bg-surface-0 dark:bg-surface-900 rounded-xl shadow-sm border border-surface-200 dark:border-surface-700"
  >
    <div class="p-4 border-b border-surface-200 dark:border-surface-700">
      <h3 class="font-semibold text-surface-900 dark:text-surface-0 flex items-center gap-2">
        <i class="pi pi-trophy text-xl text-amber-500" aria-hidden="true"></i>
        {{ title || t('leaderboardCard.title') }}
      </h3>
    </div>
    <div class="divide-y divide-surface-100 dark:divide-surface-800">
      <div
        v-for="entry in entries"
        :key="entry.userId"
        class="flex items-center gap-4 p-4 transition-colors"
        :class="{
          'bg-blue-50 dark:bg-blue-900/30': entry.isCurrentUser,
        }"
      >
        <!-- Rank -->
        <div class="flex-shrink-0 w-8 text-center">
          <i
            v-if="entry.rank <= 3"
            class="pi pi-trophy text-xl"
            :class="{
              'text-amber-500': entry.rank === 1,
              'text-surface-400': entry.rank === 2,
              'text-orange-700 dark:text-orange-500': entry.rank === 3,
            }"
            :data-rank="entry.rank"
            aria-hidden="true"
          ></i>
          <span v-else class="text-lg font-bold text-surface-500 dark:text-surface-400"
            >#{{ entry.rank }}</span
          >
        </div>

        <!-- Avatar & Name -->
        <div class="flex items-center gap-3 flex-1 min-w-0">
          <div
            class="w-10 h-10 rounded-full bg-blue-400 flex items-center justify-center text-white font-bold"
          >
            {{ entry.displayName.charAt(0).toUpperCase() }}
          </div>
          <div class="min-w-0">
            <p class="font-medium text-surface-900 dark:text-surface-0 truncate">
              {{ entry.displayName }}
              <span
                v-if="entry.isCurrentUser"
                class="text-xs text-blue-600 dark:text-blue-400 ml-1"
                >{{ t('leaderboardCard.youTag') }}</span
              >
            </p>
            <p class="text-xs text-surface-500 dark:text-surface-400">
              {{ t('leaderboardCard.labsCompleted', { count: entry.labsCompleted }) }}
            </p>
          </div>
        </div>

        <!-- Achievements -->
        <div v-if="showAchievements" class="flex-shrink-0 text-center">
          <p class="text-sm font-medium text-surface-900 dark:text-surface-0">
            {{ entry.achievementCount }}
          </p>
          <p class="text-xs text-surface-500 dark:text-surface-400">
            {{ t('leaderboardCard.badges') }}
          </p>
        </div>

        <!-- Points -->
        <div class="flex-shrink-0 text-right">
          <p class="text-lg font-bold text-surface-900 dark:text-surface-0">
            {{ formatNumber(entry.points) }}
          </p>
          <p class="text-xs text-surface-500 dark:text-surface-400">
            {{ t('leaderboardCard.points') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
