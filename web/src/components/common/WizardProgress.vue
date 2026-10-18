<script setup lang="ts">
/**
 * Reusable multi-step wizard progress indicator
 * Shows numbered steps with labels, completion status, and connector lines
 */
import { useI18n } from 'vue-i18n'

interface Props {
  /** Array of step labels (e.g., ['Basic Info', 'VMs', 'Objectives', 'Review']) */
  steps: string[]
  /** Current active step (1-indexed) */
  currentStep: number
  /** Whether the user can proceed to the next step */
  canProceed?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  canProceed: true,
})

const emit = defineEmits<{
  stepClick: [step: number]
}>()

const { t } = useI18n()

function handleStepClick(step: number) {
  // Allow clicking on completed steps or current step, or next step if canProceed
  if (step <= props.currentStep || (step === props.currentStep + 1 && props.canProceed)) {
    emit('stepClick', step)
  }
}

function isStepClickable(step: number): boolean {
  return step <= props.currentStep || (step === props.currentStep + 1 && props.canProceed)
}

function getStepAriaLabel(step: number, label: string): string {
  const statusKey = props.currentStep > step
    ? 'completed'
    : props.currentStep === step
      ? 'current'
      : 'upcoming'
  return t('common.wizardProgress.stepAria', {
    step,
    label,
    status: t(`common.wizardProgress.stepStatus.${statusKey}`),
  })
}
</script>

<template>
  <nav :aria-label="t('common.wizardProgress.aria')" class="flex items-center justify-between">
    <template v-for="(label, index) in steps" :key="index">
      <button
        type="button"
        @click="handleStepClick(index + 1)"
        :disabled="!isStepClickable(index + 1)"
        :aria-label="getStepAriaLabel(index + 1, label)"
        :aria-current="currentStep === index + 1 ? 'step' : undefined"
        class="flex items-center gap-2 group"
        :class="{
          'cursor-not-allowed opacity-50': !isStepClickable(index + 1),
          'cursor-pointer': isStepClickable(index + 1)
        }"
      >
        <!-- Step indicator circle -->
        <div
          class="w-10 h-10 rounded-full flex items-center justify-center text-sm font-medium transition-colors"
          :class="{
            'bg-primary-500 text-white': currentStep === index + 1,
            'bg-green-500 text-white': currentStep > index + 1,
            'bg-surface-200 dark:bg-surface-700 text-surface-600 dark:text-surface-400': currentStep < index + 1,
          }"
        >
          <i v-if="currentStep > index + 1" class="pi pi-check" aria-hidden="true" />
          <span v-else>{{ index + 1 }}</span>
        </div>

        <!-- Step label (hidden on small screens) -->
        <span
          class="text-sm font-medium hidden sm:block"
          :class="{
            'text-primary-600 dark:text-primary-400': currentStep === index + 1,
            'text-green-600 dark:text-green-400': currentStep > index + 1,
            'text-surface-500': currentStep < index + 1,
          }"
        >
          {{ label }}
        </span>
      </button>

      <!-- Connector line between steps -->
      <div
        v-if="index < steps.length - 1"
        class="flex-1 h-0.5 mx-2"
        :class="{
          'bg-green-500': currentStep > index + 1,
          'bg-surface-200 dark:bg-surface-700': currentStep <= index + 1,
        }"
        role="presentation"
      />
    </template>
  </nav>
</template>
