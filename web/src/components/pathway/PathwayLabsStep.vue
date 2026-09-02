<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Select from '@volt/Select.vue'
import Checkbox from '@volt/Checkbox.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { getDifficultyColor } from '@/utils/status'
import type { Lab } from '@/api'
import type { WizardModule } from './wizard-types'

const { t } = useI18n()

const props = defineProps<{
  modules: WizardModule[]
  availableLabs: Lab[]
  loadingLabs: boolean
  isStep3Valid: boolean
}>()

const emit = defineEmits<{
  'add-lab': [moduleIndex: number, lab: Lab]
  'remove-lab': [moduleIndex: number, labIndex: number]
  'move-lab-up': [moduleIndex: number, labIndex: number]
  'move-lab-down': [moduleIndex: number, labIndex: number]
}>()

function availableFor(moduleIndex: number): Lab[] {
  const module = props.modules[moduleIndex]
  if (!module) return []
  return props.availableLabs.filter(l => !module.labs.some(ml => ml.labTemplateId === l.id))
}

function handleLabSelect(moduleIndex: number, lab: Lab | null): void {
  if (lab) emit('add-lab', moduleIndex, lab)
}
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.labs.heading') }}</h2>
        <p class="text-surface-600 dark:text-surface-400">
          {{ t('pathway.wizard.labs.description') }}
        </p>

        <div v-if="loadingLabs" class="flex justify-center py-8">
          <ProgressSpinner />
        </div>

        <div v-else class="space-y-6">
          <div
            v-for="(module, moduleIndex) in modules"
            :key="module.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg overflow-hidden"
          >
            <div class="bg-surface-100 dark:bg-surface-800 px-4 py-3 flex items-center justify-between">
              <h3 class="font-medium text-surface-900 dark:text-surface-100">
                {{ moduleIndex + 1 }}. {{ module.name }}
              </h3>
              <span class="text-sm text-surface-500">{{ t('pathway.wizard.labs.moduleLabsCount', { count: module.labs.length }) }}</span>
            </div>

            <div class="p-4 space-y-4">
              <div v-if="module.labs.length > 0" class="space-y-2">
                <div
                  v-for="(labItem, labIndex) in module.labs"
                  :key="labItem.labTemplateId"
                  class="flex items-center gap-3 bg-surface-50 dark:bg-surface-800 rounded-lg p-3"
                >
                  <span class="text-sm text-surface-500 w-6">{{ labIndex + 1 }}.</span>
                  <div class="flex-1">
                    <div class="font-medium text-surface-900 dark:text-surface-100">{{ labItem.lab.name }}</div>
                    <div class="text-xs text-surface-500 flex items-center gap-2">
                      <span :class="getDifficultyColor(labItem.lab.difficulty)">{{ labItem.lab.difficulty }}</span>
                      <span>{{ t('pathway.wizard.labs.durationMinutes', { count: labItem.lab.durationMinutes }) }}</span>
                      <span v-if="labItem.lab.maxPoints">{{ t('pathway.wizard.labs.pointsSuffix', { count: labItem.lab.maxPoints }) }}</span>
                    </div>
                  </div>
                  <label class="flex items-center gap-2 text-sm">
                    <Checkbox v-model="labItem.isRequired" :binary="true" />
                    {{ t('pathway.wizard.labs.requiredCheckbox') }}
                  </label>
                  <div class="flex items-center gap-1">
                    <Button
                      icon="pi pi-chevron-up"
                      severity="secondary"
                      text
                      size="small"
                      :disabled="labIndex === 0"
                      @click="emit('move-lab-up', moduleIndex, labIndex)"
                    />
                    <Button
                      icon="pi pi-chevron-down"
                      severity="secondary"
                      text
                      size="small"
                      :disabled="labIndex === module.labs.length - 1"
                      @click="emit('move-lab-down', moduleIndex, labIndex)"
                    />
                    <Button
                      icon="pi pi-times"
                      severity="danger"
                      text
                      size="small"
                      @click="emit('remove-lab', moduleIndex, labIndex)"
                    />
                  </div>
                </div>
              </div>

              <div class="flex gap-2">
                <Select
                  :modelValue="null"
                  :options="availableFor(moduleIndex)"
                  optionLabel="name"
                  :placeholder="t('pathway.wizard.labs.selectPlaceholder')"
                  :aria-label="t('pathway.wizard.labs.selectAria')"
                  class="flex-1"
                  :filter="true"
                  @update:modelValue="(lab: Lab | null) => handleLabSelect(moduleIndex, lab)"
                >
                  <template #option="{ option }">
                    <div class="py-1">
                      <div class="font-medium">{{ option.name }}</div>
                      <div class="text-xs text-surface-500 flex items-center gap-2">
                        <span :class="getDifficultyColor(option.difficulty)">{{ option.difficulty }}</span>
                        <span>{{ t('pathway.wizard.labs.durationMinutes', { count: option.durationMinutes }) }}</span>
                        <span>{{ option.platform }}</span>
                      </div>
                    </div>
                  </template>
                </Select>
              </div>
            </div>
          </div>
        </div>

        <Message v-if="!isStep3Valid" severity="warn">
          {{ t('pathway.wizard.labs.emptyWarning') }}
        </Message>
      </div>
    </template>
  </Card>
</template>
