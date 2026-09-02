<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PodVM } from '@/api'
import { getStatusSeverity } from '@/utils/status'
import VMPowerControls from './VMPowerControls.vue'
import VMSnapshotManager from './VMSnapshotManager.vue'
import Tag from '@volt/Tag.vue'
import Button from '@volt/Button.vue'
import Menu from '@volt/Menu.vue'

const { t } = useI18n()

/**
 * Collapsible card that renders one VM in the pod-detail VM list:
 *   - header (name, platformId, status, IP, current snapshot)
 *   - expanded body: <VMPowerControls> + Console button + <VMSnapshotManager>
 *
 * Extracted from PodDetailView.vue to pull ~85 lines of template + ~30
 * lines of menu/console wiring out of the god view. The console ticket
 * fetch is NOT done here; we emit open-console with the VM descriptor so
 * the parent's useConsoleLauncher instance can own the modal state.
 */
const props = defineProps<{
  vm: PodVM
  podId: string
  expanded: boolean
}>()

const emit = defineEmits<{
  /** Fired when the user clicks the card header to toggle expansion. */
  (e: 'toggle'): void
  /** Bubbled up from VMPowerControls / VMSnapshotManager on successful action. */
  (e: 'changed'): void
  /** Bubbled up from children on failure. */
  (e: 'error', message: string): void
  /** User asked to open a VNC console. Parent should call useConsoleLauncher.openConsole(vm). */
  (e: 'open-console', vm: { name: string; platformId: string; node?: string }): void
  /** User asked to download the SPICE .vv launcher file. */
  (e: 'download-spice', vm: { name: string; platformId: string; node?: string }): void
}>()

// Per-row console menu. PrimeVue's Menu works on a ref; we keep it local.
const menuRef = ref<InstanceType<typeof Menu> | null>(null)

function toggleConsoleMenu(event: Event) {
  menuRef.value?.toggle(event)
}

// Wrap emit callbacks so the inline @click handlers in the template
// stay small and readable.
function onOpenConsole() {
  emit('open-console', {
    name: props.vm.name,
    platformId: props.vm.platformId,
  })
}
function onDownloadSpice() {
  emit('download-spice', {
    name: props.vm.name,
    platformId: props.vm.platformId,
  })
}

// Computed so menu labels update when the active locale changes.
const consoleMenuItems = computed(() => [
  { label: t('pod.vmListItem.consoleMenu.novnc'), icon: 'pi pi-desktop', command: onOpenConsole },
  { label: t('pod.vmListItem.consoleMenu.downloadSpice'), icon: 'pi pi-download', command: onDownloadSpice },
])
</script>

<template>
  <div class="border border-surface-200 dark:border-surface-700 rounded-lg overflow-hidden">
    <!-- VM Header (click to expand/collapse) -->
    <div
      class="p-4 cursor-pointer hover:bg-surface-50 dark:hover:bg-surface-800 flex items-center justify-between transition-colors"
      role="button"
      tabindex="0"
      :aria-expanded="expanded"
      @click="emit('toggle')"
      @keydown.enter="emit('toggle')"
      @keydown.space.prevent="emit('toggle')"
    >
      <div class="flex items-center gap-4">
        <div>
          <h3 class="font-medium text-surface-900 dark:text-surface-100">{{ vm.name }}</h3>
          <p class="text-sm text-surface-500">{{ vm.platformId }}</p>
        </div>
        <Tag :value="vm.status" :severity="getStatusSeverity(vm.status)" />
      </div>
      <div class="flex items-center gap-4">
        <div class="text-sm text-surface-600 dark:text-surface-400 text-right">
          <div v-if="vm.ipAddress">{{ t('pod.vmListItem.ipLabel', { address: vm.ipAddress }) }}</div>
          <div v-if="vm.currentSnapshot">{{ t('pod.vmListItem.snapshotLabel', { name: vm.currentSnapshot }) }}</div>
        </div>
        <i
          class="pi pi-chevron-down text-surface-400 transition-transform"
          :class="{ 'rotate-180': expanded }"
        />
      </div>
    </div>

    <!-- Expanded body -->
    <div
      v-if="expanded"
      class="border-t border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800 p-4"
    >
      <div class="mb-4">
        <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">{{ t('pod.vmListItem.powerControlsHeading') }}</h4>
        <VMPowerControls
          :vm="vm"
          :pod-id="podId"
          @changed="emit('changed')"
          @error="(msg) => emit('error', msg)"
        />
      </div>

      <div class="flex flex-wrap gap-2 mb-4">
        <h4 class="w-full text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">{{ t('pod.vmListItem.consoleHeading') }}</h4>
        <Button
          icon="pi pi-desktop"
          :label="t('pod.vmListItem.consoleButton')"
          severity="info"
          size="small"
          :disabled="vm.status !== 'running' && vm.status !== 'created'"
          @click.stop="toggleConsoleMenu"
        />
        <Menu ref="menuRef" :model="consoleMenuItems" :popup="true" />
      </div>

      <VMSnapshotManager
        :vm="vm"
        :pod-id="podId"
        @changed="emit('changed')"
        @error="(msg) => emit('error', msg)"
      />
    </div>
  </div>
</template>
