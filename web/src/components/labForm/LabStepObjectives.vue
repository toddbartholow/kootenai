<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ObjectiveConfig, VMConfig } from '@/types/lab-form'
import { triggerTypeOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'

defineProps<{
  objectives: ObjectiveConfig[]
  vms: VMConfig[]
  newObjectiveDescription: string
  newObjectivePoints: number
  totalObjectivePoints: number
}>()

const emit = defineEmits<{
  'update:newObjectiveDescription': [value: string]
  'update:newObjectivePoints': [value: number]
  addObjective: []
  removeObjective: [index: number]
  moveObjectiveUp: [index: number]
  moveObjectiveDown: [index: number]
  addTrigger: [objIndex: number]
  removeTrigger: [objIndex: number, trigIndex: number]
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('createLab.objectivesStep.heading') }}
        </h2>
        <p class="text-surface-600 dark:text-surface-400">
          {{ t('createLab.objectivesStep.intro') }}
        </p>

        <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-4 space-y-4">
          <h3 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.objectivesStep.addHeading') }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div class="md:col-span-2 space-y-2">
              <label
                for="new-obj-desc"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.objectivesStep.descriptionLabel') }}</label
              >
              <InputText
                id="new-obj-desc"
                :modelValue="newObjectiveDescription"
                @update:modelValue="emit('update:newObjectiveDescription', $event as string)"
                :placeholder="t('createLab.objectivesStep.descriptionPlaceholder')"
                class="w-full"
              />
            </div>
            <div class="space-y-2">
              <label
                for="new-obj-points"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.objectivesStep.pointsLabel') }}</label
              >
              <InputNumber
                id="new-obj-points"
                :modelValue="newObjectivePoints"
                @update:modelValue="emit('update:newObjectivePoints', $event as number)"
                :min="1"
                :max="100"
                class="w-full"
              />
            </div>
          </div>
          <Button
            :label="t('createLab.objectivesStep.addAction')"
            icon="pi pi-plus"
            @click="emit('addObjective')"
            :disabled="newObjectiveDescription.trim().length < 2 || vms.length === 0"
          />
          <p v-if="vms.length === 0" class="text-sm text-orange-600">
            {{ t('createLab.objectivesStep.requiresVmHint') }}
          </p>
        </div>

        <div v-if="objectives.length > 0" class="space-y-4">
          <div
            v-for="(obj, objIndex) in objectives"
            :key="obj.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 space-y-4"
          >
            <div class="flex items-start justify-between">
              <div class="flex-1 space-y-2">
                <div class="flex items-center gap-2">
                  <span class="text-sm text-surface-500">{{ obj.order }}.</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{
                    obj.description
                  }}</span>
                  <Tag
                    :value="t('createLab.objectivesStep.pointsTag', { points: obj.points })"
                    severity="info"
                  />
                </div>
                <div class="space-y-1">
                  <label :for="`obj-hint-${objIndex}`" class="block text-xs text-surface-500">{{
                    t('createLab.objectivesStep.hintLabel')
                  }}</label>
                  <InputText
                    :id="`obj-hint-${objIndex}`"
                    v-model="obj.hint"
                    :placeholder="t('createLab.objectivesStep.hintPlaceholder')"
                    class="w-full text-sm"
                  />
                </div>
              </div>
              <div class="flex items-center gap-1">
                <Button
                  icon="pi pi-chevron-up"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveObjectiveUp', objIndex)"
                  :disabled="objIndex === 0"
                  :aria-label="t('createLab.objectivesStep.moveUpAria')"
                />
                <Button
                  icon="pi pi-chevron-down"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveObjectiveDown', objIndex)"
                  :disabled="objIndex === objectives.length - 1"
                  :aria-label="t('createLab.objectivesStep.moveDownAria')"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  @click="emit('removeObjective', objIndex)"
                  :aria-label="t('createLab.objectivesStep.removeAria')"
                />
              </div>
            </div>

            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <label class="text-sm font-medium text-surface-700 dark:text-surface-300">{{
                  t('createLab.objectivesStep.triggersLabel')
                }}</label>
                <Button
                  :label="t('createLab.objectivesStep.addTrigger')"
                  icon="pi pi-plus"
                  size="small"
                  severity="secondary"
                  text
                  @click="emit('addTrigger', objIndex)"
                />
              </div>
              <div
                v-for="(trigger, trigIndex) in obj.triggers"
                :key="trigIndex"
                class="bg-surface-100 dark:bg-surface-700 rounded p-3 space-y-3"
              >
                <div class="flex items-center gap-2">
                  <Select
                    v-model="trigger.type"
                    :options="triggerTypeOptions"
                    optionLabel="label"
                    optionValue="value"
                    class="flex-1"
                    :aria-label="
                      t('createLab.objectivesStep.triggerTypeAria', { index: trigIndex + 1 })
                    "
                  />
                  <Select
                    v-model="trigger.target"
                    :options="vms.map(vm => ({ label: vm.name, value: vm.name }))"
                    optionLabel="label"
                    optionValue="value"
                    class="flex-1"
                    :aria-label="t('createLab.objectivesStep.targetAria', { index: trigIndex + 1 })"
                  />
                  <Button
                    icon="pi pi-times"
                    severity="danger"
                    text
                    size="small"
                    @click="emit('removeTrigger', objIndex, trigIndex)"
                    :disabled="obj.triggers.length <= 1"
                    :aria-label="t('createLab.objectivesStep.removeTriggerAria')"
                  />
                </div>
                <div
                  v-if="trigger.type === 'file_exists' || trigger.type === 'file_content'"
                  class="space-y-2"
                >
                  <InputText
                    v-model="trigger.matchPath"
                    :placeholder="t('createLab.objectivesStep.filePathPlaceholder')"
                    class="w-full text-sm"
                    :aria-label="t('createLab.objectivesStep.filePathAria')"
                  />
                  <InputText
                    v-if="trigger.type === 'file_content'"
                    v-model="trigger.matchContains"
                    :placeholder="t('createLab.objectivesStep.fileContainsPlaceholder')"
                    class="w-full text-sm"
                    :aria-label="t('createLab.objectivesStep.fileContainsAria')"
                  />
                </div>
                <div
                  v-else-if="
                    trigger.type === 'command_executed' || trigger.type === 'service_running'
                  "
                >
                  <InputText
                    v-model="trigger.matchPattern"
                    :placeholder="
                      trigger.type === 'command_executed'
                        ? t('createLab.objectivesStep.commandPlaceholder')
                        : t('createLab.objectivesStep.servicePlaceholder')
                    "
                    class="w-full text-sm"
                    :aria-label="t('createLab.objectivesStep.patternAria')"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <Message v-else severity="info">{{ t('createLab.objectivesStep.empty') }}</Message>

        <div v-if="objectives.length > 0" class="text-right">
          <span class="text-surface-600 dark:text-surface-400"
            >{{ t('createLab.objectivesStep.summary') }}
            <strong class="text-primary-600">{{ totalObjectivePoints }}</strong></span
          >
        </div>
      </div>
    </template>
  </Card>
</template>
