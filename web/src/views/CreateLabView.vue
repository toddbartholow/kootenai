<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { labsApi, type CreateLabRequest } from '@/api'
import { useAuthStore } from '../stores/auth'
import { useLabForm } from '@/composables/useLabForm'
import { WizardProgress } from '@/components/common'
import {
  LabStepBasicInfo,
  LabStepVMs,
  LabStepObjectives,
  LabStepQuestions,
  LabStepReview,
} from '@/components/labForm'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import { useToast } from 'primevue/usetoast'
import { ref } from 'vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()

const stepLabels = computed(() => [
  t('createLab.steps.basicInfo'),
  t('createLab.steps.vms'),
  t('createLab.steps.objectives'),
  t('createLab.steps.questions'),
  t('createLab.steps.review'),
])

const {
  labData,
  vms,
  objectives,
  questions,
  currentStep,
  totalSteps,
  tagInput,
  newVMName,
  newVMTemplate,
  newObjectiveDescription,
  newObjectivePoints,
  newQuestionDescription,
  newQuestionPoints,
  newQuestionType,
  visibilityOptions,
  isStep1Valid,
  isStep2Valid,
  isStep3Valid,
  canProceed,
  totalObjectivePoints,
  totalQuestionPoints,
  totalPoints,
  totalVMs,
  nextStep,
  prevStep,
  goToStep,
  addTag,
  removeTag,
  addVM,
  removeVM,
  addSnapshot,
  removeSnapshot,
  setDefaultSnapshot,
  addObjective,
  removeObjective,
  moveObjectiveUp,
  moveObjectiveDown,
  addTrigger,
  removeTrigger,
  addQuestion,
  removeQuestion,
  moveQuestionUp,
  moveQuestionDown,
  duplicateQuestion,
  handleQuestionTypeChange,
  addOption,
  removeOption,
  getAvailableDependencies,
  buildLabSpec,
} = useLabForm()

// Submission state
const creating = ref(false)
const error = ref<string | null>(null)

const canCreate = computed(() => {
  return authStore.isAdmin || authStore.isInstructor
})

onMounted(async () => {
  if (!canCreate.value) {
    toast.add({
      severity: 'error',
      summary: t('createLab.accessDenied.summary'),
      detail: t('createLab.accessDenied.detail'),
      life: 5000,
    })
    router.push('/labs')
    return
  }
})

async function createLab() {
  if (!isStep1Valid.value || !isStep2Valid.value || !isStep3Valid.value) return

  try {
    creating.value = true
    error.value = null

    const spec = buildLabSpec()
    const request: CreateLabRequest = {
      name: labData.value.name,
      description: labData.value.description,
      version: labData.value.version,
      platform: labData.value.platform,
      durationMinutes: labData.value.durationMinutes,
      difficulty: labData.value.difficulty,
      tags: labData.value.tags,
      maxPoints: totalPoints.value,
      passThreshold: labData.value.passThreshold,
      spec: JSON.stringify(spec, null, 2),
      isActive: labData.value.isActive,
      visibility: labData.value.visibility,
    }

    const result = await labsApi.create(request)

    toast.add({
      severity: 'success',
      summary: t('createLab.toasts.createdSummary'),
      detail: t('createLab.toasts.createdDetail', { name: result.name }),
      life: 3000,
    })

    router.push('/labs')
  } catch (err) {
    console.error('Failed to create lab:', err)
    error.value = err instanceof Error ? err.message : t('createLab.errors.createFailed')
  } finally {
    creating.value = false
  }
}

function cancel() {
  router.push('/labs')
}
</script>

<template>
  <div class="max-w-4xl mx-auto space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
        {{ t('createLab.title') }}
      </h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('createLab.subtitle') }}</p>
    </div>

    <!-- Progress Steps -->
    <WizardProgress
      :steps="stepLabels"
      :current-step="currentStep"
      :can-proceed="canProceed"
      @step-click="goToStep"
    />

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Step 1: Basic Info -->
    <LabStepBasicInfo
      v-show="currentStep === 1"
      :lab-data="labData"
      :tag-input="tagInput"
      :visibility-options="visibilityOptions"
      :is-active-label="t('createLab.basic.publishLabel')"
      @update:tag-input="tagInput = $event"
      @add-tag="addTag"
      @remove-tag="removeTag"
    />

    <!-- Step 2: VMs -->
    <LabStepVMs
      v-show="currentStep === 2"
      :vms="vms"
      :new-v-m-name="newVMName"
      :new-v-m-template="newVMTemplate"
      @update:new-v-m-name="newVMName = $event"
      @update:new-v-m-template="newVMTemplate = $event"
      @add-v-m="addVM"
      @remove-v-m="removeVM"
      @add-snapshot="addSnapshot"
      @remove-snapshot="removeSnapshot"
      @set-default-snapshot="setDefaultSnapshot"
    />

    <!-- Step 3: Objectives -->
    <LabStepObjectives
      v-show="currentStep === 3"
      :objectives="objectives"
      :vms="vms"
      :new-objective-description="newObjectiveDescription"
      :new-objective-points="newObjectivePoints"
      :total-objective-points="totalObjectivePoints"
      @update:new-objective-description="newObjectiveDescription = $event"
      @update:new-objective-points="newObjectivePoints = $event"
      @add-objective="addObjective"
      @remove-objective="removeObjective"
      @move-objective-up="moveObjectiveUp"
      @move-objective-down="moveObjectiveDown"
      @add-trigger="addTrigger"
      @remove-trigger="removeTrigger"
    />

    <!-- Step 4: Questions -->
    <LabStepQuestions
      v-show="currentStep === 4"
      :questions="questions"
      :objectives="objectives"
      :new-question-description="newQuestionDescription"
      :new-question-points="newQuestionPoints"
      :new-question-type="newQuestionType"
      :total-question-points="totalQuestionPoints"
      :total-points="totalPoints"
      :get-available-dependencies="getAvailableDependencies"
      @update:new-question-description="newQuestionDescription = $event"
      @update:new-question-points="newQuestionPoints = $event"
      @update:new-question-type="newQuestionType = $event"
      @add-question="addQuestion"
      @remove-question="removeQuestion"
      @move-question-up="moveQuestionUp"
      @move-question-down="moveQuestionDown"
      @duplicate-question="duplicateQuestion"
      @handle-question-type-change="handleQuestionTypeChange"
      @add-option="addOption"
      @remove-option="removeOption"
    />

    <!-- Step 5: Review -->
    <LabStepReview
      v-show="currentStep === 5"
      :lab-data="labData"
      :vms="vms"
      :objectives="objectives"
      :questions="questions"
      :total-v-ms="totalVMs"
      :total-objective-points="totalObjectivePoints"
      :total-question-points="totalQuestionPoints"
      :total-points="totalPoints"
      :show-status-notice="true"
    />

    <!-- Navigation -->
    <div class="flex justify-between">
      <Button
        v-if="currentStep > 1"
        :label="t('createLab.nav.back')"
        icon="pi pi-arrow-left"
        severity="secondary"
        @click="prevStep"
        :disabled="creating"
      />
      <div v-else />

      <div class="flex gap-3">
        <Button
          :label="t('createLab.nav.cancel')"
          severity="secondary"
          text
          @click="cancel"
          :disabled="creating"
        />
        <Button
          v-if="currentStep < totalSteps"
          :label="t('createLab.nav.next')"
          icon="pi pi-arrow-right"
          iconPos="right"
          @click="nextStep"
          :disabled="!canProceed"
        />
        <Button
          v-else
          :label="t('createLab.nav.createLab')"
          icon="pi pi-check"
          @click="createLab"
          :loading="creating"
          :disabled="creating"
        />
      </div>
    </div>
  </div>
</template>
