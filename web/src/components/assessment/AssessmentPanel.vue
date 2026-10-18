<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAssessmentStore } from '@/stores/assessment'
import AssessmentTree from './AssessmentTree.vue'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDateTime } = useFormatters()

// Props defined for future use (parent may pass sessionId for context)
defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  (e: 'run'): void
}>()

const store = useAssessmentStore()

const components = computed(() => store.result?.components ?? [])

// Progress bar color based on percentage
const progressColor = computed(() => {
  const pct = store.percentage
  if (pct >= 90) return 'bg-green-500'
  if (pct >= 70) return 'bg-yellow-500'
  if (pct >= 50) return 'bg-orange-500'
  return 'bg-red-500'
})

async function runAssessment() {
  emit('run')
}
</script>

<template>
  <div class="assessment-panel space-y-4">
    <!-- Score Summary Card -->
    <div class="bg-surface-0 dark:bg-surface-800 rounded-lg shadow overflow-hidden border border-surface-200 dark:border-surface-700">
      <div class="px-4 py-3 border-b border-surface-200 dark:border-surface-700 bg-gradient-to-r from-primary-500 to-primary-600">
        <h2 class="text-lg font-semibold text-white">{{ t('assessment.panel.resultsHeading') }}</h2>
      </div>

      <div class="p-4">
        <!-- Score Display -->
        <div class="flex items-center justify-between mb-4">
          <div>
            <div class="text-3xl font-bold text-surface-800 dark:text-surface-100">
              {{ store.score }}<span class="text-xl text-surface-500 dark:text-surface-400">/{{ store.maxScore }}</span>
            </div>
            <div class="text-sm text-surface-500 dark:text-surface-400">{{ t('assessment.panel.pointsEarned') }}</div>
          </div>
          <div class="text-right">
            <div class="text-3xl font-bold" :class="store.percentage >= 70 ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
              {{ store.percentage.toFixed(0) }}%
            </div>
            <div class="text-sm text-surface-500 dark:text-surface-400">
              {{ t('assessment.panel.itemsFraction', { passed: store.passedCount, total: store.itemCount }) }}
            </div>
          </div>
        </div>

        <!-- Progress Bar -->
        <div class="w-full bg-surface-200 dark:bg-surface-700 rounded-full h-3 mb-4">
          <div
            class="h-3 rounded-full transition-all duration-500"
            :class="progressColor"
            :style="{ width: `${store.percentage}%` }"
          ></div>
        </div>

        <!-- Run Assessment Button -->
        <button
          @click="runAssessment"
          :disabled="store.isRunning"
          class="w-full py-2 px-4 rounded-lg font-medium transition-colors"
          :class="[
            store.isRunning
              ? 'bg-surface-300 dark:bg-surface-600 text-surface-500 dark:text-surface-400 cursor-not-allowed'
              : 'bg-primary-600 text-white hover:bg-primary-700'
          ]"
        >
          <span v-if="store.isRunning" class="flex items-center justify-center">
            <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ t('assessment.panel.checking') }}
          </span>
          <span v-else>{{ t('assessment.panel.checkMyWork') }}</span>
        </button>

        <!-- Status -->
        <div v-if="store.result" class="mt-3 text-center text-sm text-surface-500 dark:text-surface-400">
          {{ t('assessment.panel.lastChecked', { when: formatDateTime(store.result.lastChecked) }) }}
        </div>
      </div>
    </div>

    <!-- Component Breakdown -->
    <div v-if="components.length > 0" class="bg-surface-0 dark:bg-surface-800 rounded-lg shadow border border-surface-200 dark:border-surface-700">
      <div class="px-4 py-3 border-b border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-900">
        <h3 class="text-sm font-semibold text-surface-700 dark:text-surface-300">{{ t('assessment.panel.componentScoresHeading') }}</h3>
      </div>
      <div class="divide-y divide-surface-100 dark:divide-surface-700">
        <div
          v-for="component in components"
          :key="component.id"
          class="px-4 py-3 flex items-center justify-between"
        >
          <div class="flex-1">
            <div class="text-sm font-medium text-surface-800 dark:text-surface-200">{{ component.description }}</div>
            <div class="text-xs text-surface-500 dark:text-surface-400">
              {{ t('assessment.panel.componentItems', { passed: component.passedItems, total: component.totalItems }) }}
            </div>
          </div>
          <div class="text-right">
            <div class="text-sm font-semibold" :class="component.percentage >= 100 ? 'text-green-600 dark:text-green-400' : 'text-surface-700 dark:text-surface-300'">
              {{ component.earnedPoints }}/{{ component.maxPoints }}
            </div>
            <div class="w-24 bg-surface-200 dark:bg-surface-700 rounded-full h-1.5 mt-1">
              <div
                class="h-1.5 rounded-full transition-all duration-300"
                :class="component.percentage >= 100 ? 'bg-green-500' : 'bg-primary-500'"
                :style="{ width: `${component.percentage}%` }"
              ></div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Assessment Tree -->
    <AssessmentTree />

    <!-- Error Display -->
    <div
      v-if="store.error"
      class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4"
    >
      <div class="flex items-start">
        <svg class="w-5 h-5 text-red-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <div class="flex-1">
          <div class="flex items-center justify-between">
            <h4 class="text-sm font-medium text-red-800 dark:text-red-300">
              {{ store.error.type === 'network' ? t('assessment.panel.errorTypes.network') :
                 store.error.type === 'server' ? t('assessment.panel.errorTypes.server') : t('assessment.panel.errorTypes.generic') }}
            </h4>
            <button
              @click="store.clearError()"
              class="text-red-400 hover:text-red-600 dark:hover:text-red-300"
              :aria-label="t('assessment.panel.dismissErrorAria')"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <p class="mt-1 text-sm text-red-700 dark:text-red-400">{{ store.errorMessage }}</p>
          <div class="mt-2 flex items-center gap-2">
            <span v-if="store.error.code" class="text-xs text-red-500 dark:text-red-400">
              {{ t('assessment.panel.errorCode', { code: store.error.code }) }}
            </span>
            <button
              v-if="store.canRetry"
              @click="runAssessment"
              class="text-xs text-red-700 dark:text-red-400 hover:text-red-900 dark:hover:text-red-300 underline"
            >
              {{ t('assessment.panel.tryAgain') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.assessment-panel {
  max-width: 400px;
}
</style>
