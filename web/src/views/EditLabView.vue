<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { labsApi, type CreateLabRequest, type LabWithSpec } from '@/api'
import { useAuthStore } from '../stores/auth'
import {
  useLabForm,
  type VMConfig,
  type ObjectiveConfig,
  type QuestionConfig,
} from '@/composables/useLabForm'
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
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useToast } from 'primevue/usetoast'

const { t } = useI18n()

const stepLabels = computed(() => [
  t('createLab.steps.basicInfo'),
  t('createLab.steps.vms'),
  t('createLab.steps.objectives'),
  t('createLab.steps.questions'),
  t('createLab.steps.review'),
])

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const labId = computed(() => route.params['labId'] as string)

const loading = ref(true)
const originalLab = ref<LabWithSpec | null>(null)

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

const saving = ref(false)
const error = ref<string | null>(null)

const canEdit = computed(() => {
  if (!originalLab.value) return false
  if (authStore.isAdmin) return true
  if (authStore.user?.roles?.includes('instructor')) return true
  return originalLab.value.createdBy === authStore.user?.id
})

onMounted(async () => {
  await loadLab()
})

async function loadLab() {
  try {
    loading.value = true
    error.value = null

    const lab = await labsApi.getWithSpec(labId.value)
    originalLab.value = lab

    if (!canEdit.value) {
      toast.add({
        severity: 'error',
        summary: t('editLab.accessDenied.summary'),
        detail: t('editLab.accessDenied.detail'),
        life: 5000,
      })
      router.push('/labs')
      return
    }

    labData.value = {
      name: lab.name,
      description: lab.description || '',
      difficulty: lab.difficulty || 'beginner',
      durationMinutes: lab.durationMinutes || 60,
      platform: lab.platform || 'proxmox',
      version: lab.version || '1.0.0',
      passThreshold: lab.passThreshold || 70,
      // Must come from the lab. Hardcoding 'private' here meant every save
      // through this wizard demoted the lab, so a global lab silently vanished
      // from every student's list.
      visibility: lab.visibility || 'private',
      isActive: lab.isActive ?? true,
      tags: lab.tags || [],
    }

    if (lab.spec) {
      try {
        const spec = typeof lab.spec === 'string' ? JSON.parse(lab.spec) : lab.spec
        parseSpec(spec)
      } catch (e) {
        console.error('Failed to parse lab spec:', e)
      }
    }
  } catch (err) {
    console.error('Failed to load lab:', err)
    error.value = t('editLab.loadFailed')
  } finally {
    loading.value = false
  }
}

function parseSpec(spec: Record<string, unknown>) {
  const specData = spec['spec'] as Record<string, unknown> | undefined

  const specVms = specData?.['vms']
  if (specVms && Array.isArray(specVms)) {
    vms.value = specVms.map((vm: Record<string, unknown>, idx: number): VMConfig => {
      const resources = vm['resources'] as Record<string, number> | undefined
      const vmSnapshots = vm['snapshots']
      return {
        id: `vm-${idx}`,
        name: (vm['name'] as string) || '',
        template: (vm['template'] as string) || 'ubuntu-22.04-server',
        cpu: resources?.['cpu'] || 2,
        memory: resources?.['memory'] || 2048,
        disk: resources?.['disk'] || 16,
        wazuhAgent: (vm['wazuhAgent'] as boolean) ?? true,
        snapshots: Array.isArray(vmSnapshots)
          ? vmSnapshots.map((s: Record<string, unknown>) => ({
              name: (s['name'] as string) || '',
              description: (s['description'] as string) || '',
              isDefault: (s['default'] as boolean) ?? false,
            }))
          : [{ name: 'initial', description: 'Initial state', isDefault: true }],
      }
    })
  }

  const specObjectives = specData?.['objectives']
  if (specObjectives && Array.isArray(specObjectives)) {
    objectives.value = specObjectives.map(
      (obj: Record<string, unknown>, idx: number): ObjectiveConfig => {
        const objTriggers = obj['triggers']
        return {
          id: (obj['id'] as string) || `obj-${idx}`,
          description: (obj['description'] as string) || '',
          points: (obj['points'] as number) || 20,
          hint: (obj['hint'] as string) || '',
          order: (obj['order'] as number) || idx + 1,
          dependsOn: (obj['depends_on'] as string[]) || [],
          triggers: Array.isArray(objTriggers)
            ? objTriggers.map((t: Record<string, unknown>) => {
                const match = t['match'] as Record<string, string> | undefined
                return {
                  type: (t['type'] as string) || 'file_exists',
                  target: (t['target'] as string) || '',
                  matchPath: match?.['path'] || '',
                  matchContains: match?.['contains'] || '',
                  matchPattern: match?.['pattern'] || match?.['name'] || '',
                }
              })
            : [],
        }
      },
    )
  }

  const specQuestions = specData?.['questions']
  if (specQuestions && Array.isArray(specQuestions)) {
    questions.value = specQuestions.map(
      (q: Record<string, unknown>, idx: number): QuestionConfig => {
        const validation = q['validation'] as Record<string, unknown> | undefined
        const options = q['options'] as Array<Record<string, unknown>> | undefined
        return {
          id: (q['id'] as string) || `q-${idx}`,
          type: (q['type'] as 'text' | 'multiple_choice') || 'text',
          description: (q['description'] as string) || '',
          points: (q['points'] as number) || 10,
          hint: (q['hint'] as string) || '',
          order: (q['order'] as number) || idx + 1,
          dependsOn: (q['depends_on'] as string[]) || [],
          validation: {
            type: (validation?.['type'] as 'exact' | 'regex') || 'exact',
            answer: (validation?.['answer'] as string) || '',
            pattern: (validation?.['pattern'] as string) || '',
            caseSensitive: (validation?.['case_sensitive'] as boolean) || false,
          },
          options:
            options?.map(opt => ({
              id: (opt['id'] as string) || '',
              text: (opt['text'] as string) || '',
              correct: (opt['correct'] as boolean) || false,
            })) || [],
          multiSelect: (q['multi_select'] as boolean) || false,
        }
      },
    )
  }
}

async function saveLab() {
  if (!isStep1Valid.value || !isStep2Valid.value || !isStep3Valid.value) return

  try {
    saving.value = true
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

    await labsApi.update(labId.value, request)

    toast.add({
      severity: 'success',
      summary: t('editLab.toast.updatedSummary'),
      detail: t('editLab.toast.updatedDetail', { name: labData.value.name }),
      life: 3000,
    })

    router.push('/labs')
  } catch (err) {
    console.error('Failed to update lab:', err)
    error.value = err instanceof Error ? err.message : t('editLab.saveFailed')
  } finally {
    saving.value = false
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
        {{ t('editLab.title') }}
      </h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('editLab.subtitle') }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner :aria-label="t('editLab.loadingAria')" />
    </div>

    <!-- Error -->
    <Message v-else-if="error && !originalLab" severity="error">{{ error }}</Message>

    <template v-else>
      <!-- Progress Steps -->
      <WizardProgress
        :steps="stepLabels"
        :current-step="currentStep"
        :can-proceed="canProceed"
        @step-click="goToStep"
      />

      <!-- Error message -->
      <Message v-if="error" severity="error" :closable="true" @close="error = null">{{
        error
      }}</Message>

      <!-- Step 1: Basic Info -->
      <LabStepBasicInfo
        v-show="currentStep === 1"
        :lab-data="labData"
        :tag-input="tagInput"
        :visibility-options="visibilityOptions"
        :is-active-label="t('editLab.isActiveLabel')"
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
      />

      <!-- Navigation -->
      <div class="flex justify-between">
        <Button
          v-if="currentStep > 1"
          :label="t('createLab.nav.back')"
          icon="pi pi-arrow-left"
          severity="secondary"
          @click="prevStep"
          :disabled="saving"
        />
        <div v-else />

        <div class="flex gap-3">
          <Button
            :label="t('createLab.nav.cancel')"
            severity="secondary"
            text
            @click="cancel"
            :disabled="saving"
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
            :label="t('editLab.save')"
            icon="pi pi-check"
            @click="saveLab"
            :loading="saving"
            :disabled="saving"
          />
        </div>
      </div>
    </template>
  </div>
</template>
