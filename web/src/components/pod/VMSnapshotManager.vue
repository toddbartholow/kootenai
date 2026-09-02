<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import { podsApi, snapshotsApi, type PodVM, type Snapshot } from '@/api'
import { useFocusRestore } from '@/composables'
import Button from '@volt/Button.vue'
import Dialog from '@volt/Dialog.vue'
import InputText from '@volt/InputText.vue'
import Checkbox from '@volt/Checkbox.vue'
import RadioButton from '@volt/RadioButton.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'

const { t } = useI18n()

/**
 * Per-VM snapshot controls. Owns:
 *   - The "Revert to Snapshot", "Create Snapshot", and "Reset to Initial"
 *     action buttons.
 *   - The inline snapshots list with inline Revert / Delete per row.
 *   - The Create Snapshot and Revert Modal dialogs.
 *
 * Extracted from PodDetailView.vue (~200 lines of template + ~80 lines
 * of state and handlers). The parent view emits nothing back for
 * snapshot-level operations: it just listens for `@changed` and reloads
 * pod data when one fires.
 *
 * Snapshot list is loaded lazily on mount (cheap network call) and
 * refreshed after any mutation.
 */
const props = defineProps<{
  vm: PodVM
  podId: string
}>()

const emit = defineEmits<{
  (e: 'changed'): void
  (e: 'error', message: string): void
}>()

const toast = useToast()
const confirm = useConfirm()

// List state
const snapshots = ref<Snapshot[]>([])
const loadingList = ref(false)

// Create-modal state
const showCreateModal = ref(false)
const newName = ref('')
const newDescription = ref('')
const newIncludeRam = ref(false)
const creating = ref(false)
useFocusRestore(showCreateModal)

// Revert-modal state
const showRevertModal = ref(false)
const selectedForRevert = ref('')
const reverting = ref(false)
useFocusRestore(showRevertModal)

async function loadList(): Promise<void> {
  loadingList.value = true
  try {
    snapshots.value = await snapshotsApi.list(props.podId, props.vm.name)
  } catch (err) {
    console.error('Failed to load snapshots:', err)
    snapshots.value = []
    emit('error', t('snapshot.errors.loadFailed', { vm: props.vm.name }))
  } finally {
    loadingList.value = false
  }
}

function openCreate(): void {
  newName.value = ''
  newDescription.value = ''
  newIncludeRam.value = false
  showCreateModal.value = true
}

async function doCreate(): Promise<void> {
  if (!newName.value) return
  creating.value = true
  try {
    await snapshotsApi.create(
      props.podId,
      props.vm.name,
      newName.value,
      newDescription.value || undefined,
      newIncludeRam.value,
    )
    toast.add({
      severity: 'success',
      summary: t('snapshot.toasts.created.summary'),
      detail: t('snapshot.toasts.created.detail', { name: newName.value, vm: props.vm.name }),
      life: 3000,
    })
    showCreateModal.value = false
    await loadList()
    emit('changed')
  } catch (err) {
    console.error('Failed to create snapshot:', err)
    emit('error', t('snapshot.errors.createFailed', { vm: props.vm.name }))
  } finally {
    creating.value = false
  }
}

function confirmDelete(snapshotName: string): void {
  confirm.require({
    message: t('snapshot.confirmDelete.message', { name: snapshotName }),
    header: t('snapshot.confirmDelete.header'),
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: t('snapshot.confirmDelete.reject') },
    acceptProps: { label: t('snapshot.confirmDelete.accept') },
    accept: () => void doDelete(snapshotName),
  })
}

async function doDelete(snapshotName: string): Promise<void> {
  try {
    await snapshotsApi.delete(props.podId, props.vm.name, snapshotName)
    await loadList()
    emit('changed')
  } catch (err) {
    console.error('Failed to delete snapshot:', err)
    emit('error', t('snapshot.errors.deleteFailed', { name: snapshotName }))
  }
}

function openRevert(): void {
  selectedForRevert.value = ''
  showRevertModal.value = true
  if (snapshots.value.length === 0) {
    void loadList()
  }
}

async function revertTo(snapshotName: string): Promise<void> {
  reverting.value = true
  try {
    await podsApi.resetVM(props.podId, props.vm.name, snapshotName)
    showRevertModal.value = false
    emit('changed')
  } catch (err) {
    console.error('Failed to revert VM:', err)
    emit('error', t('snapshot.errors.revertFailed', { vm: props.vm.name }))
  } finally {
    reverting.value = false
  }
}

function doInlineRevert(snapshotName: string): void {
  void revertTo(snapshotName)
}

function doModalRevert(): void {
  if (!selectedForRevert.value) return
  void revertTo(selectedForRevert.value)
}

function resetToInitial(): void {
  void revertTo('initial')
}

// Load list when the VM changes so switching the expanded card refreshes.
watch(
  () => `${props.podId}:${props.vm.name}`,
  () => {
    void loadList()
  },
  { immediate: true },
)
</script>

<template>
  <div class="space-y-4">
    <!-- Action buttons -->
    <div class="flex flex-wrap gap-2">
      <h4 class="w-full text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">{{ t('snapshot.sectionHeading') }}</h4>
      <Button
        icon="pi pi-history"
        :label="t('snapshot.actions.revert')"
        severity="secondary"
        size="small"
        :disabled="reverting"
        @click.stop="openRevert"
      />
      <Button
        icon="pi pi-camera"
        :label="t('snapshot.actions.create')"
        severity="success"
        size="small"
        :disabled="creating"
        @click.stop="openCreate"
      />
      <Button
        icon="pi pi-refresh"
        :label="t('snapshot.actions.resetInitial')"
        severity="secondary"
        size="small"
        :disabled="reverting"
        @click.stop="resetToInitial"
      />
    </div>

    <!-- Inline snapshot list -->
    <div>
      <div v-if="loadingList" class="flex justify-center py-4">
        <ProgressSpinner style="width: 24px; height: 24px" />
      </div>
      <div v-else-if="snapshots.length === 0" class="text-sm text-surface-500 py-2">
        {{ t('snapshot.emptyList') }}
      </div>
      <div v-else class="space-y-2">
        <div
          v-for="snap in snapshots"
          :key="snap.name"
          class="flex items-center justify-between bg-white dark:bg-surface-900 rounded-lg border border-surface-200 dark:border-surface-700 px-3 py-2"
        >
          <div>
            <span class="font-medium text-surface-900 dark:text-surface-100">{{ snap.name }}</span>
            <span v-if="snap.description" class="ml-2 text-sm text-surface-500">{{ snap.description }}</span>
            <span v-if="snap.parent" class="ml-2 text-xs text-surface-400">{{ t('snapshot.parentLabel', { name: snap.parent }) }}</span>
          </div>
          <div class="flex gap-2">
            <Button
              icon="pi pi-history"
              :label="t('snapshot.actions.inlineRevert')"
              size="small"
              severity="info"
              :disabled="reverting"
              @click.stop="doInlineRevert(snap.name)"
            />
            <Button
              icon="pi pi-trash"
              :label="t('snapshot.actions.inlineDelete')"
              size="small"
              severity="danger"
              @click.stop="confirmDelete(snap.name)"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Create Snapshot Modal -->
    <Dialog
      v-model:visible="showCreateModal"
      modal
      :header="t('snapshot.create.header')"
      :style="{ width: '450px' }"
    >
      <p class="text-sm text-surface-600 dark:text-surface-400 mb-4">
        {{ t('snapshot.create.intro', { name: vm.name }) }}
      </p>
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">{{ t('snapshot.create.nameLabel') }}</label>
          <InputText v-model="newName" :placeholder="t('snapshot.create.namePlaceholder')" class="w-full" />
        </div>
        <div>
          <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">{{ t('snapshot.create.descriptionLabel') }}</label>
          <InputText v-model="newDescription" :placeholder="t('snapshot.create.descriptionPlaceholder')" class="w-full" />
        </div>
        <div class="flex items-center gap-2">
          <Checkbox v-model="newIncludeRam" inputId="includeRam" binary />
          <label for="includeRam" class="text-sm text-surface-700 dark:text-surface-300">{{ t('snapshot.create.includeRamLabel') }}</label>
        </div>
      </div>
      <template #footer>
        <Button :label="t('snapshot.create.cancel')" severity="secondary" @click="showCreateModal = false" />
        <Button
          :label="t('snapshot.create.submit')"
          :disabled="!newName || creating"
          :loading="creating"
          @click="doCreate"
        />
      </template>
    </Dialog>

    <!-- Revert to Snapshot Modal -->
    <Dialog
      v-model:visible="showRevertModal"
      modal
      :header="t('snapshot.revertModal.header')"
      :style="{ width: '450px' }"
    >
      <p class="text-sm text-surface-600 dark:text-surface-400 mb-4">
        {{ t('snapshot.revertModal.intro', { name: vm.name }) }}
      </p>
      <div v-if="loadingList" class="flex justify-center py-4">
        <ProgressSpinner style="width: 24px; height: 24px" />
      </div>
      <div v-else-if="snapshots.length === 0" class="text-sm text-surface-500 py-4 text-center">
        {{ t('snapshot.revertModal.empty') }}
      </div>
      <div v-else class="space-y-2 max-h-64 overflow-y-auto">
        <label
          v-for="snap in snapshots"
          :key="snap.name"
          class="flex items-center gap-3 p-3 border border-surface-200 dark:border-surface-700 rounded-lg cursor-pointer hover:bg-surface-50 dark:hover:bg-surface-800"
          :class="{ 'border-primary-500 bg-primary-50 dark:bg-primary-900/20': selectedForRevert === snap.name }"
        >
          <RadioButton v-model="selectedForRevert" :value="snap.name" :inputId="snap.name" />
          <div>
            <div class="font-medium text-surface-900 dark:text-surface-100">{{ snap.name }}</div>
            <div v-if="snap.description" class="text-sm text-surface-500">{{ snap.description }}</div>
          </div>
        </label>
      </div>
      <template #footer>
        <Button :label="t('snapshot.revertModal.cancel')" severity="secondary" @click="showRevertModal = false" />
        <Button
          :label="t('snapshot.revertModal.submit')"
          :disabled="!selectedForRevert || reverting"
          :loading="reverting"
          @click="doModalRevert"
        />
      </template>
    </Dialog>
  </div>
</template>
