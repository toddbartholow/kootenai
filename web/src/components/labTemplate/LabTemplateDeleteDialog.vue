<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import { useFocusRestore } from '@/composables'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  templateName: string
  saving: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  confirm: []
}>()

const visibleRef = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
useFocusRestore(visibleRef)
</script>

<template>
  <Dialog
    v-model:visible="visibleRef"
    :header="t('labTemplate.delete.header')"
    :modal="true"
    :style="{ width: '450px' }"
    :closable="!saving"
  >
    <div class="flex items-start gap-4">
      <i class="pi pi-exclamation-triangle text-3xl text-red-500" />
      <div>
        <p class="font-medium">{{ t('labTemplate.delete.confirmQuestion') }}</p>
        <p class="text-sm text-surface-500 mt-2">
          {{ t('labTemplate.delete.confirmDetail', { name: templateName }) }}
        </p>
      </div>
    </div>

    <template #footer>
      <Button :label="t('labTemplate.delete.cancel')" severity="secondary" :disabled="saving" @click="visibleRef = false" />
      <Button
        :label="t('labTemplate.delete.delete')"
        severity="danger"
        icon="pi pi-trash"
        :loading="saving"
        @click="emit('confirm')"
      />
    </template>
  </Dialog>
</template>
