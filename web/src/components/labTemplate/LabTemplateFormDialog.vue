<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import InputSwitch from 'primevue/inputswitch'
import InputNumber from 'primevue/inputnumber'
import { useFocusRestore } from '@/composables'
import {
  platformOptions,
  difficultyOptions,
  visibilityOptions,
} from '@/constants/formOptions'

const { t } = useI18n()

export interface LabTemplateFormData {
  name: string
  description: string
  version: string
  platform: string
  durationMinutes: number
  difficulty: string
  category: string
  tags: string[]
  maxPoints: number
  passThreshold: number
  spec: string
  isActive: boolean
  visibility: string
}

const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  formData: LabTemplateFormData
  saving: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'update:formData': [value: LabTemplateFormData]
  save: []
}>()

// Track open/close for focus-restore.
const visibleRef = computed({
  get: () => props.visible,
  set: (value: boolean) => emit('update:visible', value),
})
useFocusRestore(visibleRef)

const header = computed(() =>
  props.mode === 'create' ? t('labTemplate.form.createHeader') : t('labTemplate.form.editHeader'),
)
const submitLabel = computed(() =>
  props.mode === 'create' ? t('labTemplate.form.submitCreate') : t('labTemplate.form.submitEdit'),
)
const specLabel = computed(() =>
  props.mode === 'create' ? t('labTemplate.form.specLabelCreate') : t('labTemplate.form.specLabelEdit'),
)
const specPlaceholder = computed(() =>
  props.mode === 'create' ? t('labTemplate.form.specPlaceholder') : undefined,
)

// Write helpers — keep the existing `formData.x =` ergonomics for the
// template without requiring callers to pass a separate update handler
// per field.
function update<K extends keyof LabTemplateFormData>(key: K, value: LabTemplateFormData[K]): void {
  emit('update:formData', { ...props.formData, [key]: value })
}
</script>

<template>
  <Dialog
    v-model:visible="visibleRef"
    :header="header"
    :modal="true"
    :style="{ width: '800px' }"
    :closable="!saving"
  >
    <div class="space-y-4">
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.nameLabel') }}</label>
          <InputText
            :modelValue="formData.name"
            class="w-full"
            :placeholder="t('labTemplate.form.fields.namePlaceholder')"
            :aria-label="t('labTemplate.form.fields.nameAria')"
            @update:modelValue="(v) => update('name', v as string)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.versionLabel') }}</label>
          <InputText
            :modelValue="formData.version"
            class="w-full"
            :placeholder="t('labTemplate.form.fields.versionPlaceholder')"
            :aria-label="t('labTemplate.form.fields.versionAria')"
            @update:modelValue="(v) => update('version', v as string)"
          />
        </div>
      </div>

      <div>
        <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.descriptionLabel') }}</label>
        <Textarea
          :modelValue="formData.description"
          rows="2"
          class="w-full"
          :placeholder="t('labTemplate.form.fields.descriptionPlaceholder')"
          :aria-label="t('labTemplate.form.fields.descriptionAria')"
          @update:modelValue="(v) => update('description', (v as string) ?? '')"
        />
      </div>

      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.platformLabel') }}</label>
          <Select
            :modelValue="formData.platform"
            :options="platformOptions"
            optionLabel="label"
            optionValue="value"
            class="w-full"
            :aria-label="t('labTemplate.form.fields.platformAria')"
            @update:modelValue="(v) => update('platform', v as string)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.difficultyLabel') }}</label>
          <Select
            :modelValue="formData.difficulty"
            :options="difficultyOptions"
            optionLabel="label"
            optionValue="value"
            class="w-full"
            :aria-label="t('labTemplate.form.fields.difficultyAria')"
            @update:modelValue="(v) => update('difficulty', v as string)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.visibilityLabel') }}</label>
          <Select
            :modelValue="formData.visibility"
            :options="visibilityOptions"
            optionLabel="label"
            optionValue="value"
            class="w-full"
            :aria-label="t('labTemplate.form.fields.visibilityAria')"
            @update:modelValue="(v) => update('visibility', v as string)"
          />
        </div>
      </div>

      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.durationLabel') }}</label>
          <InputNumber
            :modelValue="formData.durationMinutes"
            :min="1"
            :max="480"
            class="w-full"
            @update:modelValue="(v) => update('durationMinutes', (v as number) ?? 0)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.maxPointsLabel') }}</label>
          <InputNumber
            :modelValue="formData.maxPoints"
            :min="0"
            class="w-full"
            @update:modelValue="(v) => update('maxPoints', (v as number) ?? 0)"
          />
        </div>
        <div>
          <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.passThresholdLabel') }}</label>
          <InputNumber
            :modelValue="formData.passThreshold"
            :min="0"
            :max="100"
            class="w-full"
            @update:modelValue="(v) => update('passThreshold', (v as number) ?? 0)"
          />
        </div>
      </div>

      <div class="flex items-center gap-2">
        <InputSwitch
          :modelValue="formData.isActive"
          @update:modelValue="(v) => update('isActive', Boolean(v))"
        />
        <label class="text-sm text-surface-700 dark:text-surface-300">{{ t('labTemplate.form.fields.activeLabel') }}</label>
      </div>

      <div>
        <label class="block text-sm font-medium mb-1 text-surface-700 dark:text-surface-300">{{ specLabel }}</label>
        <Textarea
          :modelValue="formData.spec"
          rows="15"
          class="w-full font-mono text-sm"
          :placeholder="specPlaceholder"
          :aria-label="specLabel"
          @update:modelValue="(v) => update('spec', (v as string) ?? '')"
        />
      </div>
    </div>

    <template #footer>
      <Button
        :label="t('labTemplate.form.cancel')"
        severity="secondary"
        :disabled="saving"
        @click="visibleRef = false"
      />
      <Button
        :label="submitLabel"
        icon="pi pi-check"
        :loading="saving"
        @click="emit('save')"
      />
    </template>
  </Dialog>
</template>
