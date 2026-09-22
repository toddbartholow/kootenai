<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, RouterLink } from 'vue-router'
import { pathwaysApi, enrollmentsApi, type Pathway, type PathwayEnrollment } from '@/api'
import { useAuthStore } from '../stores/auth'
import { difficultyFilterOptions } from '@/constants/formOptions'

const { t } = useI18n()
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import PathwayCard from '@/components/pathway/PathwayCard.vue'
import { SkeletonCardGrid } from '@/components/common'

const router = useRouter()
const authStore = useAuthStore()

// Check if user can create pathways (instructor or admin)
const canCreatePathway = computed(() => {
  return authStore.isAdmin || authStore.user?.roles?.includes('instructor')
})
const pathways = ref<Pathway[]>([])
const enrollments = ref<PathwayEnrollment[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const enrollingPathway = ref<string | null>(null)

// Filter state
const searchQuery = ref('')
const selectedDifficulty = ref<string | null>(null)
const showEnrolledOnly = ref(false)

// Use shared filter options
const difficultyOptions = difficultyFilterOptions

// Map of pathway ID to enrollment
const enrollmentMap = computed(() => {
  const map = new Map<string, PathwayEnrollment>()
  enrollments.value.forEach(e => map.set(e.pathwayId, e))
  return map
})

// Filtered pathways based on search and filters
const filteredPathways = computed(() => {
  return pathways.value.filter(pathway => {
    // Search filter
    const matchesSearch = !searchQuery.value ||
      pathway.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      pathway.description?.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      pathway.tags?.some(tag => tag.toLowerCase().includes(searchQuery.value.toLowerCase()))

    // Difficulty filter
    const matchesDifficulty = !selectedDifficulty.value ||
      pathway.difficulty === selectedDifficulty.value

    // Enrolled filter
    const matchesEnrolled = !showEnrolledOnly.value ||
      enrollmentMap.value.has(pathway.id)

    return matchesSearch && matchesDifficulty && matchesEnrolled
  })
})

// Featured pathways (first 3 featured ones)
const featuredPathways = computed(() => {
  return pathways.value
    .filter(p => p.isFeatured)
    .slice(0, 3)
})

// Active enrollments (in progress)
const activeEnrollments = computed(() => {
  return enrollments.value.filter(e =>
    e.status === 'enrolled' || e.status === 'in_progress'
  )
})

// Check if filters are active
const hasActiveFilters = computed(() => {
  return searchQuery.value || selectedDifficulty.value || showEnrolledOnly.value
})

onMounted(async () => {
  try {
    // Fetch published pathways for everyone
    const publishedPromise = pathwaysApi.list({ status: 'published' })
    const enrollmentsPromise = enrollmentsApi.list()

    // For instructors/admins, also fetch draft pathways
    const draftPromise = canCreatePathway.value
      ? pathwaysApi.list({ status: 'draft' })
      : Promise.resolve([])

    const [publishedList, draftList, enrollmentsList] = await Promise.all([
      publishedPromise,
      draftPromise,
      enrollmentsPromise
    ])

    // Combine published and draft pathways (drafts first for visibility)
    pathways.value = [...draftList, ...publishedList]
    enrollments.value = enrollmentsList
  } catch (err) {
    console.error('Failed to load pathways:', err)
    error.value = t('pathway.list.errors.loadFailed')
  } finally {
    loading.value = false
  }
})

function viewPathway(pathway: Pathway) {
  router.push(`/pathways/${pathway.slug}`)
}

async function enrollInPathway(pathway: Pathway, event: Event) {
  event.stopPropagation()
  try {
    enrollingPathway.value = pathway.id
    error.value = null

    const enrollment = await pathwaysApi.enroll(pathway.id)
    enrollments.value.push(enrollment)

    // Navigate to the pathway view
    router.push(`/pathways/${pathway.slug}`)
  } catch (err) {
    console.error('Failed to enroll:', err)
    error.value = t('pathway.list.errors.enrollFailed', { name: pathway.name })
  } finally {
    enrollingPathway.value = null
  }
}

function continuePathway(enrollment: PathwayEnrollment, event: Event) {
  event.stopPropagation()
  const pathway = pathways.value.find(p => p.id === enrollment.pathwayId)
  if (pathway) {
    router.push(`/pathways/${pathway.slug}`)
  } else {
    router.push(`/enrollments/${enrollment.id}`)
  }
}

function clearFilters() {
  searchQuery.value = ''
  selectedDifficulty.value = null
  showEnrolledOnly.value = false
}

const STATUS_LABEL_KEYS: Record<string, string> = {
  completed: 'pathway.list.statusCompleted',
  in_progress: 'pathway.list.statusInProgress',
  enrolled: 'pathway.list.statusEnrolled',
  abandoned: 'pathway.list.statusAbandoned',
}

function getStatusLabel(status: string): string {
  const key = STATUS_LABEL_KEYS[status]
  return key ? t(key) : status
}
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('pathway.list.title') }}</h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('pathway.list.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-4">
        <div class="text-sm text-surface-500">
          {{ t('pathway.list.countSummary', { filtered: filteredPathways.length, total: pathways.length }) }}
        </div>
        <RouterLink v-if="canCreatePathway" to="/pathways/create">
          <Button
            icon="pi pi-plus"
            :label="t('pathway.list.createAction')"
            severity="primary"
          />
        </RouterLink>
      </div>
    </div>

    <!-- Continue Learning Section -->
    <section v-if="activeEnrollments.length > 0 && !loading" aria-labelledby="continue-learning-heading">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-sm bg-blue-500"></div>
        <h2 id="continue-learning-heading" class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.list.continueHeading') }}</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" data-pseudo-skip>
        <button
          v-for="enrollment in activeEnrollments.slice(0, 3)"
          :key="enrollment.id"
          type="button"
          class="w-full text-left p-4 bg-white dark:bg-surface-800 rounded-lg border border-surface-200 dark:border-surface-700 hover:shadow-md transition-shadow focus:ring-2 focus:ring-primary-500 focus:outline-none"
          @click="continuePathway(enrollment, $event)"
        >
          <div class="flex items-start justify-between mb-3">
            <h3 class="font-semibold text-surface-900 dark:text-surface-100 line-clamp-1">
              {{ pathways.find(p => p.id === enrollment.pathwayId)?.name || t('pathway.list.unknownPathway') }}
            </h3>
            <span class="px-2 py-0.5 text-xs font-medium rounded bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300">
              {{ getStatusLabel(enrollment.status) }}
            </span>
          </div>
          <div class="space-y-2">
            <div class="flex justify-between text-sm text-surface-500">
              <span>{{ t('pathway.list.moduleProgress', { completed: enrollment.completedModules, total: enrollment.totalModules }) }}</span>
              <span class="font-medium">{{ Math.round(enrollment.percentage) }}%</span>
            </div>
            <div class="h-2 bg-surface-200 dark:bg-surface-700 rounded-full overflow-hidden">
              <div
                class="h-full bg-blue-500 rounded-full transition-all"
                :style="{ width: `${enrollment.percentage}%` }"
              ></div>
            </div>
          </div>
        </button>
      </div>
    </section>

    <!-- Featured Pathways Section -->
    <section v-if="featuredPathways.length > 0 && !loading && !hasActiveFilters" aria-labelledby="featured-heading">
      <div class="flex items-center gap-3 mb-4">
        <div class="w-3 h-3 rounded-sm bg-amber-500"></div>
        <h2 id="featured-heading" class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.list.featuredHeading') }}</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4" data-pseudo-skip>
        <PathwayCard
          v-for="pathway in featuredPathways"
          :key="pathway.id"
          :pathway="pathway"
          :enrollment="enrollmentMap.get(pathway.id)"
          :featured="true"
          :enrolling="enrollingPathway === pathway.id"
          @view="viewPathway"
          @enroll="enrollInPathway"
          @continue="continuePathway"
        />
      </div>
    </section>

    <!-- Search and Filters -->
    <div class="flex flex-col md:flex-row gap-3 p-4 bg-white dark:bg-surface-800 rounded-lg border border-surface-200 dark:border-surface-700">
      <div class="flex-1 relative">
        <i class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-surface-400" aria-hidden="true" />
        <InputText
          v-model="searchQuery"
          :placeholder="t('pathway.list.searchPlaceholder')"
          :aria-label="t('pathway.list.searchAria')"
          class="w-full pl-10"
        />
      </div>
      <Select
        v-model="selectedDifficulty"
        :options="difficultyOptions"
        optionLabel="label"
        optionValue="value"
        :placeholder="t('pathway.list.difficultyAllPlaceholder')"
        :aria-label="t('pathway.list.difficultyAria')"
        class="w-full md:w-48"
      />
      <Button
        @click="showEnrolledOnly = !showEnrolledOnly"
        :label="t('pathway.list.myEnrollmentsAction')"
        :severity="showEnrolledOnly ? 'primary' : 'secondary'"
        :outlined="!showEnrolledOnly"
      />
      <Button
        v-if="hasActiveFilters"
        @click="clearFilters"
        :label="t('pathway.list.clearAction')"
        severity="secondary"
        outlined
      />
    </div>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading state -->
    <SkeletonCardGrid v-if="loading" :count="6" :show-tags="true" />

    <!-- Empty state -->
    <Card v-else-if="pathways.length === 0">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-map text-4xl text-surface-400 mb-4" />
          <h2 class="text-surface-500 text-lg">{{ t('pathway.list.empty.title') }}</h2>
          <p class="text-surface-400 mt-2">{{ t('pathway.list.empty.body') }}</p>
        </div>
      </template>
    </Card>

    <!-- No filter results -->
    <Card v-else-if="filteredPathways.length === 0">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-search text-4xl text-surface-400 mb-4" />
          <h2 class="text-surface-500 text-lg">{{ t('pathway.list.noMatch.title') }}</h2>
          <p class="text-surface-400 mt-2">{{ t('pathway.list.noMatch.body') }}</p>
          <Button @click="clearFilters" :label="t('pathway.list.clearFiltersAction')" severity="secondary" class="mt-4" />
        </div>
      </template>
    </Card>

    <!-- All Pathways Grid -->
    <section v-else aria-labelledby="all-pathways-heading">
      <h2 id="all-pathways-heading" class="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-4">
        {{ showEnrolledOnly ? t('pathway.list.yourEnrollmentsHeading') : t('pathway.list.allPathwaysHeading') }}
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" data-pseudo-skip>
        <PathwayCard
          v-for="pathway in filteredPathways"
          :key="pathway.id"
          :pathway="pathway"
          :enrollment="enrollmentMap.get(pathway.id)"
          :enrolling="enrollingPathway === pathway.id"
          @view="viewPathway"
          @enroll="enrollInPathway"
          @continue="continuePathway"
        />
      </div>
    </section>
  </div>
</template>
