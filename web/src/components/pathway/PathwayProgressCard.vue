<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import { formatDuration } from '@/utils/format'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatNumber } = useFormatters()

interface Props {
  /** Whether user is enrolled */
  isEnrolled: boolean
  /** Overall progress percentage (0-100) */
  progress: number
  /** Total number of modules */
  moduleCount: number
  /** Number of completed modules */
  completedModules: number
  /** Total number of labs */
  labCount: number
  /** Estimated duration in hours */
  estimatedHours: number
  /** Total possible points */
  totalPoints: number
  /** Earned points (when enrolled) */
  earnedPoints?: number
  /** Whether enrollment is in progress */
  enrolling?: boolean
  /** Whether the pathway is complete */
  isComplete?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  earnedPoints: 0,
  enrolling: false,
  isComplete: false,
})

const emit = defineEmits<{
  (e: 'enroll'): void
  (e: 'continue'): void
  (e: 'unenroll'): void
  (e: 'view-certificate'): void
}>()

const progressLabel = computed(() => {
  if (props.isComplete) return t('pathway.progressCard.completedState')
  return t('pathway.progressCard.moduleCountProgress', {
    completed: props.completedModules,
    total: props.moduleCount,
  })
})

// formatDuration imported from @/utils/format
// Note: PathwayProgressCard passes hours * 60 for minute-based formatting
function formatHours(hours: number): string {
  return formatDuration(hours * 60)
}
</script>

<template>
  <Card class="pathway-progress-card">
    <template #content>
      <div class="space-y-4">
        <!-- Progress header (when enrolled) -->
        <div v-if="isEnrolled" class="space-y-2">
          <div class="flex justify-between items-center">
            <span class="text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.progressCard.yourProgress') }}</span>
            <span
              class="text-sm font-bold"
              :class="isComplete ? 'text-green-600 dark:text-green-400' : 'text-primary-600 dark:text-primary-400'"
            >
              {{ progress }}%
            </span>
          </div>
          <ProgressBar
            :value="progress"
            :showValue="false"
            class="h-2"
            :class="isComplete ? '[&_.p-progressbar-value]:bg-green-500' : ''"
            :aria-label="t('pathway.progressCard.progressAria', { percent: progress })"
          />
          <p class="text-xs text-surface-500">{{ progressLabel }}</p>
        </div>

        <!-- Stats grid -->
        <div class="grid grid-cols-2 gap-4 text-center">
          <div>
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
              {{ moduleCount }}
            </p>
            <p class="text-sm text-surface-500">{{ t('pathway.progressCard.statsModules') }}</p>
          </div>
          <div>
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
              {{ labCount }}
            </p>
            <p class="text-sm text-surface-500">{{ t('pathway.progressCard.statsLabs') }}</p>
          </div>
          <div>
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
              {{ formatHours(estimatedHours) }}
            </p>
            <p class="text-sm text-surface-500">{{ t('pathway.progressCard.statsDuration') }}</p>
          </div>
          <div>
            <p
              class="text-2xl font-bold"
              :class="isEnrolled ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'"
            >
              {{ isEnrolled ? formatNumber(earnedPoints ?? 0) : formatNumber(totalPoints) }}
            </p>
            <p class="text-sm text-surface-500">{{ t('pathway.progressCard.statsPoints') }}</p>
          </div>
        </div>

        <!-- Action button -->
        <div class="border-t border-surface-200 dark:border-surface-700 pt-4">
          <Button
            v-if="isComplete"
            :label="t('pathway.progressCard.viewCertificate')"
            icon="pi pi-trophy"
            class="w-full"
            size="large"
            severity="success"
            @click="emit('view-certificate')"
          />
          <Button
            v-else-if="isEnrolled"
            :label="t('pathway.progressCard.continueLearning')"
            icon="pi pi-play"
            class="w-full"
            size="large"
            @click="emit('continue')"
          />
          <Button
            v-else
            :label="t('pathway.progressCard.enrollNow')"
            icon="pi pi-plus"
            class="w-full"
            size="large"
            :loading="enrolling"
            @click="emit('enroll')"
          />
        </div>

        <!-- Unenroll option (when enrolled and not complete) -->
        <div v-if="isEnrolled && !isComplete" class="text-center">
          <button
            type="button"
            class="text-sm text-surface-400 hover:text-surface-600 dark:hover:text-surface-300 transition-colors"
            @click="emit('unenroll')"
          >
            {{ t('pathway.progressCard.unenroll') }}
          </button>
        </div>
      </div>
    </template>
  </Card>
</template>
