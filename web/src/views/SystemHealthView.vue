<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useAuthStore } from '../stores/auth'
import { getStatusSeverity } from '@/utils/status'
import { api } from '@/api/config'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatTime: fmtTime } = useFormatters()
const authStore = useAuthStore()

// Check if current user is admin
const isAdmin = computed(() => authStore.isAdmin)

// Types
interface ServiceCheck {
  status: string
  latency_ms?: number
  error?: string
  version?: string
  connected?: boolean
  state?: string
  connection_count?: number
  max_connections?: number
}

interface HealthResponse {
  status: string
  checks: Record<string, ServiceCheck>
}

interface SystemMetrics {
  activeSessions: number
  activePods: number
  totalUsers: number
  totalLabs: number
}

// State
const loading = ref(true)
const error = ref<string | null>(null)
const healthData = ref<HealthResponse | null>(null)
const metrics = ref<SystemMetrics>({
  activeSessions: 0,
  activePods: 0,
  totalUsers: 0,
  totalLabs: 0,
})
const lastUpdated = ref<Date | null>(null)
const autoRefresh = ref(true)
let refreshInterval: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  await loadHealth()
  await loadMetrics()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})

function startAutoRefresh() {
  if (refreshInterval) clearInterval(refreshInterval)
  refreshInterval = setInterval(async () => {
    if (autoRefresh.value) {
      await loadHealth()
      await loadMetrics()
    }
  }, 30000) // Refresh every 30 seconds
}

function stopAutoRefresh() {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
}

async function loadHealth() {
  try {
    loading.value = true
    error.value = null

    // /ready is a root-level endpoint, not under /api/v1
    const { data } = await api.get<HealthResponse>('/ready', { baseURL: '' })
    healthData.value = data
    lastUpdated.value = new Date()
  } catch (err) {
    console.error('Failed to load health data:', err)
    error.value = t('systemHealth.loadFailed')
    healthData.value = null
  } finally {
    loading.value = false
  }
}

async function loadMetrics() {
  try {
    // Try to load pods count
    const { data: podsData } = await api.get<{ pods?: unknown[] }>('/pods')
    metrics.value.activePods = podsData.pods?.length || 0

    // Try to load labs count
    const { data: labsData } = await api.get<{ count?: number; labs?: unknown[] }>('/labs')
    metrics.value.totalLabs = labsData.count || labsData.labs?.length || 0

    // Try to load sessions count
    const { data: sessionsData } = await api.get<{ sessions?: unknown[] }>('/sessions', {
      params: { status: 'active' },
    })
    metrics.value.activeSessions = sessionsData.sessions?.length || 0

    // Try to load users count (admin only)
    if (isAdmin.value) {
      const { data: usersData } = await api.get<{ total?: number; users?: unknown[] }>('/users')
      metrics.value.totalUsers = usersData.total || usersData.users?.length || 0
    }
  } catch (err) {
    console.error('Failed to load metrics:', err)
  }
}

async function refresh() {
  await loadHealth()
  await loadMetrics()
}

// getStatusSeverity imported from @/utils/status

function getStatusIcon(status: string): string {
  switch (status) {
    case 'healthy':
    case 'connected':
    case 'ready':
    case 'ok':
      return 'pi pi-check-circle'
    case 'degraded':
    case 'warning':
      return 'pi pi-exclamation-triangle'
    case 'unhealthy':
    case 'disconnected':
    case 'error':
      return 'pi pi-times-circle'
    default:
      return 'pi pi-question-circle'
  }
}

function formatLatency(ms: number | undefined): string {
  if (ms === undefined) return t('systemHealth.service.latencyPlaceholder')
  if (ms < 1) return t('systemHealth.service.latencySub1ms')
  return t('systemHealth.service.latencyMs', { ms: Math.round(ms) })
}

function formatTime(date: Date | null): string {
  if (!date) return t('systemHealth.service.latencyPlaceholder')
  return fmtTime(date)
}

const overallStatus = computed((): 'healthy' | 'degraded' | 'unhealthy' | 'unknown' => {
  if (!healthData.value) return 'unknown'
  if (healthData.value.status === 'ready') return 'healthy'
  // Check if any critical service is unhealthy
  const checks = healthData.value.checks
  if (checks['database']?.status === 'unhealthy' || checks['nats']?.status === 'unhealthy') {
    return 'unhealthy'
  }
  return 'degraded'
})

interface ServiceDisplay extends ServiceCheck {
  name: string
}

const serviceOrder = ['database', 'nats', 'redis', 'proxmox', 'cloudstack']
const sortedServices = computed((): ServiceDisplay[] => {
  if (!healthData.value?.checks) return []
  const checks = healthData.value.checks
  return serviceOrder
    .filter(name => name in checks)
    .map(name => ({ name, status: 'unknown', ...checks[name] }))
})
</script>

<template>
  <div class="space-y-6">
    <!-- Access denied message for non-admins -->
    <Message v-if="!isAdmin" severity="warn" :closable="false">
      <i class="pi pi-info-circle mr-2" />
      {{ t('systemHealth.adminNotice') }}
    </Message>

    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ t('systemHealth.title') }}
        </h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ t('systemHealth.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <div class="text-sm text-surface-500">
          <span v-if="lastUpdated">{{
            t('systemHealth.lastUpdated', { when: formatTime(lastUpdated) })
          }}</span>
        </div>
        <label
          class="flex items-center gap-2 text-sm text-surface-600 dark:text-surface-400 cursor-pointer"
        >
          <input type="checkbox" v-model="autoRefresh" class="accent-primary" />
          {{ t('systemHealth.autoRefresh') }}
        </label>
        <Button
          @click="refresh"
          :loading="loading"
          :disabled="loading"
          icon="pi pi-refresh"
          severity="secondary"
        />
      </div>
    </div>

    <!-- Error Message -->
    <Message v-if="error" severity="error" :closable="false">
      {{ error }}
    </Message>

    <!-- Overall Status Card -->
    <Card>
      <template #content>
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <div
              class="w-16 h-16 rounded-full flex items-center justify-center"
              :class="{
                'bg-green-100 dark:bg-green-900/30': overallStatus === 'healthy',
                'bg-yellow-100 dark:bg-yellow-900/30': overallStatus === 'degraded',
                'bg-red-100 dark:bg-red-900/30': overallStatus === 'unhealthy',
                'bg-gray-100 dark:bg-gray-800': overallStatus === 'unknown',
              }"
            >
              <i
                class="text-3xl"
                :class="{
                  'pi pi-check-circle text-green-600 dark:text-green-400':
                    overallStatus === 'healthy',
                  'pi pi-exclamation-triangle text-yellow-600 dark:text-yellow-400':
                    overallStatus === 'degraded',
                  'pi pi-times-circle text-red-600 dark:text-red-400':
                    overallStatus === 'unhealthy',
                  'pi pi-question-circle text-gray-600 dark:text-gray-400':
                    overallStatus === 'unknown',
                }"
              />
            </div>
            <div>
              <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
                {{ t(`systemHealth.overall.${overallStatus}`) }}
              </h2>
              <p class="text-surface-600 dark:text-surface-400">
                {{ t('systemHealth.overall.servicesMonitored', { count: sortedServices.length }) }}
              </p>
            </div>
          </div>
          <Tag :severity="getStatusSeverity(overallStatus)" :value="overallStatus.toUpperCase()" />
        </div>
      </template>
    </Card>

    <!-- Metrics Cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      <Card>
        <template #content>
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center"
            >
              <i class="pi pi-play text-xl text-blue-600 dark:text-blue-400" />
            </div>
            <div>
              <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {{ metrics.activeSessions }}
              </div>
              <div class="text-sm text-surface-600 dark:text-surface-400">
                {{ t('systemHealth.metrics.activeSessions') }}
              </div>
            </div>
          </div>
        </template>
      </Card>

      <Card>
        <template #content>
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center"
            >
              <i class="pi pi-server text-xl text-green-600 dark:text-green-400" />
            </div>
            <div>
              <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {{ metrics.activePods }}
              </div>
              <div class="text-sm text-surface-600 dark:text-surface-400">
                {{ t('systemHealth.metrics.activePods') }}
              </div>
            </div>
          </div>
        </template>
      </Card>

      <Card>
        <template #content>
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 rounded-lg bg-accent-100 dark:bg-accent-900/30 flex items-center justify-center"
            >
              <i class="pi pi-book text-xl text-accent-600 dark:text-accent-400" />
            </div>
            <div>
              <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {{ metrics.totalLabs }}
              </div>
              <div class="text-sm text-surface-600 dark:text-surface-400">
                {{ t('systemHealth.metrics.labTemplates') }}
              </div>
            </div>
          </div>
        </template>
      </Card>

      <Card v-if="isAdmin">
        <template #content>
          <div class="flex items-center gap-4">
            <div
              class="w-12 h-12 rounded-lg bg-orange-100 dark:bg-orange-900/30 flex items-center justify-center"
            >
              <i class="pi pi-users text-xl text-orange-600 dark:text-orange-400" />
            </div>
            <div>
              <div class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                {{ metrics.totalUsers }}
              </div>
              <div class="text-sm text-surface-600 dark:text-surface-400">
                {{ t('systemHealth.metrics.totalUsers') }}
              </div>
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- Loading State -->
    <div v-if="loading && !healthData" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Services Grid -->
    <div v-else-if="healthData" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <Card v-for="service in sortedServices" :key="service.name">
        <template #content>
          <div class="flex items-start justify-between mb-4">
            <div class="flex items-center gap-3">
              <div
                class="w-10 h-10 rounded-lg flex items-center justify-center"
                :class="{
                  'bg-green-100 dark:bg-green-900/30': service.status === 'healthy',
                  'bg-yellow-100 dark:bg-yellow-900/30':
                    service.status === 'degraded' || service.status === 'warning',
                  'bg-red-100 dark:bg-red-900/30':
                    service.status === 'unhealthy' || service.status === 'error',
                  'bg-gray-100 dark:bg-gray-800': ![
                    'healthy',
                    'degraded',
                    'warning',
                    'unhealthy',
                    'error',
                  ].includes(service.status),
                }"
              >
                <i
                  :class="[
                    getStatusIcon(service.status),
                    {
                      'text-green-600 dark:text-green-400': service.status === 'healthy',
                      'text-yellow-600 dark:text-yellow-400':
                        service.status === 'degraded' || service.status === 'warning',
                      'text-red-600 dark:text-red-400':
                        service.status === 'unhealthy' || service.status === 'error',
                      'text-gray-600 dark:text-gray-400': ![
                        'healthy',
                        'degraded',
                        'warning',
                        'unhealthy',
                        'error',
                      ].includes(service.status),
                    },
                  ]"
                />
              </div>
              <div>
                <h3 class="font-semibold text-surface-900 dark:text-surface-100 capitalize">
                  {{ service.name }}
                </h3>
                <div class="text-xs text-surface-500" v-if="service.version">
                  {{ t('systemHealth.service.versionPrefix', { version: service.version }) }}
                </div>
              </div>
            </div>
            <Tag :severity="getStatusSeverity(service.status)" :value="service.status" />
          </div>

          <div class="space-y-2 text-sm">
            <div v-if="service.latency_ms !== undefined" class="flex justify-between">
              <span class="text-surface-600 dark:text-surface-400">{{
                t('systemHealth.service.latencyLabel')
              }}</span>
              <span class="font-medium text-surface-900 dark:text-surface-100">
                {{ formatLatency(service.latency_ms) }}
              </span>
            </div>

            <div v-if="service.state" class="flex justify-between">
              <span class="text-surface-600 dark:text-surface-400">{{
                t('systemHealth.service.stateLabel')
              }}</span>
              <span class="font-medium text-surface-900 dark:text-surface-100">
                {{ service.state }}
              </span>
            </div>

            <div v-if="service.connection_count !== undefined" class="flex justify-between">
              <span class="text-surface-600 dark:text-surface-400">{{
                t('systemHealth.service.connectionsLabel')
              }}</span>
              <span class="font-medium text-surface-900 dark:text-surface-100">
                {{ service.connection_count }}
                <span v-if="service.max_connections">/ {{ service.max_connections }}</span>
              </span>
            </div>

            <div
              v-if="service.error"
              class="mt-2 p-2 bg-red-50 dark:bg-red-900/20 rounded text-red-700 dark:text-red-300 text-xs"
            >
              {{ service.error }}
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- Empty State -->
    <Card v-else>
      <template #content>
        <div class="text-center py-12">
          <i class="pi pi-server text-4xl text-surface-400 mb-4" />
          <h3 class="text-lg font-medium text-surface-700 dark:text-surface-300 mb-2">
            {{ t('systemHealth.empty.title') }}
          </h3>
          <p class="text-surface-500 mb-4">{{ t('systemHealth.empty.body') }}</p>
          <Button @click="refresh" icon="pi pi-refresh" :label="t('systemHealth.empty.retry')" />
        </div>
      </template>
    </Card>
  </div>
</template>
