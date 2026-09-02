<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { StudentSummary } from '@/api'
import Card from '@volt/Card.vue'
import Tag from '@volt/Tag.vue'

defineProps<{
  students: StudentSummary[]
  getStatusSeverity: (status: string) => 'success' | 'info' | 'warn' | 'danger' | 'secondary'
  getStatusLabel: (status: string) => string
  formatDate: (dateStr?: string) => string
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #title>
      <div class="flex items-center gap-2">
        <i class="pi pi-exclamation-triangle text-yellow-500" />
        {{ t('instructorDashboard.struggling.heading') }}
      </div>
    </template>
    <template #content>
      <div v-if="students.length === 0" class="text-center py-12">
        <i class="pi pi-check-circle text-4xl text-green-500 mb-4" />
        <p class="text-surface-600 dark:text-surface-400">
          {{ t('instructorDashboard.struggling.emptyMessage') }}
        </p>
      </div>
      <div v-else class="space-y-4">
        <div
          v-for="student in students"
          :key="student.id"
          class="p-4 rounded-lg border border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800"
        >
          <div class="flex items-start justify-between mb-3">
            <div>
              <h3 class="font-semibold text-surface-900 dark:text-surface-100 text-base">
                {{ student.name }}
              </h3>
              <p class="text-sm text-surface-500">{{ student.email }}</p>
            </div>
            <Tag
              :value="getStatusLabel(student.status)"
              :severity="getStatusSeverity(student.status)"
            />
          </div>

          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <div class="text-surface-500">
                {{ t('instructorDashboard.struggling.labsCompleted') }}
              </div>
              <div class="font-medium">
                {{ student.labsCompleted }} / {{ student.labsAttempted }}
              </div>
            </div>
            <div>
              <div class="text-surface-500">
                {{ t('instructorDashboard.struggling.averageScore') }}
              </div>
              <div class="font-medium" :class="student.averageScore < 60 ? 'text-red-500' : ''">
                {{ student.averageScore }}%
              </div>
            </div>
            <div>
              <div class="text-surface-500">
                {{ t('instructorDashboard.struggling.currentStreak') }}
              </div>
              <div class="font-medium">
                {{
                  t('instructorDashboard.struggling.streakDays', { count: student.currentStreak })
                }}
              </div>
            </div>
            <div>
              <div class="text-surface-500">
                {{ t('instructorDashboard.struggling.lastActive') }}
              </div>
              <div
                class="font-medium"
                :class="
                  !student.lastActiveAt ||
                  new Date(student.lastActiveAt) < new Date(Date.now() - 7 * 24 * 60 * 60 * 1000)
                    ? 'text-red-500'
                    : ''
                "
              >
                {{ formatDate(student.lastActiveAt) }}
              </div>
            </div>
          </div>

          <div class="mt-3 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
            <div class="flex items-start gap-2">
              <i class="pi pi-info-circle text-yellow-600 dark:text-yellow-400 mt-0.5" />
              <div class="text-sm text-yellow-800 dark:text-yellow-200">
                <template v-if="student.status === 'inactive'">
                  <strong>{{
                    t('instructorDashboard.struggling.inactiveHeadline', {
                      count: Math.floor(
                        (Date.now() - new Date(student.lastActiveAt || 0).getTime()) /
                          (1000 * 60 * 60 * 24),
                      ),
                    })
                  }}</strong>
                  {{ ' ' + t('instructorDashboard.struggling.inactiveAdvice') }}
                </template>
                <template v-else-if="student.averageScore < 60">
                  <strong>{{
                    t('instructorDashboard.struggling.lowScoreHeadline', {
                      score: student.averageScore,
                    })
                  }}</strong>
                  {{ ' ' + t('instructorDashboard.struggling.lowScoreAdvice') }}
                </template>
                <template v-else>
                  <strong>{{ t('instructorDashboard.struggling.fallingBehindHeadline') }}</strong>
                  {{ ' ' + t('instructorDashboard.struggling.fallingBehindAdvice') }}
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>
