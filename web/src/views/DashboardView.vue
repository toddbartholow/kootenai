<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useToast } from 'primevue/usetoast'
import {
  labsApi,
  podsApi,
  sessionsApi,
  pathwaysApi,
  enrollmentsApi,
  achievementsApi,
  recommendationsApi,
  dashboardApi,
  type Session,
  type PathwayEnrollment,
  type AchievementWithProgress,
  type Pathway,
  type LabRecommendation,
  type PathwayRecommendation,
} from '@/api'
import { useAuthStore } from '../stores/auth'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import { useDashboardDrag } from '@/composables'
import { formatDate } from '@/utils/format'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import { SkeletonDashboard } from '@/components/common'
import InsightsPanel from '@/components/dashboard/InsightsPanel.vue'
import DashboardWidget from '@/components/dashboard/DashboardWidget.vue'
import AvailableWidgets from '@/components/dashboard/AvailableWidgets.vue'
import DashboardToolbar from '@/components/dashboard/DashboardToolbar.vue'
import DashboardSettingsPopover from '@/components/dashboard/DashboardSettingsPopover.vue'
import DashboardStatsGrid from '@/components/dashboard/DashboardStatsGrid.vue'
import EnrollmentsList from '@/components/dashboard/EnrollmentsList.vue'
import RecommendationsList from '@/components/dashboard/RecommendationsList.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const authStore = useAuthStore()
const layout = useDashboardLayout()
const { locked, addWidget, getSettings } = layout

const drag = useDashboardDrag(layout)
const { announcement } = drag

const settingsPopover = ref<InstanceType<typeof DashboardSettingsPopover> | null>(null)

function openSettings(widgetId: string, event: Event) {
  settingsPopover.value?.open(widgetId, event)
}

// Stats
const stats = ref({
  activePods: 0,
  availableLabs: 0,
  activeSessions: 0,
  totalPoints: 0,
  earnedAchievements: 0,
  totalTimeSpentMins: 0,
})

// Data
const enrollments = ref<PathwayEnrollment[]>([])
const featuredPathways = ref<Pathway[]>([])
const activeSessions = ref<Session[]>([])
const recentAchievements = ref<AchievementWithProgress[]>([])
const labRecommendations = ref<LabRecommendation[]>([])
const pathwayRecommendations = ref<PathwayRecommendation[]>([])

// Loading states
const loading = ref(true)
const error = ref<string | null>(null)

// Current user
const currentUser = computed(() => authStore.user?.id || 'demo-user')
const userName = computed(
  () => authStore.user?.name || authStore.user?.email?.split('@')[0] || 'Learner',
)

function showDenialToastIfRedirected(): void {
  const denied = route.query['denied']
  if (!denied) return
  const required = route.query['required']
  const detail =
    denied === 'role'
      ? `This page requires the ${required} role or higher.`
      : denied === 'feature'
        ? `This page requires the following feature${String(required).includes(',') ? 's' : ''}: ${required}.`
        : 'You do not have permission to view that page.'
  toast.add({
    severity: 'warn',
    summary: 'Access denied',
    detail,
    life: 5000,
  })
  router.replace({ path: route.path, query: {} })
}

onMounted(async () => {
  showDenialToastIfRedirected()
  try {
    const userId = currentUser.value

    const results = await Promise.allSettled([
      labsApi.list(),
      podsApi.list(authStore.user?.id),
      sessionsApi.list(authStore.user?.id, true),
      enrollmentsApi.list(),
      pathwaysApi.list({ featured: true }),
      achievementsApi.getUserAchievements(userId),
      recommendationsApi.get(10),
      dashboardApi.get(),
    ])

    const labsResponse = results[0].status === 'fulfilled' ? results[0].value : { labs: [] }
    const labs = labsResponse.labs
    const pods = results[1].status === 'fulfilled' ? results[1].value : []
    const sessions = results[2].status === 'fulfilled' ? results[2].value : []
    const userEnrollments = results[3].status === 'fulfilled' ? results[3].value : []
    const pathways = results[4].status === 'fulfilled' ? results[4].value : []
    const achievements = results[5].status === 'fulfilled' ? results[5].value : []
    const recommendations =
      results[6].status === 'fulfilled' ? results[6].value : { labs: [], pathways: [] }
    const dashboardData = results[7].status === 'fulfilled' ? results[7].value : null

    results.forEach((result, index) => {
      if (result.status === 'rejected') {
        const names = [
          'labs',
          'pods',
          'sessions',
          'enrollments',
          'pathways',
          'achievements',
          'recommendations',
          'dashboard',
        ]
        console.warn(`Failed to load ${names[index]}:`, result.reason)
      }
    })

    stats.value = {
      availableLabs: labs.length,
      activePods: pods.filter(p => p.status === 'running' || p.status === 'provisioning').length,
      activeSessions: sessions.length,
      totalPoints: userEnrollments.reduce((sum, e) => sum + e.earnedPoints, 0),
      earnedAchievements: achievements.filter(a => a.earned).length,
      totalTimeSpentMins: dashboardData?.stats?.totalTimeSpentMins ?? 0,
    }

    enrollments.value = userEnrollments.slice(0, 10)
    featuredPathways.value = pathways.slice(0, 10)
    activeSessions.value = sessions.slice(0, 10)
    recentAchievements.value = achievements.filter(a => a.earned).slice(0, 10)
    labRecommendations.value = recommendations.labs
    pathwayRecommendations.value = recommendations.pathways
  } catch (err) {
    console.error('Failed to load dashboard:', err)
    error.value = 'Failed to load dashboard data'
  } finally {
    loading.value = false
  }
})

function navigateToSession(session: Session) {
  router.push(`/session/${session.id}`)
}
</script>

<template>
  <div class="space-y-8">
    <!-- Accessibility live region -->
    <div aria-live="assertive" class="sr-only">{{ announcement }}</div>

    <DashboardToolbar :user-name="userName" />

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="false">
      {{ error }}
    </Message>

    <!-- Available Widgets Accordion (when unlocked) -->
    <AvailableWidgets v-if="!locked" :cards="layout.cards.value" @add="addWidget" />

    <!-- Loading state -->
    <SkeletonDashboard v-if="loading" />

    <template v-else>
      <!-- Dashboard Cards Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Stats Grid -->
        <DashboardWidget
          id="stats"
          :label="t('dashboard.widgets.stats')"
          icon="pi-chart-bar"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('stats', $event)"
        >
          <DashboardStatsGrid :stats="stats" />
        </DashboardWidget>

        <!-- Learning Insights -->
        <DashboardWidget
          id="insights"
          :label="t('dashboard.widgets.insights')"
          icon="pi-chart-pie"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('insights', $event)"
        >
          <InsightsPanel />
        </DashboardWidget>

        <!-- My Learning Pathways -->
        <DashboardWidget
          id="enrollments"
          :label="t('dashboard.widgets.enrollments')"
          icon="pi-graduation-cap"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('enrollments', $event)"
        >
          <template #actions>
            <RouterLink to="/pathways" class="text-sm text-primary-500 hover:text-primary-600 mr-1">
              {{ t('dashboard.enrollments.viewAll') }}
            </RouterLink>
          </template>
          <EnrollmentsList
            :enrollments="enrollments"
            :item-count="(getSettings('enrollments')['itemCount'] as number) || 3"
          />
        </DashboardWidget>

        <!-- Recent Achievements -->
        <DashboardWidget
          id="achievements"
          :label="t('dashboard.widgets.achievements')"
          icon="pi-trophy"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('achievements', $event)"
        >
          <template #actions>
            <RouterLink
              to="/achievements"
              class="text-sm text-primary-500 hover:text-primary-600 mr-1"
            >
              {{ t('dashboard.achievements.viewAll') }}
            </RouterLink>
          </template>
          <div>
            <div v-if="recentAchievements.length === 0" class="text-center py-6">
              <i
                class="pi pi-trophy text-3xl text-surface-300 dark:text-surface-600 mb-3"
                aria-hidden="true"
              />
              <p class="text-sm text-surface-500">{{ t('dashboard.achievements.empty') }}</p>
            </div>
            <div v-else class="space-y-3" data-pseudo-skip>
              <div
                v-for="item in recentAchievements.slice(
                  0,
                  (getSettings('achievements')['itemCount'] as number) || 4,
                )"
                :key="item.achievement?.id || item.id"
                class="flex items-center gap-3 p-2 rounded-lg"
              >
                <div
                  class="w-10 h-10 rounded-full bg-amber-400 flex items-center justify-center text-white shadow-md"
                >
                  <i class="pi pi-star" aria-hidden="true" />
                </div>
                <div class="flex-1 min-w-0">
                  <p class="font-medium text-surface-900 dark:text-surface-100 truncate">
                    {{ item.achievement?.name || t('dashboard.achievements.defaultName') }}
                  </p>
                  <p class="text-xs text-surface-500">
                    {{ t('dashboard.achievements.earnedAt', { when: formatDate(item.earnedAt) }) }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </DashboardWidget>

        <!-- Active Lab Sessions -->
        <DashboardWidget
          id="sessions"
          :label="t('dashboard.widgets.sessions')"
          icon="pi-desktop"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('sessions', $event)"
        >
          <template #actions>
            <RouterLink to="/sessions" class="text-sm text-primary-500 hover:text-primary-600 mr-1">
              {{ t('dashboard.sessions.viewAll') }}
            </RouterLink>
          </template>
          <div>
            <div v-if="activeSessions.length === 0" class="text-center py-8">
              <i
                class="pi pi-play-circle text-4xl text-surface-300 dark:text-surface-600 mb-4"
                aria-hidden="true"
              />
              <p class="text-surface-500 mb-4">{{ t('dashboard.sessions.empty') }}</p>
              <RouterLink to="/labs">
                <Button
                  :label="t('dashboard.sessions.startAction')"
                  icon="pi pi-play"
                  size="small"
                />
              </RouterLink>
            </div>
            <ul v-else class="space-y-3" aria-label="Active lab sessions" data-pseudo-skip>
              <li
                v-for="session in activeSessions.slice(
                  0,
                  (getSettings('sessions')['itemCount'] as number) || 3,
                )"
                :key="session.id"
              >
                <article
                  tabindex="0"
                  role="button"
                  class="flex items-center justify-between p-3 rounded-lg bg-surface-50 dark:bg-surface-800 hover:bg-surface-100 dark:hover:bg-surface-700 focus:ring-2 focus:ring-primary-500 cursor-pointer transition-colors"
                  :aria-label="`Lab session, ${session.percentage}% complete, ${session.earnedPoints} of ${session.maxPoints} points`"
                  @click="navigateToSession(session)"
                  @keydown.enter="navigateToSession(session)"
                  @keydown.space.prevent="navigateToSession(session)"
                >
                  <div class="flex items-center gap-3">
                    <div
                      class="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/50 flex items-center justify-center"
                      aria-hidden="true"
                    >
                      <i class="pi pi-desktop text-blue-600 dark:text-blue-400" />
                    </div>
                    <div>
                      <p class="font-medium text-surface-900 dark:text-surface-100">
                        {{ t('dashboard.sessions.itemTitle') }}
                      </p>
                      <p class="text-sm text-surface-500">
                        {{
                          t('dashboard.sessions.startedAt', { when: formatDate(session.startedAt) })
                        }}
                      </p>
                    </div>
                  </div>
                  <div class="text-right">
                    <p class="font-medium text-amber-600 dark:text-amber-400">
                      {{ session.earnedPoints }} / {{ session.maxPoints }} pts
                    </p>
                    <p class="text-sm text-surface-500">{{ session.percentage }}%</p>
                  </div>
                </article>
              </li>
            </ul>
          </div>
        </DashboardWidget>

        <!-- Recommended for You -->
        <DashboardWidget
          id="recommendations"
          :label="t('dashboard.widgets.recommendations')"
          icon="pi-lightbulb"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('recommendations', $event)"
        >
          <RecommendationsList
            :lab-recommendations="labRecommendations"
            :pathway-recommendations="pathwayRecommendations"
            :featured-pathways="featuredPathways"
            :item-count="(getSettings('recommendations')['itemCount'] as number) || 3"
          />
        </DashboardWidget>

        <!-- Quick Actions -->
        <DashboardWidget
          id="quickActions"
          :label="t('dashboard.widgets.quickActions')"
          icon="pi-bolt"
          :layout="layout"
          :drag="drag"
          :has-settings="true"
          @configure="openSettings('quickActions', $event)"
        >
          <div class="space-y-2">
            <RouterLink to="/labs" class="block">
              <Button
                :label="t('dashboard.quickActions.browseLabs')"
                icon="pi pi-book"
                severity="secondary"
                class="w-full justify-start"
              />
            </RouterLink>
            <RouterLink to="/pods" class="block">
              <Button
                :label="t('dashboard.quickActions.myPods')"
                icon="pi pi-server"
                severity="secondary"
                class="w-full justify-start"
              />
            </RouterLink>
            <RouterLink to="/progress" class="block">
              <Button
                :label="t('dashboard.quickActions.viewProgress')"
                icon="pi pi-chart-line"
                severity="secondary"
                class="w-full justify-start"
              />
            </RouterLink>
            <RouterLink to="/pathway-mockup" class="block">
              <Button
                :label="t('dashboard.quickActions.pathwayPreview')"
                icon="pi pi-eye"
                severity="secondary"
                class="w-full justify-start"
              />
            </RouterLink>
          </div>
        </DashboardWidget>
      </div>
    </template>

    <DashboardSettingsPopover ref="settingsPopover" />
  </div>
</template>
