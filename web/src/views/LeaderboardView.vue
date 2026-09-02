<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import SelectButton from '@volt/SelectButton.vue'
import ProgressSpinner from 'primevue/progressspinner'
import { dashboardApi, type LeaderboardResponse } from '@/api'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber } = useFormatters()

// Loading and error states
const loading = ref(true)
const error = ref<string | null>(null)

// Leaderboard data
const leaderboardData = ref<LeaderboardResponse | null>(null)

// Time range filter — computed so labels refresh when the active locale changes.
const timeRange = ref('all')
const timeRangeOptions = computed(() => [
  { label: t('leaderboardView.timeRangeAll'), value: 'all' },
  { label: t('leaderboardView.timeRangeMonth'), value: 'month' },
  { label: t('leaderboardView.timeRangeWeek'), value: 'week' },
])

// Number of entries to show
const limit = ref(25)

// Computed entries with current user highlighted
const entries = computed(() => leaderboardData.value?.entries || [])
const currentUser = computed(() => leaderboardData.value?.currentUser)
const totalUsers = computed(() => leaderboardData.value?.totalUsers || 0)

// Check if current user is in the visible list
const currentUserInList = computed(() =>
  entries.value.some(e => e.isCurrentUser)
)

// Fetch leaderboard data
async function fetchLeaderboard() {
  loading.value = true
  error.value = null

  try {
    leaderboardData.value = await dashboardApi.getLeaderboard(limit.value)
  } catch (e) {
    console.error('Failed to fetch leaderboard:', e)
    error.value = e instanceof Error ? e.message : t('leaderboardView.loadFailed')
  } finally {
    loading.value = false
  }
}

// Refresh with new limit
function loadMore() {
  limit.value += 25
  fetchLeaderboard()
}

onMounted(() => {
  fetchLeaderboard()
})

// Get tier color based on rank
function getTierColor(rank: number): string {
  if (rank === 1) return 'text-yellow-500'
  if (rank === 2) return 'text-gray-400'
  if (rank === 3) return 'text-amber-600'
  if (rank <= 10) return 'text-blue-500'
  return 'text-surface-500'
}
</script>

<template>
  <div class="p-6 max-w-5xl mx-auto">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-0 flex items-center gap-2">
          <span class="text-3xl">🏆</span>
          {{ t('leaderboardView.title') }}
        </h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ t('leaderboardView.subtitle') }}
        </p>
      </div>

      <!-- Time range filter (currently visual only - API doesn't support yet) -->
      <SelectButton
        v-model="timeRange"
        :options="timeRangeOptions"
        optionLabel="label"
        optionValue="value"
        :allowEmpty="false"
        class="hidden sm:flex"
      />
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center items-center py-20">
      <ProgressSpinner />
    </div>

    <!-- Error State -->
    <Card v-else-if="error" class="bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800">
      <template #content>
        <div class="text-center py-8">
          <p class="text-red-600 dark:text-red-400 mb-4">{{ error }}</p>
          <Button :label="t('leaderboardView.tryAgain')" @click="fetchLeaderboard" />
        </div>
      </template>
    </Card>

    <!-- Leaderboard Content -->
    <div v-else>
      <!-- Stats Summary -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <Card class="bg-gradient-to-br from-yellow-50 to-amber-50 dark:from-yellow-900/20 dark:to-amber-900/20 border-yellow-200 dark:border-yellow-800">
          <template #content>
            <div class="text-center">
              <p class="text-3xl font-bold text-yellow-600 dark:text-yellow-400">{{ totalUsers }}</p>
              <p class="text-sm text-surface-600 dark:text-surface-400">{{ t('leaderboardView.statTotalLearners') }}</p>
            </div>
          </template>
        </Card>

        <Card v-if="currentUser" class="bg-gradient-to-br from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 border-blue-200 dark:border-blue-800">
          <template #content>
            <div class="text-center">
              <p class="text-3xl font-bold text-blue-600 dark:text-blue-400">#{{ currentUser.rank }}</p>
              <p class="text-sm text-surface-600 dark:text-surface-400">{{ t('leaderboardView.statYourRank') }}</p>
            </div>
          </template>
        </Card>

        <Card v-if="currentUser" class="bg-gradient-to-br from-purple-50 to-pink-50 dark:from-purple-900/20 dark:to-pink-900/20 border-purple-200 dark:border-purple-800">
          <template #content>
            <div class="text-center">
              <p class="text-3xl font-bold text-purple-600 dark:text-purple-400">{{ formatNumber(currentUser.totalPoints) }}</p>
              <p class="text-sm text-surface-600 dark:text-surface-400">{{ t('leaderboardView.statYourPoints') }}</p>
            </div>
          </template>
        </Card>
      </div>

      <!-- Main Leaderboard Table -->
      <Card>
        <template #content>
          <div class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="border-b border-surface-200 dark:border-surface-700">
                  <th class="text-left py-3 px-4 font-semibold text-surface-600 dark:text-surface-400 w-16">{{ t('leaderboardView.columnRank') }}</th>
                  <th class="text-left py-3 px-4 font-semibold text-surface-600 dark:text-surface-400">{{ t('leaderboardView.columnLearner') }}</th>
                  <th class="text-center py-3 px-4 font-semibold text-surface-600 dark:text-surface-400 hidden sm:table-cell">{{ t('leaderboardView.columnLabs') }}</th>
                  <th class="text-center py-3 px-4 font-semibold text-surface-600 dark:text-surface-400 hidden md:table-cell">{{ t('leaderboardView.columnAchievements') }}</th>
                  <th class="text-right py-3 px-4 font-semibold text-surface-600 dark:text-surface-400">{{ t('leaderboardView.columnPoints') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-surface-100 dark:divide-surface-800" data-pseudo-skip>
                <tr
                  v-for="entry in entries"
                  :key="entry.userId"
                  class="transition-colors hover:bg-surface-50 dark:hover:bg-surface-800"
                  :class="{ 'bg-blue-50 dark:bg-blue-900/30': entry.isCurrentUser }"
                >
                  <!-- Rank -->
                  <td class="py-4 px-4">
                    <div class="flex items-center justify-center w-8 h-8 rounded-full" :class="{
                      'bg-gradient-to-br from-yellow-400 to-amber-500': entry.rank === 1,
                      'bg-gradient-to-br from-gray-300 to-gray-400': entry.rank === 2,
                      'bg-gradient-to-br from-amber-500 to-orange-600': entry.rank === 3,
                    }">
                      <span v-if="entry.rank <= 3" class="text-white font-bold text-sm">{{ entry.rank }}</span>
                      <span v-else class="font-bold text-surface-500 dark:text-surface-400">#{{ entry.rank }}</span>
                    </div>
                  </td>

                  <!-- User -->
                  <td class="py-4 px-4">
                    <div class="flex items-center gap-3">
                      <div class="w-10 h-10 rounded-full bg-gradient-to-br from-blue-400 to-purple-500 flex items-center justify-center text-white font-bold flex-shrink-0">
                        {{ entry.displayName?.charAt(0)?.toUpperCase() || t('leaderboardView.avatarFallback') }}
                      </div>
                      <div class="min-w-0">
                        <p class="font-medium text-surface-900 dark:text-surface-0 truncate">
                          {{ entry.displayName }}
                          <span v-if="entry.isCurrentUser" class="text-xs text-blue-600 dark:text-blue-400 ml-1">{{ t('leaderboardView.youTag') }}</span>
                        </p>
                      </div>
                    </div>
                  </td>

                  <!-- Labs Completed -->
                  <td class="py-4 px-4 text-center hidden sm:table-cell">
                    <span class="text-surface-700 dark:text-surface-300">{{ entry.labsCompleted }}</span>
                  </td>

                  <!-- Achievements -->
                  <td class="py-4 px-4 text-center hidden md:table-cell">
                    <span class="text-surface-700 dark:text-surface-300">{{ entry.achievementCount }}</span>
                  </td>

                  <!-- Points -->
                  <td class="py-4 px-4 text-right">
                    <span class="font-bold text-lg" :class="getTierColor(entry.rank)">
                      {{ formatNumber(entry.totalPoints) }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Load More Button -->
          <div v-if="entries.length >= limit" class="mt-4 pt-4 border-t border-surface-200 dark:border-surface-700 text-center">
            <Button
              :label="t('leaderboardView.loadMore')"
              icon="pi pi-chevron-down"
              outlined
              @click="loadMore"
              :loading="loading"
            />
          </div>

          <!-- Empty State -->
          <div v-if="entries.length === 0" class="text-center py-12">
            <span class="text-5xl mb-4 block">🏆</span>
            <p class="text-surface-600 dark:text-surface-400">{{ t('leaderboardView.emptyTitle') }}</p>
            <p class="text-sm text-surface-500 dark:text-surface-500 mt-2">
              {{ t('leaderboardView.emptyBody') }}
            </p>
          </div>
        </template>
      </Card>

      <!-- Current User Card (if not in visible list) -->
      <Card v-if="currentUser && !currentUserInList" class="mt-6 bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800">
        <template #content>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-full bg-gradient-to-br from-blue-400 to-purple-500 flex items-center justify-center text-white font-bold text-lg">
                {{ currentUser.displayName?.charAt(0)?.toUpperCase() || '?' }}
              </div>
              <div>
                <p class="font-semibold text-surface-900 dark:text-surface-0">{{ t('leaderboardView.yourPositionHeading') }}</p>
                <p class="text-sm text-surface-600 dark:text-surface-400">
                  {{ t('leaderboardView.labsCompleted', { count: currentUser.labsCompleted }) }}
                </p>
              </div>
            </div>
            <div class="text-right">
              <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">#{{ currentUser.rank }}</p>
              <p class="text-surface-600 dark:text-surface-400">{{ t('leaderboardView.pointsSuffix', { points: formatNumber(currentUser.totalPoints) }) }}</p>
            </div>
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
