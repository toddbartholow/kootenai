<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import Textarea from 'primevue/textarea'
import { useFocusRestore } from '@/composables'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  import: [yaml: string]
}>()

const visibleRef = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
useFocusRestore(visibleRef)

const yaml = ref('')

// Clear the paste area when the dialog closes so the next open starts
// blank. Keeps ergonomics identical to the inlined version, which
// reset `importYaml` via a dedicated helper in the parent.
watch(visibleRef, (open) => {
  if (!open) yaml.value = ''
})

function handleImport(): void {
  emit('import', yaml.value)
}
</script>

<template>
  <Dialog
    v-model:visible="visibleRef"
    :header="t('labTemplate.import.header')"
    :modal="true"
    :style="{ width: '800px' }"
    :closable="!saving"
  >
    <div class="space-y-4">
      <p class="text-sm text-surface-600 dark:text-surface-400">
        {{ t('labTemplate.import.description') }}
      </p>
      <Textarea
        v-model="yaml"
        rows="20"
        class="w-full font-mono text-sm"
        :placeholder="t('labTemplate.import.yamlPlaceholder')"
        :aria-label="t('labTemplate.import.yamlAria')"
      />
    </div>
    <template #footer>
      <Button :label="t('labTemplate.import.cancel')" severity="secondary" :disabled="saving" @click="visibleRef = false" />
      <Button
        :label="t('labTemplate.import.import')"
        icon="pi pi-upload"
        :loading="saving"
        :disabled="!yaml.trim()"
        @click="handleImport"
      />
    </template>
  </Dialog>
</template>
