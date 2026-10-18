<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PathwayModule, ModuleLab } from '@/api'
import { formatDuration } from '@/utils/format'

const { t } = useI18n()

interface Props {
  modules: PathwayModule[]
  moduleProgress?: Map<string, { status: string; percentage: number }>
  onModuleClick?: (module: PathwayModule) => void
  onLabClick?: (lab: ModuleLab, module: PathwayModule) => void
}

const props = defineProps<Props>()

function getModuleStatus(moduleId: string): string {
  return props.moduleProgress?.get(moduleId)?.status || 'locked'
}

// Sort modules by display order
const sortedModules = computed(() => {
  return [...props.modules].sort((a, b) => a.displayOrder - b.displayOrder)
})

function handleModuleClick(module: PathwayModule) {
  const status = getModuleStatus(module.id)
  if (status !== 'locked' && props.onModuleClick) {
    props.onModuleClick(module)
  }
}

function isModuleClickable(moduleId: string): boolean {
  const status = getModuleStatus(moduleId)
  return status !== 'locked'
}

function getStatusBadge(status: string): { label: string; class: string } {
  switch (status) {
    case 'completed':
      return { label: t('pathway.roadmap.badgeCompleted'), class: 'bg-green-500 text-white' }
    case 'in_progress':
      return { label: t('pathway.roadmap.badgeInProgress'), class: 'bg-blue-500 text-white' }
    case 'unlocked':
      return { label: t('pathway.roadmap.badgeStart'), class: 'bg-amber-500 text-white' }
    default:
      return { label: t('pathway.roadmap.badgeLocked'), class: 'bg-surface-400 text-white' }
  }
}

function getCircleColor(status: string): string {
  switch (status) {
    case 'completed':
      return 'bg-green-500 text-white'
    case 'in_progress':
      return 'bg-blue-500 text-white'
    case 'unlocked':
      return 'bg-amber-500 text-white'
    default:
      return 'bg-surface-300 dark:bg-surface-600 text-surface-500 dark:text-surface-400'
  }
}

function getLineColor(index: number): string {
  // Line is colored based on whether the module above is completed
  if (index === 0) return 'bg-surface-300 dark:bg-surface-600'

  const prevModule = sortedModules.value[index - 1]
  if (prevModule) {
    const status = getModuleStatus(prevModule.id)
    if (status === 'completed') {
      return 'bg-green-400 dark:bg-green-600'
    }
  }
  return 'bg-surface-300 dark:bg-surface-600'
}

// formatDuration imported from @/utils/format
</script>

<template>
  <div class="pathway-roadmap">
    <!-- Legend -->
    <div class="flex flex-wrap items-center justify-end gap-4 mb-6 text-sm">
      <div class="flex items-center gap-2">
        <div class="w-3 h-3 rounded-full bg-green-500"></div>
        <span class="text-surface-600 dark:text-surface-400">{{ t('pathway.roadmap.legendCompleted') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="w-3 h-3 rounded-full bg-blue-500"></div>
        <span class="text-surface-600 dark:text-surface-400">{{ t('pathway.roadmap.legendInProgress') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="w-3 h-3 rounded-full bg-amber-500"></div>
        <span class="text-surface-600 dark:text-surface-400">{{ t('pathway.roadmap.legendUnlocked') }}</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="w-3 h-3 rounded-full bg-surface-300 dark:bg-surface-600"></div>
        <span class="text-surface-600 dark:text-surface-400">{{ t('pathway.roadmap.legendLocked') }}</span>
      </div>
    </div>

    <!-- Module list with timeline -->
    <div class="space-y-0">
      <div
        v-for="(module, index) in sortedModules"
        :key="module.id"
        class="relative flex gap-4"
      >
        <!-- Timeline column -->
        <div class="flex flex-col items-center w-8 flex-shrink-0">
          <!-- Connecting line above (except first) -->
          <div
            v-if="index > 0"
            :class="['w-0.5 h-4', getLineColor(index)]"
          ></div>

          <!-- Numbered circle -->
          <div
            :class="[
              'w-8 h-8 rounded-full flex items-center justify-center text-sm font-semibold z-10',
              getCircleColor(getModuleStatus(module.id))
            ]"
          >
            <i v-if="getModuleStatus(module.id) === 'completed'" class="pi pi-check text-sm" />
            <span v-else>{{ index + 1 }}</span>
          </div>

          <!-- Connecting line below (except last) -->
          <div
            v-if="index < sortedModules.length - 1"
            :class="[
              'w-0.5 flex-1',
              getModuleStatus(module.id) === 'completed' ? 'bg-green-400 dark:bg-green-600' : 'bg-surface-300 dark:bg-surface-600'
            ]"
          ></div>
        </div>

        <!-- Module card -->
        <div class="flex-1 pb-6">
          <button
            type="button"
            :class="[
              'w-full text-left p-4 rounded-lg border transition-all',
              isModuleClickable(module.id)
                ? 'cursor-pointer hover:shadow-md hover:border-primary-300 dark:hover:border-primary-700'
                : 'cursor-not-allowed',
              getModuleStatus(module.id) === 'in_progress'
                ? 'border-blue-300 dark:border-blue-700 bg-blue-50/50 dark:bg-blue-900/10'
                : 'border-surface-200 dark:border-surface-700 bg-white dark:bg-surface-800',
              getModuleStatus(module.id) === 'locked' ? 'opacity-60' : ''
            ]"
            :disabled="!isModuleClickable(module.id)"
            @click="handleModuleClick(module)"
          >
            <!-- Header with title and status badge -->
            <div class="flex items-start justify-between gap-3 mb-2">
              <h3 class="font-semibold text-surface-900 dark:text-surface-100">
                {{ t('pathway.roadmap.moduleHeader', { number: index + 1, name: module.name }) }}
              </h3>
              <span
                :class="[
                  'px-2 py-0.5 text-xs font-medium rounded whitespace-nowrap',
                  getStatusBadge(getModuleStatus(module.id)).class
                ]"
              >
                {{ getStatusBadge(getModuleStatus(module.id)).label }}
              </span>
            </div>

            <!-- Description -->
            <p class="text-sm text-surface-600 dark:text-surface-400 mb-3 line-clamp-2">
              {{ module.description }}
            </p>

            <!-- Stats row -->
            <div class="flex items-center gap-4 text-sm text-surface-500">
              <span>{{ t('pathway.roadmap.labsCount', { count: module.labCount || module.labs?.length || 0 }) }}</span>
              <span class="text-surface-300 dark:text-surface-600">•</span>
              <span>{{ formatDuration(module.estimatedMinutes) }}</span>
              <span class="text-surface-300 dark:text-surface-600">•</span>
              <span>{{ t('pathway.roadmap.pointsSuffix', { count: module.totalPoints || 0 }) }}</span>
            </div>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pathway-roadmap {
  padding: 0.5rem;
}
</style>
