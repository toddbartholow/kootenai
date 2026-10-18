<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import Dialog from '@volt/Dialog.vue'
import { useToast } from 'primevue/usetoast'
import { formatDate } from '@/utils/format'
import { api } from '@/api/config'
import { useFocusRestore } from '@/composables'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const toast = useToast()

const labId = computed(() => route.params['labId'] as string)

interface VersionSummary {
  id: string
  versionNumber: number
  name: string
  version: string
  changeSummary: string
  createdAt: string
}

interface VersionDetail {
  id: string
  versionNumber: number
  name: string
  slug: string
  description: string
  version: string
  platform: string
  durationMinutes: number
  difficulty: string
  maxPoints: number
  passThreshold: number
  spec: unknown
  checkpoints: unknown
  instructions: unknown
  isActive: boolean
  visibility: string
  changeSummary: string
  createdAt: string
}

const loading = ref(true)
const error = ref<string | null>(null)
const versions = ref<VersionSummary[]>([])
const selectedVersion = ref<VersionDetail | null>(null)
const detailLoading = ref(false)
const showRestoreDialog = ref(false)
useFocusRestore(showRestoreDialog)
const restoring = ref(false)
const labName = ref('')

onMounted(async () => {
  await loadLabInfo()
  await loadVersions()
})

async function loadLabInfo() {
  try {
    const { data } = await api.get(`/labs/${labId.value}`)
    labName.value = data.name || labId.value
  } catch {
    labName.value = labId.value
  }
}

async function loadVersions() {
  try {
    loading.value = true
    error.value = null
    const { data } = await api.get(`/labs/${labId.value}/versions`)
    versions.value = data.versions || []
  } catch (err) {
    console.error('Failed to load versions:', err)
    error.value = t('labVersionHistory.loadFailed')
  } finally {
    loading.value = false
  }
}

async function selectVersion(v: VersionSummary) {
  try {
    detailLoading.value = true
    const { data } = await api.get(`/labs/${labId.value}/versions/${v.versionNumber}`)
    selectedVersion.value = data
  } catch (err) {
    console.error('Failed to load version detail:', err)
    toast.add({
      severity: 'error',
      summary: t('labVersionHistory.toasts.detailFailedSummary'),
      detail: t('labVersionHistory.toasts.detailFailedDetail'),
      life: 5000,
    })
  } finally {
    detailLoading.value = false
  }
}

async function restoreVersion() {
  if (!selectedVersion.value) return
  try {
    restoring.value = true
    await api.post(
      `/labs/${labId.value}/versions/${selectedVersion.value.versionNumber}/restore`,
      {
        changeSummary: t('labVersionHistory.changeSummaryTemplate', {
          number: selectedVersion.value.versionNumber,
        }),
      }
    )
    toast.add({
      severity: 'success',
      summary: t('labVersionHistory.toasts.restoredSummary'),
      detail: t('labVersionHistory.toasts.restoredDetail', { number: selectedVersion.value.versionNumber }),
      life: 3000,
    })
    showRestoreDialog.value = false
    await loadVersions()
  } catch (err) {
    console.error('Restore failed:', err)
    toast.add({
      severity: 'error',
      summary: t('labVersionHistory.toasts.restoreFailedSummary'),
      detail: t('labVersionHistory.toasts.restoreFailedDetail'),
      life: 5000,
    })
  } finally {
    restoring.value = false
  }
}

// formatDate imported from @/utils/format

function formatSpec(spec: unknown): string {
  try {
    return JSON.stringify(spec, null, 2)
  } catch {
    return String(spec)
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <div class="flex items-center gap-2 mb-1">
          <Button icon="pi pi-arrow-left" text severity="secondary" size="small" @click="router.back()" />
          <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('labVersionHistory.title') }}</h1>
        </div>
        <p class="text-surface-600 dark:text-surface-400 ml-10">{{ labName }}</p>
      </div>
      <Button @click="loadVersions" :loading="loading" icon="pi pi-refresh" severity="secondary" />
    </div>

    <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>

    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <div v-else-if="versions.length === 0" class="text-center py-12">
      <i class="pi pi-history text-4xl text-surface-300 mb-4" />
      <h3 class="text-lg font-medium text-surface-700 dark:text-surface-300 mb-2">{{ t('labVersionHistory.empty.title') }}</h3>
      <p class="text-surface-500">{{ t('labVersionHistory.empty.hint') }}</p>
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Version list sidebar -->
      <div class="lg:col-span-1 space-y-2">
        <h3 class="text-sm font-semibold text-surface-500 uppercase tracking-wide mb-3">{{ t('labVersionHistory.versionsHeading') }}</h3>
        <div
          v-for="v in versions"
          :key="v.id"
          role="button"
          tabindex="0"
          :aria-pressed="selectedVersion?.versionNumber === v.versionNumber"
          @click="selectVersion(v)"
          @keydown.enter="selectVersion(v)"
          @keydown.space.prevent="selectVersion(v)"
          class="p-3 rounded-lg border cursor-pointer transition-colors"
          :class="selectedVersion?.versionNumber === v.versionNumber
            ? 'border-primary bg-primary/5'
            : 'border-surface-200 dark:border-surface-700 hover:border-primary/50'"
        >
          <div class="flex items-center justify-between">
            <span class="font-medium text-surface-900 dark:text-surface-100">v{{ v.versionNumber }}</span>
            <Tag :value="v.version" severity="secondary" />
          </div>
          <div class="text-xs text-surface-500 mt-1">{{ formatDate(v.createdAt) }}</div>
          <div v-if="v.changeSummary" class="text-xs text-surface-400 mt-1 italic truncate">{{ v.changeSummary }}</div>
        </div>
      </div>

      <!-- Detail panel -->
      <div class="lg:col-span-2">
        <div v-if="detailLoading" class="flex justify-center py-12">
          <ProgressSpinner />
        </div>
        <Card v-else-if="selectedVersion">
          <template #content>
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  {{ selectedVersion.name }}
                </h2>
                <Button
                  @click="showRestoreDialog = true"
                  :label="t('labVersionHistory.restoreAction')"
                  icon="pi pi-replay"
                  severity="warn"
                  size="small"
                />
              </div>

              <div class="grid grid-cols-2 gap-4 text-sm">
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.version') }}</span> {{ selectedVersion.version }}</div>
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.platform') }}</span> {{ selectedVersion.platform }}</div>
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.difficulty') }}</span> {{ selectedVersion.difficulty }}</div>
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.duration') }}</span> {{ t('labVersionHistory.fields.durationValue', { count: selectedVersion.durationMinutes }) }}</div>
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.maxPoints') }}</span> {{ selectedVersion.maxPoints }}</div>
                <div><span class="text-surface-500">{{ t('labVersionHistory.fields.passThreshold') }}</span> {{ selectedVersion.passThreshold }}%</div>
              </div>

              <div v-if="selectedVersion.description" class="text-sm text-surface-600 dark:text-surface-400">
                {{ selectedVersion.description }}
              </div>

              <div>
                <h3 class="text-sm font-semibold text-surface-500 mb-2">{{ t('labVersionHistory.fields.specHeading') }}</h3>
                <pre class="bg-surface-100 dark:bg-surface-800 p-4 rounded-lg overflow-auto max-h-64 text-xs font-mono">{{ formatSpec(selectedVersion.spec) }}</pre>
              </div>
            </div>
          </template>
        </Card>
        <div v-else class="text-center py-12 text-surface-500">
          {{ t('labVersionHistory.selectHint') }}
        </div>
      </div>
    </div>

    <!-- Restore Confirmation -->
    <Dialog
      v-model:visible="showRestoreDialog"
      :header="t('labVersionHistory.restoreDialog.header')"
      :modal="true"
      :style="{ width: '450px' }"
    >
      <div class="flex items-start gap-4">
        <i class="pi pi-exclamation-triangle text-3xl text-orange-500" />
        <div>
          <p class="font-medium">{{ t('labVersionHistory.restoreDialog.question', { number: selectedVersion?.versionNumber }) }}</p>
          <p class="text-sm text-surface-500 mt-2">
            {{ t('labVersionHistory.restoreDialog.notice') }}
          </p>
        </div>
      </div>
      <template #footer>
        <Button :label="t('labVersionHistory.restoreDialog.cancel')" severity="secondary" @click="showRestoreDialog = false" :disabled="restoring" />
        <Button :label="t('labVersionHistory.restoreDialog.submit')" severity="warn" icon="pi pi-replay" @click="restoreVersion" :loading="restoring" />
      </template>
    </Dialog>
  </div>
</template>
