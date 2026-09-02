<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PodVM } from '@/api'
import { getVMStatusColor } from '@/utils/status'

const { t } = useI18n()

/**
 * Tab-row selector for choosing a VM to attach the inline console to.
 * Extracted from SessionView.vue (~34 lines). Running VMs are clickable;
 * non-running ones are visibly disabled.
 *
 * Props:
 *   vms: the full VM list for the pod
 *   selectedName: the currently selected VM's name (used for aria-pressed)
 *   active: whether the inline console is actually attached (affects
 *     the "currently selected" badge semantics)
 * Emits:
 *   select: the user picked a (running) VM
 */
defineProps<{
  vms: PodVM[]
  selectedName: string | null
  active: boolean
}>()

const emit = defineEmits<{
  (e: 'select', vm: PodVM): void
}>()

function isRunning(vm: PodVM): boolean {
  return vm.status === 'running'
}

function onClick(vm: PodVM): void {
  if (isRunning(vm)) emit('select', vm)
}
</script>

<template>
  <div v-if="vms.length" class="mb-4">
    <div class="flex items-center gap-3 mb-3">
      <span class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
        <i class="pi pi-desktop" />
        {{ t('pod.vmSelector.heading') }}
      </span>
      <span class="px-2 py-0.5 text-xs font-medium rounded bg-primary-100 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300">
        {{ t('pod.vmSelector.countBadge', { count: vms.length }) }}
      </span>
    </div>
    <div class="flex flex-wrap gap-2" role="group" :aria-label="t('pod.vmSelector.groupAria')">
      <button
        v-for="vm in vms"
        :key="vm.name"
        :disabled="!isRunning(vm)"
        :aria-label="selectedName === vm.name && active
          ? t('pod.vmSelector.itemAriaSelected', { name: vm.name, status: vm.status })
          : t('pod.vmSelector.itemAria', { name: vm.name, status: vm.status })"
        :aria-pressed="selectedName === vm.name && active"
        :class="[
          'px-3 py-2 rounded-lg border text-sm font-medium transition-all flex items-center gap-2',
          selectedName === vm.name && active
            ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 text-primary-700 dark:text-primary-300'
            : isRunning(vm)
              ? 'border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800 hover:border-primary-300 hover:bg-surface-100 dark:hover:bg-surface-700'
              : 'border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800 opacity-50 cursor-not-allowed'
        ]"
        @click="onClick(vm)"
      >
        <span :class="['w-2 h-2 rounded-full', getVMStatusColor(vm.status)]" aria-hidden="true" />
        {{ vm.name }}
        <span class="text-xs text-surface-400 dark:text-surface-500">{{ t('pod.vmSelector.statusParen', { status: vm.status }) }}</span>
      </button>
    </div>
  </div>
</template>
