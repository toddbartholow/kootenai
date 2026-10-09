<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import { getDifficultySeverity } from '@/utils/status'
import ProgressBar from '@volt/ProgressBar.vue'
import Breadcrumb from '@volt/Breadcrumb.vue'
import Message from '@volt/Message.vue'
import ObjectiveItem, { type Objective } from '../components/progress/ObjectiveItem.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

// Mock data - would come from API
const lab = ref({
  id: route.params['labId'] as string || 'lab-firewall-101',
  name: 'Firewall Configuration 101',
  description: 'Learn to configure pfSense firewall rules to protect your network',
  course: {
    id: 'course-netsec',
    name: 'Network Security Fundamentals'
  },
  difficulty: 'intermediate',
  estimatedDuration: 45,
  totalPoints: 1000,
  earnedPoints: 500,
  timeSpent: 28,
  status: 'in_progress' as const
})

const objectives = ref<Objective[]>([
  {
    id: 'obj-1',
    name: 'Access pfSense Web Interface',
    description: 'Login to the pfSense firewall using the provided credentials',
    order: 1,
    points: 100,
    status: 'completed',
    completedAt: '2024-01-15T10:30:00Z'
  },
  {
    id: 'obj-2',
    name: 'Create LAN to WAN Allow Rule',
    description: 'Configure a firewall rule to allow HTTP traffic from LAN to WAN',
    order: 2,
    points: 200,
    status: 'completed',
    completedAt: '2024-01-15T10:45:00Z'
  },
  {
    id: 'obj-3',
    name: 'Test HTTP Connectivity',
    description: 'Verify that the allow rule works by accessing an external website',
    order: 3,
    points: 200,
    status: 'current'
  },
  {
    id: 'obj-4',
    name: 'Create ICMP Block Rule',
    description: 'Block all ICMP (ping) traffic from external sources',
    order: 4,
    points: 200,
    status: 'locked'
  },
  {
    id: 'obj-5',
    name: 'Verify Security Configuration',
    description: 'Run the verification script to confirm all rules are correctly configured',
    order: 5,
    points: 300,
    status: 'locked'
  }
])

const breadcrumbItems = computed(() => [
  { label: t('labProgress.breadcrumbProgress'), route: '/progress' },
  { label: lab.value.course.name, route: `/progress/course/${lab.value.course.id}` },
  { label: lab.value.name }
])

const progress = computed(() => {
  const completed = objectives.value.filter(o => o.status === 'completed').length
  return Math.round((completed / objectives.value.length) * 100)
})

const completedObjectives = computed(() =>
  objectives.value.filter(o => o.status === 'completed').length
)

const currentObjective = computed(() =>
  objectives.value.find(o => o.status === 'current')
)

function goBack() {
  router.push(`/progress/course/${lab.value.course.id}`)
}

// getDifficultySeverity imported from @/utils/status
</script>

<template>
  <div class="space-y-6">
    <!-- Breadcrumb -->
    <Breadcrumb :model="breadcrumbItems" />

    <!-- Lab Header -->
    <Card>
      <template #content>
        <div class="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
          <div class="flex-1">
            <div class="flex items-center gap-3 mb-2">
              <i class="pi pi-flask text-3xl text-primary-500" />
              <div>
                <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ lab.name }}</h1>
                <p class="text-sm text-surface-500">{{ lab.course.name }}</p>
              </div>
            </div>
            <p class="text-surface-600 dark:text-surface-400 mt-2">{{ lab.description }}</p>

            <div class="flex flex-wrap gap-4 mt-4 text-sm">
              <Tag :value="lab.difficulty" :severity="getDifficultySeverity(lab.difficulty)" />
              <span class="text-surface-500 flex items-center gap-1">
                <i class="pi pi-clock" />
                {{ t('labProgress.estimatedMinutes', { count: lab.estimatedDuration }) }}
              </span>
              <span class="text-surface-500 flex items-center gap-1">
                <i class="pi pi-bolt" />
                {{ t('labProgress.timeSpentMinutes', { count: lab.timeSpent }) }}
              </span>
            </div>
          </div>

          <!-- Progress Circle -->
          <div class="flex-shrink-0 flex flex-col items-center">
            <div class="relative w-32 h-32">
              <svg class="w-32 h-32 transform -rotate-90">
                <circle
                  cx="64"
                  cy="64"
                  r="56"
                  stroke="currentColor"
                  stroke-width="12"
                  fill="none"
                  class="text-surface-200 dark:text-surface-700"
                />
                <circle
                  cx="64"
                  cy="64"
                  r="56"
                  stroke="currentColor"
                  stroke-width="12"
                  fill="none"
                  stroke-linecap="round"
                  class="text-blue-600"
                  :stroke-dasharray="`${progress * 3.52} 352`"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center">
                <span class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ progress }}%</span>
                <span class="text-xs text-surface-500">{{ t('labProgress.progressComplete') }}</span>
              </div>
            </div>
            <div class="text-center mt-2">
              <p class="text-lg font-bold text-surface-900 dark:text-surface-100">{{ lab.earnedPoints }} / {{ lab.totalPoints }}</p>
              <p class="text-xs text-surface-500">{{ t('labProgress.pointsEarned') }}</p>
            </div>
          </div>
        </div>
      </template>
    </Card>

    <!-- Objectives List -->
    <Card>
      <template #title>
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <i class="pi pi-bullseye text-xl" />
            {{ t('labProgress.objectivesHeading') }}
          </div>
          <span class="text-sm font-normal text-surface-500">
            {{ t('labProgress.objectivesCompleted', { completed: completedObjectives, total: objectives.length }) }}
          </span>
        </div>
      </template>
      <template #content>
        <!-- Progress Bar -->
        <div class="mb-6">
          <ProgressBar :value="progress" :showValue="false" />
        </div>

        <!-- Objectives -->
        <div class="space-y-3">
          <ObjectiveItem
            v-for="objective in objectives"
            :key="objective.id"
            :objective="objective"
          />
        </div>
      </template>
    </Card>

    <!-- Hint Section -->
    <Message v-if="currentObjective" severity="warn" :closable="false">
      <template #messageicon>
        <i class="pi pi-lightbulb text-2xl" />
      </template>
      <div>
        <h3 class="font-medium">{{ t('labProgress.hint.heading') }}</h3>
        <p class="text-sm mt-1">
          {{ t('labProgress.hint.workingOn') }} <strong>{{ currentObjective.name }}</strong>
        </p>
        <Button
          :label="t('labProgress.hint.showHint')"
          link
          size="small"
          class="mt-2 p-0"
        />
      </div>
    </Message>

    <!-- Action Buttons -->
    <div class="flex gap-4">
      <Button
        @click="goBack"
        icon="pi pi-arrow-left"
        :label="t('labProgress.actions.backToCourse')"
        severity="secondary"
      />
      <RouterLink to="/pods">
        <Button
          icon="pi pi-play"
          :label="t('labProgress.actions.continueLab')"
        />
      </RouterLink>
    </div>
  </div>
</template>
