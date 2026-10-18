<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  eventsApi,
  type MonitoringEvent,
  type EventStatsResponse,
  type EventsFilterParams,
} from '@/api'
import { useWebSocket, type MonitoringEventPayload } from '@/composables/useWebSocket'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDate: fmtDate, formatTime: fmtTime, formatDateTime } = useFormatters()

// State
const events = ref<MonitoringEvent[]>([])
const stats = ref<EventStatsResponse | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const autoRefresh = ref(true)
const refreshInterval = ref<number | null>(null)
const useRealtimeUpdates = ref(true)
const selectedEvent = ref<MonitoringEvent | null>(null)
const showEventDetail = ref(false)

// Filter state
const filters = ref<EventsFilterParams>({
  limit: 50,
  eventType: '',
  podId: '',
  sessionId: '',
})

// Computed
const eventTypeOptions = ['', 'syscheck', 'audit', 'auth', 'ssh', 'generic']

const filteredEvents = computed(() => {
  return events.value.filter(event => {
    if (filters.value.eventType && event.eventType !== filters.value.eventType) {
      return false
    }
    if (filters.value.podId && !event.podId?.includes(filters.value.podId)) {
      return false
    }
    if (filters.value.sessionId && !event.sessionId?.includes(filters.value.sessionId)) {
      return false
    }
    return true
  })
})

const eventTypeColors: Record<string, string> = {
  syscheck: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
  audit: 'bg-accent-100 text-accent-800 dark:bg-accent-900 dark:text-accent-300',
  auth: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
  ssh: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
  generic: 'bg-surface-100 text-surface-800 dark:bg-surface-700 dark:text-surface-300',
}

const getEventTypeColor = (type: string) => {
  return eventTypeColors[type] || eventTypeColors['generic']
}

// Methods
const fetchEvents = async () => {
  try {
    const params: EventsFilterParams = {}
    if (filters.value.limit) params.limit = filters.value.limit
    if (filters.value.eventType) params.eventType = filters.value.eventType
    if (filters.value.podId) params.podId = filters.value.podId
    if (filters.value.sessionId) params.sessionId = filters.value.sessionId

    const response = await eventsApi.list(params)
    events.value = response.events
  } catch (e) {
    console.error('Failed to fetch events:', e)
    error.value = t('eventsMonitoring.loadFailed')
  }
}

const fetchStats = async () => {
  try {
    stats.value = await eventsApi.getStats()
  } catch (e) {
    console.error('Failed to fetch stats:', e)
  }
}

const fetchAll = async () => {
  loading.value = true
  error.value = null
  await Promise.all([fetchEvents(), fetchStats()])
  loading.value = false
}

const formatTime = (timestamp: string) => fmtTime(timestamp)

const formatDate = (timestamp: string) => fmtDate(timestamp)

const toggleAutoRefresh = () => {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

const startAutoRefresh = () => {
  if (refreshInterval.value) return
  refreshInterval.value = window.setInterval(() => {
    fetchAll()
  }, 5000) // Refresh every 5 seconds
}

const stopAutoRefresh = () => {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

const applyFilters = () => {
  fetchEvents()
}

const clearFilters = () => {
  filters.value = {
    limit: 50,
    eventType: '',
    podId: '',
    sessionId: '',
  }
  fetchEvents()
}

const openEventDetail = (event: MonitoringEvent) => {
  selectedEvent.value = event
  showEventDetail.value = true
}

const closeEventDetail = () => {
  showEventDetail.value = false
  selectedEvent.value = null
}

// WebSocket for real-time updates
const handleMonitoringEvent = (eventPayload: MonitoringEventPayload) => {
  if (!useRealtimeUpdates.value) return

  // Convert payload to MonitoringEvent type
  const newEvent: MonitoringEvent = {
    id: eventPayload.id,
    timestamp: eventPayload.timestamp,
    podId: eventPayload.podId,
    sessionId: eventPayload.sessionId || '',
    vmName: eventPayload.vmName,
    eventType: eventPayload.eventType,
    ruleId: eventPayload.ruleId || '',
    ruleLevel: eventPayload.ruleLevel || 0,
    description: eventPayload.description || '',
    processed: eventPayload.processed,
    matchedCheckpoints: eventPayload.matchedCheckpoints || [],
  }

  // Add to beginning of list
  events.value = [
    newEvent,
    ...events.value.slice(0, filters.value.limit ? filters.value.limit - 1 : 49),
  ]

  // Update stats
  if (stats.value) {
    stats.value.eventsToday++
    stats.value.eventsThisHour++
    stats.value.totalEvents++
    if (newEvent.processed) {
      stats.value.processedEvents++
    } else {
      stats.value.pendingEvents++
    }
    if (newEvent.matchedCheckpoints && newEvent.matchedCheckpoints.length > 0) {
      stats.value.checkpointsMatched++
      stats.value.checkpointsPassed += newEvent.matchedCheckpoints.length
    }
    // Update event type counts
    const eventType = newEvent.eventType
    if (stats.value.eventsByType) {
      stats.value.eventsByType[eventType] = (stats.value.eventsByType[eventType] || 0) + 1
    }
  }
}

const { connected: wsConnected } = useWebSocket({
  onMonitoringEvent: handleMonitoringEvent,
  autoConnect: true,
})

// Lifecycle
onMounted(() => {
  fetchAll()
  if (autoRefresh.value) {
    startAutoRefresh()
  }
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div class="min-h-screen bg-surface-50 dark:bg-surface-950">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <!-- Header -->
      <div class="flex items-center justify-between mb-8">
        <div>
          <h1 class="text-3xl font-bold text-gray-900 dark:text-white">
            {{ t('eventsMonitoring.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('eventsMonitoring.subtitle') }}
          </p>
        </div>
        <div class="flex items-center space-x-4">
          <!-- WebSocket Status Indicator -->
          <div class="flex items-center">
            <span
              :class="[
                'inline-flex items-center px-3 py-1 rounded-md text-xs font-medium',
                wsConnected
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                  : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300',
              ]"
            >
              <span
                :class="[
                  'w-2 h-2 rounded-full mr-2',
                  wsConnected ? 'bg-green-500 animate-pulse' : 'bg-red-500',
                ]"
              ></span>
              {{
                wsConnected
                  ? t('eventsMonitoring.status.live')
                  : t('eventsMonitoring.status.disconnected')
              }}
            </span>
          </div>
          <button
            @click="toggleAutoRefresh"
            :class="[
              'inline-flex items-center px-4 py-2 rounded-md text-sm font-medium',
              autoRefresh
                ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                : 'bg-surface-100 text-surface-800 dark:bg-surface-700 dark:text-surface-300',
            ]"
          >
            <svg
              :class="['w-4 h-4 mr-2', autoRefresh ? 'animate-spin' : '']"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            {{
              autoRefresh
                ? t('eventsMonitoring.autoRefreshOn')
                : t('eventsMonitoring.autoRefreshOff')
            }}
          </button>
          <button
            @click="fetchAll"
            class="inline-flex items-center px-4 py-2 bg-primary-600 text-white rounded-md text-sm font-medium hover:bg-primary-700"
          >
            {{ t('eventsMonitoring.refreshNow') }}
          </button>
        </div>
      </div>

      <!-- Stats Cards -->
      <div v-if="stats" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
          <div class="flex items-center">
            <div class="flex-shrink-0 bg-blue-500 rounded-md p-3">
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                {{ t('eventsMonitoring.stats.eventsToday') }}
              </p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ stats.eventsToday }}
              </p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
          <div class="flex items-center">
            <div class="flex-shrink-0 bg-green-500 rounded-md p-3">
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                {{ t('eventsMonitoring.stats.checkpointsPassed') }}
              </p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ stats.checkpointsPassed }}
              </p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
          <div class="flex items-center">
            <div class="flex-shrink-0 bg-accent-500 rounded-md p-3">
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                {{ t('eventsMonitoring.stats.thisHour') }}
              </p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ stats.eventsThisHour }}
              </p>
            </div>
          </div>
        </div>

        <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
          <div class="flex items-center">
            <div class="flex-shrink-0 bg-yellow-500 rounded-md p-3">
              <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div class="ml-4">
              <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                {{ t('eventsMonitoring.stats.pending') }}
              </p>
              <p class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ stats.pendingEvents }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Event Type Breakdown -->
      <div
        v-if="stats && stats.eventsByType"
        class="bg-white dark:bg-surface-900 rounded-lg shadow mb-8 p-6"
      >
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          {{ t('eventsMonitoring.breakdown.heading') }}
        </h2>
        <div class="flex flex-wrap gap-4">
          <div v-for="(count, type) in stats.eventsByType" :key="type" class="flex items-center">
            <span :class="['px-3 py-1 rounded-md text-sm font-medium', getEventTypeColor(type)]">
              {{ type }}
            </span>
            <span class="ml-2 text-gray-600 dark:text-gray-400">{{ count }}</span>
          </div>
        </div>
      </div>

      <!-- Filters -->
      <div class="bg-white dark:bg-surface-900 rounded-lg shadow mb-8 p-6">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          {{ t('eventsMonitoring.filters.heading') }}
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
              t('eventsMonitoring.filters.typeLabel')
            }}</label>
            <select
              v-model="filters.eventType"
              :aria-label="t('eventsMonitoring.filters.typeAria')"
              class="w-full rounded-md border-surface-300 dark:border-surface-600 dark:bg-surface-700 dark:text-white shadow-sm focus:border-primary-500 focus:ring-primary-500"
            >
              <option value="">{{ t('eventsMonitoring.filters.typeAllOption') }}</option>
              <option v-for="type in eventTypeOptions.slice(1)" :key="type" :value="type">
                {{ type }}
              </option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
              t('eventsMonitoring.filters.podLabel')
            }}</label>
            <input
              v-model="filters.podId"
              type="text"
              :placeholder="t('eventsMonitoring.filters.podPlaceholder')"
              :aria-label="t('eventsMonitoring.filters.podAria')"
              class="w-full rounded-md border-surface-300 dark:border-surface-600 dark:bg-surface-700 dark:text-white shadow-sm focus:border-primary-500 focus:ring-primary-500"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{
              t('eventsMonitoring.filters.sessionLabel')
            }}</label>
            <input
              v-model="filters.sessionId"
              type="text"
              :placeholder="t('eventsMonitoring.filters.sessionPlaceholder')"
              :aria-label="t('eventsMonitoring.filters.sessionAria')"
              class="w-full rounded-md border-surface-300 dark:border-surface-600 dark:bg-surface-700 dark:text-white shadow-sm focus:border-primary-500 focus:ring-primary-500"
            />
          </div>
          <div class="flex items-end space-x-2">
            <button
              @click="applyFilters"
              class="px-4 py-2 bg-primary-600 text-white rounded-md text-sm font-medium hover:bg-primary-700"
            >
              {{ t('eventsMonitoring.filters.apply') }}
            </button>
            <button
              @click="clearFilters"
              class="px-4 py-2 bg-surface-200 text-surface-800 dark:bg-surface-600 dark:text-white rounded-md text-sm font-medium hover:bg-surface-300 dark:hover:bg-surface-500"
            >
              {{ t('eventsMonitoring.filters.clear') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Events Table -->
      <div class="bg-white dark:bg-surface-900 rounded-lg shadow overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-200 dark:border-surface-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('eventsMonitoring.table.heading') }}
          </h2>
        </div>

        <!-- Loading state -->
        <div v-if="loading" class="p-8 text-center">
          <div
            class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"
          ></div>
          <p class="mt-2 text-gray-500 dark:text-gray-400">
            {{ t('eventsMonitoring.table.loading') }}
          </p>
        </div>

        <!-- Error state -->
        <div v-else-if="error" class="p-8 text-center">
          <p class="text-red-600 dark:text-red-400">{{ error }}</p>
          <button @click="fetchAll" class="mt-4 text-primary-600 hover:text-primary-700">
            {{ t('eventsMonitoring.table.tryAgain') }}
          </button>
        </div>

        <!-- Events list -->
        <div v-else-if="filteredEvents.length > 0" class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-surface-700">
            <thead class="bg-surface-50 dark:bg-surface-800">
              <tr>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.time') }}
                </th>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.type') }}
                </th>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.vm') }}
                </th>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.description') }}
                </th>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.checkpoints') }}
                </th>
                <th
                  class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider"
                >
                  {{ t('eventsMonitoring.table.status') }}
                </th>
              </tr>
            </thead>
            <tbody
              class="bg-white dark:bg-surface-900 divide-y divide-gray-200 dark:divide-surface-700"
            >
              <tr
                v-for="event in filteredEvents"
                :key="event.id"
                role="button"
                tabindex="0"
                class="hover:bg-surface-50 dark:hover:bg-surface-800 cursor-pointer focus:outline-none focus:ring-2 focus:ring-inset focus:ring-primary-500"
                @click="openEventDetail(event)"
                @keydown.enter="openEventDetail(event)"
                @keydown.space.prevent="openEventDetail(event)"
              >
                <td class="px-6 py-4 whitespace-nowrap">
                  <div class="text-sm text-gray-900 dark:text-white">
                    {{ formatTime(event.timestamp) }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ formatDate(event.timestamp) }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <span
                    :class="[
                      'px-2 py-1 rounded-md text-xs font-medium',
                      getEventTypeColor(event.eventType),
                    ]"
                  >
                    {{ event.eventType }}
                  </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <div class="text-sm text-gray-900 dark:text-white">{{ event.vmName }}</div>
                  <div
                    class="text-xs text-gray-500 dark:text-gray-400 truncate max-w-[150px]"
                    :title="event.podId"
                  >
                    {{
                      t('eventsMonitoring.table.podIdTruncated', { id: event.podId?.slice(0, 8) })
                    }}
                  </div>
                </td>
                <td class="px-6 py-4">
                  <div
                    class="text-sm text-gray-900 dark:text-white max-w-md truncate"
                    :title="event.description"
                  >
                    {{ event.description || t('eventsMonitoring.table.dashPlaceholder') }}
                  </div>
                  <div v-if="event.ruleId" class="text-xs text-gray-500 dark:text-gray-400">
                    {{
                      t('eventsMonitoring.table.ruleLine', {
                        ruleId: event.ruleId,
                        level: event.ruleLevel,
                      })
                    }}
                  </div>
                </td>
                <td class="px-6 py-4">
                  <div
                    v-if="event.matchedCheckpoints && event.matchedCheckpoints.length > 0"
                    class="flex flex-wrap gap-1"
                  >
                    <span
                      v-for="cp in event.matchedCheckpoints"
                      :key="cp"
                      class="px-2 py-0.5 bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 rounded text-xs"
                    >
                      {{ cp }}
                    </span>
                  </div>
                  <span v-else class="text-gray-400 text-sm">{{
                    t('eventsMonitoring.table.dashPlaceholder')
                  }}</span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <span
                    :class="[
                      'px-2 py-1 rounded-md text-xs font-medium',
                      event.processed
                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                        : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
                    ]"
                  >
                    {{
                      event.processed
                        ? t('eventsMonitoring.table.statusProcessed')
                        : t('eventsMonitoring.table.statusPending')
                    }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Empty state -->
        <div v-else class="p-8 text-center">
          <svg
            class="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            />
          </svg>
          <h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
            {{ t('eventsMonitoring.empty.title') }}
          </h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('eventsMonitoring.empty.body') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Event Detail Modal -->
    <Teleport to="body">
      <div v-if="showEventDetail && selectedEvent" class="fixed inset-0 z-50 overflow-y-auto">
        <div
          class="flex min-h-full items-end justify-center p-4 text-center sm:items-center sm:p-0"
        >
          <!-- Backdrop -->
          <!-- eslint-disable-next-line vuejs-accessibility/click-events-have-key-events -- Modal backdrop; Escape-to-close is the keyboard affordance, not keydown on the backdrop. -->
          <div
            class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
            aria-hidden="true"
            @click="closeEventDetail"
          ></div>

          <!-- Modal panel -->
          <div
            class="relative transform overflow-hidden rounded-lg bg-white dark:bg-surface-900 text-left shadow-xl transition-all sm:my-8 sm:w-full sm:max-w-2xl"
          >
            <div class="bg-white dark:bg-surface-900 px-4 pb-4 pt-5 sm:p-6 sm:pb-4">
              <!-- Header -->
              <div class="flex items-center justify-between mb-4">
                <div class="flex items-center">
                  <span
                    :class="[
                      'px-3 py-1 rounded-md text-sm font-medium mr-3',
                      getEventTypeColor(selectedEvent.eventType),
                    ]"
                  >
                    {{ selectedEvent.eventType }}
                  </span>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('eventsMonitoring.detail.header', { id: selectedEvent.id }) }}
                  </h3>
                </div>
                <button @click="closeEventDetail" class="text-gray-400 hover:text-gray-500">
                  <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>

              <!-- Content -->
              <div class="space-y-4">
                <!-- Timestamp -->
                <div>
                  <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                    t('eventsMonitoring.detail.timestampLabel')
                  }}</label>
                  <p class="mt-1 text-sm text-gray-900 dark:text-white">
                    {{ formatDateTime(selectedEvent.timestamp) }}
                  </p>
                </div>

                <!-- Description -->
                <div v-if="selectedEvent.description">
                  <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                    t('eventsMonitoring.detail.descriptionLabel')
                  }}</label>
                  <p class="mt-1 text-sm text-gray-900 dark:text-white">
                    {{ selectedEvent.description }}
                  </p>
                </div>

                <!-- Rule Info -->
                <div v-if="selectedEvent.ruleId" class="grid grid-cols-2 gap-4">
                  <div>
                    <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                      t('eventsMonitoring.detail.ruleIdLabel')
                    }}</label>
                    <p class="mt-1 text-sm text-gray-900 dark:text-white">
                      {{ selectedEvent.ruleId }}
                    </p>
                  </div>
                  <div>
                    <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                      t('eventsMonitoring.detail.ruleLevelLabel')
                    }}</label>
                    <p class="mt-1 text-sm text-gray-900 dark:text-white">
                      {{ selectedEvent.ruleLevel }}
                    </p>
                  </div>
                </div>

                <!-- VM and Pod Info -->
                <div class="grid grid-cols-2 gap-4">
                  <div>
                    <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                      t('eventsMonitoring.detail.vmNameLabel')
                    }}</label>
                    <p class="mt-1 text-sm text-gray-900 dark:text-white">
                      {{ selectedEvent.vmName }}
                    </p>
                  </div>
                  <div>
                    <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                      t('eventsMonitoring.detail.podIdLabel')
                    }}</label>
                    <p
                      class="mt-1 text-sm text-gray-900 dark:text-white font-mono text-xs break-all"
                    >
                      {{ selectedEvent.podId }}
                    </p>
                  </div>
                </div>

                <!-- Session ID -->
                <div v-if="selectedEvent.sessionId">
                  <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                    t('eventsMonitoring.detail.sessionIdLabel')
                  }}</label>
                  <p class="mt-1 text-sm text-gray-900 dark:text-white font-mono text-xs break-all">
                    {{ selectedEvent.sessionId }}
                  </p>
                </div>

                <!-- Processing Status -->
                <div>
                  <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                    t('eventsMonitoring.detail.statusLabel')
                  }}</label>
                  <span
                    :class="[
                      'inline-flex mt-1 px-2 py-1 rounded-md text-xs font-medium',
                      selectedEvent.processed
                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
                        : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300',
                    ]"
                  >
                    {{
                      selectedEvent.processed
                        ? t('eventsMonitoring.detail.statusProcessed')
                        : t('eventsMonitoring.detail.statusPending')
                    }}
                  </span>
                </div>

                <!-- Matched Checkpoints -->
                <div
                  v-if="
                    selectedEvent.matchedCheckpoints && selectedEvent.matchedCheckpoints.length > 0
                  "
                >
                  <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{
                    t('eventsMonitoring.detail.matchedCheckpointsLabel')
                  }}</label>
                  <div class="mt-1 flex flex-wrap gap-2">
                    <span
                      v-for="cp in selectedEvent.matchedCheckpoints"
                      :key="cp"
                      class="px-2 py-1 bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 rounded text-sm"
                    >
                      {{ cp }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Footer -->
            <div
              class="bg-surface-50 dark:bg-surface-800 px-4 py-3 sm:flex sm:flex-row-reverse sm:px-6"
            >
              <button
                type="button"
                @click="closeEventDetail"
                class="mt-3 inline-flex w-full justify-center rounded-md bg-white dark:bg-surface-600 px-3 py-2 text-sm font-semibold text-surface-900 dark:text-white shadow-sm ring-1 ring-inset ring-surface-300 dark:ring-surface-500 hover:bg-surface-50 dark:hover:bg-surface-500 sm:mt-0 sm:w-auto"
              >
                {{ t('eventsMonitoring.detail.close') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
