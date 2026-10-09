<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { QuestionConfig, ObjectiveConfig } from '@/types/lab-form'
import { questionTypeOptions, validationTypeOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Checkbox from '@volt/Checkbox.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'

defineProps<{
  questions: QuestionConfig[]
  objectives: ObjectiveConfig[]
  newQuestionDescription: string
  newQuestionPoints: number
  newQuestionType: 'text' | 'multiple_choice'
  totalQuestionPoints: number
  totalPoints: number
  getAvailableDependencies: (id: string) => { label: string; value: string }[]
}>()

const emit = defineEmits<{
  'update:newQuestionDescription': [value: string]
  'update:newQuestionPoints': [value: number]
  'update:newQuestionType': [value: 'text' | 'multiple_choice']
  addQuestion: []
  removeQuestion: [index: number]
  moveQuestionUp: [index: number]
  moveQuestionDown: [index: number]
  duplicateQuestion: [index: number]
  handleQuestionTypeChange: [index: number, type: 'text' | 'multiple_choice']
  addOption: [qIndex: number]
  removeOption: [qIndex: number, optIndex: number]
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('createLab.questionsStep.heading') }}
        </h2>
        <p class="text-surface-600 dark:text-surface-400">
          {{ t('createLab.questionsStep.intro') }}
        </p>

        <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-4 space-y-4">
          <h3 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.questionsStep.addHeading') }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
            <div class="md:col-span-2 space-y-2">
              <label
                for="new-q-desc"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.questionsStep.descriptionLabel') }}</label
              >
              <InputText
                id="new-q-desc"
                :modelValue="newQuestionDescription"
                @update:modelValue="emit('update:newQuestionDescription', $event as string)"
                :placeholder="t('createLab.questionsStep.descriptionPlaceholder')"
                class="w-full"
              />
            </div>
            <div class="space-y-2">
              <label
                for="new-q-type"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.questionsStep.typeLabel') }}</label
              >
              <Select
                id="new-q-type"
                :modelValue="newQuestionType"
                @update:modelValue="emit('update:newQuestionType', $event)"
                :options="questionTypeOptions"
                optionLabel="label"
                optionValue="value"
                class="w-full"
              />
            </div>
            <div class="space-y-2">
              <label
                for="new-q-points"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.questionsStep.pointsLabel') }}</label
              >
              <InputNumber
                id="new-q-points"
                :modelValue="newQuestionPoints"
                @update:modelValue="emit('update:newQuestionPoints', $event as number)"
                :min="1"
                :max="100"
                class="w-full"
              />
            </div>
          </div>
          <Button
            :label="t('createLab.questionsStep.addAction')"
            icon="pi pi-plus"
            @click="emit('addQuestion')"
            :disabled="newQuestionDescription.trim().length < 2"
          />
        </div>

        <div v-if="questions.length > 0" class="space-y-4">
          <div
            v-for="(q, qIndex) in questions"
            :key="q.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 space-y-4"
          >
            <div class="flex items-start justify-between">
              <div class="flex-1 space-y-2">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-sm text-surface-500">{{ q.order }}.</span>
                  <Tag
                    :value="
                      q.type === 'text'
                        ? t('createLab.questionsStep.typeText')
                        : t('createLab.questionsStep.typeMc')
                    "
                    :severity="q.type === 'text' ? 'info' : 'secondary'"
                  />
                  <span class="font-medium text-surface-900 dark:text-surface-100">{{
                    q.description
                  }}</span>
                  <Tag
                    :value="t('createLab.questionsStep.pointsTag', { points: q.points })"
                    severity="warn"
                  />
                </div>
              </div>
              <div class="flex items-center gap-1">
                <Button
                  icon="pi pi-chevron-up"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveQuestionUp', qIndex)"
                  :disabled="qIndex === 0"
                  :aria-label="t('createLab.questionsStep.moveUpAria')"
                />
                <Button
                  icon="pi pi-chevron-down"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('moveQuestionDown', qIndex)"
                  :disabled="qIndex === questions.length - 1"
                  :aria-label="t('createLab.questionsStep.moveDownAria')"
                />
                <Button
                  icon="pi pi-copy"
                  severity="secondary"
                  text
                  size="small"
                  @click="emit('duplicateQuestion', qIndex)"
                  :title="t('createLab.questionsStep.duplicateTitle')"
                  :aria-label="t('createLab.questionsStep.duplicateAria')"
                />
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  @click="emit('removeQuestion', qIndex)"
                  :aria-label="t('createLab.questionsStep.removeAria')"
                />
              </div>
            </div>

            <div class="flex items-center gap-4">
              <label class="text-sm text-surface-600 dark:text-surface-400">{{
                t('createLab.questionsStep.typeRowLabel')
              }}</label>
              <Select
                :modelValue="q.type"
                @update:modelValue="emit('handleQuestionTypeChange', qIndex, $event)"
                :options="questionTypeOptions"
                optionLabel="label"
                optionValue="value"
                class="w-48"
                :aria-label="t('createLab.questionsStep.typeRowAria', { index: qIndex + 1 })"
              />
            </div>

            <div class="space-y-1">
              <label :for="`q-hint-${qIndex}`" class="block text-xs text-surface-500">{{
                t('createLab.questionsStep.hintLabel')
              }}</label>
              <InputText
                :id="`q-hint-${qIndex}`"
                v-model="q.hint"
                :placeholder="t('createLab.questionsStep.hintPlaceholder')"
                class="w-full text-sm"
              />
            </div>

            <div
              v-if="q.type === 'text'"
              class="space-y-3 bg-surface-50 dark:bg-surface-800 rounded-lg p-3"
            >
              <div class="flex items-center gap-4">
                <label class="text-sm text-surface-600 dark:text-surface-400">{{
                  t('createLab.questionsStep.validationLabel')
                }}</label>
                <Select
                  v-model="q.validation.type"
                  :options="validationTypeOptions"
                  optionLabel="label"
                  optionValue="value"
                  class="w-40"
                  :aria-label="t('createLab.questionsStep.validationAria', { index: qIndex + 1 })"
                />
                <div v-if="q.validation.type === 'exact'" class="flex items-center gap-2">
                  <Checkbox
                    v-model="q.validation.caseSensitive"
                    :binary="true"
                    :inputId="`case-${qIndex}`"
                  />
                  <label
                    :for="`case-${qIndex}`"
                    class="text-sm text-surface-600 dark:text-surface-400"
                    >{{ t('createLab.questionsStep.caseSensitiveLabel') }}</label
                  >
                </div>
              </div>
              <div>
                <label :for="`q-answer-${qIndex}`" class="block text-xs text-surface-500 mb-1">{{
                  q.validation.type === 'exact'
                    ? t('createLab.questionsStep.expectedAnswerLabel')
                    : t('createLab.questionsStep.regexPatternLabel')
                }}</label>
                <InputText
                  v-if="q.validation.type === 'exact'"
                  :id="`q-answer-${qIndex}`"
                  v-model="q.validation.answer"
                  :placeholder="t('createLab.questionsStep.expectedAnswerPlaceholder')"
                  class="w-full"
                />
                <InputText
                  v-else
                  :id="`q-answer-${qIndex}`"
                  v-model="q.validation.pattern"
                  :placeholder="t('createLab.questionsStep.regexPlaceholder')"
                  class="w-full font-mono text-sm"
                />
              </div>
            </div>

            <div v-else class="space-y-3 bg-surface-50 dark:bg-surface-800 rounded-lg p-3">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <Checkbox v-model="q.multiSelect" :binary="true" :inputId="`multi-${qIndex}`" />
                  <label
                    :for="`multi-${qIndex}`"
                    class="text-sm text-surface-600 dark:text-surface-400"
                    >{{ t('createLab.questionsStep.multiLabel') }}</label
                  >
                </div>
                <Button
                  :label="t('createLab.questionsStep.addOption')"
                  icon="pi pi-plus"
                  size="small"
                  severity="secondary"
                  text
                  @click="emit('addOption', qIndex)"
                  :disabled="q.options.length >= 6"
                />
              </div>
              <div class="space-y-2">
                <div
                  v-for="(opt, optIndex) in q.options"
                  :key="opt.id"
                  class="flex items-center gap-2"
                >
                  <Checkbox
                    v-model="opt.correct"
                    :binary="true"
                    :inputId="`opt-${qIndex}-${optIndex}`"
                    class="flex-shrink-0"
                  />
                  <span class="text-sm font-medium text-surface-600 w-6">{{ opt.id }}.</span>
                  <InputText
                    v-model="opt.text"
                    :placeholder="t('createLab.questionsStep.optionPlaceholder', { id: opt.id })"
                    class="flex-1"
                    :aria-label="t('createLab.questionsStep.optionAria', { id: opt.id })"
                  />
                  <Button
                    icon="pi pi-times"
                    severity="danger"
                    text
                    size="small"
                    @click="emit('removeOption', qIndex, optIndex)"
                    :disabled="q.options.length <= 2"
                    :aria-label="t('createLab.questionsStep.removeOptionAria')"
                  />
                </div>
              </div>
              <p class="text-xs text-surface-500">{{ t('createLab.questionsStep.correctHint') }}</p>
            </div>

            <div v-if="objectives.length > 0 || questions.length > 1" class="space-y-2">
              <label class="block text-xs text-surface-500">{{
                t('createLab.questionsStep.dependenciesLabel')
              }}</label>
              <Select
                v-model="q.dependsOn"
                :options="getAvailableDependencies(q.id)"
                optionLabel="label"
                optionValue="value"
                :placeholder="t('createLab.questionsStep.dependenciesPlaceholder')"
                class="w-full"
                multiple
                :aria-label="t('createLab.questionsStep.dependenciesAria', { index: qIndex + 1 })"
              />
            </div>
          </div>
        </div>

        <Message v-else severity="info">{{ t('createLab.questionsStep.empty') }}</Message>

        <div v-if="questions.length > 0" class="text-right">
          <span class="text-surface-600 dark:text-surface-400">
            {{ t('createLab.questionsStep.pointsSummary') }}
            <strong class="text-primary-600">{{ totalQuestionPoints }}</strong>
            <span class="mx-2">|</span>
            {{ t('createLab.questionsStep.totalSummary') }}
            <strong class="text-primary-600">{{ totalPoints }}</strong>
          </span>
        </div>
      </div>
    </template>
  </Card>
</template>
