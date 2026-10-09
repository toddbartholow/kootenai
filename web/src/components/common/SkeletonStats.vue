<script setup lang="ts">
import Skeleton from '@volt/Skeleton.vue'

withDefaults(defineProps<{
  columns?: number
}>(), {
  columns: 3,
})
</script>

<template>
  <div
    class="grid gap-4 p-4 bg-surface-50 dark:bg-surface-800 rounded-lg animate-pulse"
    :class="`grid-cols-${columns}`"
  >
    <div v-for="i in columns" :key="i" class="text-center space-y-2">
      <Skeleton width="3rem" height="2rem" class="mx-auto" />
      <Skeleton width="4rem" height="0.75rem" class="mx-auto" />
    </div>
  </div>
</template>
