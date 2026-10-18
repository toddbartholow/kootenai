<script setup lang="ts">
import Skeleton from '@volt/Skeleton.vue'
import { SkeletonCard } from './index'
</script>

<template>
  <div class="space-y-8">
    <!-- Stats Grid Skeleton -->
    <div class="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      <div
        v-for="i in 6"
        :key="i"
        class="flex flex-col rounded-xl bg-surface-0 dark:bg-surface-900 shadow-md p-5 border border-surface-200 dark:border-surface-700"
      >
        <div class="text-center space-y-2">
          <Skeleton width="2rem" height="2rem" shape="circle" class="mx-auto" />
          <Skeleton width="3rem" height="1.5rem" class="mx-auto" />
          <Skeleton width="4rem" height="0.75rem" class="mx-auto" />
        </div>
      </div>
    </div>

    <!-- Main Content Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Left Column - Enrollments & Sessions -->
      <div class="lg:col-span-2 space-y-6">
        <!-- Enrollments Section -->
        <div class="rounded-xl bg-surface-0 dark:bg-surface-900 shadow-md border border-surface-200 dark:border-surface-700 p-5">
          <div class="flex items-center justify-between mb-4">
            <Skeleton width="10rem" height="1.25rem" />
            <Skeleton width="5rem" height="2rem" borderRadius="0.375rem" />
          </div>
          <div class="space-y-4">
            <div v-for="i in 2" :key="i" class="p-4 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <div class="flex justify-between items-start mb-3">
                <Skeleton width="60%" height="1rem" />
                <Skeleton width="4rem" height="1.25rem" borderRadius="9999px" />
              </div>
              <Skeleton width="100%" height="0.5rem" borderRadius="9999px" />
              <div class="flex justify-between mt-2">
                <Skeleton width="30%" height="0.75rem" />
                <Skeleton width="3rem" height="0.75rem" />
              </div>
            </div>
          </div>
        </div>

        <!-- Sessions Section -->
        <div class="rounded-xl bg-surface-0 dark:bg-surface-900 shadow-md border border-surface-200 dark:border-surface-700 p-5">
          <div class="flex items-center justify-between mb-4">
            <Skeleton width="8rem" height="1.25rem" />
            <Skeleton width="5rem" height="2rem" borderRadius="0.375rem" />
          </div>
          <div class="space-y-3">
            <div v-for="i in 2" :key="i" class="flex items-center gap-4 p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
              <Skeleton width="2.5rem" height="2.5rem" shape="circle" />
              <div class="flex-1 space-y-2">
                <Skeleton width="50%" height="0.875rem" />
                <Skeleton width="30%" height="0.75rem" />
              </div>
              <Skeleton width="4rem" height="1.25rem" borderRadius="9999px" />
            </div>
          </div>
        </div>
      </div>

      <!-- Right Column - Recommendations -->
      <div class="space-y-6">
        <SkeletonCard :lines="2" :show-tags="false" :show-actions="true" />
        <SkeletonCard :lines="2" :show-tags="false" :show-actions="true" />
      </div>
    </div>
  </div>
</template>
