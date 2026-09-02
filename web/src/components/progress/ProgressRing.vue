<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  progress: number
  size?: number
  strokeWidth?: number
  color?: 'blue' | 'green' | 'yellow' | 'purple' | 'red' | 'primary'
  showLabel?: boolean
  labelSize?: 'sm' | 'md' | 'lg'
  animate?: boolean
}>(), {
  size: 80,
  strokeWidth: 8,
  color: 'primary',
  showLabel: true,
  labelSize: 'md',
  animate: true,
})

const radius = computed(() => (props.size - props.strokeWidth) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const normalizedProgress = computed(() => Math.min(100, Math.max(0, props.progress)))
const offset = computed(() => circumference.value - (normalizedProgress.value / 100) * circumference.value)
const center = computed(() => props.size / 2)

const colorClass = computed(() => {
  switch (props.color) {
    case 'blue': return 'text-blue-500'
    case 'green': return 'text-green-500'
    case 'yellow': return 'text-yellow-500'
    case 'purple': return 'text-purple-500'
    case 'red': return 'text-red-500'
    case 'primary':
    default: return 'text-primary'
  }
})

const labelSizeClass = computed(() => {
  switch (props.labelSize) {
    case 'sm': return 'text-xs'
    case 'lg': return 'text-xl font-bold'
    case 'md':
    default: return 'text-sm font-semibold'
  }
})
</script>

<template>
  <div class="relative inline-flex items-center justify-center">
    <svg
      :width="size"
      :height="size"
      class="transform -rotate-90"
    >
      <!-- Background circle -->
      <circle
        :cx="center"
        :cy="center"
        :r="radius"
        class="stroke-gray-200 dark:stroke-gray-700"
        :stroke-width="strokeWidth"
        fill="none"
      />
      <!-- Progress circle -->
      <circle
        :cx="center"
        :cy="center"
        :r="radius"
        class="stroke-current"
        :class="[colorClass, { 'transition-all duration-500 ease-out': animate }]"
        :stroke-width="strokeWidth"
        fill="none"
        stroke-linecap="round"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="offset"
      />
    </svg>
    <!-- Center label -->
    <div
      v-if="showLabel"
      class="absolute inset-0 flex items-center justify-center"
    >
      <span :class="[labelSizeClass, 'text-gray-900 dark:text-gray-100']">
        <slot>{{ Math.round(normalizedProgress) }}%</slot>
      </span>
    </div>
  </div>
</template>
