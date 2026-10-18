<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import {
  pathwaysApi,
  enrollmentsApi,
  podsApi,
  type Pathway,
  type PathwayModule,
  type PathwayEnrollment,
  type ModuleProgress,
  type ModuleLab,
} from '@/api'
import Card from '@volt/Card.vue'
import { useFocusRestore } from '@/composables'
import { getDifficultySeverity } from '@/utils/status'
import { formatDuration } from '@/utils/format'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import Dialog from '@volt/Dialog.vue'
import PathwayRoadmap from '../components/pathway/PathwayRoadmap.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const ERROR_DISPLAY_DURATION_MS = 3000

// State
const pathway = ref<Pathway | null>(null)
const enrollment = ref<PathwayEnrollment | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const enrolling = ref(false)
const launchingLab = ref<string | null>(null)

// Dialog state
const showLabDialog = ref(false)
useFocusRestore(showLabDialog)
const selectedLab = ref<ModuleLab | null>(null)
const selectedModule = ref<PathwayModule | null>(null)

// Get pathway slug from route
const pathwaySlug = computed(() => route.params['slug'] as string)

// Computed: modules from pathway
const modules = computed<PathwayModule[]>(() => {
  return pathway.value?.modules || []
})

// Computed: module progress map for the roadmap component
const moduleProgress = computed(() => {
  const progressMap = new Map<string, { status: string; percentage: number }>()

  if (!enrollment.value?.moduleProgress) {
    // Not enrolled - first module unlocked, rest locked
    modules.value.forEach((mod, index) => {
      progressMap.set(mod.id, {
        status: index === 0 ? 'unlocked' : 'locked',
        percentage: 0,
      })
    })
    return progressMap
  }

  // Map progress from enrollment
  enrollment.value.moduleProgress.forEach((mp: ModuleProgress) => {
    const totalLabs = mp.totalLabs || 1
    const completedLabs = mp.completedLabs || 0
    progressMap.set(mp.moduleId, {
      status: mp.status,
      percentage: totalLabs > 0 ? (completedLabs / totalLabs) * 100 : 0,
    })
  })

  return progressMap
})

// Computed: check if user is enrolled
const isEnrolled = computed(() => !!enrollment.value)

// Computed: overall progress percentage
const overallProgress = computed(() => {
  if (!enrollment.value) return 0
  return Math.round(enrollment.value.percentage)
})

// Computed: completed modules count
const completedModules = computed(() => {
  if (!enrollment.value) return 0
  return enrollment.value.completedModules
})

// Load data on mount
onMounted(async () => {
  await loadPathwayData()
})

// Watch for route changes
watch(pathwaySlug, async () => {
  await loadPathwayData()
})

async function loadPathwayData() {
  loading.value = true
  error.value = null

  try {
    // Fetch pathway details
    const fetchedPathway = await pathwaysApi.get(pathwaySlug.value)
    pathway.value = fetchedPathway

    // Try to fetch enrollment if user is authenticated
    try {
      const enrollmentData = await enrollmentsApi.getByPathway(fetchedPathway.id)
      if (enrollmentData) {
        const fullEnrollment = await enrollmentsApi.getProgress(enrollmentData.id)
        enrollment.value = fullEnrollment
      }
    } catch (err: unknown) {
      const axiosError = err as { response?: { status?: number } }
      if (axiosError?.response?.status === 404) {
        enrollment.value = null
      } else {
        console.error('Failed to check enrollment status:', err)
        enrollment.value = null
      }
    }
  } catch (err) {
    console.error('Failed to load pathway:', err)
    error.value = t('pathway.interactive.loadFailed')
  } finally {
    loading.value = false
  }
}

// Enroll in pathway
async function enroll() {
  if (!pathway.value) return

  try {
    enrolling.value = true
    error.value = null

    const newEnrollment = await pathwaysApi.enroll(pathway.value.id)
    const fullEnrollment = await enrollmentsApi.getProgress(newEnrollment.id)
    enrollment.value = fullEnrollment
  } catch (err) {
    console.error('Failed to enroll:', err)
    error.value = t('pathway.interactive.enrollFailed')
  } finally {
    enrolling.value = false
  }
}

// Helper to check if a module is locked
function isModuleLocked(moduleId: string): boolean {
  const progress = moduleProgress.value.get(moduleId)
  return isEnrolled.value && progress?.status === 'locked'
}

// Show temporary error message
function showTemporaryError(message: string) {
  error.value = message
  setTimeout(() => {
    error.value = null
  }, ERROR_DISPLAY_DURATION_MS)
}

// Handle module click from roadmap
function handleModuleClick(module: PathwayModule) {
  if (isModuleLocked(module.id)) {
    showTemporaryError(t('pathway.interactive.moduleLocked', { name: module.name }))
    return
  }

  // Select first lab if available
  if (module.labs?.length) {
    selectedModule.value = module
    selectedLab.value = module.labs[0] || null
    showLabDialog.value = true
  }
}

// Handle lab click from roadmap
function handleLabClick(lab: ModuleLab, module: PathwayModule) {
  if (isModuleLocked(module.id)) {
    showTemporaryError(t('pathway.interactive.labLocked'))
    return
  }

  selectedModule.value = module
  selectedLab.value = lab
  showLabDialog.value = true
}

// Get lab progress for display
function getLabProgress(lab: ModuleLab) {
  if (!enrollment.value?.moduleProgress) return null

  for (const mp of enrollment.value.moduleProgress) {
    const labProgress = mp.labProgress || mp.labs || []
    const progress = labProgress.find(lp => lp.labTemplateId === lab.labTemplateId)
    if (progress) return progress
  }
  return null
}

// Launch a lab
async function launchLab(lab: ModuleLab) {
  if (!lab.labTemplateId) return

  try {
    launchingLab.value = lab.labTemplateId
    error.value = null
    showLabDialog.value = false

    const pod = await podsApi.create(lab.labTemplateId, 'current-user')
    router.push(`/pods/${pod.id}`)
  } catch (err) {
    console.error('Failed to launch lab:', err)
    error.value = t('pathway.interactive.launchFailed', { name: lab.labName })
  } finally {
    launchingLab.value = null
  }
}

// View lab details
function viewLabDetails(lab: ModuleLab) {
  showLabDialog.value = false
  router.push(`/labs/${lab.labTemplateId}`)
}

// Helper functions
// formatDuration (aliased as formatLabDuration), getDifficultySeverity imported from @/utils
const formatLabDuration = formatDuration

function getDifficultyLabel(difficulty: string | undefined): string {
  if (!difficulty) return t('pathway.interactive.difficultyNotSpecified')
  return difficulty.charAt(0).toUpperCase() + difficulty.slice(1)
}
</script>

<template>
  <div class="space-y-6">
    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Error message (full page) -->
    <Message v-else-if="error && !pathway" severity="error">
      {{ error }}
      <template #closeicon>
        <Button
          @click="router.push('/pathways')"
          :label="t('pathway.interactive.backAction')"
          severity="secondary"
          size="small"
          class="ml-4"
        />
      </template>
    </Message>

    <template v-else-if="pathway">
      <!-- Breadcrumb -->
      <div class="flex items-center gap-2 text-sm text-surface-500">
        <router-link to="/pathways" class="hover:text-surface-700 dark:hover:text-surface-300">
          {{ t('pathway.interactive.breadcrumbList') }}
        </router-link>
        <i class="pi pi-chevron-right text-xs" />
        <span class="text-surface-700 dark:text-surface-300">{{ pathway.name }}</span>
      </div>

      <!-- Main content grid -->
      <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <!-- Left column: Pathway info & Roadmap -->
        <div class="lg:col-span-3 space-y-6">
          <!-- Header Card -->
          <Card>
            <template #content>
              <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100 mb-3">
                {{ pathway.name }}
              </h1>
              <p class="text-surface-600 dark:text-surface-400">
                {{ pathway.description }}
              </p>
            </template>
          </Card>

          <!-- Prerequisites -->
          <div v-if="pathway.prerequisites?.length">
            <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-3">
              {{ t('pathway.interactive.prerequisitesHeading') }}
            </h2>
            <div class="flex flex-wrap gap-2">
              <span
                v-for="(prereq, index) in pathway.prerequisites"
                :key="index"
                class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded-md text-sm"
              >
                <i class="pi pi-check text-xs" />
                {{ prereq }}
              </span>
            </div>
          </div>

          <!-- Inline Error Message -->
          <Message v-if="error && pathway" severity="error" :closable="true" @close="error = null">
            {{ error }}
          </Message>

          <!-- Learning Roadmap -->
          <div>
            <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-3">
              {{ t('pathway.interactive.roadmapHeading') }}
            </h2>
            <Card class="overflow-visible">
              <template #content>
                <PathwayRoadmap
                  :modules="modules"
                  :module-progress="moduleProgress"
                  :on-module-click="handleModuleClick"
                  :on-lab-click="handleLabClick"
                />
              </template>
            </Card>
          </div>
        </div>

        <!-- Right column: Progress card -->
        <div class="lg:col-span-1">
          <div class="sticky top-4">
            <Card>
              <template #content>
                <div class="space-y-4">
                  <!-- Progress (when enrolled) -->
                  <div v-if="isEnrolled" class="space-y-3">
                    <div class="text-center">
                      <div class="text-4xl font-bold text-surface-900 dark:text-surface-100 mb-1">
                        {{ overallProgress }}%
                      </div>
                      <p class="text-sm text-surface-500">
                        {{ t('pathway.interactive.progressComplete') }}
                      </p>
                    </div>
                    <div
                      class="h-2 bg-surface-200 dark:bg-surface-700 rounded-full overflow-hidden"
                    >
                      <div
                        class="h-full bg-blue-500 rounded-full transition-all"
                        :style="{ width: `${overallProgress}%` }"
                      ></div>
                    </div>
                    <p class="text-sm text-surface-500 text-center">
                      {{
                        t('pathway.interactive.progressModuleRatio', {
                          completed: completedModules,
                          total: modules.length,
                        })
                      }}
                    </p>
                  </div>

                  <!-- Stats -->
                  <div
                    class="grid grid-cols-2 gap-3 py-3 border-t border-surface-200 dark:border-surface-700"
                  >
                    <div class="text-center">
                      <div class="text-xl font-semibold text-surface-900 dark:text-surface-100">
                        {{ modules.length }}
                      </div>
                      <p class="text-xs text-surface-500">
                        {{ t('pathway.interactive.statModules') }}
                      </p>
                    </div>
                    <div class="text-center">
                      <div class="text-xl font-semibold text-surface-900 dark:text-surface-100">
                        {{
                          modules.reduce((sum, m) => sum + (m.labs?.length || m.labCount || 0), 0)
                        }}
                      </div>
                      <p class="text-xs text-surface-500">
                        {{ t('pathway.interactive.statLabs') }}
                      </p>
                    </div>
                  </div>

                  <!-- Action button -->
                  <Button
                    v-if="!isEnrolled"
                    @click="enroll"
                    :loading="enrolling"
                    :disabled="enrolling"
                    :label="t('pathway.interactive.enrollNow')"
                    icon="pi pi-plus"
                    class="w-full"
                    severity="success"
                  />
                  <div v-else-if="overallProgress >= 100" class="text-center py-2">
                    <i class="pi pi-trophy text-3xl text-amber-500 mb-2" />
                    <p class="font-semibold text-green-600 dark:text-green-400">
                      {{ t('pathway.interactive.completedTag') }}
                    </p>
                  </div>
                </div>
              </template>
            </Card>
          </div>
        </div>
      </div>
    </template>

    <!-- Lab Details Dialog -->
    <Dialog
      v-model:visible="showLabDialog"
      :header="selectedLab?.labName || t('pathway.interactive.labDialog.fallbackTitle')"
      :modal="true"
      :dismissableMask="true"
      class="w-full max-w-lg"
    >
      <div v-if="selectedLab && selectedModule" class="space-y-4">
        <!-- Module context -->
        <div class="text-sm text-surface-500">
          <i class="pi pi-folder mr-1" />
          {{ selectedModule.name }}
        </div>

        <!-- Lab description -->
        <p v-if="selectedLab.labDescription" class="text-surface-600 dark:text-surface-400">
          {{ selectedLab.labDescription }}
        </p>

        <!-- Lab stats -->
        <div class="flex flex-wrap gap-4 py-2">
          <div class="flex items-center gap-2">
            <i class="pi pi-clock text-surface-400" />
            <span class="text-surface-600 dark:text-surface-400">{{
              formatLabDuration(selectedLab.labDurationMinutes)
            }}</span>
          </div>
          <div class="flex items-center gap-2">
            <i class="pi pi-star-fill text-amber-500" />
            <span class="text-surface-600 dark:text-surface-400">{{
              t('pathway.interactive.labDialog.pointsSuffix', { count: selectedLab.labMaxPoints })
            }}</span>
          </div>
          <Tag
            v-if="selectedLab.labDifficulty"
            :value="getDifficultyLabel(selectedLab.labDifficulty)"
            :severity="getDifficultySeverity(selectedLab.labDifficulty)"
          />
        </div>

        <!-- Progress (if enrolled and has progress) -->
        <div v-if="isEnrolled" class="p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
          <template v-if="getLabProgress(selectedLab)">
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm text-surface-600 dark:text-surface-400">{{
                t('pathway.interactive.labDialog.yourProgress')
              }}</span>
              <Tag
                :value="
                  getLabProgress(selectedLab)?.passed
                    ? t('pathway.interactive.labDialog.passed')
                    : t('pathway.interactive.labDialog.notPassed')
                "
                :severity="getLabProgress(selectedLab)?.passed ? 'success' : 'secondary'"
              />
            </div>
            <div class="flex items-center gap-4 text-sm">
              <span
                >{{ t('pathway.interactive.labDialog.bestScoreLine')
                }}<strong>{{
                  t('pathway.interactive.labDialog.bestScoreValue', {
                    score: getLabProgress(selectedLab)?.bestScore || 0,
                    total: selectedLab.labMaxPoints,
                  })
                }}</strong></span
              >
              <span
                >{{ t('pathway.interactive.labDialog.attemptsLine')
                }}<strong>{{
                  t('pathway.interactive.labDialog.attemptsValue', {
                    count: getLabProgress(selectedLab)?.attemptCount || 0,
                  })
                }}</strong></span
              >
            </div>
          </template>
          <template v-else>
            <p class="text-sm text-surface-500">
              {{ t('pathway.interactive.labDialog.notAttempted') }}
            </p>
          </template>
        </div>

        <!-- Required badge -->
        <div
          v-if="selectedLab.isRequired"
          class="flex items-center gap-2 text-sm text-amber-600 dark:text-amber-400"
        >
          <i class="pi pi-exclamation-triangle" />
          <span>{{ t('pathway.interactive.labDialog.requiredNote') }}</span>
        </div>
      </div>

      <template #footer>
        <div v-if="selectedLab" class="flex justify-end gap-2">
          <Button
            @click="viewLabDetails(selectedLab)"
            :label="t('pathway.interactive.labDialog.viewDetails')"
            icon="pi pi-info-circle"
            severity="secondary"
            outlined
          />
          <Button
            v-if="isEnrolled"
            @click="launchLab(selectedLab)"
            :loading="launchingLab === selectedLab.labTemplateId"
            :disabled="!!launchingLab"
            :label="
              getLabProgress(selectedLab)
                ? t('pathway.interactive.labDialog.tryAgain')
                : t('pathway.interactive.labDialog.startLab')
            "
            :icon="getLabProgress(selectedLab) ? 'pi pi-replay' : 'pi pi-play'"
            severity="success"
          />
          <Button
            v-else
            @click="enroll"
            :loading="enrolling"
            :disabled="enrolling"
            :label="t('pathway.interactive.labDialog.enrollToStart')"
            icon="pi pi-plus"
            severity="success"
          />
        </div>
      </template>
    </Dialog>
  </div>
</template>
