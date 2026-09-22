<script setup lang="ts">
import Skeleton from '@volt/Skeleton.vue'

withDefaults(defineProps<{
  items?: number
  showAvatar?: boolean
  showBadge?: boolean
}>(), {
  items: 5,
  showAvatar: false,
  showBadge: true,
})
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="i in items"
      :key="i"
      class="flex items-center gap-3 p-3 bg-surface-50 dark:bg-surface-800 rounded-lg animate-pulse"
    >
      <!-- Avatar/Icon -->
      <Skeleton v-if="showAvatar" width="2.5rem" height="2.5rem" shape="circle" />
      <Skeleton v-else width="1.5rem" height="1.5rem" borderRadius="0.25rem" />

      <!-- Content -->
      <div class="flex-1 space-y-2">
        <Skeleton :style="{ width: `${60 + Math.random() * 30}%` }" height="0.875rem" />
        <Skeleton :style="{ width: `${40 + Math.random() * 20}%` }" height="0.75rem" />
      </div>

      <!-- Badge/Status -->
      <Skeleton v-if="showBadge" width="3.5rem" height="1.25rem" borderRadius="9999px" />
    </div>
  </div>
</template>
