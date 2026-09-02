<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { snapshotsApi, podsApi, type Snapshot, type Pod } from '@/api'
import { useNotifications } from '@/composables'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Dialog from '@volt/Dialog.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'

const { t } = useI18n()

const props = defineProps<{
  podId: string
}>()

const emit = defineEmits<{
  snapshotRestored: [vmName: string, snapshotName: string]
  snapshotCreated: [vmName: string, snapshotName: string]
  snapshotDeleted: [vmName: string, snapshotName: string]
}>()

const notify = useNotifications()

// Pod and VM state
const pod = ref<Pod | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

// Snapshots per VM
const vmSnapshots = ref<Map<string, Snapshot[]>>(new Map())
const loadingSnapshots = ref<Set<string>>(new Set())

// Create snapshot dialog
const showCreateDialog = ref(false)
const createVmName = ref('')
const newSnapshotName = ref('')
const newSnapshotDescription = ref('')
const creating = ref(false)

// Restore confirmation
const showRestoreDialog = ref(false)
const restoreVmName = ref('')
const restoreSnapshotName = ref('')
const restoring = ref(false)

// Delete confirmation
const showDeleteDialog = ref(false)
const deleteVmName = ref('')
const deleteSnapshotName = ref('')
const deleting = ref(false)

// Expanded VMs
const expandedVMs = ref<Set<string>>(new Set())

async function loadPod() {
  try {
    loading.value = true
    error.value = null
    pod.value = await podsApi.get(props.podId)

    // Load snapshots for all VMs
    if (pod.value?.vms) {
      await Promise.all(pod.value.vms.map(vm => loadVMSnapshots(vm.name)))
    }
  } catch (err) {
    console.error('Failed to load pod:', err)
    error.value = t('snapshotPanel.errorLoadFailed')
  } finally {
    loading.value = false
  }
}

async function loadVMSnapshots(vmName: string) {
  try {
    loadingSnapshots.value.add(vmName)
    const snapshots = await snapshotsApi.list(props.podId, vmName)
    vmSnapshots.value.set(vmName, snapshots)
  } catch (err) {
    console.error(`Failed to load snapshots for ${vmName}:`, err)
    vmSnapshots.value.set(vmName, [])
  } finally {
    loadingSnapshots.value.delete(vmName)
  }
}

function toggleVM(vmName: string) {
  if (expandedVMs.value.has(vmName)) {
    expandedVMs.value.delete(vmName)
  } else {
    expandedVMs.value.add(vmName)
  }
}

function getVMSnapshots(vmName: string): Snapshot[] {
  return vmSnapshots.value.get(vmName) || []
}

// Create snapshot
function openCreateDialog(vmName: string) {
  createVmName.value = vmName
  newSnapshotName.value = ''
  newSnapshotDescription.value = ''
  showCreateDialog.value = true
}

async function createSnapshot() {
  if (!newSnapshotName.value.trim()) {
    notify.error(t('snapshotPanel.notifications.nameRequired'))
    return
  }

  try {
    creating.value = true
    notify.info({
      title: t('snapshotPanel.notifications.creatingTitle'),
      message: t('snapshotPanel.notifications.creatingMessage', { name: newSnapshotName.value }),
    })

    await snapshotsApi.create(
      props.podId,
      createVmName.value,
      newSnapshotName.value.trim(),
      newSnapshotDescription.value.trim() || undefined
    )

    notify.success({
      title: t('snapshotPanel.notifications.createdTitle'),
      message: t('snapshotPanel.notifications.createdMessage', { name: newSnapshotName.value }),
    })
    emit('snapshotCreated', createVmName.value, newSnapshotName.value)
    showCreateDialog.value = false

    // Reload snapshots
    await loadVMSnapshots(createVmName.value)
  } catch (err) {
    console.error('Failed to create snapshot:', err)
    notify.error(t('snapshotPanel.notifications.createFailed'))
  } finally {
    creating.value = false
  }
}

// Restore snapshot
function openRestoreDialog(vmName: string, snapshotName: string) {
  restoreVmName.value = vmName
  restoreSnapshotName.value = snapshotName
  showRestoreDialog.value = true
}

async function restoreSnapshot() {
  try {
    restoring.value = true
    notify.info({
      title: t('snapshotPanel.notifications.restoringTitle'),
      message: t('snapshotPanel.notifications.restoringMessage', {
        vm: restoreVmName.value,
        snapshot: restoreSnapshotName.value,
      }),
    })

    await podsApi.resetVM(props.podId, restoreVmName.value, restoreSnapshotName.value)

    notify.vm.snapshotReverted(restoreVmName.value, restoreSnapshotName.value)
    emit('snapshotRestored', restoreVmName.value, restoreSnapshotName.value)
    showRestoreDialog.value = false

    // Reload pod to get updated VM states
    await loadPod()
  } catch (err) {
    console.error('Failed to restore snapshot:', err)
    notify.error(t('snapshotPanel.notifications.restoreFailed'))
  } finally {
    restoring.value = false
  }
}

// Delete snapshot
function openDeleteDialog(vmName: string, snapshotName: string) {
  deleteVmName.value = vmName
  deleteSnapshotName.value = snapshotName
  showDeleteDialog.value = true
}

async function deleteSnapshot() {
  try {
    deleting.value = true
    notify.info({
      title: t('snapshotPanel.notifications.deletingTitle'),
      message: t('snapshotPanel.notifications.deletingMessage', { name: deleteSnapshotName.value }),
    })

    await snapshotsApi.delete(props.podId, deleteVmName.value, deleteSnapshotName.value)

    notify.success({
      title: t('snapshotPanel.notifications.deletedTitle'),
      message: t('snapshotPanel.notifications.deletedMessage', { name: deleteSnapshotName.value }),
    })
    emit('snapshotDeleted', deleteVmName.value, deleteSnapshotName.value)
    showDeleteDialog.value = false

    // Reload snapshots
    await loadVMSnapshots(deleteVmName.value)
  } catch (err) {
    console.error('Failed to delete snapshot:', err)
    notify.error(t('snapshotPanel.notifications.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

// Restore all VMs to initial state
async function restoreAllToInitial() {
  if (!pod.value?.vms?.length) return

  try {
    restoring.value = true
    notify.info({
      title: t('snapshotPanel.notifications.restoringAllTitle'),
      message: t('snapshotPanel.notifications.restoringAllMessage'),
    })

    await Promise.all(
      pod.value.vms.map(vm => podsApi.resetVM(props.podId, vm.name, 'initial'))
    )

    notify.success({
      title: t('snapshotPanel.notifications.restoredAllTitle'),
      message: t('snapshotPanel.notifications.restoredAllMessage'),
    })
    await loadPod()
  } catch (err) {
    console.error('Failed to restore all VMs:', err)
    notify.error(t('snapshotPanel.notifications.restoreAllFailed'))
  } finally {
    restoring.value = false
  }
}

function getSnapshotIcon(snapshot: Snapshot): string {
  if (snapshot.name === 'initial' || snapshot.name === 'clean') {
    return 'pi pi-flag'
  }
  return 'pi pi-camera'
}

function isCurrentSnapshot(vmName: string, snapshotName: string): boolean {
  const vm = pod.value?.vms?.find(v => v.name === vmName)
  return vm?.currentSnapshot === snapshotName
}

onMounted(() => {
  loadPod()
})

watch(() => props.podId, () => {
  loadPod()
})
</script>

<template>
  <Card>
    <template #title>
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <i class="pi pi-history text-primary-500" />
          <span>{{ t('snapshotPanel.cardTitle') }}</span>
        </div>
        <Button
          v-if="pod?.vms?.length"
          @click="restoreAllToInitial"
          :loading="restoring"
          :disabled="restoring"
          icon="pi pi-refresh"
          :label="t('snapshotPanel.resetAllAction')"
          severity="secondary"
          size="small"
        />
      </div>
    </template>
    <template #content>
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-8">
        <ProgressSpinner style="width: 32px; height: 32px" />
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="text-center py-4">
        <i class="pi pi-exclamation-triangle text-3xl text-orange-500 mb-2" />
        <p class="text-surface-500">{{ error }}</p>
        <Button @click="loadPod" :label="t('snapshotPanel.retry')" icon="pi pi-refresh" severity="secondary" class="mt-2" size="small" />
      </div>

      <!-- No VMs -->
      <div v-else-if="!pod?.vms?.length" class="text-center text-surface-500 py-4">
        <i class="pi pi-info-circle text-2xl mb-2" />
        <p>{{ t('snapshotPanel.emptyNoVms') }}</p>
      </div>

      <!-- VM Snapshot List -->
      <div v-else class="space-y-3">
        <div
          v-for="vm in pod.vms"
          :key="vm.name"
          class="border border-surface-200 dark:border-surface-700 rounded-lg overflow-hidden"
        >
          <!-- VM Header -->
          <button
            @click="toggleVM(vm.name)"
            class="w-full flex items-center justify-between p-3 bg-surface-50 dark:bg-surface-800 hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
          >
            <div class="flex items-center gap-2">
              <i
                :class="[
                  'pi transition-transform',
                  expandedVMs.has(vm.name) ? 'pi-chevron-down' : 'pi-chevron-right'
                ]"
              />
              <i class="pi pi-desktop text-primary-500" />
              <span class="font-medium text-surface-900 dark:text-surface-100">{{ vm.name }}</span>
              <Tag
                v-if="vm.currentSnapshot"
                :value="vm.currentSnapshot"
                severity="info"
                class="text-xs"
              />
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-surface-500">
                {{ t('snapshotPanel.vmSnapshotCount', { count: getVMSnapshots(vm.name).length }) }}
              </span>
              <Button
                @click.stop="openCreateDialog(vm.name)"
                icon="pi pi-plus"
                severity="secondary"
                size="small"
                rounded
                text
                v-tooltip.top="t('snapshotPanel.vmCreateTooltip')"
              />
            </div>
          </button>

          <!-- Snapshot List (Expanded) -->
          <div v-if="expandedVMs.has(vm.name)" class="p-3 space-y-2 bg-white dark:bg-surface-900">
            <!-- Loading -->
            <div v-if="loadingSnapshots.has(vm.name)" class="text-center py-2">
              <ProgressSpinner style="width: 20px; height: 20px" />
            </div>

            <!-- No Snapshots -->
            <div v-else-if="getVMSnapshots(vm.name).length === 0" class="text-center text-surface-500 py-2 text-sm">
              {{ t('snapshotPanel.snapshotEmpty') }}
            </div>

            <!-- Snapshot Items -->
            <div
              v-else
              v-for="snapshot in getVMSnapshots(vm.name)"
              :key="snapshot.name"
              :class="[
                'flex items-center justify-between p-2 rounded-lg transition-colors',
                isCurrentSnapshot(vm.name, snapshot.name)
                  ? 'bg-primary-50 dark:bg-primary-900/20 border border-primary-200 dark:border-primary-700'
                  : 'bg-surface-50 dark:bg-surface-800 hover:bg-surface-100 dark:hover:bg-surface-700'
              ]"
            >
              <div class="flex items-center gap-2">
                <i :class="[getSnapshotIcon(snapshot), 'text-surface-400']" />
                <div>
                  <div class="font-medium text-sm text-surface-900 dark:text-surface-100">
                    {{ snapshot.name }}
                    <Tag
                      v-if="isCurrentSnapshot(vm.name, snapshot.name)"
                      :value="t('snapshotPanel.currentTag')"
                      severity="success"
                      class="ml-2 text-xs"
                    />
                  </div>
                  <div v-if="snapshot.description" class="text-xs text-surface-500">
                    {{ snapshot.description }}
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-1">
                <Button
                  @click="openRestoreDialog(vm.name, snapshot.name)"
                  :disabled="isCurrentSnapshot(vm.name, snapshot.name)"
                  icon="pi pi-replay"
                  severity="info"
                  size="small"
                  rounded
                  text
                  v-tooltip.top="t('snapshotPanel.restoreTooltip')"
                />
                <Button
                  v-if="snapshot.name !== 'initial'"
                  @click="openDeleteDialog(vm.name, snapshot.name)"
                  icon="pi pi-trash"
                  severity="danger"
                  size="small"
                  rounded
                  text
                  v-tooltip.top="t('snapshotPanel.deleteTooltip')"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </Card>

  <!-- Create Snapshot Dialog -->
  <Dialog
    v-model:visible="showCreateDialog"
    modal
    :header="t('snapshotPanel.createDialog.header')"
    :style="{ width: '400px' }"
  >
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ t('snapshotPanel.createDialog.vmLabel') }}
        </label>
        <div class="text-surface-900 dark:text-surface-100 font-medium">
          {{ createVmName }}
        </div>
      </div>

      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ t('snapshotPanel.createDialog.nameLabel') }}
        </label>
        <InputText
          v-model="newSnapshotName"
          :placeholder="t('snapshotPanel.createDialog.namePlaceholder')"
          class="w-full"
          :disabled="creating"
        />
      </div>

      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ t('snapshotPanel.createDialog.descriptionLabel') }}
        </label>
        <Textarea
          v-model="newSnapshotDescription"
          :placeholder="t('snapshotPanel.createDialog.descriptionPlaceholder')"
          :aria-label="t('snapshotPanel.createDialog.descriptionAria')"
          rows="2"
          class="w-full"
          :disabled="creating"
        />
      </div>
    </div>

    <template #footer>
      <Button @click="showCreateDialog = false" :label="t('snapshotPanel.createDialog.cancel')" severity="secondary" :disabled="creating" />
      <Button
        @click="createSnapshot"
        :loading="creating"
        :disabled="creating || !newSnapshotName.trim()"
        :label="t('snapshotPanel.createDialog.submit')"
        icon="pi pi-camera"
      />
    </template>
  </Dialog>

  <!-- Restore Confirmation Dialog -->
  <Dialog
    v-model:visible="showRestoreDialog"
    modal
    :header="t('snapshotPanel.restoreDialog.header')"
    :style="{ width: '400px' }"
  >
    <div class="text-surface-700 dark:text-surface-300">
      <p class="mb-4">
        {{ t('snapshotPanel.restoreDialog.confirmQuestion', { vm: restoreVmName, snapshot: restoreSnapshotName }) }}
      </p>
      <p class="text-sm text-orange-600 dark:text-orange-400">
        <i class="pi pi-exclamation-triangle mr-1" />
        {{ t('snapshotPanel.restoreDialog.warning') }}
      </p>
    </div>

    <template #footer>
      <Button @click="showRestoreDialog = false" :label="t('snapshotPanel.restoreDialog.cancel')" severity="secondary" :disabled="restoring" />
      <Button
        @click="restoreSnapshot"
        :loading="restoring"
        :disabled="restoring"
        :label="t('snapshotPanel.restoreDialog.submit')"
        icon="pi pi-replay"
        severity="warn"
      />
    </template>
  </Dialog>

  <!-- Delete Confirmation Dialog -->
  <Dialog
    v-model:visible="showDeleteDialog"
    modal
    :header="t('snapshotPanel.deleteDialog.header')"
    :style="{ width: '400px' }"
  >
    <div class="text-surface-700 dark:text-surface-300">
      <p class="mb-4">
        {{ t('snapshotPanel.deleteDialog.confirmQuestion', { vm: deleteVmName, snapshot: deleteSnapshotName }) }}
      </p>
      <p class="text-sm text-red-600 dark:text-red-400">
        <i class="pi pi-exclamation-triangle mr-1" />
        {{ t('snapshotPanel.deleteDialog.warning') }}
      </p>
    </div>

    <template #footer>
      <Button @click="showDeleteDialog = false" :label="t('snapshotPanel.deleteDialog.cancel')" severity="secondary" :disabled="deleting" />
      <Button
        @click="deleteSnapshot"
        :loading="deleting"
        :disabled="deleting"
        :label="t('snapshotPanel.deleteDialog.submit')"
        icon="pi pi-trash"
        severity="danger"
      />
    </template>
  </Dialog>
</template>
