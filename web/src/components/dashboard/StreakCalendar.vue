<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  calendar: Record<string, number>
}>()

// Generate last 90 days grid as columns (weeks), each with 7 rows (days)
const weeks = computed(() => {
  const result: { date: string; count: number; day: number }[][] = []
  const today = new Date()
  const start = new Date(today)
  start.setDate(start.getDate() - 89)
  // Align to previous Sunday
  start.setDate(start.getDate() - start.getDay())

  let currentWeek: { date: string; count: number; day: number }[] = []
  const end = new Date(today)
  end.setDate(end.getDate() + 1)

  const cursor = new Date(start)
  while (cursor <= end) {
    const key = cursor.toISOString().split('T')[0] ?? ''
    currentWeek.push({
      date: key,
      count: props.calendar[key] ?? 0,
      day: cursor.getDay(),
    })
    if (currentWeek.length === 7) {
      result.push(currentWeek)
      currentWeek = []
    }
    cursor.setDate(cursor.getDate() + 1)
  }
  if (currentWeek.length > 0) result.push(currentWeek)
  return result
})

// Month labels positioned at the first week of each month
const monthLabels = computed(() => {
  const labels: { text: string; weekIndex: number }[] = []
  const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  let lastMonth = -1

  for (let i = 0; i < weeks.value.length; i++) {
    const firstDay = weeks.value[i]?.[0]
    if (!firstDay) continue
    const month = new Date(firstDay.date + 'T00:00:00').getMonth()
    if (month !== lastMonth) {
      labels.push({ text: monthNames[month] ?? '', weekIndex: i })
      lastMonth = month
    }
  }
  return labels
})

function getColor(count: number): string {
  if (count === 0) return 'bg-surface-200 dark:bg-surface-700'
  if (count === 1) return 'bg-green-200 dark:bg-green-900'
  if (count === 2) return 'bg-green-400 dark:bg-green-700'
  if (count === 3) return 'bg-green-500 dark:bg-green-600'
  return 'bg-green-600 dark:bg-green-500'
}

const totalSessions = computed(() =>
  Object.values(props.calendar).reduce((s, v) => s + v, 0)
)

const activeDays = computed(() =>
  Object.values(props.calendar).filter(v => v > 0).length
)
</script>

<template>
  <div class="streak-calendar">
    <!-- Month labels row -->
    <div class="month-row">
      <div class="day-label-spacer" />
      <div class="month-labels">
        <div
          v-for="(week, wi) in weeks"
          :key="'mh-' + wi"
          class="month-col"
        >
          <span
            v-if="monthLabels.some(m => m.weekIndex === wi)"
            class="text-xs text-surface-400"
          >
            {{ monthLabels.find(m => m.weekIndex === wi)?.text }}
          </span>
        </div>
      </div>
    </div>

    <!-- Grid: day labels + cells -->
    <div class="grid-row">
      <!-- Day-of-week labels -->
      <div class="day-labels">
        <div class="day-cell" />
        <div class="day-cell"><span class="text-xs text-surface-400">Mon</span></div>
        <div class="day-cell" />
        <div class="day-cell"><span class="text-xs text-surface-400">Wed</span></div>
        <div class="day-cell" />
        <div class="day-cell"><span class="text-xs text-surface-400">Fri</span></div>
        <div class="day-cell" />
      </div>

      <!-- Week columns -->
      <div class="week-columns">
        <div v-for="(week, wi) in weeks" :key="wi" class="week-col">
          <div
            v-for="day in week"
            :key="day.date"
            :class="[getColor(day.count), 'cell rounded-sm']"
            :title="`${day.date}: ${day.count} session${day.count !== 1 ? 's' : ''}`"
          />
        </div>
      </div>
    </div>

    <!-- Footer -->
    <div class="flex items-center justify-between mt-2 text-xs text-surface-500">
      <span>{{ activeDays }} active days</span>
      <div class="flex items-center gap-1">
        <span>Less</span>
        <div class="w-3 h-3 rounded-sm bg-surface-200 dark:bg-surface-700" />
        <div class="w-3 h-3 rounded-sm bg-green-200 dark:bg-green-900" />
        <div class="w-3 h-3 rounded-sm bg-green-400 dark:bg-green-700" />
        <div class="w-3 h-3 rounded-sm bg-green-600 dark:bg-green-500" />
        <span>More</span>
      </div>
      <span>{{ totalSessions }} sessions</span>
    </div>
  </div>
</template>

<style scoped>
.streak-calendar {
  width: 100%;
}

.month-row {
  display: flex;
  margin-bottom: 2px;
}

.day-label-spacer {
  flex-shrink: 0;
  width: 28px;
}

.month-labels {
  display: flex;
  flex: 1;
  gap: 2px;
}

.month-col {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}

.grid-row {
  display: flex;
  gap: 0;
}

.day-labels {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex-shrink: 0;
  width: 28px;
}

.day-cell {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding-right: 4px;
}

.week-columns {
  display: flex;
  flex: 1;
  gap: 2px;
}

.week-col {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.cell {
  width: 100%;
  aspect-ratio: 1;
}

.day-cell {
  aspect-ratio: 1;
}
</style>
