<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AchievementCard from '../components/progress/AchievementCard.vue'
import type { Achievement } from '@/types/progress'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Checkbox from '@volt/Checkbox.vue'
import SelectButton from '@volt/SelectButton.vue'
import ProgressSpinner from 'primevue/progressspinner'
import { achievementsApi, type AchievementWithProgress, type UserAchievementSummary } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber } = useFormatters()
const authStore = useAuthStore()

// Loading and error states
const loading = ref(true)
const error = ref<string | null>(null)

// User stats from API
const userStats = ref<UserAchievementSummary | null>(null)

// Achievements data from API
const achievementsData = ref<AchievementWithProgress[]>([])

// Map API type to component category
function mapTypeToCategory(type: string): 'progression' | 'skill' | 'special' {
  switch (type) {
    case 'lab_completion':
    case 'milestone':
    case 'streak':
      return 'progression'
    case 'perfect_score':
    case 'speed':
    case 'category':
      return 'skill'
    case 'special':
    default:
      return 'special'
  }
}

// Map API tier to icon
function mapTierToIcon(_tier: string, type: string): string {
  // Default icons by type
  const typeIcons: Record<string, string> = {
    lab_completion: 'target',
    perfect_score: 'star',
    speed: 'lightning',
    streak: 'fire',
    category: 'book',
    milestone: 'flag',
    special: 'trophy',
  }
  return typeIcons[type] || 'trophy'
}

// Helper to extract achievement data from either flat or nested structure
function getAchievementData(item: AchievementWithProgress) {
  // If nested structure (mock data), use item.achievement
  if (item.achievement) {
    return item.achievement
  }
  // Otherwise, it's flat structure (API response with Go embedded struct)
  return {
    id: item.id || '',
    name: item.name || '',
    description: item.description || '',
    tier: item.tier || 'bronze',
    type: item.type || 'special',
    points: item.points || 0,
  }
}

// Convert API data to component format
const achievements = computed<Achievement[]>(() => {
  return achievementsData.value
    .filter(item => item.achievement || item.id) // Filter out items without achievement data
    .map(item => {
      const achData = getAchievementData(item)
      const achievement: Achievement = {
        id: achData.id,
        name: achData.name,
        description: achData.description,
        icon: mapTierToIcon(achData.tier, achData.type),
        points: achData.points,
        category: mapTypeToCategory(achData.type),
        unlocked: item.earned,
      }
      if (item.earnedAt) {
        achievement.unlockedAt = item.earnedAt
      }
      if (item.progress) {
        achievement.progress = item.progress.currentValue
        achievement.progressMax = item.progress.targetValue
      }
      return achievement
    })
})

// Fetch achievements data
async function fetchAchievements() {
  loading.value = true
  error.value = null

  try {
    const userId = authStore.user?.id || '00000000-0000-0000-0000-000000000001'

    // Fetch both user achievements and summary in parallel, allowing partial failures
    const results = await Promise.allSettled([
      achievementsApi.getUserAchievements(userId),
      achievementsApi.getUserSummary(userId),
    ])

    // Extract achievements (required)
    if (results[0].status === 'fulfilled') {
      achievementsData.value = results[0].value
    } else {
      console.error('Failed to fetch achievements:', results[0].reason)
      achievementsData.value = []
    }

    // Extract summary (optional - can be computed from achievements if unavailable)
    if (results[1].status === 'fulfilled') {
      userStats.value = results[1].value
    } else {
      console.warn('Failed to fetch achievements summary, computing from data')
      // Compute summary from achievements data
      const earned = achievementsData.value.filter(a => a.earned)
      userStats.value = {
        userId,
        totalPoints: earned.reduce((sum, a) => sum + (a.achievement?.points || a.points || 0), 0),
        achievementsEarned: earned.length,
        totalAchievements: achievementsData.value.length,
        tierCounts: { bronze: 0, silver: 0, gold: 0, platinum: 0, diamond: 0 },
        recentAchievements: [],
      }
    }
  } catch (e) {
    console.error('Failed to fetch achievements:', e)
    error.value = e instanceof Error ? e.message : t('achievementsView.loadFailedFallback')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchAchievements()
})

// Computed so labels refresh when the active locale changes.
const categoryOptions = computed(() => [
  { label: t('achievementsView.categoryAll'), value: 'all' },
  { label: t('achievementsView.categoryProgression'), value: 'progression' },
  { label: t('achievementsView.categorySkillFilter'), value: 'skill' },
  { label: t('achievementsView.categorySpecial'), value: 'special' },
])

const selectedCategory = ref('all')
const showUnlockedOnly = ref(false)

const filteredAchievements = computed(() => {
  let filtered = achievements.value

  if (selectedCategory.value !== 'all') {
    filtered = filtered.filter(a => a.category === selectedCategory.value)
  }

  if (showUnlockedOnly.value) {
    filtered = filtered.filter(a => a.unlocked)
  }

  // Sort: unlocked first, then by points
  return filtered.sort((a, b) => {
    if (a.unlocked && !b.unlocked) return -1
    if (!a.unlocked && b.unlocked) return 1
    return b.points - a.points
  })
})

const categoryStats = computed(() => {
  const stats = {
    progression: { unlocked: 0, total: 0 },
    skill: { unlocked: 0, total: 0 },
    special: { unlocked: 0, total: 0 },
  }

  achievements.value.forEach(a => {
    stats[a.category].total++
    if (a.unlocked) stats[a.category].unlocked++
  })

  return stats
})

const totalPointsEarned = computed(() => {
  return achievements.value.filter(a => a.unlocked).reduce((sum, a) => sum + a.points, 0)
})

const _totalPointsPossible = computed(() => {
  return achievements.value.reduce((sum, a) => sum + a.points, 0)
})

// Extracted from the template. As two statements inline, Prettier reformats
// the handler across lines and drops the separator, which the Vue expression
// parser rejects at build time while vue-tsc still passes.
function clearFilters() {
  selectedCategory.value = 'all'
  showUnlockedOnly.value = false
}
</script>

<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div
      v-if="loading"
      class="flex flex-col items-center justify-center py-12"
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <ProgressSpinner :aria-label="t('achievementsView.loadingAria')" />
      <p class="mt-4 text-surface-500">{{ t('achievementsView.loading') }}</p>
    </div>

    <!-- Error State -->
    <Card v-else-if="error" role="alert">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-exclamation-triangle text-4xl text-red-500 mb-4" aria-hidden="true" />
          <h3 class="text-lg font-medium text-surface-900 dark:text-surface-100 mb-2">
            {{ t('achievementsView.errorTitle') }}
          </h3>
          <p class="text-surface-500 mb-4">{{ error }}</p>
          <Button
            @click="fetchAchievements"
            icon="pi pi-refresh"
            :label="t('achievementsView.retryAction')"
            :aria-label="t('achievementsView.retryAria')"
          />
        </div>
      </template>
    </Card>

    <!-- Main Content -->
    <template v-else>
      <!-- Header -->
      <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
            {{ t('achievementsView.title') }}
          </h1>
          <p class="text-surface-600 dark:text-surface-400 mt-1">
            {{ t('achievementsView.subtitle') }}
          </p>
        </div>
        <div class="text-right">
          <p class="text-3xl font-bold text-amber-600 dark:text-amber-400">
            {{ userStats?.achievementsEarned || 0 }}/{{
              userStats?.totalAchievements || achievements.length
            }}
          </p>
          <p class="text-sm text-surface-500">{{ t('achievementsView.unlockedSummary') }}</p>
        </div>
      </div>

      <!-- Stats Cards -->
      <div
        class="grid grid-cols-2 md:grid-cols-4 gap-4"
        role="group"
        :aria-label="t('achievementsView.statsGroupAria')"
      >
        <Card>
          <template #content>
            <div class="flex items-center gap-3">
              <div
                class="w-12 h-12 rounded-full bg-amber-100 dark:bg-amber-900/50 flex items-center justify-center text-2xl"
                aria-hidden="true"
              >
                <i class="pi pi-star-fill text-amber-600 dark:text-amber-400" />
              </div>
              <div>
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ formatNumber(userStats?.totalPoints || totalPointsEarned) }}
                </p>
                <p class="text-xs text-surface-500">{{ t('achievementsView.pointsFromBadges') }}</p>
              </div>
            </div>
          </template>
        </Card>

        <Card
          class="cursor-pointer transition-all hover:ring-2 hover:ring-blue-400 focus-within:ring-2 focus-within:ring-blue-500"
          :class="{ 'ring-2 ring-blue-500': selectedCategory === 'progression' }"
          tabindex="0"
          role="button"
          :aria-pressed="selectedCategory === 'progression'"
          :aria-label="t('achievementsView.categoryProgressionAria')"
          @click="selectedCategory = 'progression'"
          @keydown.enter="selectedCategory = 'progression'"
          @keydown.space.prevent="selectedCategory = 'progression'"
        >
          <template #content>
            <div class="flex items-center gap-3">
              <div
                class="w-12 h-12 rounded-full bg-blue-100 dark:bg-blue-900/50 flex items-center justify-center"
                aria-hidden="true"
              >
                <i class="pi pi-chart-line text-2xl text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <p class="text-lg font-bold text-surface-900 dark:text-surface-100">
                  {{ categoryStats.progression.unlocked }}/{{ categoryStats.progression.total }}
                </p>
                <p class="text-xs text-surface-500">
                  {{ t('achievementsView.categoryProgression') }}
                </p>
              </div>
            </div>
          </template>
        </Card>

        <Card
          class="cursor-pointer transition-all hover:ring-2 hover:ring-green-400 focus-within:ring-2 focus-within:ring-green-500"
          :class="{ 'ring-2 ring-green-500': selectedCategory === 'skill' }"
          tabindex="0"
          role="button"
          :aria-pressed="selectedCategory === 'skill'"
          :aria-label="t('achievementsView.categorySkillAria')"
          @click="selectedCategory = 'skill'"
          @keydown.enter="selectedCategory = 'skill'"
          @keydown.space.prevent="selectedCategory = 'skill'"
        >
          <template #content>
            <div class="flex items-center gap-3">
              <div
                class="w-12 h-12 rounded-full bg-green-100 dark:bg-green-900/50 flex items-center justify-center"
                aria-hidden="true"
              >
                <i class="pi pi-shield text-2xl text-green-600 dark:text-green-400" />
              </div>
              <div>
                <p class="text-lg font-bold text-surface-900 dark:text-surface-100">
                  {{ categoryStats.skill.unlocked }}/{{ categoryStats.skill.total }}
                </p>
                <p class="text-xs text-surface-500">
                  {{ t('achievementsView.categorySkillCard') }}
                </p>
              </div>
            </div>
          </template>
        </Card>

        <Card
          class="cursor-pointer transition-all hover:ring-2 hover:ring-accent-400 focus-within:ring-2 focus-within:ring-accent-500"
          :class="{ 'ring-2 ring-accent-500': selectedCategory === 'special' }"
          tabindex="0"
          role="button"
          :aria-pressed="selectedCategory === 'special'"
          :aria-label="t('achievementsView.categorySpecialAria')"
          @click="selectedCategory = 'special'"
          @keydown.enter="selectedCategory = 'special'"
          @keydown.space.prevent="selectedCategory = 'special'"
        >
          <template #content>
            <div class="flex items-center gap-3">
              <div
                class="w-12 h-12 rounded-full bg-accent-100 dark:bg-accent-900/50 flex items-center justify-center"
                aria-hidden="true"
              >
                <i class="pi pi-sparkles text-2xl text-accent-600 dark:text-accent-400" />
              </div>
              <div>
                <p class="text-lg font-bold text-surface-900 dark:text-surface-100">
                  {{ categoryStats.special.unlocked }}/{{ categoryStats.special.total }}
                </p>
                <p class="text-xs text-surface-500">{{ t('achievementsView.categorySpecial') }}</p>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Filters -->
      <div
        class="flex flex-wrap items-center gap-4"
        role="group"
        :aria-label="t('achievementsView.categoryAriaGroup')"
      >
        <SelectButton
          v-model="selectedCategory"
          :options="categoryOptions"
          optionLabel="label"
          optionValue="value"
          :aria-label="t('achievementsView.categoryFilterAria')"
        />

        <div class="flex items-center gap-2">
          <Checkbox v-model="showUnlockedOnly" inputId="showUnlocked" binary />
          <label
            for="showUnlocked"
            class="text-sm text-surface-600 dark:text-surface-400 cursor-pointer"
          >
            {{ t('achievementsView.showUnlockedOnly') }}
          </label>
        </div>
      </div>

      <!-- Achievements Grid -->
      <div
        class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
        role="list"
        :aria-label="t('achievementsView.gridAria')"
        data-pseudo-skip
      >
        <AchievementCard
          v-for="achievement in filteredAchievements"
          :key="achievement.id"
          :achievement="achievement"
          size="md"
        />
      </div>

      <!-- Empty State -->
      <Card v-if="filteredAchievements.length === 0">
        <template #content>
          <div class="text-center py-12">
            <i class="pi pi-trophy text-5xl text-surface-400 mb-4" aria-hidden="true" />
            <h3 class="text-lg font-medium text-surface-900 dark:text-surface-100">
              {{ t('achievementsView.emptyTitle') }}
            </h3>
            <p class="text-surface-500 mt-1">{{ t('achievementsView.emptyBody') }}</p>
            <Button
              @click="clearFilters"
              :label="t('achievementsView.clearFiltersAction')"
              :aria-label="t('achievementsView.clearFiltersAria')"
              severity="secondary"
              class="mt-4"
            />
          </div>
        </template>
      </Card>
    </template>
  </div>
</template>
