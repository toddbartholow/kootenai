<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import {
  labsApi,
  pathwaysApi,
  type Lab,
  type Pathway,
  type UpdatePathwayRequest,
  type CreateModuleRequest,
  type UpdateModuleRequest,
  type AddLabToModuleRequest,
} from '@/api'
import { useAuthStore } from '../stores/auth'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import Tag from '@volt/Tag.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import PathwayBasicInfo from '@/components/pathway/PathwayBasicInfo.vue'
import PathwayModuleEditor from '@/components/pathway/PathwayModuleEditor.vue'
import type { LocalModule, LocalLab } from '@/components/pathway/PathwayModuleEditor.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const confirm = useConfirm()
const toast = useToast()

// Get pathway ID/slug from route
const pathwaySlug = computed(() => route.params['slug'] as string)

// Loading states
const loadingPathway = ref(true)
const loadingLabs = ref(true)
const saving = ref(false)
const error = ref<string | null>(null)

// Original pathway data
const originalPathway = ref<Pathway | null>(null)

// Form data - Basic Info
const pathwayData = ref({
  name: '',
  description: '',
  shortDescription: '',
  difficulty: 'beginner' as 'beginner' | 'intermediate' | 'advanced' | 'mixed',
  visibility: 'global' as 'global' | 'organization' | 'private',
  estimatedHours: undefined as number | undefined,
  tags: [] as string[],
})
const tagInput = ref('')

// Form data - Modules
const modules = ref<LocalModule[]>([])
const newModuleName = ref('')
const newModuleDescription = ref('')
const newModuleUnlockType = ref<'sequential' | 'all_previous' | 'manual' | 'always'>('sequential')

// Available labs
const availableLabs = ref<Lab[]>([])

// Check if user can edit
const canEdit = computed(() => {
  return authStore.isAdmin || authStore.user?.roles?.includes('instructor')
})

// Active modules (not deleted)
const activeModules = computed(() => modules.value.filter(m => !m.isDeleted))

// Validation
const isValid = computed(() => {
  return pathwayData.value.name && pathwayData.value.name.trim().length >= 2
})

onMounted(async () => {
  if (!canEdit.value) {
    toast.add({
      severity: 'error',
      summary: t('pathway.edit.accessDeniedSummary'),
      detail: t('pathway.edit.accessDeniedDetail'),
      life: 5000,
    })
    router.push('/pathways')
    return
  }

  await Promise.all([loadPathway(), loadAvailableLabs()])
})

async function loadPathway() {
  try {
    loadingPathway.value = true
    const pathway = await pathwaysApi.get(pathwaySlug.value)
    originalPathway.value = pathway

    // Populate form data
    pathwayData.value = {
      name: pathway.name,
      description: pathway.description || '',
      shortDescription: pathway.shortDescription || '',
      difficulty: (pathway.difficulty || 'beginner') as
        | 'beginner'
        | 'intermediate'
        | 'advanced'
        | 'mixed',
      visibility: pathway.visibility || 'global',
      estimatedHours: pathway.estimatedHours,
      tags: pathway.tags || [],
    }

    // Load modules
    if (pathway.modules) {
      modules.value = pathway.modules.map(m => {
        const localModule: LocalModule = {
          id: m.id,
          isNew: false,
          name: m.name,
          unlockType: m.unlockType || 'sequential',
          displayOrder: m.displayOrder,
          labs: (m.labs || []).map(l => {
            const localLab: LocalLab = {
              id: l.id,
              labTemplateId: l.labTemplateId,
              lab: {
                id: l.labTemplateId,
                name: l.labName || l.labTemplateId,
                description: l.labDescription || '',
                difficulty: l.labDifficulty || 'beginner',
                durationMinutes: l.labDurationMinutes || 0,
                platform: l.labPlatform || 'proxmox',
                maxPoints: l.labMaxPoints || 0,
              },
              isRequired: l.isRequired ?? true,
              isNew: false,
            }
            if (l.passThresholdOverride !== undefined) {
              localLab.passThresholdOverride = l.passThresholdOverride
            }
            return localLab
          }),
        }
        if (m.description) {
          localModule.description = m.description
        }
        return localModule
      })
    }
  } catch (err) {
    console.error('Failed to load pathway:', err)
    error.value = t('pathway.edit.loadFailed')
  } finally {
    loadingPathway.value = false
  }
}

async function loadAvailableLabs() {
  try {
    loadingLabs.value = true
    const response = await labsApi.list()
    availableLabs.value = response.labs
  } catch (err) {
    console.error('Failed to load labs:', err)
  } finally {
    loadingLabs.value = false
  }
}

// Tag management
function addTag() {
  const tag = tagInput.value.trim().toLowerCase()
  if (tag && !pathwayData.value.tags.includes(tag)) {
    pathwayData.value.tags = [...pathwayData.value.tags, tag]
  }
  tagInput.value = ''
}

function removeTag(tag: string) {
  pathwayData.value.tags = pathwayData.value.tags.filter(t => t !== tag)
}

// Module management
function addModule() {
  if (newModuleName.value.trim().length < 2) return

  const maxOrder = Math.max(0, ...activeModules.value.map(m => m.displayOrder))

  const newModule: LocalModule = {
    id: `new-${Date.now()}`,
    isNew: true,
    name: newModuleName.value.trim(),
    unlockType: newModuleUnlockType.value,
    displayOrder: maxOrder + 1,
    labs: [],
  }
  const desc = newModuleDescription.value.trim()
  if (desc) {
    newModule.description = desc
  }
  modules.value.push(newModule)

  newModuleName.value = ''
  newModuleDescription.value = ''
  newModuleUnlockType.value = 'sequential'
}

function deleteModule(index: number) {
  const module = activeModules.value[index]
  if (!module) return
  if (module.isNew) {
    modules.value = modules.value.filter(m => m.id !== module.id)
  } else {
    const realIndex = modules.value.findIndex(m => m.id === module.id)
    if (realIndex >= 0) {
      const targetModule = modules.value[realIndex]
      if (targetModule) {
        targetModule.isDeleted = true
      }
    }
  }
}

function moveModuleUp(index: number) {
  if (index > 0) {
    const active = activeModules.value
    const current = active[index]
    const prev = active[index - 1]
    if (!current || !prev) return
    const temp = current.displayOrder
    current.displayOrder = prev.displayOrder
    prev.displayOrder = temp
  }
}

function moveModuleDown(index: number) {
  const active = activeModules.value
  if (index < active.length - 1) {
    const current = active[index]
    const next = active[index + 1]
    if (!current || !next) return
    const temp = current.displayOrder
    current.displayOrder = next.displayOrder
    next.displayOrder = temp
  }
}

// Lab management
function addLabToModule(moduleId: string, lab: Lab) {
  const module = modules.value.find(m => m.id === moduleId)
  if (!module) return

  if (module.labs.some(l => l.labTemplateId === lab.id && !l.isDeleted)) {
    return
  }

  module.labs.push({
    labTemplateId: lab.id,
    lab,
    isRequired: true,
    isNew: true,
  })
}

function removeLabFromModule(moduleId: string, labIndex: number) {
  const module = modules.value.find(m => m.id === moduleId)
  if (!module) return

  const activeLabs = module.labs.filter(l => !l.isDeleted)
  const lab = activeLabs[labIndex]
  if (!lab) return

  if (lab.isNew) {
    module.labs = module.labs.filter(l => l !== lab)
  } else {
    const realIndex = module.labs.findIndex(l => l === lab)
    if (realIndex >= 0) {
      const targetLab = module.labs[realIndex]
      if (targetLab) {
        targetLab.isDeleted = true
      }
    }
  }
}

function moveLabUp(moduleId: string, labIndex: number) {
  const module = modules.value.find(m => m.id === moduleId)
  if (!module || labIndex === 0) return

  const activeLabs = module.labs.filter(l => !l.isDeleted)
  const current = activeLabs[labIndex]
  const prev = activeLabs[labIndex - 1]
  if (!current || !prev) return
  activeLabs[labIndex] = prev
  activeLabs[labIndex - 1] = current

  const deletedLabs = module.labs.filter(l => l.isDeleted)
  module.labs = [...activeLabs, ...deletedLabs]
}

function moveLabDown(moduleId: string, labIndex: number) {
  const module = modules.value.find(m => m.id === moduleId)
  if (!module) return

  const activeLabs = module.labs.filter(l => !l.isDeleted)
  if (labIndex >= activeLabs.length - 1) return

  const current = activeLabs[labIndex]
  const next = activeLabs[labIndex + 1]
  if (!current || !next) return
  activeLabs[labIndex] = next
  activeLabs[labIndex + 1] = current

  const deletedLabs = module.labs.filter(l => l.isDeleted)
  module.labs = [...activeLabs, ...deletedLabs]
}

// Save changes
async function saveChanges() {
  if (!originalPathway.value || !isValid.value) return

  try {
    saving.value = true
    error.value = null

    // 1. Update pathway basic info
    const updateRequest: UpdatePathwayRequest = {
      name: pathwayData.value.name,
      ...(pathwayData.value.description && { description: pathwayData.value.description }),
      ...(pathwayData.value.shortDescription && {
        shortDescription: pathwayData.value.shortDescription,
      }),
      ...(pathwayData.value.difficulty && { difficulty: pathwayData.value.difficulty }),
      ...(pathwayData.value.visibility && { visibility: pathwayData.value.visibility }),
      ...(pathwayData.value.estimatedHours !== undefined && {
        estimatedHours: pathwayData.value.estimatedHours,
      }),
      ...(pathwayData.value.tags.length > 0 && { tags: pathwayData.value.tags }),
    }
    await pathwaysApi.update(originalPathway.value.id, updateRequest)

    // 2. Handle deleted modules
    for (const module of modules.value.filter(m => m.isDeleted && !m.isNew)) {
      await pathwaysApi.deleteModule(module.id)
    }

    // 3. Handle new modules
    for (const module of modules.value.filter(m => m.isNew && !m.isDeleted)) {
      const moduleRequest: CreateModuleRequest = {
        name: module.name,
        ...(module.description && { description: module.description }),
        ...(module.unlockType && { unlockType: module.unlockType }),
      }
      const created = await pathwaysApi.createModule(originalPathway.value.id, moduleRequest)

      for (let i = 0; i < module.labs.length; i++) {
        const lab = module.labs[i]
        if (!lab) continue
        if (!lab.isDeleted) {
          const labRequest: AddLabToModuleRequest = {
            labTemplateId: lab.labTemplateId,
            displayOrder: i,
            isRequired: lab.isRequired,
            ...(lab.passThresholdOverride !== undefined && {
              passThresholdOverride: lab.passThresholdOverride,
            }),
          }
          await pathwaysApi.addLabToModule(created.id, labRequest)
        }
      }
    }

    // 4. Handle existing modules updates
    for (const module of modules.value.filter(m => !m.isNew && !m.isDeleted)) {
      const moduleUpdateRequest: UpdateModuleRequest = {
        name: module.name,
        ...(module.description && { description: module.description }),
        ...(module.unlockType && { unlockType: module.unlockType }),
      }
      await pathwaysApi.updateModule(module.id, moduleUpdateRequest)

      for (const lab of module.labs.filter(l => l.isDeleted && !l.isNew && l.id)) {
        if (lab.id) {
          await pathwaysApi.removeLabFromModule(module.id, lab.id)
        }
      }

      const activeLabs = module.labs.filter(l => !l.isDeleted)
      for (let i = 0; i < activeLabs.length; i++) {
        const lab = activeLabs[i]
        if (!lab) continue
        if (lab.isNew) {
          const labRequest: AddLabToModuleRequest = {
            labTemplateId: lab.labTemplateId,
            displayOrder: i,
            isRequired: lab.isRequired,
            ...(lab.passThresholdOverride !== undefined && {
              passThresholdOverride: lab.passThresholdOverride,
            }),
          }
          await pathwaysApi.addLabToModule(module.id, labRequest)
        }
      }
    }

    toast.add({
      severity: 'success',
      summary: t('pathway.edit.saveSuccessSummary'),
      detail: t('pathway.edit.saveSuccessDetail'),
      life: 3000,
    })

    router.push(`/pathway/${originalPathway.value.slug || originalPathway.value.id}`)
  } catch (err) {
    console.error('Failed to save changes:', err)
    error.value = err instanceof Error ? err.message : t('pathway.edit.saveFailedFallback')
  } finally {
    saving.value = false
  }
}

function cancel() {
  confirm.require({
    message: t('pathway.edit.cancelDialog.message'),
    header: t('pathway.edit.cancelDialog.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: () => {
      router.push(`/pathway/${pathwaySlug.value}`)
    },
  })
}
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-6">
    <ConfirmDialog />

    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
          {{ t('pathway.edit.title') }}
        </h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ originalPathway?.name || t('pathway.edit.subtitleLoading') }}
        </p>
      </div>
      <Tag
        v-if="originalPathway?.status === 'draft'"
        :value="t('pathway.edit.statusDraft')"
        severity="warn"
      />
      <Tag
        v-else-if="originalPathway?.status === 'published'"
        :value="t('pathway.edit.statusPublished')"
        severity="success"
      />
    </div>

    <!-- Loading state -->
    <div v-if="loadingPathway" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Error state when pathway failed to load -->
    <Message v-else-if="error && !originalPathway" severity="error" :closable="false">
      {{ error }}
    </Message>

    <template v-else-if="originalPathway">
      <!-- Error message -->
      <Message v-if="error" severity="error" :closable="true" @close="error = null">
        {{ error }}
      </Message>

      <!-- Basic Info Section -->
      <PathwayBasicInfo
        :pathway-data="pathwayData"
        :tag-input="tagInput"
        @update:tag-input="tagInput = $event"
        @add-tag="addTag"
        @remove-tag="removeTag"
      />

      <!-- Modules Section -->
      <PathwayModuleEditor
        :modules="modules"
        :available-labs="availableLabs"
        :loading-labs="loadingLabs"
        :new-module-name="newModuleName"
        :new-module-description="newModuleDescription"
        :new-module-unlock-type="newModuleUnlockType"
        @update:new-module-name="newModuleName = $event"
        @update:new-module-description="newModuleDescription = $event"
        @update:new-module-unlock-type="newModuleUnlockType = $event"
        @add-module="addModule"
        @delete-module="deleteModule"
        @move-module-up="moveModuleUp"
        @move-module-down="moveModuleDown"
        @add-lab-to-module="addLabToModule"
        @remove-lab-from-module="removeLabFromModule"
        @move-lab-up="moveLabUp"
        @move-lab-down="moveLabDown"
      />

      <!-- Action Buttons -->
      <div class="flex justify-end gap-3">
        <Button
          :label="t('pathway.edit.cancel')"
          severity="secondary"
          @click="cancel"
          :disabled="saving"
        />
        <Button
          :label="t('pathway.edit.submit')"
          icon="pi pi-check"
          @click="saveChanges"
          :loading="saving"
          :disabled="saving || !isValid"
        />
      </div>
    </template>
  </div>
</template>
