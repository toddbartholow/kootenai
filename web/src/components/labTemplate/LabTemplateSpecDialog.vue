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
  specContent: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const visibleRef = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
useFocusRestore(visibleRef)

const header = computed(() => t('labTemplate.spec.header', { name: props.templateName }))
</script>

<template>
  <Dialog
    v-model:visible="visibleRef"
    :header="header"
    :modal="true"
    :style="{ width: '800px' }"
  >
    <pre class="bg-surface-100 dark:bg-surface-800 p-4 rounded-lg overflow-auto max-h-96 text-sm font-mono">{{ specContent }}</pre>

    <template #footer>
      <Button :label="t('labTemplate.spec.close')" severity="secondary" @click="visibleRef = false" />
    </template>
  </Dialog>
</template>
