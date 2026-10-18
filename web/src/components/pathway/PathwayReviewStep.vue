<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import { getDifficultyColor } from '@/utils/status'
import type { WizardPathwayData, WizardModule } from './wizard-types'

const { t } = useI18n()

const props = defineProps<{
  pathwayData: WizardPathwayData
  modules: WizardModule[]
}>()

const totalLabs = computed(() => props.modules.reduce((sum, m) => sum + m.labs.length, 0))
const totalPoints = computed(() =>
  props.modules.reduce((sum, m) =>
    sum + m.labs.reduce((labSum, l) => labSum + (l.lab.maxPoints || 0), 0), 0),
)
const totalDuration = computed(() =>
  props.modules.reduce((sum, m) =>
    sum + m.labs.reduce((labSum, l) => labSum + (l.lab.durationMinutes || 0), 0), 0),
)
const totalHours = computed(() => Math.round(totalDuration.value / 60))
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.review.heading') }}</h2>

        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ modules.length }}</div>
            <div class="text-sm text-surface-500">{{ t('pathway.wizard.review.statsModules') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ totalLabs }}</div>
            <div class="text-sm text-surface-500">{{ t('pathway.wizard.review.statsLabs') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ totalPoints }}</div>
            <div class="text-sm text-surface-500">{{ t('pathway.wizard.review.statsPoints') }}</div>
          </div>
          <div class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 text-center">
            <div class="text-2xl font-bold text-primary-600">{{ t('pathway.wizard.review.durationFormat', { hours: totalHours }) }}</div>
            <div class="text-sm text-surface-500">{{ t('pathway.wizard.review.statsDuration') }}</div>
          </div>
        </div>

        <div class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 space-y-3">
          <h3 class="font-semibold text-surface-900 dark:text-surface-100">{{ pathwayData.name }}</h3>
          <p v-if="pathwayData.shortDescription" class="text-surface-600 dark:text-surface-400">
            {{ pathwayData.shortDescription }}
          </p>
          <div class="flex flex-wrap gap-2 text-sm">
            <Tag :value="pathwayData.difficulty" :class="getDifficultyColor(pathwayData.difficulty || '')" />
            <Tag :value="pathwayData.visibility" severity="secondary" />
            <Tag v-if="pathwayData.estimatedHours" :value="t('pathway.wizard.review.estimatedHoursTag', { hours: pathwayData.estimatedHours })" severity="info" />
          </div>
          <div v-if="pathwayData.tags?.length" class="flex flex-wrap gap-1">
            <span
              v-for="tag in pathwayData.tags"
              :key="tag"
              class="px-2 py-0.5 text-xs bg-surface-200 dark:bg-surface-700 rounded"
            >
              {{ tag }}
            </span>
          </div>
        </div>

        <div class="space-y-3">
          <h4 class="font-medium text-surface-900 dark:text-surface-100">{{ t('pathway.wizard.review.modulesAndLabsHeading') }}</h4>
          <div
            v-for="(module, index) in modules"
            :key="module.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-3"
          >
            <div class="flex items-center gap-2 mb-2">
              <span class="text-sm text-surface-500">{{ index + 1 }}.</span>
              <span class="font-medium text-surface-900 dark:text-surface-100">{{ module.name }}</span>
              <Tag :value="module.unlockType" severity="secondary" class="text-xs" />
            </div>
            <div class="pl-6 space-y-1">
              <div
                v-for="(lab, labIndex) in module.labs"
                :key="lab.labTemplateId"
                class="text-sm text-surface-600 dark:text-surface-400 flex items-center gap-2"
              >
                <span>{{ labIndex + 1 }}. {{ lab.lab.name }}</span>
                <Tag v-if="lab.isRequired" :value="t('pathway.wizard.review.requiredTag')" severity="info" class="text-xs" />
              </div>
              <div v-if="module.labs.length === 0" class="text-sm text-surface-400 italic">
                {{ t('pathway.wizard.review.noLabsNote') }}
              </div>
            </div>
          </div>
        </div>

        <Message severity="info">
          <i18n-t keypath="pathway.wizard.review.draftMessage" tag="span">
            <template #draft>
              <strong>{{ t('pathway.wizard.review.draftWord') }}</strong>
            </template>
          </i18n-t>
        </Message>
      </div>
    </template>
  </Card>
</template>
