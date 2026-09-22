<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  labsApi,
  pathwaysApi,
  type Lab,
  type CreatePathwayRequest,
  type CreateModuleRequest,
  type AddLabToModuleRequest,
} from '@/api'
import { useAuthStore } from '../stores/auth'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import { useToast } from 'primevue/usetoast'
import type { WizardModule, WizardPathwayData, UnlockTypeOption } from '@/components/pathway/wizard-types'
import PathwayWizardSteps from '@/components/pathway/PathwayWizardSteps.vue'
import PathwayBasicInfoStep from '@/components/pathway/PathwayBasicInfoStep.vue'
import PathwayModulesStep from '@/components/pathway/PathwayModulesStep.vue'
import PathwayLabsStep from '@/components/pathway/PathwayLabsStep.vue'
import PathwayReviewStep from '@/components/pathway/PathwayReviewStep.vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()

// Wizard state
const currentStep = ref(1)
const totalSteps = 4

// Form data - Step 1: Basic Info
const pathwayData = ref<WizardPathwayData>({
  name: '',
  description: '',
  shortDescription: '',
  difficulty: 'beginner',
  visibility: 'global',
  tags: [],
})
const tagInput = ref('')

// Form data - Step 2: Modules
const modules = ref<WizardModule[]>([])
const newModuleName = ref('')
const newModuleDescription = ref('')
const newModuleUnlockType = ref<UnlockTypeOption['value']>('sequential')

// Form data - Step 3: Labs
const availableLabs = ref<Lab[]>([])
const loadingLabs = ref(true)

// Submission state
const creating = ref(false)
const error = ref<string | null>(null)

// Unlock type options with descriptions — computed so labels refresh on locale change.
const unlockTypeOptions = computed<UnlockTypeOption[]>(() => [
  { label: t('pathway.create.unlockTypes.sequential'), value: 'sequential', description: t('pathway.create.unlockTypes.sequentialDescription') },
  { label: t('pathway.create.unlockTypes.allPrevious'), value: 'all_previous', description: t('pathway.create.unlockTypes.allPreviousDescription') },
  { label: t('pathway.create.unlockTypes.manual'), value: 'manual', description: t('pathway.create.unlockTypes.manualDescription') },
  { label: t('pathway.create.unlockTypes.always'), value: 'always', description: t('pathway.create.unlockTypes.alwaysDescription') },
])

// Computed validations
const isStep1Valid = computed(() => {
  return pathwayData.value.name.trim().length >= 2
})

const isStep2Valid = computed(() => {
  return modules.value.length > 0 && modules.value.every(m => m.name.trim().length >= 2)
})

const isStep3Valid = computed(() => {
  // At least one module should have at least one lab
  return modules.value.some(m => m.labs.length > 0)
})

const canProceed = computed(() => {
  switch (currentStep.value) {
    case 1: return isStep1Valid.value
    case 2: return isStep2Valid.value
    case 3: return isStep3Valid.value
    case 4: return true
    default: return false
  }
})

// Check if user is instructor/admin
const canCreate = computed(() => {
  return authStore.isAdmin || authStore.user?.roles?.includes('instructor')
})

onMounted(async () => {
  if (!canCreate.value) {
    toast.add({
      severity: 'error',
      summary: t('pathway.create.accessDeniedSummary'),
      detail: t('pathway.create.accessDeniedDetail'),
      life: 5000,
    })
    router.push('/pathways')
    return
  }
  await loadAvailableLabs()
})

async function loadAvailableLabs() {
  try {
    loadingLabs.value = true
    const response = await labsApi.list()
    availableLabs.value = response.labs
  } catch (err) {
    console.error('Failed to load labs:', err)
    error.value = t('pathway.create.labsLoadFailed')
  } finally {
    loadingLabs.value = false
  }
}

// Navigation
function nextStep() {
  if (canProceed.value && currentStep.value < totalSteps) {
    currentStep.value++
  }
}

function prevStep() {
  if (currentStep.value > 1) {
    currentStep.value--
  }
}

function goToStep(step: number) {
  if (step <= currentStep.value || canProceed.value) {
    currentStep.value = step
  }
}

// Tag management
function addTag() {
  const tag = tagInput.value.trim().toLowerCase()
  if (tag && !pathwayData.value.tags?.includes(tag)) {
    pathwayData.value.tags = [...(pathwayData.value.tags || []), tag]
  }
  tagInput.value = ''
}

function removeTag(tag: string) {
  if (pathwayData.value.tags) {
    pathwayData.value.tags = pathwayData.value.tags.filter(t => t !== tag)
  }
}

// Module management
function addModule() {
  if (newModuleName.value.trim().length < 2) return

  const newModule: WizardModule = {
    id: `temp-${Date.now()}`,
    name: newModuleName.value.trim(),
    unlockType: newModuleUnlockType.value,
    labs: [],
  }
  const desc = newModuleDescription.value.trim()
  if (desc) {
    newModule.description = desc
  }
  modules.value.push(newModule)

  // Reset form
  newModuleName.value = ''
  newModuleDescription.value = ''
  newModuleUnlockType.value = 'sequential'
}

function removeModule(index: number) {
  modules.value.splice(index, 1)
}

function moveModuleUp(index: number) {
  if (index > 0) {
    const current = modules.value[index]
    const prev = modules.value[index - 1]
    if (!current || !prev) return
    modules.value[index] = prev
    modules.value[index - 1] = current
  }
}

function moveModuleDown(index: number) {
  if (index < modules.value.length - 1) {
    const current = modules.value[index]
    const next = modules.value[index + 1]
    if (!current || !next) return
    modules.value[index] = next
    modules.value[index + 1] = current
  }
}

// Lab management within modules
function addLabToModule(moduleIndex: number, lab: Lab) {
  const module = modules.value[moduleIndex]
  if (!module) return
  if (module.labs.some(l => l.labTemplateId === lab.id)) {
    return // Already added
  }

  module.labs.push({
    labTemplateId: lab.id,
    lab,
    isRequired: true,
  })
}

function removeLabFromModule(moduleIndex: number, labIndex: number) {
  const module = modules.value[moduleIndex]
  if (!module) return
  module.labs.splice(labIndex, 1)
}

function moveLabUp(moduleIndex: number, labIndex: number) {
  const module = modules.value[moduleIndex]
  if (!module) return
  const labs = module.labs
  if (labIndex > 0) {
    const current = labs[labIndex]
    const prev = labs[labIndex - 1]
    if (!current || !prev) return
    labs[labIndex] = prev
    labs[labIndex - 1] = current
  }
}

function moveLabDown(moduleIndex: number, labIndex: number) {
  const module = modules.value[moduleIndex]
  if (!module) return
  const labs = module.labs
  if (labIndex < labs.length - 1) {
    const current = labs[labIndex]
    const next = labs[labIndex + 1]
    if (!current || !next) return
    labs[labIndex] = next
    labs[labIndex + 1] = current
  }
}

// Create pathway
async function createPathway() {
  if (!isStep1Valid.value || !isStep2Valid.value) return

  try {
    creating.value = true
    error.value = null

    // Step 1: Create the pathway - build request without undefined values
    const createRequest: CreatePathwayRequest = {
      name: pathwayData.value.name,
      ...(pathwayData.value.description && { description: pathwayData.value.description }),
      ...(pathwayData.value.shortDescription && { shortDescription: pathwayData.value.shortDescription }),
      ...(pathwayData.value.difficulty && { difficulty: pathwayData.value.difficulty }),
      ...(pathwayData.value.visibility && { visibility: pathwayData.value.visibility }),
      ...(pathwayData.value.tags && pathwayData.value.tags.length > 0 && { tags: pathwayData.value.tags }),
    }
    const pathway = await pathwaysApi.create(createRequest)

    // Step 2: Create modules
    for (let i = 0; i < modules.value.length; i++) {
      const module = modules.value[i]
      if (!module) continue
      const moduleRequest: CreateModuleRequest = {
        name: module.name,
        ...(module.description && { description: module.description }),
        ...(module.unlockType && { unlockType: module.unlockType }),
      }
      const createdModule = await pathwaysApi.createModule(pathway.id, moduleRequest)

      // Step 3: Add labs to module
      for (let j = 0; j < module.labs.length; j++) {
        const lab = module.labs[j]
        if (!lab) continue
        const labRequest: AddLabToModuleRequest = {
          labTemplateId: lab.labTemplateId,
          displayOrder: j,
          isRequired: lab.isRequired,
          ...(lab.passThresholdOverride !== undefined && { passThresholdOverride: lab.passThresholdOverride }),
        }
        await pathwaysApi.addLabToModule(createdModule.id, labRequest)
      }
    }

    toast.add({
      severity: 'success',
      summary: t('pathway.create.createSuccessSummary'),
      detail: t('pathway.create.createSuccessDetail', { name: pathway.name }),
      life: 3000,
    })

    // Navigate to the pathway detail
    router.push(`/pathways/${pathway.slug || pathway.id}`)
  } catch (err) {
    console.error('Failed to create pathway:', err)
    error.value = err instanceof Error ? err.message : t('pathway.create.createFailedFallback')
  } finally {
    creating.value = false
  }
}

function cancel() {
  router.push('/pathways')
}


// Expose wizard state and transitions so tests can drive the multi-step
// flow without reaching into private refs via casts.
defineExpose({
  pathwayData,
  tagInput,
  addTag,
  currentStep,
  modules,
  newModuleName,
  addModule,
  createPathway,
  nextStep,
  prevStep,
})
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('pathway.create.title') }}</h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('pathway.create.subtitle') }}</p>
    </div>

    <PathwayWizardSteps
      :current-step="currentStep"
      :total-steps="totalSteps"
      :can-proceed="canProceed"
      @go-to-step="goToStep"
    />

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <PathwayBasicInfoStep
      v-show="currentStep === 1"
      :pathway-data="pathwayData"
      :tag-input="tagInput"
      @update:tagInput="(v) => (tagInput = v)"
      @add-tag="addTag"
      @remove-tag="removeTag"
    />

    <PathwayModulesStep
      v-show="currentStep === 2"
      :modules="modules"
      :new-module-name="newModuleName"
      :new-module-description="newModuleDescription"
      :new-module-unlock-type="newModuleUnlockType"
      :unlock-type-options="unlockTypeOptions"
      @update:newModuleName="(v) => (newModuleName = v)"
      @update:newModuleDescription="(v) => (newModuleDescription = v)"
      @update:newModuleUnlockType="(v) => (newModuleUnlockType = v)"
      @add-module="addModule"
      @remove-module="removeModule"
      @move-module-up="moveModuleUp"
      @move-module-down="moveModuleDown"
    />

    <PathwayLabsStep
      v-show="currentStep === 3"
      :modules="modules"
      :available-labs="availableLabs"
      :loading-labs="loadingLabs"
      :is-step3-valid="isStep3Valid"
      @add-lab="addLabToModule"
      @remove-lab="removeLabFromModule"
      @move-lab-up="moveLabUp"
      @move-lab-down="moveLabDown"
    />

    <PathwayReviewStep
      v-show="currentStep === 4"
      :pathway-data="pathwayData"
      :modules="modules"
    />

    <!-- Navigation -->
    <div class="flex justify-between">
      <Button
        v-if="currentStep > 1"
        :label="t('pathway.create.back')"
        icon="pi pi-arrow-left"
        severity="secondary"
        @click="prevStep"
        :disabled="creating"
      />
      <div v-else />

      <div class="flex gap-3">
        <Button
          :label="t('pathway.create.cancel')"
          severity="secondary"
          text
          @click="cancel"
          :disabled="creating"
        />
        <Button
          v-if="currentStep < totalSteps"
          :label="t('pathway.create.next')"
          icon="pi pi-arrow-right"
          iconPos="right"
          @click="nextStep"
          :disabled="!canProceed"
          :class="{ 'disabled:!bg-surface-300 disabled:!text-surface-500 dark:disabled:!bg-surface-700 dark:disabled:!text-surface-400': !canProceed }"
        />
        <Button
          v-else
          :label="t('pathway.create.submit')"
          icon="pi pi-check"
          @click="createPathway"
          :loading="creating"
          :disabled="creating"
        />
      </div>
    </div>
  </div>
</template>
