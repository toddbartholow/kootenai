<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  currentStep: number
  totalSteps: number
  canProceed: boolean
}>()

const emit = defineEmits<{
  'go-to-step': [step: number]
}>()

const STEP_KEYS = [
  'pathway.wizard.steps.basicInfo',
  'pathway.wizard.steps.modules',
  'pathway.wizard.steps.labs',
  'pathway.wizard.steps.review',
]

function labelFor(step: number): string {
  const key = STEP_KEYS[step - 1]
  return key ? t(key) : ''
}
</script>

<template>
  <div class="flex items-center justify-between">
    <template v-for="step in totalSteps" :key="step">
      <button
        :disabled="step > currentStep && !canProceed"
        class="flex items-center gap-2 group"
        :class="{ 'cursor-not-allowed opacity-50': step > currentStep && !canProceed }"
        @click="emit('go-to-step', step)"
      >
        <div
          class="w-10 h-10 rounded-full flex items-center justify-center text-sm font-medium transition-colors"
          :class="{
            'bg-primary-500 text-white': currentStep === step,
            'bg-green-500 text-white': currentStep > step,
            'bg-surface-200 dark:bg-surface-700 text-surface-600 dark:text-surface-400': currentStep < step,
          }"
        >
          <i v-if="currentStep > step" class="pi pi-check" />
          <span v-else>{{ step }}</span>
        </div>
        <span
          class="text-sm font-medium hidden sm:block"
          :class="{
            'text-primary-600 dark:text-primary-400': currentStep === step,
            'text-green-600 dark:text-green-400': currentStep > step,
            'text-surface-500': currentStep < step,
          }"
        >
          {{ labelFor(step) }}
        </span>
      </button>
      <div
        v-if="step < totalSteps"
        class="flex-1 h-0.5 mx-2"
        :class="{
          'bg-green-500': currentStep > step,
          'bg-surface-200 dark:bg-surface-700': currentStep <= step,
        }"
      />
    </template>
  </div>
</template>
