<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

export interface TimeSlot {
  time: string
  status: 'available' | 'reserved' | 'selected'
  reservedBy?: string
}

export interface CalendarDay {
  date: number
  month: number
  year: number
  isCurrentMonth: boolean
  isToday: boolean
  slots: TimeSlot[]
}

const props = defineProps<{
  selectedSlot?: { day: CalendarDay; slot: TimeSlot } | null
}>()

const emit = defineEmits<{
  (e: 'select', day: CalendarDay, slot: TimeSlot): void
}>()

const currentDate = ref(new Date())
const viewStartDate = ref(new Date())

// Generate time slots for a day
function generateTimeSlots(dayIndex: number): TimeSlot[] {
  const times = ['8:00 AM', '9:00 AM', '10:00 AM', '11:00 AM', '12:00 PM', '1:00 PM', '2:00 PM', '3:00 PM', '4:00 PM', '5:00 PM']

  // Mock some reserved slots based on day
  const reservedPatterns: Record<number, number[]> = {
    0: [2, 3], // Sunday: 10am, 11am reserved
    1: [],     // Monday: all available
    2: [0, 1, 2], // Tuesday: 8am-10am reserved
    3: [4, 5], // Wednesday: 12pm, 1pm reserved
    4: [6, 7], // Thursday: 2pm, 3pm reserved
    5: [3],    // Friday: 11am reserved
    6: [],     // Saturday: all available
  }

  return times.map((time, index) => {
    const isReserved = reservedPatterns[dayIndex]?.includes(index)
    const slot: TimeSlot = {
      time,
      status: isReserved ? 'reserved' : 'available',
    }
    if (isReserved) {
      slot.reservedBy = 'Another User'
    }
    return slot
  })
}

// Get the current week's days
const weekDays = computed(() => {
  const days: CalendarDay[] = []
  const start = new Date(viewStartDate.value)

  // Adjust to start of week (Sunday)
  const dayOfWeek = start.getDay()
  start.setDate(start.getDate() - dayOfWeek)

  for (let i = 0; i < 7; i++) {
    const date = new Date(start)
    date.setDate(start.getDate() + i)

    days.push({
      date: date.getDate(),
      month: date.getMonth(),
      year: date.getFullYear(),
      isCurrentMonth: date.getMonth() === currentDate.value.getMonth(),
      isToday: date.toDateString() === new Date().toDateString(),
      slots: generateTimeSlots(i),
    })
  }

  return days
})

const { t, locale } = useI18n()

const weekDateRange = computed(() => {
  const start = weekDays.value[0]
  const end = weekDays.value[6]

  if (!start || !end) return ''

  const startDate = new Date(start.year, start.month, start.date)
  const endDate = new Date(end.year, end.month, end.date)

  const fmt = new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric' })
  return t('reservations.calendar.weekRange', {
    start: fmt.format(startDate),
    end: fmt.format(endDate),
    year: end.year,
  })
})

const currentMonthYear = computed(() =>
  new Intl.DateTimeFormat(locale.value, { month: 'long', year: 'numeric' }).format(
    currentDate.value,
  ),
)

function previousWeek() {
  const newDate = new Date(viewStartDate.value)
  newDate.setDate(newDate.getDate() - 7)
  viewStartDate.value = newDate
}

function nextWeek() {
  const newDate = new Date(viewStartDate.value)
  newDate.setDate(newDate.getDate() + 7)
  viewStartDate.value = newDate
}

function previousMonth() {
  const newDate = new Date(currentDate.value)
  newDate.setMonth(newDate.getMonth() - 1)
  currentDate.value = newDate
  viewStartDate.value = new Date(newDate.getFullYear(), newDate.getMonth(), 1)
}

function nextMonth() {
  const newDate = new Date(currentDate.value)
  newDate.setMonth(newDate.getMonth() + 1)
  currentDate.value = newDate
  viewStartDate.value = new Date(newDate.getFullYear(), newDate.getMonth(), 1)
}

function selectSlot(day: CalendarDay, slot: TimeSlot) {
  if (slot.status === 'reserved') return
  emit('select', day, slot)
}

function isSelected(day: CalendarDay, slot: TimeSlot): boolean {
  if (!props.selectedSlot) return false
  return props.selectedSlot.day.date === day.date &&
         props.selectedSlot.day.month === day.month &&
         props.selectedSlot.slot.time === slot.time
}

const DAY_KEYS = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'] as const
const dayNames = computed(() => DAY_KEYS.map(k => t(`reservations.calendar.dayNames.${k}`)))
</script>

<template>
  <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
    <!-- Month Navigation -->
    <div class="flex justify-between items-center mb-6">
      <!-- Intl.DateTimeFormat output is always a real language (no pseudo-localization), skip sweep. -->
      <h2 class="text-lg font-medium text-gray-900 dark:text-white" data-pseudo-skip>{{ currentMonthYear }}</h2>
      <div class="flex space-x-2">
        <button
          @click="previousMonth"
          :aria-label="t('reservations.calendar.nav.previousMonthAria')"
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-surface-700 transition-colors"
        >
          <svg class="h-5 w-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <button
          @click="nextMonth"
          :aria-label="t('reservations.calendar.nav.nextMonthAria')"
          class="p-2 rounded-md hover:bg-gray-100 dark:hover:bg-surface-700 transition-colors"
        >
          <svg class="h-5 w-5 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Week Navigation -->
    <div class="flex justify-between items-center mb-4">
      <button
        @click="previousWeek"
        class="px-4 py-2 text-sm bg-surface-100 dark:bg-surface-700 text-gray-700 dark:text-gray-300 rounded-md hover:bg-surface-200 dark:hover:bg-surface-600 transition-colors"
      >
        {{ t('reservations.calendar.nav.previousWeek') }}
      </button>
      <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ weekDateRange }}</h3>
      <button
        @click="nextWeek"
        class="px-4 py-2 text-sm bg-surface-100 dark:bg-surface-700 text-gray-700 dark:text-gray-300 rounded-md hover:bg-surface-200 dark:hover:bg-surface-600 transition-colors"
      >
        {{ t('reservations.calendar.nav.nextWeek') }}
      </button>
    </div>

    <!-- Calendar Grid -->
    <div class="grid grid-cols-7 gap-2 mb-4">
      <!-- Day Headers -->
      <div
        v-for="day in dayNames"
        :key="day"
        class="text-center text-sm font-medium text-gray-500 dark:text-gray-400 py-2"
      >
        {{ day }}
      </div>

      <!-- Calendar Days -->
      <div
        v-for="day in weekDays"
        :key="`${day.month}-${day.date}`"
        class="border dark:border-surface-700 rounded-md p-2 min-h-[280px]"
        :class="{
          'bg-blue-50 dark:bg-blue-900/20': day.isToday,
          'bg-gray-50 dark:bg-surface-800/50': !day.isCurrentMonth && !day.isToday,
        }"
      >
        <div
          class="font-medium text-sm mb-2"
          :class="{
            'text-blue-600 dark:text-blue-400': day.isToday,
            'text-gray-400 dark:text-gray-500': !day.isCurrentMonth,
            'text-gray-900 dark:text-white': day.isCurrentMonth && !day.isToday,
          }"
        >
          {{ day.date }}
        </div>
        <div class="space-y-1">
          <button
            v-for="slot in day.slots"
            :key="slot.time"
            @click="selectSlot(day, slot)"
            class="w-full rounded-md p-1.5 text-xs transition-all duration-200"
            :class="{
              'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 hover:bg-green-200 dark:hover:bg-green-900/50 cursor-pointer': slot.status === 'available' && !isSelected(day, slot),
              'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 cursor-not-allowed': slot.status === 'reserved',
              'bg-blue-200 dark:bg-blue-800 text-blue-700 dark:text-blue-300 border-2 border-blue-500 dark:border-blue-400': isSelected(day, slot),
            }"
            :disabled="slot.status === 'reserved'"
            :title="slot.status === 'reserved'
              ? t('reservations.calendar.slot.reservedTitle', { name: slot.reservedBy ?? '' })
              : t('reservations.calendar.slot.selectTitle')"
          >
            {{ slot.time }}
          </button>
        </div>
      </div>
    </div>

    <!-- Legend -->
    <div class="flex flex-wrap gap-6 text-sm">
      <div class="flex items-center">
        <div class="w-4 h-4 rounded bg-green-100 dark:bg-green-900/30 mr-2"></div>
        <span class="text-gray-600 dark:text-gray-400">{{ t('reservations.calendar.legend.available') }}</span>
      </div>
      <div class="flex items-center">
        <div class="w-4 h-4 rounded bg-red-100 dark:bg-red-900/30 mr-2"></div>
        <span class="text-gray-600 dark:text-gray-400">{{ t('reservations.calendar.legend.reserved') }}</span>
      </div>
      <div class="flex items-center">
        <div class="w-4 h-4 rounded bg-blue-200 dark:bg-blue-800 border-2 border-blue-500 mr-2"></div>
        <span class="text-gray-600 dark:text-gray-400">{{ t('reservations.calendar.legend.selected') }}</span>
      </div>
    </div>
  </div>
</template>
