<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CheckpointProgress } from '@/stores/session'
import { getCheckpointSeverity } from '@/utils/status'
import Tag from '@volt/Tag.vue'

const { t } = useI18n()

/**
 * The session checkpoint list — one row per checkpoint, with the
 * progressive-hint UI nested inside each. Extracted from SessionView.vue
 * (~90 lines of deeply nested template with five conditional branches).
 *
 * The session store owns the underlying data and the
 * `showCheckpointHint` / `hasMoreCheckpointHints` /
 * `getNextCheckpointHintLevel` logic. This component accepts the
 * checkpoints array and the three delegate callbacks as props, so it
 * doesn't reach into the store itself — that keeps the component
 * testable without a Pinia fixture.
 */
interface RevealedHint {
  level: number
  text: string
}

defineProps<{
  checkpoints: CheckpointProgress[]
  /** Revealed hints keyed by checkpoint id. Same shape as sessionStore.revealedCheckpointHints. */
  revealedHints: Record<string, RevealedHint[]>
  hasMoreHints: (checkpointId: string) => boolean
  nextHintLevel: (checkpointId: string) => number
  showNextHint: (checkpointId: string) => void
}>()
</script>

<template>
  <section
    v-if="checkpoints.length > 0"
    class="mt-6"
    aria-labelledby="checkpoints-heading"
  >
    <h3
      id="checkpoints-heading"
      class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2"
    >
      <i class="pi pi-check-square mr-1" aria-hidden="true" />
      {{ t('checkpoints.heading') }}
    </h3>
    <ul class="space-y-3">
      <li
        v-for="checkpoint in checkpoints"
        :key="checkpoint.id"
        :aria-label="t('checkpoints.itemAria', {
          name: checkpoint.name,
          status: checkpoint.status,
          score: checkpoint.score,
          maxScore: checkpoint.maxScore,
        })"
        :class="[
          'p-3 rounded-lg',
          checkpoint.status === 'passed' ? 'bg-green-50 dark:bg-green-900/20' :
          checkpoint.status === 'failed' ? 'bg-red-50 dark:bg-red-900/20' : 'bg-surface-50 dark:bg-surface-800'
        ]"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <i
              :class="[
                'pi',
                checkpoint.status === 'passed' ? 'pi-check-circle text-green-500' :
                checkpoint.status === 'failed' ? 'pi-times-circle text-red-500' : 'pi-circle text-surface-400'
              ]"
              aria-hidden="true"
            />
            <span class="text-sm text-surface-700 dark:text-surface-300">{{ checkpoint.name }}</span>
          </div>
          <Tag
            :value="t('checkpoints.pointsTag', { score: checkpoint.score, maxScore: checkpoint.maxScore })"
            :severity="getCheckpointSeverity(checkpoint.status)"
          />
        </div>

        <!-- Progressive Hints for Checkpoint -->
        <div
          v-if="checkpoint.hintCount && checkpoint.hintCount > 0 && checkpoint.status !== 'passed'"
          class="mt-2 space-y-2"
        >
          <!-- Display revealed hints -->
          <div
            v-for="hint in (revealedHints[checkpoint.id] || [])"
            :key="hint.level"
            class="p-2 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded text-xs"
          >
            <div class="flex items-start gap-1">
              <i class="pi pi-lightbulb text-yellow-500 mt-0.5" aria-hidden="true" />
              <div>
                <span class="font-medium text-yellow-600 dark:text-yellow-400">
                  {{ t('checkpoints.hint.label', { level: hint.level, total: checkpoint.hintCount }) }}
                </span>
                <span class="text-yellow-800 dark:text-yellow-200 ml-1">
                  {{ hint.text }}
                </span>
              </div>
            </div>
          </div>

          <!-- Penalty notice -->
          <div
            v-if="checkpoint.hintPenaltyApplied && checkpoint.hintPenaltyApplied > 0"
            class="text-xs text-orange-600 dark:text-orange-400 flex items-center gap-1"
          >
            <i class="pi pi-info-circle" aria-hidden="true" />
            {{ t('checkpoints.hint.pointsDeducted', checkpoint.hintPenaltyApplied) }}
          </div>

          <!-- Get Hint button -->
          <button
            v-if="hasMoreHints(checkpoint.id)"
            class="text-xs text-primary-600 dark:text-primary-400 hover:underline flex items-center gap-1"
            @click="showNextHint(checkpoint.id)"
          >
            <i class="pi pi-lightbulb" aria-hidden="true" />
            {{ t('checkpoints.hint.getNext', { level: nextHintLevel(checkpoint.id), total: checkpoint.hintCount }) }}
          </button>
          <span
            v-else-if="(checkpoint.hintLevelShown ?? 0) > 0"
            class="text-xs text-surface-500 italic"
          >
            {{ t('checkpoints.hint.allRevealed') }}
          </span>
        </div>
      </li>
    </ul>
  </section>
</template>
