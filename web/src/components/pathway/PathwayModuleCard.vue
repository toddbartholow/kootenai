<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Tag from '@volt/Tag.vue'
import type { PathwayModule, ModuleProgressStatus, UnlockType } from '@/api'
import { formatDuration } from '@/utils/format'

const { t } = useI18n()

interface Props {
  module: PathwayModule
  moduleNumber: number
  status: ModuleProgressStatus
  progress?: number
  isExpanded?: boolean
  isClickable?: boolean
  unlockMessage?: string
}

const props = withDefaults(defineProps<Props>(), {
  progress: 0,
  isExpanded: false,
  isClickable: true,
  unlockMessage: '',
})

// Generate lock reason message based on unlock type
const lockReasonMessage = computed(() => {
  if (props.status !== 'locked') return ''
  if (props.unlockMessage) return props.unlockMessage

  // Default messages based on unlock type
  const unlockType = props.module.unlockType as UnlockType
  switch (unlockType) {
    case 'sequential':
      return t('pathway.moduleCard.lockSequential')
    case 'all_previous':
      return t('pathway.moduleCard.lockAllPrevious')
    case 'manual':
      return t('pathway.moduleCard.lockManual')
    case 'always':
      return '' // Should never be locked
    default:
      return t('pathway.moduleCard.lockDefault')
  }
})

const emit = defineEmits<{
  (e: 'click', module: PathwayModule): void
  (e: 'labClick', labId: string, module: PathwayModule): void
}>()

// Status badge configuration
const statusBadge = computed(() => {
  switch (props.status) {
    case 'completed':
      return { label: t('pathway.moduleCard.badgeCompleted'), severity: 'success' as const }
    case 'in_progress':
      return { label: t('pathway.moduleCard.badgeInProgress'), severity: 'info' as const }
    case 'unlocked':
      return { label: t('pathway.moduleCard.badgeStart'), severity: 'warn' as const }
    case 'locked':
    default:
      return { label: t('pathway.moduleCard.badgeLocked'), severity: 'secondary' as const }
  }
})

// Node indicator color based on status
const nodeColor = computed(() => {
  switch (props.status) {
    case 'completed':
      return 'bg-green-500 text-white'
    case 'in_progress':
      return 'bg-blue-500 text-white'
    case 'unlocked':
      return 'bg-amber-500 text-white'
    case 'locked':
    default:
      return 'bg-surface-300 dark:bg-surface-600 text-surface-500 dark:text-surface-400'
  }
})

// Card styling based on status
const cardClasses = computed(() => {
  const base = 'rounded-lg border p-4 transition-all duration-200'

  if (props.status === 'in_progress') {
    return `${base} bg-white dark:bg-surface-800 border-blue-400 dark:border-blue-500 ring-2 ring-blue-100 dark:ring-blue-900`
  }

  if (props.status === 'locked') {
    return `${base} bg-surface-50 dark:bg-surface-800/50 border-surface-200 dark:border-surface-700 opacity-75`
  }

  return `${base} bg-white dark:bg-surface-800 border-surface-200 dark:border-surface-700 hover:shadow-md`
})

// Calculate stats display
const labCount = computed(() => props.module.labCount ?? props.module.labs?.length ?? 0)
const totalPoints = computed(() => props.module.totalPoints ?? 0)
const estimatedMinutes = computed(() => props.module.estimatedMinutes ?? 0)

// formatDuration imported from @/utils/format

function handleClick() {
  if (props.isClickable) {
    emit('click', props.module)
  }
}

function handleLabClick(labId: string, event: Event) {
  event.stopPropagation()
  emit('labClick', labId, props.module)
}
</script>

<template>
  <div class="pathway-module-card flex gap-4">
    <!-- Timeline node -->
    <div class="flex flex-col items-center">
      <!-- Node circle with number -->
      <div
        :class="[
          'w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold',
          nodeColor
        ]"
        :aria-label="t('pathway.moduleCard.moduleAriaNumber', { number: moduleNumber })"
      >
        <i v-if="status === 'completed'" class="pi pi-check text-sm" />
        <span v-else>{{ moduleNumber }}</span>
      </div>
      <!-- Timeline line -->
      <div class="w-0.5 flex-1 bg-surface-200 dark:bg-surface-700 mt-2" />
    </div>

    <!-- Card content -->
    <button
      type="button"
      :class="[cardClasses, 'flex-1 text-left', isClickable ? 'cursor-pointer' : 'cursor-default']"
      :disabled="!isClickable"
      :aria-expanded="isExpanded"
      :aria-label="t('pathway.moduleCard.cardAria', {
        name: module.name,
        status: statusBadge.label,
        labs: labCount === 1 ? t('pathway.moduleCard.labSingular', { count: labCount }) : t('pathway.moduleCard.labPlural', { count: labCount }),
        duration: formatDuration(estimatedMinutes),
        points: totalPoints,
      })"
      @click="handleClick"
    >
      <!-- Header row -->
      <div class="flex items-start justify-between gap-3 mb-2">
        <h3
          class="font-semibold text-surface-900 dark:text-surface-100"
          :class="status === 'locked' ? 'text-surface-500 dark:text-surface-400' : ''"
        >
          {{ t('pathway.moduleCard.moduleHeader', { number: moduleNumber, name: module.name }) }}
        </h3>
        <Tag
          :value="statusBadge.label"
          :severity="statusBadge.severity"
          class="flex-shrink-0 text-xs"
        />
      </div>

      <!-- Description -->
      <p
        v-if="module.description"
        class="text-sm text-surface-600 dark:text-surface-400 mb-3 line-clamp-2"
        :class="status === 'locked' ? 'text-surface-400 dark:text-surface-500' : ''"
      >
        {{ module.description }}
      </p>

      <!-- Stats row -->
      <div class="flex items-center gap-4 text-sm text-surface-500">
        <span class="flex items-center gap-1">
          <i class="pi pi-book text-xs" />
          {{ labCount === 1 ? t('pathway.moduleCard.labSingular', { count: labCount }) : t('pathway.moduleCard.labPlural', { count: labCount }) }}
        </span>
        <span v-if="estimatedMinutes > 0" class="flex items-center gap-1">
          <i class="pi pi-clock text-xs" />
          {{ formatDuration(estimatedMinutes) }}
        </span>
        <span class="flex items-center gap-1">
          <i class="pi pi-star text-xs" />
          {{ t('pathway.moduleCard.pointsSuffix', { count: totalPoints }) }}
        </span>
      </div>

      <!-- Lock reason (shown when module is locked) -->
      <div
        v-if="status === 'locked' && lockReasonMessage"
        class="mt-3 flex items-center gap-2 text-sm text-surface-400 dark:text-surface-500"
      >
        <i class="pi pi-lock text-xs" />
        <span>{{ lockReasonMessage }}</span>
      </div>

      <!-- Labs list (when expanded) -->
      <div
        v-if="module.labs?.length && isExpanded"
        class="mt-4 pt-4 border-t border-surface-200 dark:border-surface-700 space-y-2"
      >
        <button
          v-for="lab in module.labs"
          :key="lab.id"
          type="button"
          class="flex items-center gap-2 p-2 w-full rounded-lg bg-surface-50 dark:bg-surface-700/50 hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors text-left"
          @click="handleLabClick(lab.labTemplateId, $event)"
        >
          <i class="pi pi-desktop text-surface-400" />
          <span class="flex-1 text-sm text-surface-700 dark:text-surface-300 truncate">
            {{ lab.labName || t('pathway.moduleCard.labFallbackName') }}
          </span>
          <span v-if="lab.labDurationMinutes" class="text-xs text-surface-400">
            {{ t('pathway.moduleCard.labDurationMin', { count: lab.labDurationMinutes }) }}
          </span>
          <i class="pi pi-chevron-right text-xs text-surface-400" />
        </button>
      </div>
    </button>
  </div>
</template>
