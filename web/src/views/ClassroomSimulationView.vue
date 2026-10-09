<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClassroomStore } from '@/stores/classroom'
import type {
  CreateSimulationRequest,
  PersonalityType,
  ClassroomSimulation,
} from '@/api/domains/classroom'
import { useFormatters } from '@/composables/useFormatters'

const { t, te } = useI18n()
const store = useClassroomStore()
const { formatDate, formatDateTime } = useFormatters()

// Setup form state
const showSetup = ref(true)
const formName = ref('')
const formPathwayId = ref('')
const formHighPerformers = ref(6)
const formStruggling = ref(5)
const formIndustryPro = ref(14)
const formEnableCanvas = ref(false)
const formEnableVMLabs = ref(true)
const formSpeed = ref(1.0)

const totalStudents = computed(
  () => formHighPerformers.value + formStruggling.value + formIndustryPro.value,
)

const selectedSimulation = ref<ClassroomSimulation | null>(null)
const activeTab = ref<'students' | 'feedback'>('students')
const feedbackFilter = ref('')

onMounted(async () => {
  await store.fetchSimulations()
  if (store.hasSimulations) {
    showSetup.value = false
  }
})

// Load feedback when switching to feedback tab
watch(activeTab, async tab => {
  if (tab === 'feedback' && selectedSimulation.value) {
    await Promise.all([
      store.fetchFeedbackSummary(selectedSimulation.value.id),
      store.fetchFeedback(selectedSimulation.value.id),
    ])
  }
})

async function handleCreate() {
  const req: CreateSimulationRequest = {
    name: formName.value,
    pathwayId: formPathwayId.value || undefined,
    config: {
      studentCount: totalStudents.value,
      personalityMix: {
        high_performer: formHighPerformers.value,
        struggling: formStruggling.value,
        industry_professional: formIndustryPro.value,
      },
      enableCanvas: formEnableCanvas.value,
      enableVmLabs: formEnableVMLabs.value,
      speedMultiplier: formSpeed.value,
    },
  }
  const sim = await store.createSimulation(req)
  if (sim) {
    showSetup.value = false
    selectedSimulation.value = sim
  }
}

async function selectSimulation(sim: ClassroomSimulation) {
  selectedSimulation.value = sim
  activeTab.value = 'students'
  await store.fetchSimulation(sim.id)
  selectedSimulation.value = store.currentSimulation
}

async function filterFeedback() {
  if (!selectedSimulation.value) return
  await store.fetchFeedback(selectedSimulation.value.id, {
    type: feedbackFilter.value || undefined,
  })
}

function personalityLabel(p: PersonalityType): string {
  switch (p) {
    case 'high_performer':
      return t('classroomSimulation.personality.highPerformer')
    case 'struggling':
      return t('classroomSimulation.personality.struggling')
    case 'industry_professional':
      return t('classroomSimulation.personality.industryPro')
    default:
      return p
  }
}

function statusLabel(status: string): string {
  const key = `classroomSimulation.status.${status}`
  return te(key) ? t(key) : status
}

function personalityColor(p: PersonalityType): string {
  switch (p) {
    case 'high_performer':
      return 'text-green-400'
    case 'struggling':
      return 'text-red-400'
    case 'industry_professional':
      return 'text-blue-400'
    default:
      return 'text-gray-400'
  }
}

function statusColor(status: string): string {
  switch (status) {
    case 'running':
      return 'bg-green-500/20 text-green-400'
    case 'completed':
      return 'bg-blue-500/20 text-blue-400'
    case 'failed':
      return 'bg-red-500/20 text-red-400'
    case 'paused':
      return 'bg-yellow-500/20 text-yellow-400'
    default:
      return 'bg-gray-500/20 text-gray-400'
  }
}

function ratingColor(rating: number): string {
  if (rating >= 4) return 'text-green-400'
  if (rating >= 3) return 'text-yellow-400'
  return 'text-red-400'
}

function feedbackTypeLabel(type_: string): string {
  switch (type_) {
    case 'lab_quality':
      return t('classroomSimulation.feedbackType.labQuality')
    case 'pathway':
      return t('classroomSimulation.feedbackType.pathway')
    case 'infrastructure':
      return t('classroomSimulation.feedbackType.infrastructure')
    case 'content_suggestion':
      return t('classroomSimulation.feedbackType.contentSuggestion')
    default:
      return type_
  }
}

function feedbackTypeBadge(type_: string): string {
  switch (type_) {
    case 'lab_quality':
      return 'bg-primary-500/20 text-primary-400'
    case 'pathway':
      return 'bg-accent-500/20 text-accent-400'
    case 'infrastructure':
      return 'bg-red-500/20 text-red-400'
    case 'content_suggestion':
      return 'bg-teal-500/20 text-teal-400'
    default:
      return 'bg-gray-500/20 text-gray-400'
  }
}
</script>

<template>
  <div class="min-h-screen bg-gray-900 text-white p-6">
    <div class="max-w-7xl mx-auto">
      <div class="flex items-center justify-between mb-8">
        <div>
          <h1 class="text-3xl font-bold">{{ t('classroomSimulation.header.title') }}</h1>
          <p class="text-gray-400 mt-1">
            {{ t('classroomSimulation.header.subtitle') }}
          </p>
        </div>
        <button
          class="px-4 py-2 bg-primary-600 hover:bg-primary-700 rounded-lg font-medium transition-colors"
          @click="showSetup = true"
        >
          {{ t('classroomSimulation.header.newAction') }}
        </button>
      </div>

      <!-- Setup Panel -->
      <div v-if="showSetup" class="bg-gray-800 rounded-xl p-6 mb-8 border border-gray-700">
        <h2 class="text-xl font-semibold mb-6">{{ t('classroomSimulation.setup.heading') }}</h2>
        <form @submit.prevent="handleCreate" class="space-y-6">
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-1">{{
              t('classroomSimulation.setup.nameLabel')
            }}</label>
            <input
              v-model="formName"
              type="text"
              required
              :placeholder="t('classroomSimulation.setup.namePlaceholder')"
              class="w-full px-4 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500"
              :aria-label="t('classroomSimulation.setup.nameAria')"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-300 mb-1">{{
              t('classroomSimulation.setup.pathwayLabel')
            }}</label>
            <input
              v-model="formPathwayId"
              type="text"
              :placeholder="t('classroomSimulation.setup.pathwayPlaceholder')"
              class="w-full px-4 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500"
              :aria-label="t('classroomSimulation.setup.pathwayAria')"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-300 mb-3">{{
              t('classroomSimulation.setup.personalityMixLabel', { total: totalStudents })
            }}</label>
            <div class="grid grid-cols-3 gap-4">
              <div>
                <label class="block text-xs text-green-400 mb-1">{{
                  t('classroomSimulation.setup.highPerformersLabel')
                }}</label>
                <input
                  v-model.number="formHighPerformers"
                  type="number"
                  min="0"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-green-500"
                  :aria-label="t('classroomSimulation.setup.highPerformersAria')"
                />
              </div>
              <div>
                <label class="block text-xs text-red-400 mb-1">{{
                  t('classroomSimulation.setup.strugglingLabel')
                }}</label>
                <input
                  v-model.number="formStruggling"
                  type="number"
                  min="0"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-red-500"
                  :aria-label="t('classroomSimulation.setup.strugglingAria')"
                />
              </div>
              <div>
                <label class="block text-xs text-blue-400 mb-1">{{
                  t('classroomSimulation.setup.industryProLabel')
                }}</label>
                <input
                  v-model.number="formIndustryPro"
                  type="number"
                  min="0"
                  class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
                  :aria-label="t('classroomSimulation.setup.industryProAria')"
                />
              </div>
            </div>
          </div>

          <div class="grid grid-cols-3 gap-4">
            <label class="flex items-center gap-2">
              <input
                v-model="formEnableCanvas"
                type="checkbox"
                class="rounded bg-gray-700 border-gray-600"
              />
              <span class="text-sm text-gray-300">{{
                t('classroomSimulation.setup.enableCanvas')
              }}</span>
            </label>
            <label class="flex items-center gap-2">
              <input
                v-model="formEnableVMLabs"
                type="checkbox"
                class="rounded bg-gray-700 border-gray-600"
              />
              <span class="text-sm text-gray-300">{{
                t('classroomSimulation.setup.enableVmLabs')
              }}</span>
            </label>
            <div>
              <label class="block text-xs text-gray-400 mb-1">{{
                t('classroomSimulation.setup.speedLabel', { speed: formSpeed })
              }}</label>
              <input
                v-model.number="formSpeed"
                type="range"
                min="0.1"
                max="10"
                step="0.1"
                class="w-full"
                :aria-label="t('classroomSimulation.setup.speedAria')"
              />
            </div>
          </div>

          <div class="flex justify-end gap-3">
            <button
              type="button"
              class="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg transition-colors"
              @click="showSetup = false"
            >
              {{ t('classroomSimulation.setup.cancel') }}
            </button>
            <button
              type="submit"
              :disabled="store.loading || !formName"
              class="px-6 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 rounded-lg font-medium transition-colors"
            >
              {{
                store.loading
                  ? t('classroomSimulation.setup.creating')
                  : t('classroomSimulation.setup.submit')
              }}
            </button>
          </div>
        </form>
      </div>

      <!-- Error -->
      <div
        v-if="store.error"
        class="bg-red-500/10 border border-red-500/30 rounded-lg p-4 mb-6 text-red-400"
      >
        {{ store.error?.message }}
      </div>

      <!-- Simulations List -->
      <div v-if="!showSetup" class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Left: Simulation List -->
        <div class="lg:col-span-1 space-y-3">
          <h2 class="text-lg font-semibold text-gray-300 mb-3">
            {{ t('classroomSimulation.list.heading') }}
          </h2>
          <div
            v-if="store.simulations.length === 0"
            class="bg-gray-800 rounded-lg p-6 text-center text-gray-500"
          >
            {{ t('classroomSimulation.list.empty') }}
          </div>
          <div
            v-for="sim in store.simulations"
            :key="sim.id"
            class="bg-gray-800 rounded-lg p-4 cursor-pointer border transition-colors"
            :class="
              selectedSimulation?.id === sim.id
                ? 'border-primary-500'
                : 'border-gray-700 hover:border-gray-600'
            "
            role="button"
            tabindex="0"
            :aria-pressed="selectedSimulation?.id === sim.id"
            @click="selectSimulation(sim)"
            @keydown.enter="selectSimulation(sim)"
            @keydown.space.prevent="selectSimulation(sim)"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="font-medium truncate">{{ sim.name }}</h3>
              <span
                class="px-2 py-0.5 rounded-md text-xs font-medium"
                :class="statusColor(sim.status)"
              >
                {{ statusLabel(sim.status) }}
              </span>
            </div>
            <p class="text-sm text-gray-400">
              {{ formatDate(sim.createdAt) }}
            </p>
          </div>
        </div>

        <!-- Right: Selected Simulation Detail -->
        <div class="lg:col-span-2">
          <div v-if="!selectedSimulation" class="bg-gray-800 rounded-lg p-12 text-center">
            <p class="text-gray-500">{{ t('classroomSimulation.detail.placeholder') }}</p>
          </div>

          <div v-else class="space-y-6">
            <!-- Simulation Header -->
            <div
              class="bg-gray-800 rounded-lg p-6 border border-gray-700 flex items-center justify-between"
            >
              <div>
                <h2 class="text-xl font-semibold">{{ selectedSimulation.name }}</h2>
                <span
                  class="px-2 py-0.5 rounded-md text-xs font-medium mt-1 inline-block"
                  :class="statusColor(selectedSimulation.status)"
                >
                  {{ statusLabel(selectedSimulation.status) }}
                </span>
              </div>
              <div class="flex gap-2">
                <button
                  v-if="selectedSimulation.status === 'pending'"
                  class="px-3 py-1.5 bg-green-600 hover:bg-green-700 rounded-lg text-sm font-medium transition-colors"
                  @click="store.startSimulation(selectedSimulation.id)"
                >
                  {{ t('classroomSimulation.actions.start') }}
                </button>
                <button
                  v-if="selectedSimulation.status === 'running'"
                  class="px-3 py-1.5 bg-yellow-600 hover:bg-yellow-700 rounded-lg text-sm font-medium transition-colors"
                  @click="store.pauseSimulation(selectedSimulation.id)"
                >
                  {{ t('classroomSimulation.actions.pause') }}
                </button>
                <button
                  v-if="
                    selectedSimulation.status === 'running' ||
                    selectedSimulation.status === 'paused'
                  "
                  class="px-3 py-1.5 bg-red-600 hover:bg-red-700 rounded-lg text-sm font-medium transition-colors"
                  @click="store.stopSimulation(selectedSimulation.id)"
                >
                  {{ t('classroomSimulation.actions.stop') }}
                </button>
                <button
                  class="px-3 py-1.5 bg-gray-700 hover:bg-gray-600 rounded-lg text-sm transition-colors"
                  @click="store.deleteSimulation(selectedSimulation.id)"
                >
                  {{ t('classroomSimulation.actions.delete') }}
                </button>
              </div>
            </div>

            <!-- Tabs -->
            <div class="flex gap-1 bg-gray-800 rounded-lg p-1 border border-gray-700">
              <button
                class="flex-1 px-4 py-2 rounded-md text-sm font-medium transition-colors"
                :class="
                  activeTab === 'students'
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-400 hover:text-white'
                "
                @click="activeTab = 'students'"
              >
                {{ t('classroomSimulation.tabs.students') }}
              </button>
              <button
                class="flex-1 px-4 py-2 rounded-md text-sm font-medium transition-colors"
                :class="
                  activeTab === 'feedback'
                    ? 'bg-gray-700 text-white'
                    : 'text-gray-400 hover:text-white'
                "
                @click="activeTab = 'feedback'"
              >
                {{ t('classroomSimulation.tabs.feedback') }}
              </button>
            </div>

            <!-- Students Tab -->
            <div v-if="activeTab === 'students' && selectedSimulation.students?.length">
              <h3 class="text-lg font-semibold text-gray-300 mb-3">
                {{
                  t('classroomSimulation.students.heading', {
                    count: selectedSimulation.students.length,
                  })
                }}
              </h3>
              <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-3">
                <div
                  v-for="student in selectedSimulation.students"
                  :key="student.id"
                  class="bg-gray-800 rounded-lg p-3 border border-gray-700"
                >
                  <p class="font-medium text-sm truncate">{{ student.name }}</p>
                  <p class="text-xs mt-1" :class="personalityColor(student.personality)">
                    {{ personalityLabel(student.personality) }}
                  </p>
                </div>
              </div>
            </div>

            <!-- Feedback Tab -->
            <div v-if="activeTab === 'feedback'" class="space-y-6">
              <!-- Summary Cards -->
              <div v-if="store.feedbackSummary" class="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                  <p class="text-xs text-gray-400 uppercase tracking-wide">
                    {{ t('classroomSimulation.feedback.summary.overallRating') }}
                  </p>
                  <p
                    class="text-2xl font-bold mt-1"
                    :class="ratingColor(store.feedbackSummary.avgRating)"
                  >
                    {{ store.feedbackSummary.avgRating.toFixed(1) }}
                    <span class="text-sm text-gray-500">{{
                      t('classroomSimulation.feedback.ratingDenominator')
                    }}</span>
                  </p>
                </div>
                <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                  <p class="text-xs text-gray-400 uppercase tracking-wide">
                    {{ t('classroomSimulation.feedback.summary.totalFeedback') }}
                  </p>
                  <p class="text-2xl font-bold mt-1">{{ store.feedbackSummary.totalFeedback }}</p>
                </div>
                <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                  <p class="text-xs text-gray-400 uppercase tracking-wide">
                    {{ t('classroomSimulation.feedback.summary.infraIssues') }}
                  </p>
                  <p class="text-2xl font-bold mt-1 text-red-400">
                    {{ store.feedbackSummary.infrastructureIssues }}
                  </p>
                </div>
                <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                  <p class="text-xs text-gray-400 uppercase tracking-wide">
                    {{ t('classroomSimulation.feedback.summary.labsReviewed') }}
                  </p>
                  <p class="text-2xl font-bold mt-1">{{ store.feedbackSummary.byLab.length }}</p>
                </div>
              </div>

              <!-- Per-type breakdown -->
              <div
                v-if="store.feedbackSummary && Object.keys(store.feedbackSummary.byType).length"
                class="bg-gray-800 rounded-lg p-4 border border-gray-700"
              >
                <h4 class="text-sm font-medium text-gray-300 mb-3">
                  {{ t('classroomSimulation.feedback.byCategory') }}
                </h4>
                <div class="space-y-2">
                  <div
                    v-for="(summary, type_) in store.feedbackSummary.byType"
                    :key="type_"
                    class="flex items-center justify-between"
                  >
                    <span
                      class="px-2 py-0.5 rounded-md text-xs font-medium"
                      :class="feedbackTypeBadge(String(type_))"
                    >
                      {{ feedbackTypeLabel(String(type_)) }}
                    </span>
                    <div class="flex items-center gap-3">
                      <span class="text-sm text-gray-400">{{
                        t('classroomSimulation.feedback.entriesCount', { count: summary.count })
                      }}</span>
                      <span
                        v-if="summary.avgRating"
                        class="text-sm font-medium"
                        :class="ratingColor(summary.avgRating)"
                      >
                        {{ summary.avgRating.toFixed(1) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Per-lab breakdown -->
              <div
                v-if="store.feedbackSummary?.byLab?.length"
                class="bg-gray-800 rounded-lg p-4 border border-gray-700"
              >
                <h4 class="text-sm font-medium text-gray-300 mb-3">
                  {{ t('classroomSimulation.feedback.byLab') }}
                </h4>
                <div class="space-y-2">
                  <div
                    v-for="lab in store.feedbackSummary.byLab"
                    :key="lab.labTemplateId"
                    class="flex items-center justify-between"
                  >
                    <span class="text-sm text-gray-300 truncate">{{ lab.labTemplateId }}</span>
                    <div class="flex items-center gap-3">
                      <span class="text-sm text-gray-400">{{
                        t('classroomSimulation.feedback.reviewsCount', { count: lab.count })
                      }}</span>
                      <span class="text-sm font-medium" :class="ratingColor(lab.avgRating)">
                        {{ lab.avgRating.toFixed(1) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Filter -->
              <div class="flex gap-2">
                <!-- eslint-disable-next-line vuejs-accessibility/no-onchange -- @change on <select> fires on user selection; @blur would be semantically wrong here. -->
                <select
                  v-model="feedbackFilter"
                  class="px-3 py-1.5 bg-gray-700 border border-gray-600 rounded-lg text-sm text-white"
                  :aria-label="t('classroomSimulation.feedback.filterAria')"
                  @change="filterFeedback"
                >
                  <option value="">{{ t('classroomSimulation.feedback.filterAll') }}</option>
                  <option value="lab_quality">
                    {{ t('classroomSimulation.feedbackType.labQuality') }}
                  </option>
                  <option value="pathway">
                    {{ t('classroomSimulation.feedbackType.pathway') }}
                  </option>
                  <option value="infrastructure">
                    {{ t('classroomSimulation.feedbackType.infrastructure') }}
                  </option>
                  <option value="content_suggestion">
                    {{ t('classroomSimulation.feedbackType.contentSuggestion') }}
                  </option>
                </select>
              </div>

              <!-- Feedback List -->
              <div
                v-if="store.feedback.length === 0"
                class="bg-gray-800 rounded-lg p-6 text-center text-gray-500"
              >
                {{ t('classroomSimulation.feedback.empty') }}
              </div>
              <div v-else class="space-y-3">
                <div
                  v-for="fb in store.feedback"
                  :key="fb.id"
                  class="bg-gray-800 rounded-lg p-4 border border-gray-700"
                >
                  <div class="flex items-center justify-between mb-2">
                    <span
                      class="px-2 py-0.5 rounded-md text-xs font-medium"
                      :class="feedbackTypeBadge(fb.feedbackType)"
                    >
                      {{ feedbackTypeLabel(fb.feedbackType) }}
                    </span>
                    <div class="flex items-center gap-2">
                      <span
                        v-if="fb.rating"
                        class="text-sm font-medium"
                        :class="ratingColor(fb.rating)"
                      >
                        {{ t('classroomSimulation.feedback.ratingOutOf', { rating: fb.rating }) }}
                      </span>
                      <span
                        v-if="fb.checkpointsPassed != null && fb.checkpointsTotal != null"
                        class="text-xs text-gray-400"
                      >
                        {{
                          t('classroomSimulation.feedback.checkpoints', {
                            passed: fb.checkpointsPassed,
                            total: fb.checkpointsTotal,
                          })
                        }}
                      </span>
                    </div>
                  </div>
                  <p class="text-sm text-gray-300">{{ fb.summary }}</p>
                  <div class="mt-2 flex items-center gap-3 text-xs text-gray-500">
                    <span v-if="fb.labTemplateId">{{
                      t('classroomSimulation.feedback.labRef', { id: fb.labTemplateId })
                    }}</span>
                    <span v-if="fb.executionDurationMs">{{
                      t('classroomSimulation.feedback.durationMs', { ms: fb.executionDurationMs })
                    }}</span>
                    <span>{{ formatDateTime(fb.createdAt) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
