<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDifficultyColor } from '@/utils/status'
import type { Lab } from '@/api'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import Checkbox from '@volt/Checkbox.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'

export interface LocalLab {
  id?: string
  labTemplateId: string
  lab: Lab
  isRequired: boolean
  passThresholdOverride?: number
  isNew?: boolean
  isDeleted?: boolean
}

export interface LocalModule {
  id: string
  isNew: boolean
  name: string
  description?: string
  unlockType: 'sequential' | 'all_previous' | 'manual' | 'always'
  displayOrder: number
  labs: LocalLab[]
  isDeleted?: boolean
}

const props = defineProps<{
  modules: LocalModule[]
  availableLabs: Lab[]
  loadingLabs: boolean
  newModuleName: string
  newModuleDescription: string
  newModuleUnlockType: 'sequential' | 'all_previous' | 'manual' | 'always'
}>()

const emit = defineEmits<{
  'update:newModuleName': [value: string]
  'update:newModuleDescription': [value: string]
  'update:newModuleUnlockType': [value: 'sequential' | 'all_previous' | 'manual' | 'always']
  addModule: []
  deleteModule: [index: number]
  moveModuleUp: [index: number]
  moveModuleDown: [index: number]
  addLabToModule: [moduleId: string, lab: Lab]
  removeLabFromModule: [moduleId: string, labIndex: number]
  moveLabUp: [moduleId: string, labIndex: number]
  moveLabDown: [moduleId: string, labIndex: number]
}>()

const { t } = useI18n()

const activeModules = computed(() => props.modules.filter(m => !m.isDeleted))

const unlockTypeOptions = computed(() => [
  {
    label: t('pathway.create.unlockTypes.sequential'),
    value: 'sequential',
    description: t('pathway.create.unlockTypes.sequentialDescription'),
  },
  {
    label: t('pathway.create.unlockTypes.allPrevious'),
    value: 'all_previous',
    description: t('pathway.create.unlockTypes.allPreviousDescription'),
  },
  {
    label: t('pathway.create.unlockTypes.manual'),
    value: 'manual',
    description: t('pathway.create.unlockTypes.manualDescription'),
  },
  {
    label: t('pathway.create.unlockTypes.always'),
    value: 'always',
    description: t('pathway.create.unlockTypes.alwaysDescription'),
  },
])
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('pathway.wizard.modules.heading') }}
        </h2>

        <!-- Add Module Form -->
        <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-4 space-y-4">
          <h3 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('pathway.wizard.modules.addHeading') }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{
                t('pathway.wizard.modules.nameLabel')
              }}</label>
              <InputText
                :modelValue="newModuleName"
                @update:modelValue="emit('update:newModuleName', $event as string)"
                :placeholder="t('pathway.wizard.modules.namePlaceholder')"
                class="w-full"
              />
            </div>
            <div class="space-y-2">
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{
                t('pathway.wizard.modules.unlockTypeLabel')
              }}</label>
              <Select
                :modelValue="newModuleUnlockType"
                @update:modelValue="emit('update:newModuleUnlockType', $event)"
                :options="unlockTypeOptions"
                optionLabel="label"
                optionValue="value"
                :aria-label="t('pathway.wizard.modules.unlockTypeAria')"
                class="w-full"
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
              <label class="block text-sm text-surface-600 dark:text-surface-400">{{
                t('pathway.wizard.modules.descriptionLabel')
              }}</label>
              <InputText
                :modelValue="newModuleDescription"
                @update:modelValue="emit('update:newModuleDescription', $event as string)"
                :placeholder="t('pathway.wizard.modules.descriptionPlaceholder')"
                class="w-full"
              />
            </div>
          </div>
          <Button
            :label="t('pathway.wizard.modules.addAction')"
            icon="pi pi-plus"
            @click="emit('addModule')"
            :disabled="newModuleName.trim().length < 2"
          />
        </div>

        <!-- Modules List -->
        <div v-if="activeModules.length > 0" class="space-y-4">
          <div
            v-for="(module, index) in activeModules.sort((a, b) => a.displayOrder - b.displayOrder)"
            :key="module.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg overflow-hidden"
          >
            <!-- Module Header -->
            <div
              class="bg-surface-100 dark:bg-surface-800 px-4 py-3 flex items-center justify-between"
            >
              <div class="flex items-center gap-2">
                <span class="text-sm text-surface-500">{{ index + 1 }}.</span>
                <h4 class="font-medium text-surface-900 dark:text-surface-100">
                  {{ module.name }}
                </h4>
                <Tag :value="module.unlockType" severity="secondary" class="text-xs" />
                <Tag
                  v-if="module.isNew"
                  :value="t('pathway.edit.newTag')"
                  severity="info"
                  class="text-xs"
                />
              </div>
              <div class="flex items-center gap-1">
                <Button
                  icon="pi pi-chevron-up"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveModuleUp', index)"
                  :disabled="index === 0"
                />
                <Button
                  icon="pi pi-chevron-down"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveModuleDown', index)"
                  :disabled="index === activeModules.length - 1"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  @click="emit('deleteModule', index)"
                />
              </div>
            </div>

            <!-- Module Content -->
            <div class="p-4 space-y-4">
              <p v-if="module.description" class="text-sm text-surface-500">
                {{ module.description }}
              </p>

              <!-- Labs in module -->
              <div v-if="module.labs.filter(l => !l.isDeleted).length > 0" class="space-y-2">
                <div
                  v-for="(lab, labIndex) in module.labs.filter(l => !l.isDeleted)"
                  :key="lab.labTemplateId"
                  class="flex items-center gap-3 bg-surface-50 dark:bg-surface-800 rounded-lg p-3"
                >
                  <span class="text-sm text-surface-500 w-6">{{ labIndex + 1 }}.</span>
                  <div class="flex-1">
                    <div class="font-medium text-surface-900 dark:text-surface-100">
                      {{ lab.lab.name }}
                    </div>
                    <div class="text-xs text-surface-500 flex items-center gap-2">
                      <span :class="getDifficultyColor(lab.lab.difficulty || '')">{{
                        lab.lab.difficulty
                      }}</span>
                      <span>{{
                        t('pathway.wizard.labs.durationMinutes', { count: lab.lab.durationMinutes })
                      }}</span>
                      <Tag
                        v-if="lab.isNew"
                        :value="t('pathway.edit.newTag')"
                        severity="info"
                        class="text-xs"
                      />
                    </div>
                  </div>
                  <label class="flex items-center gap-2 text-sm">
                    <Checkbox v-model="lab.isRequired" :binary="true" />
                    {{ t('pathway.wizard.labs.requiredCheckbox') }}
                  </label>
                  <div class="flex items-center gap-1">
                    <Button
                      icon="pi pi-chevron-up"
                      severity="secondary"
                      text
                      size="small"
                      @click="emit('moveLabUp', module.id, labIndex)"
                      :disabled="labIndex === 0"
                    />
                    <Button
                      icon="pi pi-chevron-down"
                      severity="secondary"
                      text
                      size="small"
                      @click="emit('moveLabDown', module.id, labIndex)"
                      :disabled="labIndex === module.labs.filter(l => !l.isDeleted).length - 1"
                    />
                    <Button
                      icon="pi pi-times"
                      severity="danger"
                      text
                      size="small"
                      @click="emit('removeLabFromModule', module.id, labIndex)"
                    />
                  </div>
                </div>
              </div>

              <!-- Add lab dropdown -->
              <div class="flex gap-2">
                <Select
                  :options="
                    availableLabs.filter(
                      l => !module.labs.some(ml => ml.labTemplateId === l.id && !ml.isDeleted),
                    )
                  "
                  optionLabel="name"
                  :placeholder="t('pathway.wizard.labs.selectPlaceholder')"
                  :aria-label="t('pathway.wizard.labs.selectAria')"
                  class="flex-1"
                  :filter="true"
                  :loading="loadingLabs"
                  @update:modelValue="
                    lab => {
                      if (lab) emit('addLabToModule', module.id, lab)
                    }
                  "
                >
                  <template #option="{ option }">
                    <div class="py-1">
                      <div class="font-medium">{{ option.name }}</div>
                      <div class="text-xs text-surface-500 flex items-center gap-2">
                        <span :class="getDifficultyColor(option.difficulty)">{{
                          option.difficulty
                        }}</span>
                        <span>{{
                          t('pathway.wizard.labs.durationMinutes', {
                            count: option.durationMinutes,
                          })
                        }}</span>
                      </div>
                    </div>
                  </template>
                </Select>
              </div>
            </div>
          </div>
        </div>

        <Message v-else severity="info">
          {{ t('pathway.edit.emptyModules') }}
        </Message>
      </div>
    </template>
  </Card>
</template>
