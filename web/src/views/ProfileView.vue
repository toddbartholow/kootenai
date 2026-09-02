<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationStore } from '@/stores/organization'
import { achievementsApi, enrollmentsApi, usersApi, type AchievementWithProgress, type PathwayEnrollment } from '@/api'
import { formatDate } from '@/utils/format'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'

const { t, te } = useI18n()
const authStore = useAuthStore()
const orgStore = useOrganizationStore()

const loading = ref(true)
const achievements = ref<AchievementWithProgress[]>([])
const enrollments = ref<PathwayEnrollment[]>([])
const userCreatedAt = ref<string | null>(null)

const user = computed(() => authStore.user)
const userName = computed(() => user.value?.name || user.value?.email?.split('@')[0] || t('profile.fallbackName'))
const userEmail = computed(() => user.value?.email || '')
const userRole = computed(() => user.value?.roles?.[0] || 'student')
const memberSince = computed(() => userCreatedAt.value ? formatDate(userCreatedAt.value) : '')

/** Resolve role label through profile.roles.*, falling back to the raw value. */
function roleLabel(role: string): string {
  const key = `profile.roles.${role}`
  return te(key) ? t(key) : role
}

const totalPoints = computed(() => enrollments.value.reduce((sum, e) => sum + e.earnedPoints, 0))
const earnedAchievements = computed(() => achievements.value.filter(a => a.earned))
const completedPathways = computed(() => enrollments.value.filter(e => e.completedAt))

onMounted(async () => {
  try {
    const userId = user.value?.id
    if (!userId) return

    const [achievementsData, enrollmentsData, userProfile] = await Promise.all([
      achievementsApi.getUserAchievements(userId).catch(() => []),
      enrollmentsApi.list().catch(() => []),
      usersApi.get(userId).catch(() => null)
    ])

    achievements.value = achievementsData
    enrollments.value = enrollmentsData
    if (userProfile?.createdAt) {
      userCreatedAt.value = userProfile.createdAt
    }
  } catch (err) {
    console.error('Failed to load profile data:', err)
  } finally {
    loading.value = false
  }
})

function getRoleSeverity(role: string): 'info' | 'success' | 'warn' | 'danger' | 'secondary' | 'contrast' {
  switch (role) {
    case 'admin': return 'danger'
    case 'instructor': return 'warn'
    case 'student': return 'info'
    default: return 'secondary'
  }
}
</script>

<template>
  <div class="p-6 max-w-6xl mx-auto">
    <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-0 mb-6">{{ t('profile.title') }}</h1>

    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Profile Card -->
      <Card class="lg:col-span-1">
        <template #content>
          <div class="flex flex-col items-center text-center">
            <div class="w-24 h-24 rounded-full bg-primary-100 dark:bg-primary-900 flex items-center justify-center mb-4">
              <i class="pi pi-user text-4xl text-primary-600 dark:text-primary-400"></i>
            </div>
            <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-0">{{ userName }}</h2>
            <p class="text-surface-600 dark:text-surface-400 mb-2">{{ userEmail }}</p>
            <Tag :value="roleLabel(userRole)" :severity="getRoleSeverity(userRole)" />

            <div class="w-full mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
              <div class="flex justify-between text-sm mb-2">
                <span class="text-surface-600 dark:text-surface-400">{{ t('profile.memberSinceLabel') }}</span>
                <span class="text-surface-900 dark:text-surface-0 font-medium">{{ memberSince }}</span>
              </div>
              <div v-if="orgStore.currentOrganization" class="flex justify-between text-sm">
                <span class="text-surface-600 dark:text-surface-400">{{ t('profile.organizationLabel') }}</span>
                <span class="text-surface-900 dark:text-surface-0 font-medium">{{ orgStore.currentOrganization.name }}</span>
              </div>
            </div>

            <Button
              :label="t('profile.changePasswordAction')"
              icon="pi pi-lock"
              class="mt-6 w-full"
              outlined
              @click="$router.push('/change-password')"
            />
          </div>
        </template>
      </Card>

      <!-- Stats & Progress -->
      <Card class="lg:col-span-2">
        <template #title>{{ t('profile.progressHeading') }}</template>
        <template #content>
          <!-- Stats Grid -->
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <div class="text-center p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <div class="text-2xl font-bold text-primary-600 dark:text-primary-400">{{ totalPoints }}</div>
              <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('profile.stats.totalPoints') }}</div>
            </div>
            <div class="text-center p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ earnedAchievements.length }}</div>
              <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('profile.stats.achievements') }}</div>
            </div>
            <div class="text-center p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ enrollments.length }}</div>
              <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('profile.stats.enrollments') }}</div>
            </div>
            <div class="text-center p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">{{ completedPathways.length }}</div>
              <div class="text-sm text-surface-600 dark:text-surface-400">{{ t('profile.stats.completed') }}</div>
            </div>
          </div>

          <!-- Active Enrollments -->
          <h3 class="text-lg font-semibold text-surface-900 dark:text-surface-0 mb-4">{{ t('profile.enrollments.heading') }}</h3>
          <div v-if="enrollments.length === 0" class="text-center py-8 text-surface-500">
            <i class="pi pi-book text-4xl mb-2"></i>
            <p>{{ t('profile.enrollments.empty') }}</p>
            <Button :label="t('profile.enrollments.browseAction')" icon="pi pi-compass" class="mt-4" @click="$router.push('/pathways')" />
          </div>
          <div v-else class="space-y-4">
            <div
              v-for="enrollment in enrollments.slice(0, 5)"
              :key="enrollment.id"
              class="p-4 bg-surface-50 dark:bg-surface-800 rounded-lg cursor-pointer hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
              role="link"
              tabindex="0"
              @click="$router.push(`/enrollments/${enrollment.id}`)"
              @keydown.enter="$router.push(`/enrollments/${enrollment.id}`)"
              @keydown.space.prevent="$router.push(`/enrollments/${enrollment.id}`)"
            >
              <div class="flex justify-between items-start mb-2">
                <span class="font-medium text-surface-900 dark:text-surface-0">{{ enrollment.pathway?.name || t('profile.enrollments.unknownPathway') }}</span>
                <Tag
                  :value="enrollment.completedAt ? t('profile.enrollments.completedTag') : t('profile.enrollments.inProgressTag')"
                  :severity="enrollment.completedAt ? 'success' : 'info'"
                  size="small"
                />
              </div>
              <ProgressBar :value="enrollment.percentage" :showValue="false" class="h-2" />
              <div class="flex justify-between text-sm mt-2 text-surface-600 dark:text-surface-400">
                <span>{{ t('profile.enrollments.percentLabel', { percent: enrollment.percentage }) }}</span>
                <span>{{ t('profile.enrollments.pointsLabel', { points: enrollment.earnedPoints }) }}</span>
              </div>
            </div>
          </div>
        </template>
      </Card>

      <!-- Recent Achievements -->
      <Card class="lg:col-span-3">
        <template #title>{{ t('profile.achievements.heading') }}</template>
        <template #content>
          <div v-if="earnedAchievements.length === 0" class="text-center py-8 text-surface-500">
            <i class="pi pi-trophy text-4xl mb-2"></i>
            <p>{{ t('profile.achievements.empty') }}</p>
          </div>
          <div v-else class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
            <div
              v-for="achievement in earnedAchievements.slice(0, 6)"
              :key="achievement.id"
              class="flex flex-col items-center text-center p-4 bg-surface-50 dark:bg-surface-800 rounded-lg"
            >
              <div class="w-16 h-16 rounded-full bg-yellow-100 dark:bg-yellow-900 flex items-center justify-center mb-2">
                <i class="pi pi-trophy text-2xl text-yellow-600 dark:text-yellow-400"></i>
              </div>
              <span class="font-medium text-surface-900 dark:text-surface-0 text-sm">{{ achievement.name }}</span>
              <span class="text-xs text-surface-500">{{ t('profile.achievements.pointsSuffix', { points: achievement.points }) }}</span>
            </div>
          </div>
          <div v-if="earnedAchievements.length > 6" class="text-center mt-4">
            <Button :label="t('profile.achievements.viewAllAction')" text @click="$router.push('/achievements')" />
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
