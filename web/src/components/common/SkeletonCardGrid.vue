<script setup lang="ts">
import SkeletonCard from './SkeletonCard.vue'

const props = withDefaults(defineProps<{
  /** Number of skeleton cards to show */
  count?: number
  /** Grid columns on different breakpoints (Tailwind classes) */
  columns?: string
  /** Show tags in cards */
  showTags?: boolean
  /** Number of action buttons */
  showActions?: boolean
}>(), {
  count: 6,
  columns: 'grid-cols-1 md:grid-cols-2 lg:grid-cols-3',
  showTags: false,
  showActions: true,
})
</script>

<template>
  <div class="grid gap-6" :class="props.columns">
    <SkeletonCard
      v-for="i in props.count"
      :key="i"
      :show-tags="props.showTags"
      :show-actions="props.showActions"
    />
  </div>
</template>
