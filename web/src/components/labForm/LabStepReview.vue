<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { LabFormData, VMConfig, ObjectiveConfig, QuestionConfig } from '@/types/lab-form'
import { getDifficultyColor } from '@/utils/status'
import Card from '@volt/Card.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'

defineProps<{
  labData: LabFormData
  vms: VMConfig[]
  objectives: ObjectiveConfig[]
  questions: QuestionConfig[]
  totalVMs: number
  totalObjectivePoints: number
  totalQuestionPoints: number
  totalPoints: number
  showStatusNotice?: boolean
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('createLab.review.heading') }}
        </h2>

        <div class="grid grid-cols-2 md:grid-cols-5 gap-4">
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ totalVMs }}</div>
            <div class="text-sm text-surface-500">{{ t('createLab.review.stats.vms') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ objectives.length }}</div>
            <div class="text-sm text-surface-500">{{ t('createLab.review.stats.objectives') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ questions.length }}</div>
            <div class="text-sm text-surface-500">{{ t('createLab.review.stats.questions') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ totalPoints }}</div>
            <div class="text-sm text-surface-500">
              {{ t('createLab.review.stats.totalPoints') }}
            </div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">
              {{ t('createLab.review.stats.durationValue', { count: labData.durationMinutes }) }}
            </div>
            <div class="text-sm text-surface-500">{{ t('createLab.review.stats.duration') }}</div>
          </div>
        </div>

        <div class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 space-y-3">
          <h3 class="font-semibold text-surface-900 dark:text-surface-100">{{ labData.name }}</h3>
          <p v-if="labData.description" class="text-surface-600 dark:text-surface-400">
            {{ labData.description }}
          </p>
          <div class="flex flex-wrap gap-2 text-sm">
            <Tag :value="labData.difficulty" :class="getDifficultyColor(labData.difficulty)" />
            <Tag :value="labData.platform" severity="secondary" />
            <Tag
              :value="t('createLab.review.versionFormat', { version: labData.version })"
              severity="info"
            />
            <Tag
              v-if="showStatusNotice"
              :value="t('createLab.review.passTag', { value: labData.passThreshold })"
              severity="warn"
            />
          </div>
          <div v-if="showStatusNotice && labData.tags.length" class="flex flex-wrap gap-1">
            <span
              v-for="tag in labData.tags"
              :key="tag"
              class="px-2 py-0.5 text-xs bg-surface-200 dark:bg-surface-700 rounded"
              >{{ tag }}</span
            >
          </div>
        </div>

        <div v-if="showStatusNotice" class="space-y-3">
          <h4 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.review.vmsHeading') }}
          </h4>
          <div class="grid gap-2">
            <div
              v-for="vm in vms"
              :key="vm.id"
              class="border border-surface-200 dark:border-surface-700 rounded p-3"
            >
              <div class="flex items-center justify-between">
                <span class="font-medium text-surface-900 dark:text-surface-100">{{
                  vm.name
                }}</span>
                <span class="text-sm text-surface-500">{{ vm.template }}</span>
              </div>
              <div class="text-xs text-surface-500 mt-1">
                {{
                  t('createLab.review.vmSpecs', { cpu: vm.cpu, memory: vm.memory, disk: vm.disk })
                }}
                <span v-if="vm.snapshots.length > 1">
                  |
                  {{ t('createLab.review.vmSnapshotSuffix', { count: vm.snapshots.length }) }}</span
                >
              </div>
            </div>
          </div>
        </div>

        <div v-if="showStatusNotice && objectives.length > 0" class="space-y-3">
          <h4 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.review.objectivesHeading', { points: totalObjectivePoints }) }}
          </h4>
          <div class="space-y-2">
            <div
              v-for="obj in objectives"
              :key="obj.id"
              class="flex items-center justify-between text-sm border-b border-surface-100 dark:border-surface-700 pb-2"
            >
              <span class="text-surface-700 dark:text-surface-300"
                >{{ obj.order }}. {{ obj.description }}</span
              >
              <Tag
                :value="t('createLab.objectivesStep.pointsTag', { points: obj.points })"
                severity="info"
                class="text-xs"
              />
            </div>
          </div>
        </div>

        <div v-if="questions.length > 0" class="space-y-3">
          <h4 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.review.questionsHeading', { points: totalQuestionPoints }) }}
          </h4>
          <div class="space-y-2">
            <div
              v-for="q in questions"
              :key="q.id"
              class="flex items-center justify-between text-sm border-b border-surface-100 dark:border-surface-700 pb-2"
            >
              <div class="flex items-center gap-2">
                <Tag
                  :value="
                    q.type === 'text'
                      ? t('createLab.questionsStep.typeText')
                      : t('createLab.questionsStep.typeMcShort')
                  "
                  :severity="q.type === 'text' ? 'info' : 'secondary'"
                  class="text-xs"
                />
                <span class="text-surface-700 dark:text-surface-300"
                  >{{ q.order }}. {{ q.description }}</span
                >
              </div>
              <Tag
                :value="t('createLab.questionsStep.pointsTag', { points: q.points })"
                severity="warn"
                class="text-xs"
              />
            </div>
          </div>
        </div>

        <Message v-if="showStatusNotice" severity="info">
          {{
            labData.isActive
              ? t('createLab.review.statusNotice.asActive')
              : t('createLab.review.statusNotice.asDraft')
          }}
          {{
            labData.visibility === 'organization'
              ? t('createLab.review.statusNotice.orgVisible')
              : t('createLab.review.statusNotice.privateVisible')
          }}
        </Message>
      </div>
    </template>
  </Card>
</template>
