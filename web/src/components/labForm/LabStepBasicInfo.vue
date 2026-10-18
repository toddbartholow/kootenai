<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { LabFormData, SelectOption } from '@/types/lab-form'
import { difficultyOptions, platformOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Checkbox from '@volt/Checkbox.vue'
import Tag from '@volt/Tag.vue'

defineProps<{
  labData: LabFormData
  tagInput: string
  visibilityOptions: SelectOption[]
  isActiveLabel: string
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
          {{ t('createLab.basic.heading') }}
        </h2>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div class="md:col-span-2 space-y-2">
            <label
              for="lab-name"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
            >
              {{ t('createLab.basic.nameLabel') }}
              <span class="text-red-500">{{ t('createLab.required') }}</span>
            </label>
            <InputText
              id="lab-name"
              v-model="labData.name"
              :placeholder="t('createLab.basic.namePlaceholder')"
              class="w-full"
            />
          </div>

          <div class="md:col-span-2 space-y-2">
            <label
              for="lab-description"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.descriptionLabel') }}</label
            >
            <Textarea
              id="lab-description"
              v-model="labData.description"
              :placeholder="t('createLab.basic.descriptionPlaceholder')"
              class="w-full"
              rows="3"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-difficulty"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.difficultyLabel') }}</label
            >
            <Select
              id="lab-difficulty"
              v-model="labData.difficulty"
              :options="difficultyOptions"
              optionLabel="label"
              optionValue="value"
              class="w-full"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-duration"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.durationLabel') }}</label
            >
            <InputNumber
              id="lab-duration"
              v-model="labData.durationMinutes"
              class="w-full"
              :min="5"
              :max="480"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-platform"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.platformLabel') }}</label
            >
            <Select
              id="lab-platform"
              v-model="labData.platform"
              :options="platformOptions"
              optionLabel="label"
              optionValue="value"
              class="w-full"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-version"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.versionLabel') }}</label
            >
            <InputText
              id="lab-version"
              v-model="labData.version"
              :placeholder="t('createLab.basic.versionPlaceholder')"
              class="w-full"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-threshold"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.thresholdLabel') }}</label
            >
            <InputNumber
              id="lab-threshold"
              v-model="labData.passThreshold"
              class="w-full"
              :min="0"
              :max="100"
            />
          </div>

          <div class="space-y-2">
            <label
              for="lab-visibility"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.visibilityLabel') }}</label
            >
            <Select
              id="lab-visibility"
              v-model="labData.visibility"
              :options="visibilityOptions"
              optionLabel="label"
              optionValue="value"
              class="w-full"
            />
          </div>

          <div class="md:col-span-2 space-y-2">
            <label
              for="lab-tags"
              class="block text-sm font-medium text-surface-700 dark:text-surface-300"
              >{{ t('createLab.basic.tagsLabel') }}</label
            >
            <div class="flex gap-2">
              <InputText
                id="lab-tags"
                :modelValue="tagInput"
                @update:modelValue="emit('update:tagInput', $event as string)"
                :placeholder="t('createLab.basic.tagsPlaceholder')"
                class="flex-1"
                @keyup.enter="emit('addTag')"
              />
              <Button
                icon="pi pi-plus"
                @click="emit('addTag')"
                severity="secondary"
                :aria-label="t('createLab.basic.addTagAria')"
              />
            </div>
            <div v-if="labData.tags.length" class="flex flex-wrap gap-1 mt-2">
              <Tag
                v-for="tag in labData.tags"
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

          <div class="flex items-center gap-2">
            <Checkbox v-model="labData.isActive" :binary="true" inputId="isActive" />
            <label for="isActive" class="text-sm text-surface-700 dark:text-surface-300">{{
              isActiveLabel
            }}</label>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>
