<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { auditApi, type AuditEntry, type AuditLogFilter } from '@/api'
import { useOrganizationStore } from '@/stores/organization'
import { useFocusRestore } from '@/composables'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import Dialog from '@volt/Dialog.vue'
import DatePicker from 'primevue/datepicker'
import { useToast } from 'primevue/usetoast'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDateTime } = useFormatters()
const route = useRoute()
const toast = useToast()
const orgStore = useOrganizationStore()

// Data
const entries = ref<AuditEntry[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const totalCount = ref(0)
const selectedEntry = ref<AuditEntry | null>(null)
const showDetailDialog = ref(false)
useFocusRestore(showDetailDialog)
const exporting = ref(false)

// Filter state
const searchActor = ref('')
const filterAction = ref<{ label: string; value: string | null }>({ label: t('auditLog.actionOptions.all'), value: null })
const filterResource = ref<{ label: string; value: string | null }>({ label: t('auditLog.resourceOptions.all'), value: null })
const dateRange = ref<Date[] | null>(null)
const currentPage = ref(0)
const pageSize = 25

// Filter options are computed so labels refresh when the active locale changes.
const actionOptions = computed(() => [
  { label: t('auditLog.actionOptions.all'), value: null },
  { label: t('auditLog.actionOptions.orgUpdated'), value: 'organization.updated' },
  { label: t('auditLog.actionOptions.orgDeleted'), value: 'organization.deleted' },
  { label: t('auditLog.actionOptions.memberInvited'), value: 'member.invited' },
  { label: t('auditLog.actionOptions.memberRoleChanged'), value: 'member.role_changed' },
  { label: t('auditLog.actionOptions.memberRemoved'), value: 'member.removed' },
  { label: t('auditLog.actionOptions.loginSuccess'), value: 'auth.login.success' },
  { label: t('auditLog.actionOptions.loginFailed'), value: 'auth.login.failed' },
  { label: t('auditLog.actionOptions.passwordChanged'), value: 'auth.password.changed' },
])

const resourceOptions = computed(() => [
  { label: t('auditLog.resourceOptions.all'), value: null },
  { label: t('auditLog.resourceOptions.organization'), value: 'organization' },
  { label: t('auditLog.resourceOptions.membership'), value: 'membership' },
  { label: t('auditLog.resourceOptions.user'), value: 'user' },
  { label: t('auditLog.resourceOptions.session'), value: 'session' },
  { label: t('auditLog.resourceOptions.pod'), value: 'pod' },
  { label: t('auditLog.resourceOptions.lab'), value: 'lab' },
])

const orgId = computed(() => route.params['orgId'] as string || orgStore.currentOrganization?.id || '')

// Watch for filter changes and reload
watch([filterAction, filterResource, dateRange, currentPage], () => {
  loadAuditLogs()
}, { deep: true })

onMounted(async () => {
  await loadAuditLogs()
})

async function loadAuditLogs() {
  if (!orgId.value) {
    error.value = t('auditLog.noOrgError')
    loading.value = false
    return
  }

  try {
    loading.value = true
    error.value = null

    const filter: AuditLogFilter = {
      limit: pageSize,
      offset: currentPage.value * pageSize,
    }

    if (searchActor.value) {
      filter.actor = searchActor.value
    }
    if (filterAction.value?.value) {
      filter.action = filterAction.value.value
    }
    if (filterResource.value?.value) {
      filter.resourceType = filterResource.value.value
    }
    if (dateRange.value && dateRange.value.length === 2) {
      const startDate = dateRange.value[0]
      const endDate = dateRange.value[1]
      if (startDate) {
        filter.startTime = startDate.toISOString()
      }
      if (endDate) {
        filter.endTime = endDate.toISOString()
      }
    }

    const response = await auditApi.list(orgId.value, filter)
    entries.value = response.entries
    totalCount.value = response.totalCount
  } catch (err) {
    console.error('Failed to load audit logs:', err)
    error.value = t('auditLog.loadFailed')
  } finally {
    loading.value = false
  }
}

function searchByActor() {
  currentPage.value = 0
  loadAuditLogs()
}

function clearFilters() {
  searchActor.value = ''
  filterAction.value = { label: t('auditLog.actionOptions.all'), value: null }
  filterResource.value = { label: t('auditLog.resourceOptions.all'), value: null }
  dateRange.value = null
  currentPage.value = 0
  loadAuditLogs()
}

function showDetails(entry: AuditEntry) {
  selectedEntry.value = entry
  showDetailDialog.value = true
}

async function exportLogs() {
  if (!orgId.value) return

  try {
    exporting.value = true
    const startTime = dateRange.value?.[0]?.toISOString()
    const endTime = dateRange.value?.[1]?.toISOString()

    const exportedEntries = await auditApi.export(orgId.value, startTime, endTime)

    // Create and download JSON file
    const blob = new Blob([JSON.stringify(exportedEntries, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit-log-${new Date().toISOString().split('T')[0]}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)

    toast.add({
      severity: 'success',
      summary: t('auditLog.exportToast.successSummary'),
      detail: t('auditLog.exportToast.successDetail', { count: exportedEntries.length }),
      life: 3000,
    })
  } catch (err) {
    console.error('Failed to export audit logs:', err)
    toast.add({
      severity: 'error',
      summary: t('auditLog.exportToast.failedSummary'),
      detail: t('auditLog.exportToast.failedDetail'),
      life: 3000,
    })
  } finally {
    exporting.value = false
  }
}

function formatTimestamp(timestamp: string): string {
  return formatDateTime(timestamp)
}

function formatAction(action: string): string {
  return action
    .split('.')
    .map(part => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

function getActionSeverity(action: string): 'success' | 'info' | 'warn' | 'danger' | 'secondary' | 'contrast' | undefined {
  if (action.includes('deleted') || action.includes('removed') || action.includes('failed')) {
    return 'danger'
  }
  if (action.includes('created') || action.includes('invited') || action.includes('success')) {
    return 'success'
  }
  if (action.includes('updated') || action.includes('changed')) {
    return 'warn'
  }
  return 'info'
}

function formatDetails(details: Record<string, unknown> | undefined): string {
  if (!details) return t('auditLog.detail.noDetails')
  return JSON.stringify(details, null, 2)
}

const totalPages = computed(() => Math.ceil(totalCount.value / pageSize))

function prevPage() {
  if (currentPage.value > 0) {
    currentPage.value--
  }
}

function nextPage() {
  if (currentPage.value < totalPages.value - 1) {
    currentPage.value++
  }
}
</script>

<template>
  <div class="p-4">
    <div class="flex justify-between items-center mb-6">
      <div>
        <h1 class="text-2xl font-bold">{{ t('auditLog.title') }}</h1>
        <p class="text-gray-600 dark:text-gray-400">
          {{ t('auditLog.subtitle') }}
        </p>
      </div>
      <Button
        :label="t('auditLog.exportAction')"
        icon="pi pi-download"
        :loading="exporting"
        @click="exportLogs"
      />
    </div>

    <!-- Filters -->
    <Card class="mb-4">
      <template #content>
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          <div>
            <label for="actor-search" class="block text-sm font-medium mb-1">{{ t('auditLog.filters.actorLabel') }}</label>
            <div class="flex gap-2">
              <InputText
                id="actor-search"
                v-model="searchActor"
                :placeholder="t('auditLog.filters.actorPlaceholder')"
                class="w-full"
                @keyup.enter="searchByActor"
              />
            </div>
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">{{ t('auditLog.filters.actionLabel') }}</label>
            <Select
              v-model="filterAction"
              :options="actionOptions"
              optionLabel="label"
              :aria-label="t('auditLog.filters.actionAria')"
              class="w-full"
            />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">{{ t('auditLog.filters.resourceLabel') }}</label>
            <Select
              v-model="filterResource"
              :options="resourceOptions"
              optionLabel="label"
              :aria-label="t('auditLog.filters.resourceAria')"
              class="w-full"
            />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">{{ t('auditLog.filters.dateRangeLabel') }}</label>
            <DatePicker
              v-model="dateRange"
              selectionMode="range"
              :manualInput="false"
              showIcon
              class="w-full"
              :placeholder="t('auditLog.filters.dateRangePlaceholder')"
            />
          </div>

          <div class="flex items-end">
            <Button
              :label="t('auditLog.filters.clearAction')"
              severity="secondary"
              icon="pi pi-filter-slash"
              @click="clearFilters"
            />
          </div>
        </div>
      </template>
    </Card>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="false" class="mb-4">
      {{ error }}
    </Message>

    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Audit log table -->
    <Card v-else>
      <template #content>
        <div v-if="entries.length === 0" class="text-center py-8 text-gray-500">
          {{ t('auditLog.empty') }}
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b dark:border-gray-700">
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.timestamp') }}</th>
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.action') }}</th>
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.resource') }}</th>
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.actor') }}</th>
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.ipAddress') }}</th>
                <th class="text-left py-3 px-4 font-medium">{{ t('auditLog.table.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="entry in entries"
                :key="entry.id"
                class="border-b dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800"
              >
                <td class="py-3 px-4 text-sm">
                  {{ formatTimestamp(entry.timestamp) }}
                </td>
                <td class="py-3 px-4">
                  <Tag :severity="getActionSeverity(entry.action)">
                    {{ formatAction(entry.action) }}
                  </Tag>
                </td>
                <td class="py-3 px-4 text-sm">
                  <span class="font-medium">{{ entry.resourceType }}</span>
                  <span v-if="entry.resourceId" class="text-gray-500 dark:text-gray-400 ml-1">
                    {{ t('auditLog.table.idTruncated', { id: entry.resourceId.substring(0, 8) }) }}
                  </span>
                </td>
                <td class="py-3 px-4 text-sm font-mono">
                  {{ t('auditLog.table.actorTruncated', { id: entry.actorId.substring(0, 8) }) }}
                </td>
                <td class="py-3 px-4 text-sm font-mono">
                  {{ entry.ipAddress || t('auditLog.table.dashPlaceholder') }}
                </td>
                <td class="py-3 px-4">
                  <Button
                    icon="pi pi-eye"
                    severity="secondary"
                    text
                    rounded
                    @click="showDetails(entry)"
                  />
                </td>
              </tr>
            </tbody>
          </table>

          <!-- Pagination -->
          <div class="flex justify-between items-center mt-4 pt-4 border-t dark:border-gray-700">
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {{ t('auditLog.pagination.showing', {
                from: currentPage * pageSize + 1,
                to: Math.min((currentPage + 1) * pageSize, totalCount),
                total: totalCount,
              }) }}
            </span>
            <div class="flex gap-2">
              <Button
                icon="pi pi-chevron-left"
                severity="secondary"
                :disabled="currentPage === 0"
                @click="prevPage"
              />
              <span class="px-3 py-2">
                {{ t('auditLog.pagination.pageOfTotal', { current: currentPage + 1, total: totalPages || 1 }) }}
              </span>
              <Button
                icon="pi pi-chevron-right"
                severity="secondary"
                :disabled="currentPage >= totalPages - 1"
                @click="nextPage"
              />
            </div>
          </div>
        </div>
      </template>
    </Card>

    <!-- Detail Dialog -->
    <Dialog
      v-model:visible="showDetailDialog"
      :header="t('auditLog.detail.header')"
      modal
      :style="{ width: '600px' }"
    >
      <div v-if="selectedEntry" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.idLabel') }}</label>
            <p class="font-mono text-sm">{{ selectedEntry.id }}</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.timestampLabel') }}</label>
            <p>{{ formatTimestamp(selectedEntry.timestamp) }}</p>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.actionLabel') }}</label>
            <Tag :severity="getActionSeverity(selectedEntry.action)">
              {{ formatAction(selectedEntry.action) }}
            </Tag>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.resourceLabel') }}</label>
            <p>
              {{ selectedEntry.resourceType }}
              <span v-if="selectedEntry.resourceId" class="text-gray-500"> {{ t('auditLog.detail.resourceWithId', { id: selectedEntry.resourceId }) }}</span>
            </p>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.actorLabel') }}</label>
            <p class="font-mono text-sm">{{ selectedEntry.actorId }}</p>
            <p class="text-xs text-gray-500">{{ selectedEntry.actorType }}</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.ipLabel') }}</label>
            <p class="font-mono text-sm">{{ selectedEntry.ipAddress || t('auditLog.table.dashPlaceholder') }}</p>
          </div>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('auditLog.detail.userAgentLabel') }}</label>
          <p class="text-sm text-gray-600 dark:text-gray-400 break-all">
            {{ selectedEntry.userAgent || t('auditLog.table.dashPlaceholder') }}
          </p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">{{ t('auditLog.detail.detailsLabel') }}</label>
          <pre class="bg-gray-100 dark:bg-gray-800 p-3 rounded text-sm overflow-auto max-h-48">{{ formatDetails(selectedEntry.details) }}</pre>
        </div>
      </div>

      <template #footer>
        <Button :label="t('auditLog.detail.close')" @click="showDetailDialog = false" />
      </template>
    </Dialog>
  </div>
</template>
