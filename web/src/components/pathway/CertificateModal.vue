<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Certificate } from '@/api'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  certificate: Certificate | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { formatLongDate } = useFormatters()

const formattedDate = computed(() =>
  props.certificate?.completedAt ? formatLongDate(props.certificate.completedAt, '') : '',
)

const formattedIssuedDate = computed(() =>
  props.certificate?.issuedAt ? formatLongDate(props.certificate.issuedAt, '') : '',
)

const scorePercentage = computed(() => {
  if (!props.certificate) return 0
  return Math.round(props.certificate.percentage)
})

function handlePrint() {
  window.print()
}

function copyVerificationUrl() {
  if (props.certificate?.verificationUrl) {
    navigator.clipboard.writeText(props.certificate.verificationUrl)
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :closable="true"
    :draggable="false"
    class="certificate-modal"
    :style="{ width: '500px' }"
    @update:visible="!$event && emit('close')"
  >
    <template #header>
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-full bg-gradient-to-br from-amber-400 to-yellow-500 flex items-center justify-center">
          <i class="pi pi-trophy text-white text-xl" />
        </div>
        <div>
          <h2 class="text-xl font-bold text-surface-900 dark:text-surface-100">
            {{ t('pathway.certificateModal.headerTitle') }}
          </h2>
        </div>
      </div>
    </template>

    <div v-if="certificate" class="certificate-content">
      <!-- Certificate Preview -->
      <div class="certificate-preview p-6 rounded-xl border-2 border-amber-300 dark:border-amber-600 bg-gradient-to-br from-amber-50 to-yellow-50 dark:from-amber-900/20 dark:to-yellow-900/20 text-center mb-6">
        <!-- Decorative Header -->
        <div class="mb-4">
          <i class="pi pi-verified text-5xl text-amber-500" />
        </div>

        <!-- Title -->
        <h3 class="text-lg font-semibold text-surface-700 dark:text-surface-300 mb-1">
          {{ t('pathway.certificateModal.certifies') }}
        </h3>

        <!-- User Name -->
        <p class="text-2xl font-bold text-surface-900 dark:text-surface-100 mb-4">
          {{ certificate.userName }}
        </p>

        <!-- Achievement Text -->
        <p class="text-surface-600 dark:text-surface-400 mb-2">
          {{ t('pathway.certificateModal.achievement') }}
        </p>

        <!-- Pathway Name -->
        <p class="text-xl font-bold text-amber-600 dark:text-amber-400 mb-4">
          {{ certificate.pathwayName }}
        </p>

        <!-- Score -->
        <div class="flex justify-center gap-6 mb-4">
          <div class="text-center">
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
              {{ certificate.earnedPoints }}
            </p>
            <p class="text-xs text-surface-500">{{ t('pathway.certificateModal.pointsEarnedLabel') }}</p>
          </div>
          <div class="text-center">
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
              {{ scorePercentage }}%
            </p>
            <p class="text-xs text-surface-500">{{ t('pathway.certificateModal.scoreLabel') }}</p>
          </div>
        </div>

        <!-- Date -->
        <p class="text-sm text-surface-500">
          {{ t('pathway.certificateModal.completedOn', { when: formattedDate }) }}
        </p>
      </div>

      <!-- Verification Info -->
      <div class="space-y-3">
        <div class="flex items-center justify-between p-3 rounded-lg bg-surface-100 dark:bg-surface-800">
          <div>
            <p class="text-xs text-surface-500 mb-1">{{ t('pathway.certificateModal.verificationCode') }}</p>
            <p class="font-mono font-semibold text-surface-900 dark:text-surface-100">
              {{ certificate.verificationCode }}
            </p>
          </div>
          <Button
            icon="pi pi-copy"
            severity="secondary"
            text
            size="small"
            :aria-label="t('pathway.certificateModal.copyAria')"
            @click="copyVerificationUrl"
          />
        </div>

        <p class="text-xs text-surface-500 text-center">
          {{ t('pathway.certificateModal.issuedOn', { when: formattedIssuedDate }) }}
        </p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          :label="t('pathway.certificateModal.close')"
          severity="secondary"
          outlined
          @click="emit('close')"
        />
        <Button
          :label="t('pathway.certificateModal.print')"
          icon="pi pi-print"
          @click="handlePrint"
        />
      </div>
    </template>
  </Dialog>
</template>

<style scoped>
/* Print styles */
@media print {
  .certificate-modal :deep(.p-dialog-header),
  .certificate-modal :deep(.p-dialog-footer) {
    display: none !important;
  }

  .certificate-preview {
    border: 3px solid #d97706 !important;
    padding: 2rem !important;
    margin: 0 !important;
  }

  .certificate-content > *:not(.certificate-preview) {
    display: none !important;
  }
}
</style>
