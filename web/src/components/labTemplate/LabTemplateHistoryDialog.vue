<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useFocusRestore } from '@/composables'
import { formatDate } from '@/utils/format'

const { t } = useI18n()

export interface LabTemplateVersion {
  id: string
  versionNumber: number
  name: string
  version: string
  changeSummary: string
  createdAt: string
}

const props = defineProps<{
  visible: boolean
  templateName: string
  versions: LabTemplateVersion[]
  loading: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  restore: [versionNumber: number]
}>()

const visibleRef = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
useFocusRestore(visibleRef)

const header = computed(() => t('labTemplate.history.header', { name: props.templateName }))
</script>

<template>
  <Dialog
    v-model:visible="visibleRef"
    :header="header"
    :modal="true"
    :style="{ width: '700px' }"
  >
    <div v-if="loading" class="flex justify-center py-8">
      <ProgressSpinner />
    </div>
    <div v-else-if="versions.length === 0" class="text-center py-8 text-surface-500">
      {{ t('labTemplate.history.empty') }}
    </div>
    <div v-else class="space-y-3 max-h-96 overflow-auto">
      <div
        v-for="v in versions"
        :key="v.id"
        class="flex items-center justify-between p-3 rounded-lg border border-surface-200 dark:border-surface-700"
      >
        <div>
          <div class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('labTemplate.history.versionLabel', { number: v.versionNumber }) }}
            <span class="text-sm text-surface-500 ml-2">{{ v.version }}</span>
          </div>
          <div class="text-sm text-surface-500">
            {{ formatDate(v.createdAt) }}
            <span v-if="v.changeSummary" class="ml-2 italic">{{ v.changeSummary }}</span>
          </div>
        </div>
        <Button
          :label="t('labTemplate.history.restore')"
          icon="pi pi-replay"
          severity="secondary"
          size="small"
          :loading="saving"
          @click="emit('restore', v.versionNumber)"
        />
      </div>
    </div>
    <template #footer>
      <Button :label="t('labTemplate.history.close')" severity="secondary" @click="visibleRef = false" />
    </template>
  </Dialog>
</template>
