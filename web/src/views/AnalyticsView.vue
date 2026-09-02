<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useAuthStore } from '../stores/auth'
import { formatDate } from '@/utils/format'
import { useFormatters } from '@/composables/useFormatters'
import { api } from '@/api/config'
import type { LeaderboardResponse, ActivityItem } from '@/api/domains/dashboard'

const { t } = useI18n()
const { formatNumber } = useFormatters()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)

// Types
interface PlatformStats {
  totalUsers: number
  totalSessions: number
  totalLabsCompleted: number
  totalAchievementsEarned: number
  activeSessionsToday: number
  avgCompletionRate: number
}

// State
const loading = ref(true)
const error = ref<string | null>(null)
const leaderboard = ref<LeaderboardResponse | null>(null)
const recentActivity = ref<ActivityItem[]>([])
const platformStats = ref<PlatformStats>({
  totalUsers: 0,
  totalSessions: 0,
  totalLabsCompleted: 0,
  totalAchievementsEarned: 0,
  activeSessionsToday: 0,
  avgCompletionRate: 0
})

onMounted(async () => {
  await Promise.all([
    loadLeaderboard(),
    loadActivity(),
    loadPlatformStats()
  ])
  loading.value = false
})

async function loadLeaderboard() {
  try {
    const { data } = await api.get<LeaderboardResponse>('/leaderboard', { params: { limit: 10 } })
    leaderboard.value = data
  } catch (err) {
    console.error('Failed to load leaderboard:', err)
    error.value = t('analytics.loadFailed')
  }
}

async function loadActivity() {
  try {
    const { data } = await api.get<{ activities: ActivityItem[] }>('/activity')
    recentActivity.value = data.activities || []
  } catch (err) {
    console.error('Failed to load activity:', err)
    error.value = t('analytics.loadFailed')
  }
}

async function loadPlatformStats() {
  try {
    // Try to get user count
    if (isAdmin.value) {
      const { data } = await api.get<{ total?: number; users?: unknown[] }>('/users')
      platformStats.value.totalUsers = data.total || data.users?.length || 0
    }

    // Get sessions stats
    const { data: sessionsData } = await api.get<{ sessions?: Array<{ passed?: boolean }> }>('/sessions')
    const sessions = sessionsData.sessions || []
    platformStats.value.totalSessions = sessions.length
    platformStats.value.totalLabsCompleted = sessions.filter((s) => s.passed).length

    // Get achievements stats
    const { data: achievementsData } = await api.get<{ achievements?: unknown[] }>('/achievements/recent')
    platformStats.value.totalAchievementsEarned = achievementsData.achievements?.length || 0

    // Calculate completion rate from leaderboard
    if (leaderboard.value && leaderboard.value.entries.length > 0) {
      const totalLabs = leaderboard.value.entries.reduce((sum, e) => sum + e.labsCompleted, 0)
      platformStats.value.avgCompletionRate = totalLabs / leaderboard.value.entries.length
    }
  } catch (err) {
    console.error('Failed to load platform stats:', err)
    error.value = t('analytics.loadFailed')
  }
}

async function refresh() {
  loading.value = true
  error.value = null
  await Promise.all([
    loadLeaderboard(),
    loadActivity(),
    loadPlatformStats()
  ])
  loading.value = false
}

function getActivityIcon(type: string): string {
  switch (type) {
    case 'lab_completed':
    case 'lab_passed':
      return 'pi pi-check-circle'
    case 'achievement_earned':
      return 'pi pi-star'
    case 'pathway_started':
      return 'pi pi-play'
    case 'module_completed':
      return 'pi pi-flag'
    default:
      return 'pi pi-circle'
  }
}

function getActivityColor(type: string): string {
  switch (type) {
    case 'lab_passed':
      return 'text-green-500'
    case 'lab_completed':
      return 'text-blue-500'
    case 'achievement_earned':
      return 'text-yellow-500'
    case 'pathway_started':
      return 'text-purple-500'
    case 'module_completed':
      return 'text-cyan-500'
    default:
      return 'text-gray-500'
  }
}

// formatDate imported from @/utils/format

function getRankClass(rank: number): string {
  switch (rank) {
    case 1: return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300'
    case 2: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300'
    case 3: return 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300'
    default: return 'bg-surface-100 text-surface-600 dark:bg-surface-800 dark:text-surface-400'
  }
}

function getRankIcon(rank: number): string {
  switch (rank) {
    case 1: return 'pi pi-crown'
    case 2: return 'pi pi-circle-fill'
    case 3: return 'pi pi-circle-fill'
    default: return ''
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('analytics.title') }}</h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ t('analytics.subtitle') }}
        </p>
      </div>
      <Button
        @click="refresh"
        :loading="loading"
        :disabled="loading"
        icon="pi pi-refresh"
        severity="secondary"
      />
    </div>

    <!-- Error Message -->
    <Message v-if="error" severity="error" :closable="false">
      {{ error }}
    </Message>

    <!-- Loading State -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <template v-else>
      <!-- Platform Stats Cards -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card v-if="isAdmin">
          <template #content>
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
                <i class="pi pi-users text-xl text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ platformStats.totalUsers }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('analytics.stats.totalUsers') }}</div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
                <i class="pi pi-check-circle text-xl text-green-600 dark:text-green-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ platformStats.totalLabsCompleted }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('analytics.stats.labsCompleted') }}</div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-lg bg-yellow-100 dark:bg-yellow-900/30 flex items-center justify-center">
                <i class="pi pi-star text-xl text-yellow-600 dark:text-yellow-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ platformStats.totalAchievementsEarned }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('analytics.stats.achievementsEarned') }}</div>
              </div>
            </div>
          </template>
        </Card>

        <Card>
          <template #content>
            <div class="flex items-center gap-4">
              <div class="w-12 h-12 rounded-lg bg-purple-100 dark:bg-purple-900/30 flex items-center justify-center">
                <i class="pi pi-clock text-xl text-purple-600 dark:text-purple-400" />
              </div>
              <div>
                <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ platformStats.totalSessions }}
                </div>
                <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('analytics.stats.totalSessions') }}</div>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Two Column Layout: Leaderboard and Activity -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Leaderboard -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-trophy text-yellow-500" />
              <span>{{ t('analytics.leaderboard.heading') }}</span>
            </div>
          </template>
          <template #content>
            <div v-if="leaderboard && leaderboard.entries.length > 0" class="space-y-3">
              <div
                v-for="entry in leaderboard.entries"
                :key="entry.userId"
                class="flex items-center gap-4 p-3 rounded-lg"
                :class="entry.isCurrentUser ? 'bg-primary-50 dark:bg-primary-900/20' : 'bg-surface-50 dark:bg-surface-800/50'"
              >
                <!-- Rank -->
                <div
                  class="w-10 h-10 rounded-full flex items-center justify-center font-bold"
                  :class="getRankClass(entry.rank)"
                >
                  <i v-if="getRankIcon(entry.rank)" :class="getRankIcon(entry.rank)" class="text-sm" />
                  <span v-else>{{ entry.rank }}</span>
                </div>

                <!-- User Info -->
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-surface-900 dark:text-surface-100 truncate">
                    {{ entry.displayName }}
                    <Tag v-if="entry.isCurrentUser" :value="t('analytics.leaderboard.youTag')" severity="info" class="ml-2" />
                  </div>
                  <div class="text-xs text-surface-500">
                    {{ t('analytics.leaderboard.userStats', { labs: entry.labsCompleted, achievements: entry.achievementCount }) }}
                  </div>
                </div>

                <!-- Points -->
                <div class="text-right">
                  <div class="font-bold text-primary-600 dark:text-primary-400">
                    {{ formatNumber(entry.totalPoints) }}
                  </div>
                  <div class="text-xs text-surface-500">{{ t('analytics.leaderboard.pointsSuffix') }}</div>
                </div>
              </div>

              <!-- Current User (if not in top 10) -->
              <div v-if="leaderboard.currentUser && !leaderboard.entries.find(e => e.isCurrentUser)" class="border-t border-surface-200 dark:border-surface-700 pt-3 mt-3">
                <div class="flex items-center gap-4 p-3 rounded-lg bg-primary-50 dark:bg-primary-900/20">
                  <div class="w-10 h-10 rounded-full flex items-center justify-center font-bold bg-surface-100 text-surface-600 dark:bg-surface-800 dark:text-surface-400">
                    {{ leaderboard.currentUser.rank }}
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="font-medium text-surface-900 dark:text-surface-100 truncate">
                      {{ leaderboard.currentUser.displayName }}
                      <Tag value="You" severity="info" class="ml-2" />
                    </div>
                    <div class="text-xs text-surface-500">
                      {{ t('analytics.leaderboard.userStats', { labs: leaderboard.currentUser.labsCompleted, achievements: leaderboard.currentUser.achievementCount }) }}
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="font-bold text-primary-600 dark:text-primary-400">
                      {{ formatNumber(leaderboard.currentUser.totalPoints) }}
                    </div>
                    <div class="text-xs text-surface-500">{{ t('analytics.leaderboard.pointsSuffix') }}</div>
                  </div>
                </div>
              </div>

              <div class="text-center text-sm text-surface-500 pt-2">
                {{ t('analytics.leaderboard.totalUsers', { count: leaderboard.totalUsers }) }}
              </div>
            </div>

            <div v-else class="text-center py-8">
              <i class="pi pi-trophy text-4xl text-surface-300 mb-4" />
              <p class="text-surface-500">{{ t('analytics.leaderboard.empty') }}</p>
            </div>
          </template>
        </Card>

        <!-- Recent Activity -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-history text-blue-500" />
              <span>{{ t('analytics.activity.heading') }}</span>
            </div>
          </template>
          <template #content>
            <div v-if="recentActivity.length > 0" class="space-y-4">
              <div
                v-for="activity in recentActivity.slice(0, 10)"
                :key="activity.id"
                class="flex gap-4"
              >
                <div
                  class="w-10 h-10 rounded-full flex items-center justify-center bg-surface-100 dark:bg-surface-800"
                >
                  <i
                    :class="[getActivityIcon(activity.type), getActivityColor(activity.type)]"
                    class="text-lg"
                  />
                </div>
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-surface-900 dark:text-surface-100 truncate">
                    {{ activity.title }}
                  </div>
                  <div class="text-sm text-surface-500 truncate">
                    {{ activity.description }}
                  </div>
                  <div class="text-xs text-surface-400 mt-1">
                    {{ formatDate(activity.timestamp) }}
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="text-center py-8">
              <i class="pi pi-history text-4xl text-surface-300 mb-4" />
              <p class="text-surface-500">{{ t('analytics.activity.empty') }}</p>
            </div>
          </template>
        </Card>
      </div>
    </template>
  </div>
</template>
