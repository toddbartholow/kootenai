<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Pathway, PathwayEnrollment } from '@/api'
import { getDifficultyBadgeColor, getDifficultyLabel } from '@/utils/status'
import { formatDuration } from '@/utils/format'
import Button from '@volt/Button.vue'

const { t } = useI18n()

interface Props {
  pathway: Pathway
  enrollment?: PathwayEnrollment | undefined
  featured?: boolean
  enrolling?: boolean
}

interface Emits {
  (e: 'view', pathway: Pathway): void
  (e: 'enroll', pathway: Pathway, event: Event): void
  (e: 'continue', enrollment: PathwayEnrollment, event: Event): void
}

const props = withDefaults(defineProps<Props>(), {
  featured: false,
  enrolling: false,
})

const emit = defineEmits<Emits>()

// formatDuration imported from @/utils/format
// Note: PathwayCard passes hours * 60 for minute-based formatting
function formatHours(hours: number | undefined): string {
  if (!hours) return t('pathway.card.naDuration')
  return formatDuration(hours * 60)
}

function handleClick() {
  emit('view', props.pathway)
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') {
    emit('view', props.pathway)
  }
}

function handleEnroll(event: Event) {
  emit('enroll', props.pathway, event)
}

function handleContinue(event: Event) {
  if (props.enrollment) {
    emit('continue', props.enrollment, event)
  }
}
</script>

<template>
  <article
    class="bg-white dark:bg-surface-800 rounded-lg border border-surface-200 dark:border-surface-700 p-5 hover:shadow-lg transition-shadow cursor-pointer flex flex-col"
    tabindex="0"
    role="button"
    @click="handleClick"
    @keydown="handleKeydown"
  >
    <!-- Tags Row (Featured badge + Difficulty) -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <span
          v-if="featured"
          class="px-2 py-0.5 text-xs font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300"
        >
          {{ t('pathway.card.featured') }}
        </span>
        <span
          v-if="pathway.status === 'draft'"
          class="px-2 py-0.5 text-xs font-medium rounded bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300"
        >
          {{ t('pathway.card.draft') }}
        </span>
      </div>
      <span :class="['px-2 py-0.5 text-xs font-medium rounded whitespace-nowrap', getDifficultyBadgeColor(pathway.difficulty)]">
        {{ getDifficultyLabel(pathway.difficulty) }}
      </span>
    </div>

    <!-- Title & Description -->
    <h3 class="font-semibold text-lg text-surface-900 dark:text-surface-100 mb-2 line-clamp-1">
      {{ pathway.name }}
    </h3>
    <p class="text-sm text-surface-600 dark:text-surface-400 line-clamp-2 mb-3">
      {{ pathway.shortDescription || pathway.description }}
    </p>

    <!-- Tags (non-featured mode) -->
    <div v-if="!featured && pathway.tags?.length" class="flex flex-wrap gap-1 mb-3">
      <span
        v-for="tag in pathway.tags.slice(0, 3)"
        :key="tag"
        class="px-2 py-0.5 text-xs bg-surface-100 dark:bg-surface-700 text-surface-600 dark:text-surface-400 rounded"
      >
        {{ tag }}
      </span>
      <span
        v-if="pathway.tags.length > 3"
        class="px-2 py-0.5 text-xs bg-surface-100 dark:bg-surface-700 text-surface-600 dark:text-surface-400 rounded"
      >
        {{ t('pathway.card.moreTags', { count: pathway.tags.length - 3 }) }}
      </span>
    </div>

    <!-- Stats Row -->
    <div class="flex items-center gap-4 text-sm text-surface-500 mb-4">
      <span>{{ formatHours(pathway.estimatedHours) }}</span>
      <span class="text-surface-300 dark:text-surface-600">•</span>
      <span>{{ t('pathway.card.modulesLabel', { count: pathway.moduleCount || 0 }) }}</span>
    </div>

    <!-- Enrollment Progress (if enrolled) -->
    <div v-if="enrollment" class="mb-4">
      <div class="flex justify-between text-sm mb-1">
        <span class="text-surface-500">{{ t('pathway.card.progressLabel') }}</span>
        <span class="font-medium text-surface-700 dark:text-surface-300">
          {{ Math.round(enrollment.percentage) }}%
        </span>
      </div>
      <div class="h-2 bg-surface-200 dark:bg-surface-700 rounded-full overflow-hidden">
        <div
          class="h-full bg-blue-500 rounded-full transition-all"
          :style="{ width: `${enrollment.percentage}%` }"
        />
      </div>
    </div>

    <!-- Spacer to push buttons to bottom -->
    <div class="flex-1" />

    <!-- Action Buttons -->
    <div class="flex gap-2" @click.stop>
      <!-- Featured mode: single button -->
      <template v-if="featured">
        <Button
          v-if="enrollment"
          @click="handleContinue"
          :label="t('pathway.card.continueAction')"
          icon="pi pi-arrow-right"
          class="w-full"
          severity="success"
        />
        <Button
          v-else
          @click="handleEnroll"
          :loading="enrolling"
          :disabled="enrolling"
          :label="t('pathway.card.enrollAction')"
          class="w-full"
          severity="success"
        />
      </template>

      <!-- Regular mode: two buttons -->
      <template v-else>
        <Button
          @click="handleClick"
          :label="enrollment ? t('pathway.card.viewProgress') : t('pathway.card.details')"
          severity="secondary"
          outlined
          size="small"
          class="flex-1"
        />
        <Button
          v-if="enrollment"
          @click="handleContinue"
          :label="t('pathway.card.continueAction')"
          severity="success"
          size="small"
          class="flex-1"
        />
        <Button
          v-else
          @click="handleEnroll"
          :loading="enrolling"
          :disabled="enrolling"
          :label="t('pathway.card.enrollAction')"
          severity="success"
          size="small"
          class="flex-1"
        />
      </template>
    </div>
  </article>
</template>
