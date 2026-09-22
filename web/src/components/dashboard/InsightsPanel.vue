<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { dashboardApi, type InsightsResponse } from '@/api/domains/dashboard'
import SkillRadarChart from './SkillRadarChart.vue'
import StreakCalendar from './StreakCalendar.vue'
import Card from '@volt/Card.vue'
import Tag from '@volt/Tag.vue'

const insights = ref<InsightsResponse | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    insights.value = await dashboardApi.getInsights()
  } catch (e) {
    error.value = 'Failed to load insights'
    console.warn('Failed to load insights:', e)
  } finally {
    loading.value = false
  }
})

function formatMinutes(mins: number): string {
  const h = Math.floor(mins / 60)
  const m = mins % 60
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
}
</script>

<template>
  <div class="space-y-6">
    <!-- Loading -->
    <div v-if="loading" class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <Card v-for="i in 4" :key="i">
        <template #content>
          <div class="animate-pulse space-y-3">
            <div class="h-4 bg-surface-200 dark:bg-surface-700 rounded w-1/3" />
            <div class="h-32 bg-surface-200 dark:bg-surface-700 rounded" />
          </div>
        </template>
      </Card>
    </div>

    <template v-else-if="insights">
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Skill Radar Chart -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-chart-pie text-primary-500" />
              <span>Skill Proficiency</span>
            </div>
          </template>
          <template #content>
            <SkillRadarChart :skills="insights.skills" />
          </template>
        </Card>

        <!-- Strengths & Weaknesses -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-chart-bar text-green-500" />
              <span>Strengths & Weaknesses</span>
            </div>
          </template>
          <template #content>
            <div class="space-y-4">
              <div>
                <h4 class="text-sm font-medium text-surface-600 dark:text-surface-400 mb-2">Strongest Areas</h4>
                <div class="flex flex-wrap gap-2">
                  <Tag
                    v-for="tag in insights.strengths"
                    :key="tag"
                    :value="tag"
                    severity="success"
                  />
                </div>
                <p v-if="insights.strengths.length === 0" class="text-xs text-surface-500">Complete more labs to identify strengths</p>
              </div>
              <div>
                <h4 class="text-sm font-medium text-surface-600 dark:text-surface-400 mb-2">Areas to Improve</h4>
                <div class="flex flex-wrap gap-2">
                  <Tag
                    v-for="tag in insights.weaknesses"
                    :key="tag"
                    :value="tag"
                    severity="warn"
                  />
                </div>
                <p v-if="insights.weaknesses.length === 0" class="text-xs text-surface-500">Complete more labs to identify weaknesses</p>
              </div>

              <!-- Skill bars -->
              <div class="space-y-2 mt-4">
                <div v-for="skill in insights.skills.slice(0, 6)" :key="skill.tag" class="flex items-center gap-3">
                  <span class="text-xs text-surface-600 dark:text-surface-400 w-20 truncate">{{ skill.tag }}</span>
                  <div class="flex-1 bg-surface-200 dark:bg-surface-700 rounded-full h-2">
                    <div
                      class="bg-primary-500 h-2 rounded-full transition-all"
                      :style="{ width: `${skill.proficiency}%` }"
                    />
                  </div>
                  <span class="text-xs font-medium text-surface-700 dark:text-surface-300 w-8 text-right">{{ Math.round(skill.proficiency) }}%</span>
                </div>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Streak Calendar -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-calendar text-orange-500" />
              <span>Activity (Last 90 Days)</span>
            </div>
          </template>
          <template #content>
            <StreakCalendar :calendar="insights.streakCalendar" />
          </template>
        </Card>

        <!-- Weekly Time Trend -->
        <Card>
          <template #title>
            <div class="flex items-center gap-2">
              <i class="pi pi-clock text-blue-500" />
              <span>Weekly Lab Time</span>
            </div>
          </template>
          <template #content>
            <div v-if="insights.weeklyTime.length === 0" class="text-sm text-surface-500 py-4 text-center">
              No time data yet
            </div>
            <div v-else class="space-y-3">
              <div v-for="entry in insights.weeklyTime" :key="entry.week" class="flex items-center gap-3">
                <span class="text-xs text-surface-600 dark:text-surface-400 w-20">{{ entry.week }}</span>
                <div class="flex-1 bg-surface-200 dark:bg-surface-700 rounded-full h-3">
                  <div
                    class="bg-blue-500 h-3 rounded-full transition-all"
                    :style="{ width: `${Math.min((entry.minutes / Math.max(...insights!.weeklyTime.map(w => w.minutes))) * 100, 100)}%` }"
                  />
                </div>
                <span class="text-xs font-medium text-surface-700 dark:text-surface-300 w-12 text-right">{{ formatMinutes(entry.minutes) }}</span>
              </div>
            </div>
          </template>
        </Card>
      </div>
    </template>
  </div>
</template>
