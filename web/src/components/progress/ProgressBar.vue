<script setup lang="ts">
defineProps<{
  progress: number
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
  color?: 'blue' | 'green' | 'yellow' | 'purple'
}>()
</script>

<template>
  <div class="w-full">
    <div class="flex justify-between items-center mb-1" v-if="showLabel">
      <slot name="label"></slot>
      <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ progress }}%</span>
    </div>
    <div
      class="w-full bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden"
      :class="{
        'h-1.5': size === 'sm',
        'h-2.5': size === 'md' || !size,
        'h-4': size === 'lg'
      }"
    >
      <div
        class="h-full rounded-full transition-all duration-500 ease-out"
        :class="{
          'bg-blue-600': color === 'blue' || !color,
          'bg-green-600': color === 'green',
          'bg-yellow-500': color === 'yellow',
          'bg-purple-600': color === 'purple'
        }"
        :style="{ width: `${Math.min(100, Math.max(0, progress))}%` }"
      ></div>
    </div>
  </div>
</template>
