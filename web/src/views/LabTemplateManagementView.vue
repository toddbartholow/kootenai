<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useAuthStore } from '../stores/auth'
import { useToast } from 'primevue/usetoast'
import { api } from '@/api/config'
import { getDifficultySeverity } from '@/utils/status'
import { formatDate } from '@/utils/format'
import LabTemplateFormDialog, {
  type LabTemplateFormData,
} from '@/components/labTemplate/LabTemplateFormDialog.vue'
import LabTemplateDeleteDialog from '@/components/labTemplate/LabTemplateDeleteDialog.vue'
import LabTemplateSpecDialog from '@/components/labTemplate/LabTemplateSpecDialog.vue'
import LabTemplateImportDialog from '@/components/labTemplate/LabTemplateImportDialog.vue'
import LabTemplateHistoryDialog, {
  type LabTemplateVersion,
} from '@/components/labTemplate/LabTemplateHistoryDialog.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const toast = useToast()
const isAdmin = computed(() => authStore.isAdmin)

// Types
interface LabTemplate {
  id: string
  name: string
  slug: string
  description: string
  version: string
  platform: string
  durationMinutes: number
  difficulty: string
  category: string
  tags: string[]
  maxPoints: number
  passThreshold: number
  isActive: boolean
  visibility: string
  createdAt: string
  updatedAt: string
}

// State
const loading = ref(true)
const error = ref<string | null>(null)
const templates = ref<LabTemplate[]>([])
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showDeleteDialog = ref(false)
const showSpecDialog = ref(false)
const selectedTemplate = ref<LabTemplate | null>(null)
const specContent = ref('')
const saving = ref(false)
const showImportDialog = ref(false)
const showHistoryDialog = ref(false)
const versionHistory = ref<LabTemplateVersion[]>([])
const historyLoading = ref(false)

// Form state for create/edit. Focus restoration on dialog close lives
// inside each extracted dialog component (via useFocusRestore).
const formData = ref<LabTemplateFormData>({
  name: '',
  description: '',
  version: '1.0.0',
  platform: 'proxmox',
  durationMinutes: 60,
  difficulty: 'beginner',
  category: '',
  tags: [],
  maxPoints: 100,
  passThreshold: 70,
  spec: '',
  isActive: true,
  visibility: 'global',
})

// Options imported from @/constants/formOptions

onMounted(async () => {
  await loadTemplates()
})

async function loadTemplates() {
  try {
    loading.value = true
    error.value = null
    const { data } = await api.get('/labs', { params: { active: 'all' } })
    templates.value = data.labs || []
  } catch (err) {
    console.error('Failed to load templates:', err)
    error.value = t('labTemplateMgmt.loadFailed')
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  formData.value = {
    name: '',
    description: '',
    version: '1.0.0',
    platform: 'proxmox',
    durationMinutes: 60,
    difficulty: 'beginner',
    category: '',
    tags: [],
    maxPoints: 100,
    passThreshold: 70,
    spec: getDefaultSpec(),
    isActive: true,
    visibility: 'global',
  }
  showCreateDialog.value = true
}

function openEditDialog(template: LabTemplate) {
  selectedTemplate.value = template
  formData.value = {
    name: template.name,
    description: template.description || '',
    version: template.version,
    platform: template.platform,
    durationMinutes: template.durationMinutes || 60,
    difficulty: template.difficulty || 'beginner',
    category: template.category || '',
    tags: template.tags || [],
    maxPoints: template.maxPoints,
    passThreshold: template.passThreshold,
    spec: '', // Will be loaded separately
    isActive: template.isActive,
    visibility: template.visibility || 'global',
  }
  // Load full spec
  loadTemplateSpec(template.id)
  showEditDialog.value = true
}

async function loadTemplateSpec(templateId: string) {
  try {
    const { data } = await api.get(`/labs/${templateId}`, { params: { include_spec: 'true' } })
    if (data.spec) {
      formData.value.spec = JSON.stringify(data.spec, null, 2)
    }
  } catch (err) {
    console.error('Failed to load template spec:', err)
  }
}

function openDeleteDialog(template: LabTemplate) {
  selectedTemplate.value = template
  showDeleteDialog.value = true
}

function openSpecDialog(template: LabTemplate) {
  selectedTemplate.value = template
  loadTemplateSpecForView(template.id)
  showSpecDialog.value = true
}

async function loadTemplateSpecForView(templateId: string) {
  try {
    specContent.value = t('labTemplateMgmt.spec.loading')
    const { data } = await api.get(`/labs/${templateId}`, { params: { include_spec: 'true' } })
    if (data.spec) {
      specContent.value = JSON.stringify(data.spec, null, 2)
    } else {
      specContent.value = t('labTemplateMgmt.spec.empty')
    }
  } catch (err) {
    console.error('Failed to load spec:', err)
    specContent.value = t('labTemplateMgmt.spec.loadFailed')
  }
}

async function createTemplate() {
  try {
    saving.value = true
    await api.post('/labs', formData.value)

    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.successSummary'),
      detail: t('labTemplateMgmt.toasts.createdDetail'),
      life: 3000,
    })

    showCreateDialog.value = false
    await loadTemplates()
  } catch (err) {
    console.error('Failed to create template:', err)
    const message =
      err instanceof Error ? err.message : t('labTemplateMgmt.toasts.createFailedDetail')
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function updateTemplate() {
  if (!selectedTemplate.value) return

  try {
    saving.value = true
    await api.put(`/labs/${selectedTemplate.value.id}`, formData.value)

    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.successSummary'),
      detail: t('labTemplateMgmt.toasts.updatedDetail'),
      life: 3000,
    })

    showEditDialog.value = false
    await loadTemplates()
  } catch (err) {
    console.error('Failed to update template:', err)
    const message =
      err instanceof Error ? err.message : t('labTemplateMgmt.toasts.updateFailedDetail')
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function deleteTemplate() {
  if (!selectedTemplate.value) return

  try {
    saving.value = true
    await api.delete(`/labs/${selectedTemplate.value.id}`)

    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.successSummary'),
      detail: t('labTemplateMgmt.toasts.deletedDetail'),
      life: 3000,
    })

    showDeleteDialog.value = false
    await loadTemplates()
  } catch (err) {
    console.error('Failed to delete template:', err)
    const message =
      err instanceof Error ? err.message : t('labTemplateMgmt.toasts.deleteFailedDetail')
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function toggleActive(template: LabTemplate) {
  try {
    await api.put(`/labs/${template.id}/active`, { isActive: !template.isActive })

    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.successSummary'),
      detail: template.isActive
        ? t('labTemplateMgmt.toasts.statusChangedInactive')
        : t('labTemplateMgmt.toasts.statusChangedActive'),
      life: 3000,
    })

    await loadTemplates()
  } catch (err) {
    console.error('Failed to toggle active status:', err)
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: t('labTemplateMgmt.toasts.statusFailed'),
      life: 5000,
    })
  }
}

async function exportTemplate(template: LabTemplate) {
  try {
    const { data: yamlText } = await api.get(`/labs/${template.id}/export`, {
      responseType: 'text',
    })
    const blob = new Blob([yamlText], { type: 'application/x-yaml' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${template.name.replace(/\s+/g, '-')}.yaml`
    a.click()
    URL.revokeObjectURL(url)
    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.exportedSummary'),
      detail: t('labTemplateMgmt.toasts.exportedDetail'),
      life: 3000,
    })
  } catch (err) {
    console.error('Export failed:', err)
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: t('labTemplateMgmt.toasts.exportFailedDetail'),
      life: 5000,
    })
  }
}

function openImportDialog() {
  showImportDialog.value = true
}

async function importTemplate(yaml: string) {
  if (!yaml.trim()) return
  try {
    saving.value = true
    await api.post('/labs/import', { yaml })
    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.importedSummary'),
      detail: t('labTemplateMgmt.toasts.importedDetail'),
      life: 3000,
    })
    showImportDialog.value = false
    await loadTemplates()
  } catch (err) {
    console.error('Import failed:', err)
    const message =
      err instanceof Error ? err.message : t('labTemplateMgmt.toasts.importFailedDetail')
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function openHistoryDialog(template: LabTemplate) {
  selectedTemplate.value = template
  versionHistory.value = []
  historyLoading.value = true
  showHistoryDialog.value = true
  try {
    const { data } = await api.get(`/labs/${template.id}/versions`)
    versionHistory.value = data.versions || []
  } catch (err) {
    console.error('Failed to load history:', err)
  } finally {
    historyLoading.value = false
  }
}

async function restoreVersion(versionNumber: number) {
  if (!selectedTemplate.value) return
  try {
    saving.value = true
    await api.post(`/labs/${selectedTemplate.value.id}/versions/${versionNumber}/restore`, {
      changeSummary: t('labTemplateMgmt.changeSummaryTemplate', { number: versionNumber }),
    })
    toast.add({
      severity: 'success',
      summary: t('labTemplateMgmt.toasts.restoredSummary'),
      detail: t('labTemplateMgmt.toasts.restoredDetail', { number: versionNumber }),
      life: 3000,
    })
    showHistoryDialog.value = false
    await loadTemplates()
  } catch (err) {
    console.error('Restore failed:', err)
    toast.add({
      severity: 'error',
      summary: t('labTemplateMgmt.toasts.errorSummary'),
      detail: t('labTemplateMgmt.toasts.restoreFailedDetail'),
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

// getDifficultySeverity imported from @/utils/status
const getDifficultyColor = getDifficultySeverity

// formatDate imported from @/utils/format

function getDefaultSpec(): string {
  return `apiVersion: v1
kind: LabTemplate
metadata:
  name: "New Lab"
  description: "Lab description"
  duration: "60m"
  difficulty: "beginner"
  tags:
    - example
spec:
  platform: proxmox
  network:
    segments:
      - name: lab-network
        vlan: 100
        subnet: "10.0.100.0/24"
  vms:
    - name: student-vm
      template: "ubuntu-22.04-template"
      memory: 2048
      cores: 2
      network: lab-network
  objectives:
    - id: obj-1
      name: "Complete first task"
      description: "Description of the first objective"
      points: 50
      triggers:
        - type: file_exists
          target: "/home/student/task1.txt"
`
}
</script>

<template>
  <div class="space-y-6">
    <!-- Access denied for non-admins -->
    <Message v-if="!isAdmin" severity="error" :closable="false">
      <i class="pi pi-lock mr-2" />
      {{ t('labTemplateMgmt.accessDenied') }}
    </Message>

    <template v-else>
      <!-- Header -->
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
            {{ t('labTemplateMgmt.title') }}
          </h1>
          <p class="text-surface-600 dark:text-surface-400 mt-1">
            {{ t('labTemplateMgmt.subtitle') }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <Button
            @click="loadTemplates"
            :loading="loading"
            icon="pi pi-refresh"
            severity="secondary"
          />
          <Button
            @click="openImportDialog"
            icon="pi pi-upload"
            :label="t('labTemplateMgmt.actions.importLab')"
            severity="secondary"
          />
          <Button
            @click="openCreateDialog"
            icon="pi pi-plus"
            :label="t('labTemplateMgmt.actions.newTemplate')"
          />
        </div>
      </div>

      <!-- Error Message -->
      <Message v-if="error" severity="error" :closable="false">
        {{ error }}
      </Message>

      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <ProgressSpinner />
      </div>

      <!-- Templates Grid -->
      <div
        v-else-if="templates.length > 0"
        class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4"
      >
        <Card v-for="template in templates" :key="template.id">
          <template #content>
            <div class="space-y-4">
              <!-- Header -->
              <div class="flex items-start justify-between">
                <div class="flex-1 min-w-0">
                  <h3 class="font-semibold text-surface-900 dark:text-surface-100 truncate">
                    {{ template.name }}
                  </h3>
                  <p class="text-sm text-surface-500 truncate">{{ template.slug }}</p>
                </div>
                <Tag
                  :value="
                    template.isActive
                      ? t('labTemplateMgmt.cardTags.active')
                      : t('labTemplateMgmt.cardTags.inactive')
                  "
                  :severity="template.isActive ? 'success' : 'secondary'"
                />
              </div>

              <!-- Description -->
              <p class="text-sm text-surface-600 dark:text-surface-400 line-clamp-2">
                {{ template.description || t('labTemplateMgmt.cardTags.noDescription') }}
              </p>

              <!-- Metadata -->
              <div class="flex flex-wrap gap-2">
                <Tag :value="template.platform" severity="info" />
                <Tag
                  v-if="template.difficulty"
                  :value="template.difficulty"
                  :severity="getDifficultyColor(template.difficulty)"
                />
                <Tag
                  :value="
                    t('labTemplateMgmt.cardTags.versionFormat', { version: template.version })
                  "
                  severity="secondary"
                />
              </div>

              <!-- Stats -->
              <div class="flex items-center gap-4 text-sm text-surface-500">
                <span>
                  <i class="pi pi-star mr-1" />
                  {{ t('labTemplateMgmt.cardStats.pointsValue', { count: template.maxPoints }) }}
                </span>
                <span>
                  <i class="pi pi-clock mr-1" />
                  {{
                    t('labTemplateMgmt.cardStats.durationValue', {
                      count: template.durationMinutes || 60,
                    })
                  }}
                </span>
                <span>
                  <i class="pi pi-check mr-1" />
                  {{
                    t('labTemplateMgmt.cardStats.thresholdValue', { value: template.passThreshold })
                  }}
                </span>
              </div>

              <!-- Actions -->
              <div
                class="flex items-center gap-2 pt-2 border-t border-surface-200 dark:border-surface-700"
              >
                <Button
                  @click="openHistoryDialog(template)"
                  icon="pi pi-history"
                  severity="secondary"
                  text
                  size="small"
                  v-tooltip="t('labTemplateMgmt.cardTooltips.history')"
                />
                <Button
                  @click="exportTemplate(template)"
                  icon="pi pi-download"
                  severity="secondary"
                  text
                  size="small"
                  v-tooltip="t('labTemplateMgmt.cardTooltips.export')"
                />
                <Button
                  @click="openSpecDialog(template)"
                  icon="pi pi-code"
                  severity="secondary"
                  text
                  size="small"
                  v-tooltip="t('labTemplateMgmt.cardTooltips.viewSpec')"
                />
                <Button
                  @click="openEditDialog(template)"
                  icon="pi pi-pencil"
                  severity="secondary"
                  text
                  size="small"
                  v-tooltip="t('labTemplateMgmt.cardTooltips.edit')"
                />
                <Button
                  @click="toggleActive(template)"
                  :icon="template.isActive ? 'pi pi-eye-slash' : 'pi pi-eye'"
                  severity="secondary"
                  text
                  size="small"
                  v-tooltip="
                    template.isActive
                      ? t('labTemplateMgmt.cardTooltips.deactivate')
                      : t('labTemplateMgmt.cardTooltips.activate')
                  "
                />
                <Button
                  @click="openDeleteDialog(template)"
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  v-tooltip="t('labTemplateMgmt.cardTooltips.delete')"
                />
              </div>

              <!-- Footer -->
              <div class="text-xs text-surface-400">
                {{
                  t('labTemplateMgmt.cardFooter.updatedAt', {
                    when: formatDate(template.updatedAt),
                  })
                }}
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Empty State -->
      <Card v-else>
        <template #content>
          <div class="text-center py-12">
            <i class="pi pi-book text-4xl text-surface-300 mb-4" />
            <h3 class="text-lg font-medium text-surface-700 dark:text-surface-300 mb-2">
              {{ t('labTemplateMgmt.empty.title') }}
            </h3>
            <p class="text-surface-500 mb-4">{{ t('labTemplateMgmt.empty.hint') }}</p>
            <Button
              @click="openCreateDialog"
              icon="pi pi-plus"
              :label="t('labTemplateMgmt.actions.createTemplate')"
            />
          </div>
        </template>
      </Card>
    </template>

    <LabTemplateFormDialog
      v-model:visible="showCreateDialog"
      v-model:formData="formData"
      mode="create"
      :saving="saving"
      @save="createTemplate"
    />

    <LabTemplateFormDialog
      v-model:visible="showEditDialog"
      v-model:formData="formData"
      mode="edit"
      :saving="saving"
      @save="updateTemplate"
    />

    <LabTemplateDeleteDialog
      v-model:visible="showDeleteDialog"
      :template-name="selectedTemplate?.name ?? ''"
      :saving="saving"
      @confirm="deleteTemplate"
    />

    <LabTemplateSpecDialog
      v-model:visible="showSpecDialog"
      :template-name="selectedTemplate?.name ?? ''"
      :spec-content="specContent"
    />

    <LabTemplateImportDialog
      v-model:visible="showImportDialog"
      :saving="saving"
      @import="importTemplate"
    />

    <LabTemplateHistoryDialog
      v-model:visible="showHistoryDialog"
      :template-name="selectedTemplate?.name ?? ''"
      :versions="versionHistory"
      :loading="historyLoading"
      :saving="saving"
      @restore="restoreVersion"
    />
  </div>
</template>
