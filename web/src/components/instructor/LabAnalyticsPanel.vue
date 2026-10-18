<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { InstructorDashboardResponse } from '@/api'
import Card from '@volt/Card.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'

defineProps<{
  labStats: InstructorDashboardResponse['labStats']
  labsWithHighFailure: InstructorDashboardResponse['labStats']
}>()

const { t } = useI18n()

function getPassRateColor(rate: number): 'success' | 'warn' | 'danger' {
  if (rate >= 80) return 'success'
  if (rate >= 60) return 'warn'
  return 'danger'
}
</script>

<template>
  <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
    <!-- Labs with Low Pass Rates -->
    <Card>
      <template #title>
        <div class="flex items-center gap-2">
          <i class="pi pi-exclamation-circle text-red-500" />
          {{ t('instructorDashboard.labs.lowPassRateHeading') }}
        </div>
      </template>
      <template #content>
        <div v-if="labsWithHighFailure.length === 0" class="text-center py-8">
          <i class="pi pi-check-circle text-4xl text-green-500 mb-4" />
          <p class="text-surface-500">{{ t('instructorDashboard.labs.allPassingMessage') }}</p>
        </div>
        <div v-else class="space-y-4">
          <div
            v-for="lab in labsWithHighFailure"
            :key="lab.labId"
            class="p-4 rounded-lg border border-red-200 dark:border-red-900/50 bg-red-50 dark:bg-red-900/20"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="font-semibold text-surface-900 dark:text-surface-100 text-base">
                {{ lab.labName }}
              </h3>
              <Tag
                :value="t('instructorDashboard.labs.passRateTag', { rate: lab.passRate })"
                :severity="getPassRateColor(lab.passRate)"
              />
            </div>
            <div class="text-sm text-surface-600 dark:text-surface-400 mb-3">
              {{
                t('instructorDashboard.labs.completionLine', {
                  completed: lab.completions,
                  total: lab.totalAttempts,
                  minutes: lab.averageTimeMinutes,
                })
              }}
            </div>

            <div v-if="lab.failurePoints.length > 0" class="mt-3">
              <div class="text-xs font-medium text-surface-500 mb-2">
                {{ t('instructorDashboard.labs.failurePointsHeading') }}
              </div>
              <div class="space-y-2">
                <div
                  v-for="fp in lab.failurePoints.slice(0, 3)"
                  :key="fp.checkpointId"
                  class="flex items-center justify-between text-sm"
                >
                  <span>{{ fp.checkpointName }}</span>
                  <span class="text-red-600 dark:text-red-400 font-medium">
                    {{ t('instructorDashboard.labs.failureRate', { rate: fp.failureRate }) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </Card>

    <!-- All Lab Stats -->
    <Card>
      <template #title>
        <div class="flex items-center gap-2">
          <i class="pi pi-chart-bar text-blue-500" />
          {{ t('instructorDashboard.labs.statsHeading') }}
        </div>
      </template>
      <template #content>
        <div class="space-y-4">
          <div
            v-for="lab in labStats"
            :key="lab.labId"
            class="p-3 rounded-lg bg-surface-50 dark:bg-surface-800"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="font-medium text-surface-900 dark:text-surface-100">
                {{ lab.labName }}
              </span>
              <span class="text-sm text-surface-500">
                {{ t('instructorDashboard.labs.attempts', { count: lab.totalAttempts }) }}
              </span>
            </div>
            <div class="flex items-center gap-3">
              <ProgressBar :value="lab.passRate" class="flex-1 h-2" />
              <span
                class="text-sm font-medium w-12 text-right"
                :class="{
                  'text-green-600': lab.passRate >= 80,
                  'text-yellow-600': lab.passRate >= 60 && lab.passRate < 80,
                  'text-red-600': lab.passRate < 60,
                }"
              >
                {{ lab.passRate }}%
              </span>
            </div>
            <div class="text-xs text-surface-500 mt-1">
              {{
                t('instructorDashboard.labs.detailLine', {
                  score: lab.averageScore,
                  minutes: lab.averageTimeMinutes,
                })
              }}
            </div>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>
