<script setup lang="ts">
import { computed } from 'vue'
import type { AssessmentStatus } from '@/api'

interface Props {
  status: AssessmentStatus
  size?: 'sm' | 'md' | 'lg'
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md'
})

const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'w-4 h-4',
  md: 'w-5 h-5',
  lg: 'w-6 h-6'
}

/** Safe accessor for size classes */
const sizeClass = computed(() => sizeClasses[props.size])
</script>

<template>
  <span :class="['inline-flex items-center justify-center', sizeClass]">
    <!-- Correct - Green Checkmark -->
    <svg
      v-if="status === 'correct'"
      class="text-green-500"
      :class="sizeClass"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2.5"
        d="M5 13l4 4L19 7"
      />
    </svg>

    <!-- Incorrect - Red X -->
    <svg
      v-else-if="status === 'incorrect'"
      class="text-red-500"
      :class="sizeClass"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2.5"
        d="M6 18L18 6M6 6l12 12"
      />
    </svg>

    <!-- Incomplete - Yellow Circle -->
    <svg
      v-else-if="status === 'incomplete'"
      class="text-yellow-500"
      :class="sizeClass"
      fill="currentColor"
      viewBox="0 0 24 24"
    >
      <circle cx="12" cy="12" r="8" fill="none" stroke="currentColor" stroke-width="2" />
      <circle cx="12" cy="12" r="4" />
    </svg>

    <!-- Pending - Gray Clock -->
    <svg
      v-else-if="status === 'pending'"
      class="text-gray-400 animate-pulse"
      :class="sizeClass"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <circle cx="12" cy="12" r="9" stroke-width="2" />
      <path stroke-linecap="round" stroke-width="2" d="M12 7v5l3 3" />
    </svg>

    <!-- Error - Red Exclamation -->
    <svg
      v-else-if="status === 'error'"
      class="text-red-600"
      :class="sizeClass"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>

    <!-- Default/Unknown - Gray Circle -->
    <svg
      v-else
      class="text-gray-300"
      :class="sizeClass"
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <circle cx="12" cy="12" r="9" stroke-width="2" />
    </svg>
  </span>
</template>
