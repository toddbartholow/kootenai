<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { StudentSummary } from '@/api'
import { useFormatters } from '@/composables/useFormatters'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import DataTable from '@volt/DataTable.vue'
import Column from 'primevue/column'

defineProps<{
  students: StudentSummary[]
  studentFilter: 'all' | 'active' | 'at-risk' | 'inactive' | 'excelling'
  getStatusSeverity: (status: string) => 'success' | 'info' | 'warn' | 'danger' | 'secondary'
  getStatusLabel: (status: string) => string
  formatDate: (dateStr?: string) => string
}>()

const emit = defineEmits<{
  'update:studentFilter': [value: 'all' | 'active' | 'at-risk' | 'inactive' | 'excelling']
}>()

const { t } = useI18n()
const { formatNumber } = useFormatters()
</script>

<template>
  <Card>
    <template #title>
      <div class="flex items-center justify-between">
        <span>{{ t('instructorDashboard.students.cardTitle') }}</span>
        <div class="flex gap-2">
          <Button
            v-for="filter in ['all', 'active', 'excelling', 'at-risk', 'inactive'] as const"
            :key="filter"
            :label="
              filter === 'all' ? t('instructorDashboard.filters.all') : getStatusLabel(filter)
            "
            :severity="studentFilter === filter ? 'primary' : 'secondary'"
            size="small"
            @click="emit('update:studentFilter', filter)"
          />
        </div>
      </div>
    </template>
    <template #content>
      <DataTable
        :value="students"
        :paginator="students.length > 10"
        :rows="10"
        :rowsPerPageOptions="[10, 25, 50]"
        sortable
        class="text-sm"
      >
        <Column field="name" :header="t('instructorDashboard.students.columnName')" sortable>
          <template #body="{ data }">
            <div class="font-medium text-surface-900 dark:text-surface-100">
              {{ data.name }}
            </div>
            <div class="text-xs text-surface-500">{{ data.email }}</div>
          </template>
        </Column>
        <Column field="status" :header="t('instructorDashboard.students.columnStatus')" sortable>
          <template #body="{ data }">
            <Tag :value="getStatusLabel(data.status)" :severity="getStatusSeverity(data.status)" />
          </template>
        </Column>
        <Column
          field="labsCompleted"
          :header="t('instructorDashboard.students.columnLabs')"
          sortable
        >
          <template #body="{ data }">
            <span class="font-medium">{{ data.labsCompleted }}</span>
            <span class="text-surface-500"> / {{ data.labsAttempted }}</span>
          </template>
        </Column>
        <Column
          field="averageScore"
          :header="t('instructorDashboard.students.columnAvgScore')"
          sortable
        >
          <template #body="{ data }">
            <div class="flex items-center gap-2">
              <ProgressBar :value="data.averageScore" class="w-16 h-2" />
              <span class="text-xs">{{ data.averageScore }}%</span>
            </div>
          </template>
        </Column>
        <Column
          field="currentStreak"
          :header="t('instructorDashboard.students.columnStreak')"
          sortable
        >
          <template #body="{ data }">
            <div v-if="data.currentStreak > 0" class="flex items-center gap-1">
              <i class="pi pi-bolt text-orange-500" />
              <span>{{
                t('instructorDashboard.students.streakDays', { count: data.currentStreak })
              }}</span>
            </div>
            <span v-else class="text-surface-400">{{
              t('instructorDashboard.students.dashPlaceholder')
            }}</span>
          </template>
        </Column>
        <Column
          field="lastActiveAt"
          :header="t('instructorDashboard.students.columnLastActive')"
          sortable
        >
          <template #body="{ data }">
            <span
              :class="{
                'text-red-500':
                  !data.lastActiveAt ||
                  new Date(data.lastActiveAt) < new Date(Date.now() - 7 * 24 * 60 * 60 * 1000),
              }"
            >
              {{ formatDate(data.lastActiveAt) }}
            </span>
          </template>
        </Column>
        <Column
          field="totalPoints"
          :header="t('instructorDashboard.students.columnPoints')"
          sortable
        >
          <template #body="{ data }">
            <span class="font-bold text-primary-600 dark:text-primary-400">
              {{ formatNumber(data.totalPoints) }}
            </span>
          </template>
        </Column>
      </DataTable>
    </template>
  </Card>
</template>
