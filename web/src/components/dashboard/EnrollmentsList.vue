<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { getStatusSeverity } from '@/utils/status'
import { formatDate } from '@/utils/format'
import type { PathwayEnrollment } from '@/api'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'

defineProps<{
  enrollments: PathwayEnrollment[]
  itemCount: number
}>()

const { t } = useI18n()
const router = useRouter()

function navigateToEnrollment(enrollment: PathwayEnrollment) {
  router.push(`/enrollments/${enrollment.id}`)
}
</script>

<template>
  <div>
    <div v-if="enrollments.length === 0" class="text-center py-8">
      <i
        class="pi pi-compass text-4xl text-surface-300 dark:text-surface-600 mb-4"
        aria-hidden="true"
      />
      <p class="text-surface-500 mb-4">{{ t('dashboard.enrollments.empty') }}</p>
      <RouterLink to="/pathways">
        <Button
          :label="t('dashboard.enrollments.exploreAction')"
          icon="pi pi-arrow-right"
          iconPos="right"
          size="small"
        />
      </RouterLink>
    </div>
    <ul v-else class="space-y-4" aria-label="Your pathway enrollments" data-pseudo-skip>
      <li v-for="enrollment in enrollments.slice(0, itemCount)" :key="enrollment.id">
        <article
          tabindex="0"
          role="button"
          class="p-4 rounded-lg border border-surface-200 dark:border-surface-700 hover:bg-surface-50 dark:hover:bg-surface-800 focus:ring-2 focus:ring-primary-500 cursor-pointer transition-colors"
          :aria-label="`${enrollment.pathway?.name || 'Unknown Pathway'}, ${enrollment.percentage}% complete, ${enrollment.completedModules} of ${enrollment.totalModules} modules`"
          @click="navigateToEnrollment(enrollment)"
          @keydown.enter="navigateToEnrollment(enrollment)"
          @keydown.space.prevent="navigateToEnrollment(enrollment)"
        >
          <div class="flex items-start justify-between mb-3">
            <div>
              <h3 class="font-semibold text-surface-900 dark:text-surface-100 text-base">
                {{ enrollment.pathway?.name || t('dashboard.enrollments.unknownPathway') }}
              </h3>
              <p class="text-sm text-surface-500">
                {{
                  t('dashboard.enrollments.modulesProgress', {
                    completed: enrollment.completedModules,
                    total: enrollment.totalModules,
                  })
                }}
              </p>
            </div>
            <Tag
              :value="enrollment.status.replace('_', ' ')"
              :severity="getStatusSeverity(enrollment.status)"
            />
          </div>
          <div class="space-y-1">
            <div class="flex justify-between text-sm">
              <span class="text-surface-500">{{ t('dashboard.enrollments.progressLabel') }}</span>
              <span class="font-medium text-surface-700 dark:text-surface-300" aria-hidden="true"
                >{{ enrollment.percentage }}%</span
              >
            </div>
            <ProgressBar
              :value="enrollment.percentage"
              :showValue="false"
              class="h-2"
              :aria-label="`Pathway progress: ${enrollment.percentage} percent complete`"
            />
          </div>
          <div class="flex items-center justify-between mt-3 text-sm">
            <span class="text-amber-600 dark:text-amber-400">
              <i class="pi pi-star mr-1" aria-hidden="true" />{{ enrollment.earnedPoints }} /
              {{ enrollment.maxPoints }} pts
            </span>
            <span class="text-surface-500">
              {{
                t('dashboard.enrollments.startedAt', {
                  when: formatDate(enrollment.startedAt || enrollment.enrolledAt),
                })
              }}
            </span>
          </div>
        </article>
      </li>
    </ul>
  </div>
</template>
