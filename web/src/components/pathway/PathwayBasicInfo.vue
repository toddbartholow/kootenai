<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { difficultyOptions, visibilityOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Tag from '@volt/Tag.vue'

export interface PathwayFormData {
  name: string
  description: string
  shortDescription: string
  difficulty: 'beginner' | 'intermediate' | 'advanced' | 'mixed'
  visibility: 'global' | 'organization' | 'private'
  estimatedHours: number | undefined
  tags: string[]
}

defineProps<{
  pathwayData: PathwayFormData
  tagInput: string
}>()

const emit = defineEmits<{
  'update:tagInput': [value: string]
  addTag: []
  removeTag: [tag: string]
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('pathway.wizard.basicInfo.heading') }}
        </h2>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <!-- Name -->
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

          <!-- Short Description -->
          <div class="md:col-span-2 space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.shortDescriptionLabel') }}
            </label>
            <InputText
              v-model="pathwayData.shortDescription"
              :placeholder="t('pathway.wizard.basicInfo.shortDescriptionPlaceholder')"
              class="w-full"
              maxlength="500"
            />
          </div>

          <!-- Full Description -->
          <div class="md:col-span-2 space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.descriptionLabel') }}
            </label>
            <Textarea
              v-model="pathwayData.description"
              :placeholder="t('pathway.wizard.basicInfo.descriptionPlaceholder')"
              :aria-label="t('pathway.wizard.basicInfo.descriptionAria')"
              class="w-full"
              rows="4"
            />
          </div>

          <!-- Difficulty -->
          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.difficultyLabel') }}
            </label>
            <Select
              v-model="pathwayData.difficulty"
              :options="difficultyOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('pathway.wizard.basicInfo.difficultyAria')"
              class="w-full"
            />
          </div>

          <!-- Estimated Hours -->
          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.estimatedHoursLabel') }}
            </label>
            <InputNumber
              v-model="pathwayData.estimatedHours"
              :placeholder="t('pathway.wizard.basicInfo.estimatedHoursPlaceholder')"
              class="w-full"
              :min="1"
              :max="500"
            />
          </div>

          <!-- Visibility -->
          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.visibilityLabel') }}
            </label>
            <Select
              v-model="pathwayData.visibility"
              :options="visibilityOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('pathway.wizard.basicInfo.visibilityAria')"
              class="w-full"
            />
          </div>

          <!-- Tags -->
          <div class="space-y-2">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pathway.wizard.basicInfo.tagsLabel') }}
            </label>
            <div class="flex gap-2">
              <InputText
                :modelValue="tagInput"
                @update:modelValue="emit('update:tagInput', $event as string)"
                :placeholder="t('pathway.wizard.basicInfo.tagInputPlaceholder')"
                class="flex-1"
                @keyup.enter="emit('addTag')"
              />
              <Button icon="pi pi-plus" @click="emit('addTag')" severity="secondary" />
            </div>
            <div v-if="pathwayData.tags?.length" class="flex flex-wrap gap-1 mt-2">
              <Tag
                v-for="tag in pathwayData.tags"
                :key="tag"
                :value="tag"
                severity="info"
                class="cursor-pointer"
                @click="emit('removeTag', tag)"
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
