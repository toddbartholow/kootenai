<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import Tag from '@volt/Tag.vue'
import type { WizardModule, UnlockTypeOption } from './wizard-types'

const { t } = useI18n()

const props = defineProps<{
  modules: WizardModule[]
  newModuleName: string
  newModuleDescription: string
  newModuleUnlockType: UnlockTypeOption['value']
  unlockTypeOptions: UnlockTypeOption[]
}>()

const emit = defineEmits<{
  'update:newModuleName': [value: string]
  'update:newModuleDescription': [value: string]
  'update:newModuleUnlockType': [value: UnlockTypeOption['value']]
  'add-module': []
  'remove-module': [index: number]
  'move-module-up': [index: number]
  'move-module-down': [index: number]
}>()

// Use `void props` so the unused-variable lint doesn't fire on a
// component that reads from props via the template. Keeps the
// explicit prop declaration for TypeScript inference.
void props
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.modules.heading') }}</h2>
        <p class="text-surface-600 dark:text-surface-400">
          {{ t('pathway.wizard.modules.description') }}
        </p>

        <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-4 space-y-4">
          <h3 class="font-medium text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.modules.addHeading') }}</h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{ t('pathway.wizard.modules.nameLabel') }}</label>
              <InputText
                :modelValue="newModuleName"
                :placeholder="t('pathway.wizard.modules.namePlaceholder')"
                class="w-full"
                @update:modelValue="(v) => emit('update:newModuleName', (v as string) ?? '')"
              />
            </div>
            <div class="space-y-2">
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{ t('pathway.wizard.modules.unlockTypeLabel') }}</label>
              <Select
                :modelValue="newModuleUnlockType"
                :options="unlockTypeOptions"
                optionLabel="label"
                optionValue="value"
                :aria-label="t('pathway.wizard.modules.unlockTypeAria')"
                class="w-full"
                @update:modelValue="(v) => emit('update:newModuleUnlockType', v as UnlockTypeOption['value'])"
              >
                <template #option="{ option }">
                  <div>
                    <div class="font-medium">{{ option.label }}</div>
                    <div class="text-xs text-surface-500">{{ option.description }}</div>
                  </div>
                </template>
              </Select>
            </div>
            <div class="md:col-span-2 space-y-2">
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{ t('pathway.wizard.modules.descriptionLabel') }}</label>
              <InputText
                :modelValue="newModuleDescription"
                :placeholder="t('pathway.wizard.modules.descriptionPlaceholder')"
                class="w-full"
                @update:modelValue="(v) => emit('update:newModuleDescription', (v as string) ?? '')"
              />
            </div>
          </div>
          <Button
            :label="t('pathway.wizard.modules.addAction')"
            icon="pi pi-plus"
            :disabled="newModuleName.trim().length < 2"
            @click="emit('add-module')"
          />
        </div>

        <div v-if="modules.length > 0" class="space-y-3">
          <div
            v-for="(module, index) in modules"
            :key="module.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-4"
          >
            <div class="flex items-start justify-between">
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm text-surface-500">{{ index + 1 }}.</span>
                  <h4 class="font-medium text-surface-900 dark:text-surface-100">{{ module.name }}</h4>
                  <Tag :value="module.unlockType" severity="secondary" class="text-xs" />
                </div>
                <p v-if="module.description" class="text-sm text-surface-500 mt-1">{{ module.description }}</p>
                <p class="text-sm text-surface-500 mt-1">{{ t('pathway.wizard.modules.labsCount', { count: module.labs.length }) }}</p>
              </div>
              <div class="flex items-center gap-1">
                <Button
                  icon="pi pi-chevron-up"
                  severity="secondary"
                  text
                  size="small"
                  :disabled="index === 0"
                  @click="emit('move-module-up', index)"
                />
                <Button
                  icon="pi pi-chevron-down"
                  severity="secondary"
                  text
                  size="small"
                  :disabled="index === modules.length - 1"
                  @click="emit('move-module-down', index)"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  @click="emit('remove-module', index)"
                />
              </div>
            </div>
          </div>
        </div>

        <Message v-else severity="info">
          {{ t('pathway.wizard.modules.emptyMessage') }}
        </Message>
      </div>
    </template>
  </Card>
</template>
