<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Tag from '@volt/Tag.vue'
import { difficultyOptions, visibilityOptions } from '@/constants/formOptions'
import type { WizardPathwayData } from './wizard-types'

const { t } = useI18n()

// Reactive objects are passed by reference; children mutate fields in
// place. The parent owns the ref and observes changes without any
// additional emit boilerplate.
defineProps<{
  pathwayData: WizardPathwayData
  tagInput: string
}>()

const emit = defineEmits<{
  'update:tagInput': [value: string]
  'add-tag': []
  'remove-tag': [tag: string]
}>()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.basicInfo.heading') }}</h2>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div class="md:col-span-2 space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.nameLabel') }} <span class="text-red-500">*</span>
            </label>
            <InputText
              v-model="pathwayData.name"
              :placeholder="t('pathway.wizard.basicInfo.namePlaceholder')"
              class="w-full"
            />
          </div>

          <div class="md:col-span-2 space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.shortDescriptionLabel') }}</label>
            <InputText
              v-model="pathwayData.shortDescription"
              :placeholder="t('pathway.wizard.basicInfo.shortDescriptionPlaceholder')"
              class="w-full"
              maxlength="500"
            />
          </div>

          <div class="md:col-span-2 space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.descriptionLabel') }}</label>
            <Textarea
              v-model="pathwayData.description"
              :placeholder="t('pathway.wizard.basicInfo.descriptionPlaceholder')"
              :aria-label="t('pathway.wizard.basicInfo.descriptionAria')"
              class="w-full"
              rows="4"
            />
          </div>

          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.difficultyLabel') }}</label>
            <Select
              v-model="pathwayData.difficulty"
              :options="difficultyOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('pathway.wizard.basicInfo.difficultyAria')"
              class="w-full"
            />
          </div>

          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.estimatedHoursLabel') }}</label>
            <InputNumber
              v-model="pathwayData.estimatedHours"
              :placeholder="t('pathway.wizard.basicInfo.estimatedHoursPlaceholder')"
              class="w-full"
              :min="1"
              :max="500"
            />
          </div>

          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.visibilityLabel') }}</label>
            <Select
              v-model="pathwayData.visibility"
              :options="visibilityOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('pathway.wizard.basicInfo.visibilityAria')"
              class="w-full"
            />
          </div>

          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">{{ t('pathway.wizard.basicInfo.tagsLabel') }}</label>
            <div class="flex gap-2">
              <InputText
                :modelValue="tagInput"
                :placeholder="t('pathway.wizard.basicInfo.tagInputPlaceholder')"
                class="flex-1"
                @update:modelValue="(v) => emit('update:tagInput', (v as string) ?? '')"
                @keyup.enter="emit('add-tag')"
              />
              <Button icon="pi pi-plus" severity="secondary" @click="emit('add-tag')" />
            </div>
            <div v-if="pathwayData.tags?.length" class="flex flex-wrap gap-1 mt-2">
              <Tag
                v-for="tag in pathwayData.tags"
                :key="tag"
                :value="tag"
                severity="info"
                class="cursor-pointer"
                @click="emit('remove-tag', tag)"
              >
                {{ tag }} <i class="pi pi-times ml-1 text-xs" />
              </Tag>
            </div>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>
