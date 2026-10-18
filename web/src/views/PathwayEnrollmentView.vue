<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import {
  enrollmentsApi,
  podsApi,
  type PathwayEnrollment,
  type ModuleProgress,
  type LabProgress,
} from '@/api'
import { useAuthStore } from '../stores/auth'
import { formatDate, formatDuration } from '@/utils/format'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber } = useFormatters()
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import Accordion from '@volt/Accordion.vue'
import AccordionPanel from '@volt/AccordionPanel.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const enrollment = ref<PathwayEnrollment | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const launchingLab = ref<string | null>(null)

// Get enrollment ID from route
const enrollmentId = computed(() => route.params['id'] as string)

// Get modules from enrollment's moduleProgress
const modules = computed<ModuleProgress[]>(() => {
  return enrollment.value?.moduleProgress || []
})

// Get pathway name from enrollment's pathway
const pathwayName = computed(() => {
  return enrollment.value?.pathway?.name || t('pathway.enrollment.fallbackPathwayName')
})

onMounted(async () => {
  try {
    // getProgress returns full enrollment with moduleProgress
    const enrollmentData = await enrollmentsApi.getProgress(enrollmentId.value)
    enrollment.value = enrollmentData
  } catch (err) {
    console.error('Failed to load enrollment:', err)
    error.value = t('pathway.enrollment.loadFailed')
  } finally {
    loading.value = false
  }
})

async function launchLab(labTemplateId: string, labName: string, moduleId: string) {
  try {
    launchingLab.value = labTemplateId
    error.value = null

    // Create a pod for this lab using the template ID
    const userId = authStore.user?.id || authStore.user?.email || 'current-user'
    const pod = await podsApi.create(labTemplateId, userId)

    // Navigate to the pod view with pathway context
    // This allows the session to be linked to the pathway enrollment
    router.push({
      path: `/pods/${pod.id}`,
      query: {
        enrollmentId: enrollmentId.value,
        moduleId: moduleId,
      },
    })
  } catch (err) {
    console.error('Failed to launch lab:', err)
    error.value = t('pathway.enrollment.launchFailed', { name: labName })
  } finally {
    launchingLab.value = null
  }
}

function viewLabDetails(labTemplateId: string) {
  router.push(`/labs/${labTemplateId}`)
}

function getModuleStatusSeverity(
  status: string,
): 'success' | 'warn' | 'danger' | 'secondary' | 'info' | 'contrast' | undefined {
  switch (status) {
    case 'completed':
      return 'success'
    case 'in_progress':
      return 'info'
    case 'unlocked':
      return 'warn'
    case 'locked':
      return 'secondary'
    default:
      return 'secondary'
  }
}

const MODULE_STATUS_KEYS: Record<string, string> = {
  completed: 'pathway.enrollment.statusCompleted',
  in_progress: 'pathway.enrollment.statusInProgress',
  unlocked: 'pathway.enrollment.statusUnlocked',
  locked: 'pathway.enrollment.statusLocked',
}

function getModuleStatusLabel(status: string): string {
  const key = MODULE_STATUS_KEYS[status]
  return key ? t(key) : status
}

function getLabStatusIcon(passed: boolean, attemptCount: number): string {
  if (passed) return 'pi-check-circle'
  if (attemptCount > 0) return 'pi-exclamation-circle'
  return 'pi-circle'
}

function getLabStatusClass(passed: boolean, attemptCount: number): string {
  if (passed) return 'text-green-500'
  if (attemptCount > 0) return 'text-amber-500'
  return 'text-surface-400'
}

// formatDate, formatDuration imported from @/utils/format

// Get labs from a module (either labProgress or labs field)
function getModuleLabs(module: ModuleProgress): LabProgress[] {
  return module.labProgress || module.labs || []
}

// Calculate which modules should be expanded (in_progress or first unlocked)
const expandedModules = computed(() => {
  const expanded: string[] = []
  for (const module of modules.value) {
    if (module.status === 'in_progress') {
      expanded.push(module.moduleId)
    }
  }
  // If none in progress, expand first unlocked
  if (expanded.length === 0) {
    const firstUnlocked = modules.value.find((m: ModuleProgress) => m.status === 'unlocked')
    if (firstUnlocked) {
      expanded.push(firstUnlocked.moduleId)
    }
  }
  return expanded
})

// Find next recommended lab
const nextLab = computed(() => {
  for (const module of modules.value) {
    if (module.status === 'locked') continue
    const labs = getModuleLabs(module)
    for (const lab of labs) {
      if (!lab.passed) {
        return {
          moduleId: module.moduleId,
          moduleName: module.moduleName || module.module?.name || 'Module',
          lab,
        }
      }
    }
  }
  return null
})

// Calculate estimated time remaining based on incomplete labs
const estimatedTimeRemaining = computed(() => {
  let totalMinutes = 0
  for (const module of modules.value) {
    if (module.status === 'completed') continue
    const labs = getModuleLabs(module)
    for (const lab of labs) {
      if (!lab.passed) {
        totalMinutes += lab.durationMinutes || 30 // Default to 30 mins if not specified
      }
    }
  }
  return totalMinutes
})
</script>

<template>
  <div class="space-y-6">
    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Error message -->
    <Message v-else-if="error && !enrollment" severity="error">
      {{ error }}
      <template #closeicon>
        <Button
          @click="router.push('/pathways')"
          :label="t('pathway.enrollment.backAction')"
          severity="secondary"
          size="small"
          class="ml-4"
        />
      </template>
    </Message>

    <template v-else-if="enrollment">
      <!-- Breadcrumb -->
      <div class="flex items-center gap-2 text-sm text-surface-500">
        <router-link to="/pathways" class="hover:text-surface-700 dark:hover:text-surface-300">
          {{ t('pathway.enrollment.breadcrumbList') }}
        </router-link>
        <i class="pi pi-chevron-right text-xs" />
        <span class="text-surface-700 dark:text-surface-300">{{ pathwayName }}</span>
      </div>

      <!-- Header with Progress -->
      <div class="flex flex-col lg:flex-row gap-6">
        <div class="flex-1 space-y-4">
          <div>
            <Tag
              :value="
                enrollment.status === 'completed'
                  ? t('pathway.enrollment.statusCompleted')
                  : enrollment.status === 'in_progress'
                    ? t('pathway.enrollment.statusInProgress')
                    : t('pathway.enrollment.statusEnrolled')
              "
              :severity="
                enrollment.status === 'completed'
                  ? 'success'
                  : enrollment.status === 'in_progress'
                    ? 'info'
                    : 'secondary'
              "
              class="mb-2"
            />
            <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
              {{ pathwayName }}
            </h1>
          </div>

          <!-- Overall Progress Bar -->
          <div class="space-y-2">
            <div class="flex justify-between text-sm">
              <span class="text-surface-600 dark:text-surface-400">{{
                t('pathway.enrollment.overallProgress')
              }}</span>
              <span class="font-semibold text-surface-900 dark:text-surface-100">
                {{ Math.round(enrollment.percentage) }}%
              </span>
            </div>
            <ProgressBar :value="enrollment.percentage" :showValue="false" class="h-3" />
          </div>

          <!-- Stats Row -->
          <div class="flex flex-wrap gap-6 text-sm">
            <div class="flex items-center gap-2">
              <i class="pi pi-box text-primary-500" />
              <span class="text-surface-600 dark:text-surface-400">
                {{
                  t('pathway.enrollment.moduleProgressStat', {
                    completed: enrollment.completedModules,
                    total: enrollment.totalModules,
                  })
                }}
              </span>
            </div>
            <div class="flex items-center gap-2">
              <i class="pi pi-star-fill text-amber-500" />
              <span class="text-surface-600 dark:text-surface-400">
                {{
                  t('pathway.enrollment.pointsStat', {
                    earned: formatNumber(enrollment.earnedPoints),
                    total: formatNumber(enrollment.maxPoints),
                  })
                }}
              </span>
            </div>
            <div class="flex items-center gap-2">
              <i class="pi pi-calendar text-surface-400" />
              <span class="text-surface-600 dark:text-surface-400">
                {{
                  t('pathway.enrollment.enrolledSince', { when: formatDate(enrollment.enrolledAt) })
                }}
              </span>
            </div>
            <div v-if="estimatedTimeRemaining > 0" class="flex items-center gap-2">
              <i class="pi pi-clock text-blue-500" />
              <span class="text-surface-600 dark:text-surface-400">
                {{
                  t('pathway.enrollment.timeRemaining', {
                    duration: formatDuration(estimatedTimeRemaining),
                  })
                }}
              </span>
            </div>
          </div>
        </div>

        <!-- Quick Actions Card -->
        <Card v-if="nextLab" class="lg:w-80 flex-shrink-0 bg-primary-50 dark:bg-primary-900/20">
          <template #content>
            <div class="space-y-4">
              <div class="flex items-center gap-2 text-primary-600 dark:text-primary-400">
                <i class="pi pi-arrow-right" />
                <span class="font-semibold">{{ t('pathway.enrollment.continueLearning') }}</span>
              </div>
              <div>
                <p class="text-xs text-surface-500 mb-1">{{ nextLab.moduleName }}</p>
                <h3 class="font-semibold text-surface-900 dark:text-surface-100">
                  {{ nextLab.lab.labName || t('pathway.enrollment.nextLabFallback') }}
                </h3>
              </div>
              <div class="flex items-center gap-4 text-sm text-surface-500">
                <span>
                  <i class="pi pi-clock mr-1" />{{ formatDuration(nextLab.lab.durationMinutes) }}
                </span>
                <span class="text-amber-600">
                  {{ t('pathway.enrollment.pointsSuffix', { count: nextLab.lab.maxPoints }) }}
                </span>
              </div>
              <Button
                @click="
                  launchLab(
                    nextLab.lab.labTemplateId,
                    nextLab.lab.labName || t('pathway.enrollment.labFallbackName'),
                    nextLab.moduleId,
                  )
                "
                :loading="launchingLab === nextLab.lab.labTemplateId"
                :disabled="launchingLab === nextLab.lab.labTemplateId"
                :label="t('pathway.enrollment.startLab')"
                icon="pi pi-play"
                class="w-full"
              />
            </div>
          </template>
        </Card>

        <!-- Completed State -->
        <Card
          v-else-if="enrollment.status === 'completed'"
          class="lg:w-80 flex-shrink-0 bg-green-50 dark:bg-green-900/20"
        >
          <template #content>
            <div class="text-center space-y-4">
              <div
                class="w-16 h-16 mx-auto rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center"
              >
                <i class="pi pi-trophy text-3xl text-green-600 dark:text-green-400" />
              </div>
              <div>
                <h3 class="font-semibold text-surface-900 dark:text-surface-100">
                  {{ t('pathway.enrollment.completedHeading') }}
                </h3>
                <p class="text-sm text-surface-500 mt-1">
                  {{ t('pathway.enrollment.completedBody') }}
                </p>
              </div>
              <div class="text-2xl font-bold text-amber-600 dark:text-amber-400">
                {{
                  t('pathway.enrollment.pointsEarnedLine', {
                    points: formatNumber(enrollment.earnedPoints),
                  })
                }}
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Error for lab launch -->
      <Message v-if="error && enrollment" severity="error" :closable="true" @close="error = null">
        {{ error }}
      </Message>

      <!-- Modules Progress -->
      <div>
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-4">
          {{ t('pathway.enrollment.modulesHeading') }}
        </h2>

        <div class="space-y-4">
          <Accordion :multiple="true" :value="expandedModules">
            <AccordionPanel
              v-for="(module, index) in modules"
              :key="module.moduleId"
              :value="module.moduleId"
              :disabled="module.status === 'locked'"
            >
              <template #header>
                <div class="flex items-center justify-between w-full pr-4">
                  <div class="flex items-center gap-3">
                    <div
                      class="w-10 h-10 rounded-full flex items-center justify-center font-semibold text-sm"
                      :class="{
                        'bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-400':
                          module.status === 'completed',
                        'bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400':
                          module.status === 'in_progress',
                        'bg-amber-100 dark:bg-amber-900 text-amber-600 dark:text-amber-400':
                          module.status === 'unlocked',
                        'bg-surface-200 dark:bg-surface-700 text-surface-500':
                          module.status === 'locked',
                      }"
                    >
                      <i v-if="module.status === 'completed'" class="pi pi-check" />
                      <span v-else>{{ index + 1 }}</span>
                    </div>
                    <div>
                      <h3 class="font-semibold text-surface-900 dark:text-surface-100">
                        {{
                          module.moduleName ||
                          module.module?.name ||
                          t('pathway.enrollment.moduleFallbackName', { index: index + 1 })
                        }}
                      </h3>
                      <div class="flex items-center gap-3 text-sm text-surface-500">
                        <span>{{
                          t('pathway.enrollment.moduleLabsCount', {
                            completed: module.completedLabs,
                            total: module.totalLabs,
                          })
                        }}</span>
                        <span>{{
                          t('pathway.enrollment.modulePointsCount', {
                            earned: module.earnedPoints,
                            total: module.maxPoints,
                          })
                        }}</span>
                      </div>
                    </div>
                  </div>
                  <div class="flex items-center gap-3">
                    <div v-if="module.status !== 'locked'" class="flex items-center gap-2">
                      <ProgressBar
                        :value="
                          module.totalLabs > 0 ? (module.completedLabs / module.totalLabs) * 100 : 0
                        "
                        :showValue="false"
                        class="w-24 h-2"
                      />
                      <span
                        class="text-xs font-medium text-surface-600 dark:text-surface-400 min-w-[3rem] text-right"
                      >
                        {{
                          module.totalLabs > 0
                            ? Math.round((module.completedLabs / module.totalLabs) * 100)
                            : 0
                        }}%
                      </span>
                    </div>
                    <Tag
                      :value="getModuleStatusLabel(module.status)"
                      :severity="getModuleStatusSeverity(module.status)"
                      class="text-xs"
                    />
                  </div>
                </div>
              </template>

              <div class="space-y-3 pl-13">
                <div
                  v-for="lab in getModuleLabs(module)"
                  :key="lab.labTemplateId"
                  class="flex items-center gap-4 p-4 rounded-lg bg-surface-50 dark:bg-surface-800 hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
                >
                  <i
                    class="pi text-xl"
                    :class="[
                      getLabStatusIcon(lab.passed, lab.attemptCount),
                      getLabStatusClass(lab.passed, lab.attemptCount),
                    ]"
                  />
                  <div class="flex-1 min-w-0">
                    <h4 class="font-medium text-surface-900 dark:text-surface-100 truncate">
                      {{ lab.labName || t('pathway.enrollment.labFallbackName') }}
                    </h4>
                    <div class="flex items-center gap-3 text-sm text-surface-500">
                      <span>
                        <i class="pi pi-clock mr-1" />{{ formatDuration(lab.durationMinutes) }}
                      </span>
                      <span v-if="lab.attemptCount > 0">
                        {{
                          lab.attemptCount === 1
                            ? t('pathway.enrollment.attemptsSingular', { count: lab.attemptCount })
                            : t('pathway.enrollment.attemptsPlural', { count: lab.attemptCount })
                        }}
                      </span>
                    </div>
                  </div>
                  <div class="flex items-center gap-3">
                    <div class="text-right">
                      <div
                        class="font-semibold"
                        :class="
                          lab.passed
                            ? 'text-green-600 dark:text-green-400'
                            : 'text-surface-600 dark:text-surface-400'
                        "
                      >
                        {{
                          t('pathway.enrollment.labScore', {
                            score: lab.bestScore,
                            total: lab.maxPoints,
                          })
                        }}
                      </div>
                      <div class="text-xs text-surface-500">
                        {{ t('pathway.enrollment.labScoreLabel') }}
                      </div>
                    </div>
                    <div class="flex gap-2">
                      <Button
                        @click="viewLabDetails(lab.labTemplateId)"
                        icon="pi pi-info-circle"
                        severity="secondary"
                        size="small"
                        text
                        rounded
                        v-tooltip.top="t('pathway.enrollment.viewDetails')"
                      />
                      <Button
                        @click="
                          launchLab(
                            lab.labTemplateId,
                            lab.labName || t('pathway.enrollment.labFallbackName'),
                            module.moduleId,
                          )
                        "
                        :loading="launchingLab === lab.labTemplateId"
                        :disabled="launchingLab === lab.labTemplateId"
                        :icon="lab.passed ? 'pi pi-replay' : 'pi pi-play'"
                        :severity="lab.passed ? 'secondary' : 'primary'"
                        size="small"
                        rounded
                        v-tooltip.top="
                          lab.passed
                            ? t('pathway.enrollment.retryLab')
                            : t('pathway.enrollment.startLabTooltip')
                        "
                      />
                    </div>
                  </div>
                </div>

                <p
                  v-if="getModuleLabs(module).length === 0"
                  class="text-surface-500 italic py-4 text-center"
                >
                  {{ t('pathway.enrollment.emptyModuleLabs') }}
                </p>
              </div>
            </AccordionPanel>
          </Accordion>
        </div>
      </div>
    </template>
  </div>
</template>
