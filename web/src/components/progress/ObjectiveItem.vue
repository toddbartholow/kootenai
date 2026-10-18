<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDateTime } = useFormatters()

export interface Objective {
  id: string
  name: string
  description: string
  order: number
  points: number
  status: 'completed' | 'current' | 'locked' | 'available'
  completedAt?: string
}

defineProps<{
  objective: Objective
}>()
</script>

<template>
  <div
    class="flex items-start gap-4 p-4 rounded-lg border transition-all"
    :class="{
      'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800':
        objective.status === 'completed',
      'bg-blue-50 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700 ring-2 ring-blue-400 dark:ring-blue-600':
        objective.status === 'current',
      'bg-gray-50 dark:bg-gray-800/50 border-gray-200 dark:border-gray-700 opacity-60':
        objective.status === 'locked',
      'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700':
        objective.status === 'available',
    }"
  >
    <!-- Status Icon -->
    <div class="flex-shrink-0 mt-0.5">
      <!-- Completed -->
      <div
        v-if="objective.status === 'completed'"
        class="w-8 h-8 rounded-full bg-green-500 flex items-center justify-center"
      >
        <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 13l4 4L19 7"
          />
        </svg>
      </div>
      <!-- Current -->
      <div
        v-else-if="objective.status === 'current'"
        class="w-8 h-8 rounded-full bg-blue-500 flex items-center justify-center animate-pulse"
      >
        <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      <!-- Locked -->
      <div
        v-else-if="objective.status === 'locked'"
        class="w-8 h-8 rounded-full bg-gray-400 dark:bg-gray-600 flex items-center justify-center"
      >
        <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
          />
        </svg>
      </div>
      <!-- Available -->
      <div
        v-else
        class="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-600 flex items-center justify-center"
      >
        <span class="text-sm font-bold text-gray-600 dark:text-gray-300">{{
          objective.order
        }}</span>
      </div>
    </div>

    <!-- Content -->
    <div class="flex-1 min-w-0">
      <div class="flex items-center justify-between">
        <h4
          class="font-medium"
          :class="{
            'text-green-800 dark:text-green-300': objective.status === 'completed',
            'text-blue-800 dark:text-blue-300': objective.status === 'current',
            'text-gray-500 dark:text-gray-400': objective.status === 'locked',
            'text-gray-900 dark:text-white': objective.status === 'available',
          }"
        >
          {{ objective.name }}
        </h4>
        <span
          class="text-sm font-medium px-2 py-0.5 rounded-md"
          :class="{
            'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300':
              objective.status === 'completed',
            'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300':
              objective.status === 'current' || objective.status === 'available',
            'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400':
              objective.status === 'locked',
          }"
        >
          {{ t('objectiveItem.pointsSuffix', { points: objective.points }) }}
        </span>
      </div>
      <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">{{ objective.description }}</p>
      <p v-if="objective.completedAt" class="text-xs text-green-600 dark:text-green-400 mt-2">
        {{ t('objectiveItem.completedAt', { when: formatDateTime(objective.completedAt) }) }}
      </p>
    </div>
  </div>
</template>
