<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PathwayModuleCard from './PathwayModuleCard.vue'
import type { PathwayModule, ModuleProgressStatus } from '@/api'

const { t } = useI18n()

interface Props {
  modules: PathwayModule[]
  /** Map of moduleId to progress status */
  moduleStatus: Map<string, ModuleProgressStatus>
  /** Map of moduleId to progress percentage */
  moduleProgress?: Map<string, number>
  /** Map of moduleId to unlock message */
  unlockMessages?: Map<string, string>
  /** Whether the pathway is complete */
  isPathwayComplete?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  isPathwayComplete: false,
})

const emit = defineEmits<{
  (e: 'moduleClick', module: PathwayModule): void
  (e: 'labClick', labId: string, module: PathwayModule): void
}>()

// Track expanded module
const expandedModuleId = ref<string | null>(null)

// Sort modules by display order
const sortedModules = computed(() => {
  return [...props.modules].sort((a, b) => a.displayOrder - b.displayOrder)
})

function getModuleStatus(moduleId: string): ModuleProgressStatus {
  return props.moduleStatus.get(moduleId) ?? 'locked'
}

function getModuleProgress(moduleId: string): number {
  return props.moduleProgress?.get(moduleId) ?? 0
}

function getUnlockMessage(moduleId: string): string {
  return props.unlockMessages?.get(moduleId) ?? ''
}

function isModuleClickable(moduleId: string): boolean {
  const status = getModuleStatus(moduleId)
  return status !== 'locked'
}

function handleModuleClick(module: PathwayModule) {
  // Toggle expansion
  expandedModuleId.value = expandedModuleId.value === module.id ? null : module.id
  emit('moduleClick', module)
}

function handleLabClick(labId: string, module: PathwayModule) {
  emit('labClick', labId, module)
}

// Legend items — computed so labels refresh when the active locale changes.
const legendItems = computed(() => [
  { label: t('pathway.learningRoadmap.legendCompleted'), color: 'bg-green-500', icon: 'pi-check-circle' },
  { label: t('pathway.learningRoadmap.legendInProgress'), color: 'bg-blue-500', icon: 'pi-spinner' },
  { label: t('pathway.learningRoadmap.legendUnlocked'), color: 'bg-amber-500', icon: 'pi-lock-open' },
  { label: t('pathway.learningRoadmap.legendLocked'), color: 'bg-surface-400', icon: 'pi-lock' },
])
</script>

<template>
  <div class="pathway-learning-roadmap">
    <!-- Header with legend -->
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
      <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
        {{ t('pathway.learningRoadmap.heading') }}
      </h2>
      <div class="flex flex-wrap items-center gap-4">
        <div
          v-for="item in legendItems"
          :key="item.label"
          class="flex items-center gap-2"
        >
          <div
            :class="['w-3 h-3 rounded-full', item.color]"
            :aria-hidden="true"
          />
          <span class="text-sm text-surface-600 dark:text-surface-400">
            {{ item.label }}
          </span>
        </div>
      </div>
    </div>

    <!-- Modules list -->
    <div class="space-y-4" role="list" :aria-label="t('pathway.learningRoadmap.listAria')">
      <PathwayModuleCard
        v-for="(module, index) in sortedModules"
        :key="module.id"
        :module="module"
        :module-number="index + 1"
        :status="getModuleStatus(module.id)"
        :progress="getModuleProgress(module.id)"
        :unlock-message="getUnlockMessage(module.id)"
        :is-expanded="expandedModuleId === module.id"
        :is-clickable="isModuleClickable(module.id)"
        role="listitem"
        @click="handleModuleClick"
        @lab-click="handleLabClick"
      />
    </div>

    <!-- Pathway complete celebration -->
    <div
      v-if="isPathwayComplete"
      class="mt-8 p-6 rounded-xl bg-gradient-to-r from-green-500 to-emerald-600 text-white text-center"
      role="alert"
      aria-live="polite"
    >
      <i class="pi pi-trophy text-4xl mb-3" />
      <h3 class="text-xl font-bold mb-2">{{ t('pathway.learningRoadmap.completeHeading') }}</h3>
      <p class="text-green-100">
        {{ t('pathway.learningRoadmap.completeBody') }}
      </p>
    </div>
  </div>
</template>
