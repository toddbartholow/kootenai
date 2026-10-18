<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { podsApi, snapshotsApi, type Pod, type PodVM, type Snapshot } from '@/api'
import { useNotifications } from '@/composables'
import { getStatusSeverity } from '@/utils/status'
import { useConfirm } from 'primevue/useconfirm'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Dialog from '@volt/Dialog.vue'
import RadioButton from '@volt/RadioButton.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'

const { t } = useI18n()
const confirm = useConfirm()
const notify = useNotifications()

const props = defineProps<{
  podId: string
}>()

const emit = defineEmits<{
  openConsole: [vm: PodVM]
  podStopped: []
  podStarted: []
  vmReset: [vmName: string, snapshot: string]
}>()

const pod = ref<Pod | null>(null)
const loading = ref(true)
const actionLoading = ref(false)
const vmActionLoading = ref<string | null>(null) // Track which VM has action in progress
const error = ref<string | null>(null)

// Snapshot modal state
const showSnapshotModal = ref(false)
const selectedVM = ref<string>('')
const selectedSnapshot = ref<string>('')
const vmSnapshots = ref<Snapshot[]>([])
const loadingSnapshots = ref(false)

onMounted(async () => {
  await loadPod()
})

async function loadPod() {
  try {
    loading.value = true
    error.value = null
    pod.value = await podsApi.get(props.podId)
  } catch (err) {
    console.error('Failed to load pod:', err)
    error.value = t('podControl.errorLoadFailed')
  } finally {
    loading.value = false
  }
}

async function startPod() {
  if (!pod.value) return
  try {
    actionLoading.value = true
    error.value = null
    notify.info({
      title: t('podControl.notifications.startingPodTitle'),
      message: t('podControl.notifications.startingPodMessage'),
    })
    const updated = await podsApi.start(pod.value.id)
    pod.value = updated
    notify.success({
      title: t('podControl.notifications.startedPodTitle'),
      message: t('podControl.notifications.startedPodMessage'),
    })
    emit('podStarted')
  } catch (err) {
    console.error('Failed to start pod:', err)
    error.value = t('podControl.errors.startPodFailed')
    notify.error(t('podControl.errors.startPodFailed'))
  } finally {
    actionLoading.value = false
  }
}

function stopPod() {
  if (!pod.value) return
  confirm.require({
    message: t('podControl.confirm.stopPod.message'),
    header: t('podControl.confirm.stopPod.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        actionLoading.value = true
        error.value = null
        notify.info({
          title: t('podControl.notifications.stoppingPodTitle'),
          message: t('podControl.notifications.stoppingPodMessage'),
        })
        const updated = await podsApi.stop(pod.value!.id)
        pod.value = updated
        notify.info({
          title: t('podControl.notifications.stoppedPodTitle'),
          message: t('podControl.notifications.stoppedPodMessage'),
        })
        emit('podStopped')
      } catch (err) {
        console.error('Failed to stop pod:', err)
        error.value = t('podControl.errors.stopPodFailed')
        notify.error(t('podControl.errors.stopPodFailed'))
      } finally {
        actionLoading.value = false
      }
    },
  })
}

function resetAllVMs() {
  if (!pod.value || !pod.value.vms?.length) return
  confirm.require({
    message: t('podControl.confirm.resetAll.message'),
    header: t('podControl.confirm.resetAll.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        actionLoading.value = true
        error.value = null
        notify.info({
          title: t('podControl.notifications.resettingAllTitle'),
          message: t('podControl.notifications.resettingAllMessage'),
        })
        for (const vm of pod.value!.vms || []) {
          await podsApi.resetVM(pod.value!.id, vm.name, 'initial')
        }
        await loadPod()
        notify.success({
          title: t('podControl.notifications.resetAllTitle'),
          message: t('podControl.notifications.resetAllMessage'),
        })
      } catch (err) {
        console.error('Failed to reset VMs:', err)
        error.value = t('podControl.errors.resetAllFailed')
        notify.error(t('podControl.errors.resetAllFailed'))
      } finally {
        actionLoading.value = false
      }
    },
  })
}

// Individual VM controls
async function startVM(vmName: string) {
  if (!pod.value) return
  try {
    vmActionLoading.value = vmName
    error.value = null
    notify.vm.starting(vmName)
    await podsApi.startVM(pod.value.id, vmName)
    await loadPod()
    notify.vm.started(vmName)
  } catch (err) {
    console.error('Failed to start VM:', err)
    error.value = t('podControl.errors.startVmFailed', { name: vmName })
    notify.error(t('podControl.errors.startVmShort', { name: vmName }))
  } finally {
    vmActionLoading.value = null
  }
}

function stopVM(vmName: string) {
  if (!pod.value) return
  confirm.require({
    message: t('podControl.confirm.stopVm.message', { name: vmName }),
    header: t('podControl.confirm.stopVm.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        vmActionLoading.value = vmName
        error.value = null
        notify.vm.stopping(vmName)
        await podsApi.stopVM(pod.value!.id, vmName)
        await loadPod()
        notify.vm.stopped(vmName)
      } catch (err) {
        console.error('Failed to stop VM:', err)
        error.value = t('podControl.errors.stopVmFailed', { name: vmName })
        notify.error(t('podControl.errors.stopVmShort', { name: vmName }))
      } finally {
        vmActionLoading.value = null
      }
    },
  })
}

async function suspendVM(vmName: string) {
  if (!pod.value) return
  try {
    vmActionLoading.value = vmName
    error.value = null
    notify.info({
      title: t('podControl.notifications.suspendingTitle'),
      message: t('podControl.notifications.suspendingMessage', { name: vmName }),
    })
    await podsApi.suspendVM(pod.value.id, vmName)
    await loadPod()
    notify.success({
      title: t('podControl.notifications.suspendedTitle'),
      message: t('podControl.notifications.suspendedMessage', { name: vmName }),
    })
  } catch (err) {
    console.error('Failed to suspend VM:', err)
    error.value = t('podControl.errors.suspendVmFailed', { name: vmName })
    notify.error(t('podControl.errors.suspendVmShort', { name: vmName }))
  } finally {
    vmActionLoading.value = null
  }
}

async function resumeVM(vmName: string) {
  if (!pod.value) return
  try {
    vmActionLoading.value = vmName
    error.value = null
    notify.info({
      title: t('podControl.notifications.resumingTitle'),
      message: t('podControl.notifications.resumingMessage', { name: vmName }),
    })
    await podsApi.resumeVM(pod.value.id, vmName)
    await loadPod()
    notify.success({
      title: t('podControl.notifications.resumedTitle'),
      message: t('podControl.notifications.resumedMessage', { name: vmName }),
    })
  } catch (err) {
    console.error('Failed to resume VM:', err)
    error.value = t('podControl.errors.resumeVmFailed', { name: vmName })
    notify.error(t('podControl.errors.resumeVmShort', { name: vmName }))
  } finally {
    vmActionLoading.value = null
  }
}

async function openSnapshotModal(vmName: string) {
  selectedVM.value = vmName
  selectedSnapshot.value = ''
  showSnapshotModal.value = true
  loadingSnapshots.value = true

  try {
    vmSnapshots.value = await snapshotsApi.list(props.podId, vmName)
  } catch (err) {
    console.error('Failed to load snapshots:', err)
    vmSnapshots.value = []
  } finally {
    loadingSnapshots.value = false
  }
}

async function revertToSnapshot() {
  if (!pod.value || !selectedVM.value || !selectedSnapshot.value) return

  try {
    actionLoading.value = true
    notify.info({
      title: t('podControl.notifications.revertingTitle'),
      message: t('podControl.notifications.revertingMessage', {
        vm: selectedVM.value,
        snapshot: selectedSnapshot.value,
      }),
    })
    await podsApi.resetVM(pod.value.id, selectedVM.value, selectedSnapshot.value)
    showSnapshotModal.value = false
    emit('vmReset', selectedVM.value, selectedSnapshot.value)
    await loadPod()
    notify.vm.snapshotReverted(selectedVM.value, selectedSnapshot.value)
  } catch (err) {
    console.error('Failed to revert VM:', err)
    error.value = t('podControl.errors.revertFailed')
    notify.error(t('podControl.errors.revertShort', { name: selectedVM.value }))
  } finally {
    actionLoading.value = false
  }
}

// getStatusSeverity imported from @/utils/status

function getVMStatusIcon(status: string): string {
  switch (status) {
    case 'running': return 'pi pi-play-circle'
    case 'stopped': return 'pi pi-stop-circle'
    case 'paused':
    case 'suspended': return 'pi pi-pause-circle'
    case 'provisioning': return 'pi pi-spin pi-spinner'
    default: return 'pi pi-circle'
  }
}

function isVMRunning(status: string): boolean {
  return status === 'running'
}

function isVMStopped(status: string): boolean {
  return status === 'stopped' || status === 'created'
}

function isVMSuspended(status: string): boolean {
  return status === 'paused' || status === 'suspended'
}

const isPodRunning = computed(() => pod.value?.status === 'running')
const isPodStopped = computed(() => pod.value?.status === 'stopped')
</script>

<template>
  <Card>
    <template #title>
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <i class="pi pi-server text-primary-500" />
          <span>{{ t('podControl.cardTitle') }}</span>
        </div>
        <Tag v-if="pod" :value="pod.status" :severity="getStatusSeverity(pod.status)" />
      </div>
    </template>
    <template #content>
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-8">
        <ProgressSpinner style="width: 32px; height: 32px" />
      </div>

      <!-- Error State -->
      <div v-else-if="error && !pod" class="text-center py-4">
        <i class="pi pi-exclamation-triangle text-3xl text-orange-500 mb-2" />
        <p class="text-surface-500">{{ error }}</p>
        <Button @click="loadPod" :label="t('podControl.retry')" icon="pi pi-refresh" severity="secondary" class="mt-2" size="small" />
      </div>

      <!-- Pod Controls -->
      <div v-else-if="pod" class="space-y-4">
        <!-- Pod Actions -->
        <div class="flex gap-2">
          <Button
            v-if="isPodStopped"
            @click="startPod"
            :disabled="actionLoading"
            :loading="actionLoading"
            icon="pi pi-play"
            :label="t('podControl.actions.startAll')"
            severity="success"
            size="small"
            class="flex-1"
          />
          <Button
            v-if="isPodRunning"
            @click="stopPod"
            :disabled="actionLoading"
            :loading="actionLoading"
            icon="pi pi-stop"
            :label="t('podControl.actions.stopAll')"
            severity="warn"
            size="small"
            class="flex-1"
          />
          <Button
            @click="resetAllVMs"
            :disabled="actionLoading || !isPodRunning"
            icon="pi pi-refresh"
            :label="t('podControl.actions.resetAll')"
            severity="secondary"
            size="small"
            class="flex-1"
          />
        </div>

        <!-- VM List -->
        <div class="space-y-2">
          <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
            <i class="pi pi-desktop" />
            {{ t('podControl.vmList.heading', { count: pod.vms?.length || 0 }) }}
          </h4>

          <div v-if="!pod.vms?.length" class="text-sm text-surface-500 text-center py-2">
            {{ t('podControl.vmList.empty') }}
          </div>

          <div v-else class="space-y-2">
            <div
              v-for="vm in pod.vms"
              :key="vm.name"
              class="p-3 bg-surface-50 dark:bg-surface-800 rounded-lg"
            >
              <div class="flex items-center justify-between mb-2">
                <div class="flex items-center gap-2">
                  <i :class="[getVMStatusIcon(vm.status), isVMRunning(vm.status) ? 'text-green-500' : isVMSuspended(vm.status) ? 'text-yellow-500' : 'text-surface-400']" />
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{ vm.name }}</span>
                </div>
                <Tag :value="vm.status" :severity="getStatusSeverity(vm.status)" class="text-xs" />
              </div>

              <div class="text-xs text-surface-500 mb-2 space-y-1">
                <div v-if="vm.ipAddress">
                  <i class="pi pi-globe mr-1" />{{ t('podControl.vmList.ipLine', { address: vm.ipAddress }) }}
                </div>
                <div v-if="vm.currentSnapshot">
                  <i class="pi pi-camera mr-1" />{{ t('podControl.vmList.snapshotLine', { name: vm.currentSnapshot }) }}
                </div>
              </div>

              <!-- VM Power Controls -->
              <div class="flex gap-1 mb-2">
                <Button
                  v-if="isVMStopped(vm.status)"
                  @click="startVM(vm.name)"
                  :disabled="vmActionLoading === vm.name"
                  :loading="vmActionLoading === vm.name"
                  icon="pi pi-play"
                  severity="success"
                  size="small"
                  v-tooltip.top="t('podControl.vmActions.startTooltip')"
                  class="flex-1"
                />
                <Button
                  v-if="isVMRunning(vm.status)"
                  @click="stopVM(vm.name)"
                  :disabled="vmActionLoading === vm.name"
                  :loading="vmActionLoading === vm.name"
                  icon="pi pi-stop"
                  severity="danger"
                  size="small"
                  v-tooltip.top="t('podControl.vmActions.stopTooltip')"
                  class="flex-1"
                />
                <Button
                  v-if="isVMRunning(vm.status)"
                  @click="suspendVM(vm.name)"
                  :disabled="vmActionLoading === vm.name"
                  :loading="vmActionLoading === vm.name"
                  icon="pi pi-pause"
                  severity="warn"
                  size="small"
                  v-tooltip.top="t('podControl.vmActions.suspendTooltip')"
                  class="flex-1"
                />
                <Button
                  v-if="isVMSuspended(vm.status)"
                  @click="resumeVM(vm.name)"
                  :disabled="vmActionLoading === vm.name"
                  :loading="vmActionLoading === vm.name"
                  icon="pi pi-play"
                  severity="success"
                  size="small"
                  v-tooltip.top="t('podControl.vmActions.resumeTooltip')"
                  class="flex-1"
                />
              </div>

              <!-- VM Other Actions -->
              <div class="flex gap-2">
                <Button
                  @click="emit('openConsole', vm)"
                  :disabled="!isVMRunning(vm.status)"
                  icon="pi pi-desktop"
                  :label="t('podControl.vmActions.consoleButton')"
                  severity="info"
                  size="small"
                  class="flex-1"
                />
                <Button
                  @click="openSnapshotModal(vm.name)"
                  :disabled="actionLoading || vmActionLoading === vm.name"
                  icon="pi pi-history"
                  :label="t('podControl.vmActions.revertButton')"
                  severity="secondary"
                  size="small"
                  class="flex-1"
                />
              </div>
            </div>
          </div>
        </div>

        <!-- Error Message -->
        <div v-if="error" class="p-2 bg-red-50 dark:bg-red-900/20 rounded text-sm text-red-600 dark:text-red-400">
          <i class="pi pi-exclamation-circle mr-1" />{{ error }}
        </div>
      </div>
    </template>
  </Card>

  <!-- Snapshot Revert Modal -->
  <Dialog
    v-model:visible="showSnapshotModal"
    modal
    :header="t('podControl.revertDialog.header')"
    :style="{ width: '400px' }"
  >
    <p class="text-sm text-surface-600 dark:text-surface-400 mb-4">
      {{ t('podControl.revertDialog.intro', { name: selectedVM }) }}
    </p>

    <div v-if="loadingSnapshots" class="flex justify-center py-4">
      <ProgressSpinner style="width: 24px; height: 24px" />
    </div>

    <div v-else-if="!vmSnapshots.length" class="text-sm text-surface-500 py-4 text-center">
      {{ t('podControl.revertDialog.empty') }}
    </div>

    <div v-else class="space-y-2 max-h-48 overflow-y-auto">
      <label
        v-for="snap in vmSnapshots"
        :key="snap.name"
        class="flex items-center gap-3 p-3 border border-surface-200 dark:border-surface-700 rounded-lg cursor-pointer hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
        :class="{ 'border-primary-500 bg-primary-50 dark:bg-primary-900/20': selectedSnapshot === snap.name }"
      >
        <RadioButton v-model="selectedSnapshot" :value="snap.name" :inputId="snap.name" />
        <div>
          <div class="font-medium text-surface-900 dark:text-surface-100">{{ snap.name }}</div>
          <div v-if="snap.description" class="text-sm text-surface-500">{{ snap.description }}</div>
        </div>
      </label>
    </div>

    <template #footer>
      <Button @click="showSnapshotModal = false" :label="t('podControl.revertDialog.cancel')" severity="secondary" />
      <Button
        @click="revertToSnapshot"
        :disabled="!selectedSnapshot || actionLoading"
        :loading="actionLoading"
        :label="t('podControl.revertDialog.submit')"
        icon="pi pi-history"
      />
    </template>
  </Dialog>
</template>
