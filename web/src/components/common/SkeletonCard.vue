<script setup lang="ts">
import Skeleton from '@volt/Skeleton.vue'

withDefaults(defineProps<{
  /** Number of content lines */
  lines?: number
  /** Show header with title and badge */
  showHeader?: boolean
  /** Show action buttons at bottom */
  showActions?: boolean
  /** Show tags section */
  showTags?: boolean
  /** Show info lines with icons */
  showInfoLines?: boolean
  /** Number of info lines */
  infoLines?: number
  /** Header title width */
  headerWidth?: string
}>(), {
  lines: 2,
  showHeader: true,
  showActions: true,
  showTags: false,
  showInfoLines: true,
  infoLines: 2,
  headerWidth: '60%',
})
</script>

<template>
  <div class="rounded-xl bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 shadow-md p-5">
    <div class="space-y-4">
      <!-- Header skeleton -->
      <div v-if="showHeader" class="flex items-start justify-between">
        <Skeleton :style="{ width: headerWidth }" height="1.25rem" />
        <Skeleton width="4rem" height="1.5rem" borderRadius="9999px" />
      </div>

      <!-- Content lines -->
      <div class="space-y-2">
        <Skeleton
          v-for="i in lines"
          :key="i"
          :style="{ width: i === lines ? '85%' : '100%' }"
          height="0.875rem"
        />
      </div>

      <!-- Tags -->
      <div v-if="showTags" class="flex gap-1">
        <Skeleton width="3rem" height="1.25rem" borderRadius="9999px" />
        <Skeleton width="4rem" height="1.25rem" borderRadius="9999px" />
        <Skeleton width="3.5rem" height="1.25rem" borderRadius="9999px" />
      </div>

      <!-- Info lines with icons -->
      <div v-if="showInfoLines" class="space-y-2">
        <div v-for="n in infoLines" :key="n" class="flex items-center gap-2">
          <Skeleton width="1rem" height="1rem" shape="circle" />
          <Skeleton :style="{ width: n % 2 === 0 ? '40%' : '50%' }" height="0.875rem" />
        </div>
      </div>

      <!-- Actions skeleton -->
      <div v-if="showActions" class="flex gap-2 pt-2">
        <Skeleton class="flex-1" height="2rem" borderRadius="0.375rem" />
        <Skeleton class="flex-1" height="2rem" borderRadius="0.375rem" />
      </div>
    </div>
  </div>
</template>
