<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import { formatDuration, getDifficultySeverity } from '@/utils'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import Breadcrumb from '@volt/Breadcrumb.vue'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const route = useRoute()
const { formatNumber } = useFormatters()

interface Lab {
  id: string
  name: string
  description: string
  difficulty: 'beginner' | 'intermediate' | 'advanced'
  totalObjectives: number
  completedObjectives: number
  totalPoints: number
  earnedPoints: number
  status: 'not_started' | 'in_progress' | 'completed'
  estimatedDuration: number
}

// Mock data - would come from API
const course = ref({
  id: route.params['courseId'] as string || 'course-netsec',
  name: 'Network Security Fundamentals',
  description: 'Master the fundamentals of network security including firewalls, IDS/IPS, VPNs, and security monitoring.',
  icon: '🛡️',
  totalLabs: 8,
  completedLabs: 5,
  totalPoints: 10000,
  earnedPoints: 6800,
  estimatedHours: 12,
  timeSpentMinutes: 485
})

const labs = ref<Lab[]>([
  {
    id: 'lab-1',
    name: 'Network Fundamentals Review',
    description: 'Review TCP/IP, OSI model, and network protocols',
    difficulty: 'beginner',
    totalObjectives: 4,
    completedObjectives: 4,
    totalPoints: 800,
    earnedPoints: 800,
    status: 'completed',
    estimatedDuration: 30
  },
  {
    id: 'lab-2',
    name: 'Firewall Configuration 101',
    description: 'Learn to configure pfSense firewall rules',
    difficulty: 'intermediate',
    totalObjectives: 5,
    completedObjectives: 5,
    totalPoints: 1000,
    earnedPoints: 1000,
    status: 'completed',
    estimatedDuration: 45
  },
  {
    id: 'lab-3',
    name: 'Advanced Firewall Rules',
    description: 'Create complex firewall rules with NAT and port forwarding',
    difficulty: 'intermediate',
    totalObjectives: 6,
    completedObjectives: 6,
    totalPoints: 1200,
    earnedPoints: 1200,
    status: 'completed',
    estimatedDuration: 60
  },
  {
    id: 'lab-4',
    name: 'Intrusion Detection with Snort',
    description: 'Deploy and configure Snort IDS',
    difficulty: 'intermediate',
    totalObjectives: 5,
    completedObjectives: 5,
    totalPoints: 1200,
    earnedPoints: 1200,
    status: 'completed',
    estimatedDuration: 50
  },
  {
    id: 'lab-5',
    name: 'VPN Configuration',
    description: 'Set up site-to-site and remote access VPNs',
    difficulty: 'advanced',
    totalObjectives: 6,
    completedObjectives: 6,
    totalPoints: 1500,
    earnedPoints: 1400,
    status: 'completed',
    estimatedDuration: 75
  },
  {
    id: 'lab-6',
    name: 'Security Monitoring & SIEM',
    description: 'Configure centralized logging and monitoring',
    difficulty: 'advanced',
    totalObjectives: 5,
    completedObjectives: 3,
    totalPoints: 1500,
    earnedPoints: 1000,
    status: 'in_progress',
    estimatedDuration: 90
  },
  {
    id: 'lab-7',
    name: 'Incident Response Basics',
    description: 'Learn incident detection and response procedures',
    difficulty: 'advanced',
    totalObjectives: 6,
    completedObjectives: 0,
    totalPoints: 1400,
    earnedPoints: 0,
    status: 'not_started',
    estimatedDuration: 80
  },
  {
    id: 'lab-8',
    name: 'Capstone: Secure Network Design',
    description: 'Design and implement a complete secure network',
    difficulty: 'advanced',
    totalObjectives: 8,
    completedObjectives: 0,
    totalPoints: 1400,
    earnedPoints: 0,
    status: 'not_started',
    estimatedDuration: 120
  }
])

const breadcrumbItems = computed(() => [
  { label: t('courseProgress.breadcrumbProgress'), route: '/progress' },
  { label: course.value.name }
])

const progress = computed(() => {
  return Math.round((course.value.completedLabs / course.value.totalLabs) * 100)
})

const pointsProgress = computed(() => {
  return Math.round((course.value.earnedPoints / course.value.totalPoints) * 100)
})

function getLabProgress(lab: Lab): number {
  return Math.round((lab.completedObjectives / lab.totalObjectives) * 100)
}

// formatDuration, getDifficultySeverity imported from @/utils
</script>

<template>
  <div class="space-y-6">
    <!-- Breadcrumb -->
    <Breadcrumb :model="breadcrumbItems" />

    <!-- Course Header -->
    <Card>
      <template #content>
        <div class="flex flex-col lg:flex-row lg:items-start gap-6">
          <!-- Course Info -->
          <div class="flex-1">
            <div class="flex items-center gap-3 mb-3">
              <span class="text-4xl">{{ course.icon }}</span>
              <div>
                <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ course.name }}</h1>
                <p class="text-sm text-surface-500">{{ t('courseProgress.labsCount', { count: course.totalLabs }) }}</p>
              </div>
            </div>
            <p class="text-surface-600 dark:text-surface-400">{{ course.description }}</p>

            <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
              <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-3 text-center">
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('courseProgress.stats.labsCompleteValue', { completed: course.completedLabs, total: course.totalLabs }) }}</p>
                <p class="text-xs text-surface-500">{{ t('courseProgress.stats.labsCompleteLabel') }}</p>
              </div>
              <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-3 text-center">
                <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ formatNumber(course.earnedPoints) }}</p>
                <p class="text-xs text-surface-500">{{ t('courseProgress.stats.pointsEarnedLabel') }}</p>
              </div>
              <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-3 text-center">
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ formatDuration(course.timeSpentMinutes) }}</p>
                <p class="text-xs text-surface-500">{{ t('courseProgress.stats.timeSpentLabel') }}</p>
              </div>
              <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-3 text-center">
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('courseProgress.stats.totalDurationValue', { hours: course.estimatedHours }) }}</p>
                <p class="text-xs text-surface-500">{{ t('courseProgress.stats.totalDurationLabel') }}</p>
              </div>
            </div>
          </div>

          <!-- Progress Circle -->
          <div class="flex-shrink-0 flex flex-col items-center">
            <div class="relative w-40 h-40">
              <svg class="w-40 h-40 transform -rotate-90">
                <circle
                  cx="80"
                  cy="80"
                  r="70"
                  stroke="currentColor"
                  stroke-width="14"
                  fill="none"
                  class="text-surface-200 dark:text-surface-700"
                />
                <circle
                  cx="80"
                  cy="80"
                  r="70"
                  stroke="currentColor"
                  stroke-width="14"
                  fill="none"
                  stroke-linecap="round"
                  class="text-green-500"
                  :stroke-dasharray="`${progress * 4.4} 440`"
                />
              </svg>
              <div class="absolute inset-0 flex flex-col items-center justify-center">
                <span class="text-4xl font-bold text-surface-900 dark:text-surface-100">{{ progress }}%</span>
                <span class="text-sm text-surface-500">{{ t('courseProgress.progressComplete') }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Overall Progress Bars -->
        <div class="mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <div class="flex justify-between text-sm mb-2">
                <span class="text-surface-600 dark:text-surface-400">{{ t('courseProgress.labCompletionLabel') }}</span>
                <span class="font-medium text-surface-900 dark:text-surface-100">{{ t('courseProgress.stats.labsCompleteValue', { completed: course.completedLabs, total: course.totalLabs }) }}</span>
              </div>
              <ProgressBar :value="progress" :showValue="false" />
            </div>
            <div>
              <div class="flex justify-between text-sm mb-2">
                <span class="text-surface-600 dark:text-surface-400">{{ t('courseProgress.pointsProgressLabel') }}</span>
                <span class="font-medium text-surface-900 dark:text-surface-100">{{ t('courseProgress.pointsProgressValue', { earned: formatNumber(course.earnedPoints), total: formatNumber(course.totalPoints) }) }}</span>
              </div>
              <ProgressBar :value="pointsProgress" :showValue="false" />
            </div>
          </div>
        </div>
      </template>
    </Card>

    <!-- Labs List -->
    <Card>
      <template #title>
        <div class="flex items-center gap-2">
          <i class="pi pi-book text-xl" />
          {{ t('courseProgress.labsHeading') }}
        </div>
      </template>
      <template #content>
        <div class="divide-y divide-surface-100 dark:divide-surface-700 -mx-4">
          <RouterLink
            v-for="(lab, index) in labs"
            :key="lab.id"
            :to="`/progress/lab/${lab.id}`"
            class="flex items-center gap-4 p-4 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
          >
            <!-- Lab Number & Status -->
            <div class="flex-shrink-0 relative">
              <div
                class="w-12 h-12 rounded-full flex items-center justify-center font-bold text-lg"
                :class="{
                  'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300': lab.status === 'completed',
                  'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300': lab.status === 'in_progress',
                  'bg-surface-100 dark:bg-surface-700 text-surface-500': lab.status === 'not_started'
                }"
              >
                <i v-if="lab.status === 'completed'" class="pi pi-check" />
                <i v-else-if="lab.status === 'in_progress'" class="pi pi-play" />
                <span v-else>{{ index + 1 }}</span>
              </div>
              <!-- Connector line -->
              <div
                v-if="index < labs.length - 1"
                class="absolute top-12 left-1/2 w-0.5 h-6 -translate-x-1/2"
                :class="{
                  'bg-green-300 dark:bg-green-700': lab.status === 'completed',
                  'bg-surface-200 dark:bg-surface-600': lab.status !== 'completed'
                }"
              />
            </div>

            <!-- Lab Info -->
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2">
                <h3 class="font-medium text-surface-900 dark:text-surface-100 truncate">{{ lab.name }}</h3>
                <Tag :value="lab.difficulty" :severity="getDifficultySeverity(lab.difficulty)" />
              </div>
              <p class="text-sm text-surface-500 truncate">{{ lab.description }}</p>

              <!-- Progress bar for in-progress or completed labs -->
              <div v-if="lab.status !== 'not_started'" class="mt-2 max-w-xs">
                <div class="flex justify-between text-xs text-surface-500 mb-1">
                  <span>{{ t('courseProgress.labObjectives', { completed: lab.completedObjectives, total: lab.totalObjectives }) }}</span>
                  <span>{{ getLabProgress(lab) }}%</span>
                </div>
                <ProgressBar :value="getLabProgress(lab)" :showValue="false" />
              </div>
            </div>

            <!-- Stats -->
            <div class="flex-shrink-0 text-right hidden md:block">
              <p class="text-sm font-medium text-surface-900 dark:text-surface-100">
                {{ t('courseProgress.labPoints', { earned: formatNumber(lab.earnedPoints), total: formatNumber(lab.totalPoints) }) }}
              </p>
              <p class="text-xs text-surface-500">
                <i class="pi pi-clock mr-1" />{{ formatDuration(lab.estimatedDuration) }}
              </p>
            </div>

            <!-- Arrow -->
            <div class="flex-shrink-0 text-surface-400">
              <i class="pi pi-chevron-right" />
            </div>
          </RouterLink>
        </div>
      </template>
    </Card>
  </div>
</template>
