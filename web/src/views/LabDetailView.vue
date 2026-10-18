<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { labsApi, podsApi, type Lab } from '@/api'
import { formatDuration } from '@/utils/format'
import { getDifficultySeverity } from '@/utils/status'
import { useAuthStore } from '../stores/auth'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const lab = ref<Lab | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const launching = ref(false)

const labId = route.params['labId'] as string

onMounted(async () => {
  await loadLab()
})

async function loadLab() {
  try {
    loading.value = true
    error.value = null
    lab.value = await labsApi.get(labId)
  } catch (err) {
    console.error('Failed to load lab:', err)
    error.value = t('labDetail.errors.loadFailed')
  } finally {
    loading.value = false
  }
}

async function launchLab() {
  if (!lab.value) return

  try {
    launching.value = true
    error.value = null

    const currentUser = authStore.user?.id || authStore.user?.email
    if (!currentUser) {
      error.value = t('labDetail.errors.mustBeLoggedIn')
      return
    }
    const pod = await podsApi.create(lab.value.id, currentUser)

    // Check for pathway context from query params (set when navigating from pathway view)
    const enrollmentId = route.query['enrollmentId'] as string | undefined
    const moduleId = route.query['moduleId'] as string | undefined

    // Navigate to pod view, passing pathway context if present
    if (enrollmentId && moduleId) {
      router.push({
        path: `/pods/${pod.id}`,
        query: { enrollmentId, moduleId }
      })
    } else {
      router.push(`/pods/${pod.id}`)
    }
  } catch (err) {
    console.error('Failed to launch lab:', err)
    error.value = t('labDetail.errors.launchFailed', { name: lab.value.name })
  } finally {
    launching.value = false
  }
}

// formatDuration imported from @/utils/format
// getDifficultySeverity imported from @/utils/status

/** Resolve difficulty label through the shared difficulty.* catalog, falling
 * back to the raw value for unknown tiers. */
function getDifficultyLabel(difficulty: string): string {
  const key = `difficulty.${difficulty}`
  return te(key) ? t(key) : difficulty
}

// Check if user can edit this lab (admin or instructor)
const canEdit = computed(() => {
  return authStore.isAdmin || authStore.user?.roles?.includes('instructor')
})

// Estimated resources based on difficulty (mock for now)
const estimatedResources = computed(() => {
  if (!lab.value) return null
  switch (lab.value.difficulty) {
    case 'beginner':
      return { cpu: 2, memory: 4, storage: 40 }
    case 'intermediate':
      return { cpu: 4, memory: 8, storage: 60 }
    case 'advanced':
      return { cpu: 6, memory: 16, storage: 100 }
    default:
      return { cpu: 4, memory: 8, storage: 60 }
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Back button -->
    <div>
      <RouterLink to="/labs">
        <Button icon="pi pi-arrow-left" :label="t('labDetail.backLabel')" text />
      </RouterLink>
    </div>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Lab details -->
    <div v-else-if="lab" class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Main content -->
      <div class="lg:col-span-2 space-y-6">
        <Card>
          <template #content>
            <div class="space-y-6">
              <!-- Header -->
              <div class="flex items-start justify-between">
                <div>
                  <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ lab.name }}</h1>
                  <p class="text-sm text-surface-500 mt-1">{{ t('labDetail.versionSubtitle', { version: lab.version || '1.0.0' }) }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <Tag :value="getDifficultyLabel(lab.difficulty)" :severity="getDifficultySeverity(lab.difficulty)" />
                  <RouterLink v-if="canEdit" :to="`/labs/${labId}/edit`">
                    <Button icon="pi pi-pencil" :label="t('labDetail.edit')" severity="secondary" size="small" />
                  </RouterLink>
                </div>
              </div>

              <!-- Description -->
              <div>
                <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-2">{{ t('labDetail.descriptionHeading') }}</h2>
                <p class="text-surface-600 dark:text-surface-400 leading-relaxed">{{ lab.description }}</p>
              </div>

              <!-- Tags -->
              <div v-if="lab.tags?.length">
                <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-2">{{ t('labDetail.topicsHeading') }}</h2>
                <div class="flex flex-wrap gap-2">
                  <Tag
                    v-for="tag in lab.tags"
                    :key="tag"
                    :value="tag"
                    severity="info"
                  />
                </div>
              </div>

              <!-- Objectives (placeholder for future enhancement) -->
              <div>
                <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100 mb-2">{{ t('labDetail.objectivesHeading') }}</h2>
                <ul class="list-disc list-inside text-surface-600 dark:text-surface-400 space-y-1">
                  <li>{{ t('labDetail.objectives.core') }}</li>
                  <li>{{ t('labDetail.objectives.security') }}</li>
                  <li>{{ t('labDetail.objectives.handsOn') }}</li>
                  <li>{{ t('labDetail.objectives.assessments') }}</li>
                </ul>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Sidebar -->
      <div class="space-y-6">
        <!-- Quick Info Card -->
        <Card>
          <template #content>
            <div class="space-y-4">
              <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100">{{ t('labDetail.quickInfo.heading') }}</h2>

              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-clock mr-2" />{{ t('labDetail.quickInfo.duration') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ formatDuration(lab.durationMinutes) }}</span>
                </div>

                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-server mr-2" />{{ t('labDetail.quickInfo.platform') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100 capitalize">{{ lab.platform }}</span>
                </div>

                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-star mr-2" />{{ t('labDetail.quickInfo.maxPoints') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ lab.maxPoints || 1000 }}</span>
                </div>

                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-check-circle mr-2" />{{ t('labDetail.quickInfo.passThreshold') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ lab.passThreshold || 70 }}%</span>
                </div>
              </div>

              <hr class="border-surface-200 dark:border-surface-700" />

              <Button
                @click="launchLab"
                :loading="launching"
                :disabled="launching"
                :label="t('labDetail.launch')"
                icon="pi pi-play"
                class="w-full"
              />
            </div>
          </template>
        </Card>

        <!-- Resource Requirements Card -->
        <Card v-if="estimatedResources">
          <template #content>
            <div class="space-y-4">
              <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100">{{ t('labDetail.resources.heading') }}</h2>

              <div class="space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-microchip mr-2" />{{ t('labDetail.resources.cpu') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ t('labDetail.resources.cpuValue', { count: estimatedResources.cpu }) }}</span>
                </div>

                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-database mr-2" />{{ t('labDetail.resources.memory') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ t('labDetail.resources.memoryValue', { count: estimatedResources.memory }) }}</span>
                </div>

                <div class="flex items-center justify-between">
                  <span class="text-surface-500"><i class="pi pi-save mr-2" />{{ t('labDetail.resources.storage') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ t('labDetail.resources.storageValue', { count: estimatedResources.storage }) }}</span>
                </div>
              </div>

              <p class="text-xs text-surface-400">
                {{ t('labDetail.resources.hint') }}
              </p>
            </div>
          </template>
        </Card>
      </div>
    </div>

    <!-- Not found -->
    <Card v-else>
      <template #content>
        <div class="text-center py-12">
          <i class="pi pi-inbox text-5xl text-surface-400 mb-4" />
          <p class="text-surface-500 text-lg">{{ t('labDetail.notFound.title') }}</p>
          <RouterLink to="/labs">
            <Button :label="t('labDetail.notFound.browseAction')" link class="mt-2" />
          </RouterLink>
        </div>
      </template>
    </Card>
  </div>
</template>
