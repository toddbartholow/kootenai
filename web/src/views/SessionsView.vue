<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { sessionsApi, type Session } from '@/api'
import { getStatusSeverity } from '@/utils/status'
import { formatDate, formatDurationFromTimestamps } from '@/utils/format'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'

const { t } = useI18n()
const router = useRouter()
const confirm = useConfirm()
const toast = useToast()

const sessions = ref<Session[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const endingSession = ref<string | null>(null)
const deletingSession = ref<string | null>(null)
const cleaningUp = ref(false)

// Filter state. Status labels are resolved via i18n; option `value`
// stays stable across locales so API filter semantics don't shift.
const filterUserId = ref('')
const filterActive = ref<{ label: string; value: boolean | null }>({
  label: t('sessionsList.filters.status.active'),
  value: true,
})

const statusOptions = computed(() => [
  { label: t('sessionsList.filters.status.all'), value: null },
  { label: t('sessionsList.filters.status.active'), value: true },
  { label: t('sessionsList.filters.status.completed'), value: false },
])

onMounted(async () => {
  await loadSessions()
})

// Reload when filters change
watch([filterUserId, filterActive], async () => {
  await loadSessions()
})

async function loadSessions() {
  try {
    loading.value = true
    error.value = null
    sessions.value = await sessionsApi.list(
      filterUserId.value || undefined,
      filterActive.value?.value ?? undefined
    )
  } catch (err) {
    console.error('Failed to load sessions:', err)
    error.value = t('sessionsList.errors.loadFailed')
  } finally {
    loading.value = false
  }
}

async function endSession(sessionId: string) {
  confirm.require({
    message: t('sessionsList.confirm.end.message'),
    header: t('sessionsList.confirm.end.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        endingSession.value = sessionId
        const result = await sessionsApi.end(sessionId)
        await loadSessions()
        toast.add({
          severity: result.passed ? 'success' : 'info',
          summary: t('sessionsList.toasts.ended.summary'),
          detail: result.passed
            ? t('sessionsList.toasts.ended.detailPassed', { points: result.earnedPoints })
            : t('sessionsList.toasts.ended.detailNotPassed', { points: result.earnedPoints }),
          life: 5000
        })
      } catch (err) {
        console.error('Failed to end session:', err)
        error.value = t('sessionsList.errors.endFailed')
      } finally {
        endingSession.value = null
      }
    }
  })
}

function viewSession(sessionId: string) {
  router.push(`/session/${sessionId}`)
}

async function deleteSession(sessionId: string) {
  confirm.require({
    message: t('sessionsList.confirm.delete.message'),
    header: t('sessionsList.confirm.delete.header'),
    icon: 'pi pi-trash',
    acceptClass: 'p-button-danger',
    accept: async () => {
      try {
        deletingSession.value = sessionId
        await sessionsApi.delete(sessionId)
        await loadSessions()
        toast.add({
          severity: 'success',
          summary: t('sessionsList.toasts.deleted.summary'),
          detail: t('sessionsList.toasts.deleted.detail'),
          life: 3000
        })
      } catch (err) {
        console.error('Failed to delete session:', err)
        toast.add({
          severity: 'error',
          summary: t('sessionsList.toasts.deleteFailed.summary'),
          detail: t('sessionsList.toasts.deleteFailed.detail'),
          life: 5000
        })
      } finally {
        deletingSession.value = null
      }
    }
  })
}

// getStatusSeverity imported from @/utils/status
// formatDate, formatDurationFromTimestamps imported from @/utils/format

const STALE_THRESHOLD_MS = 24 * 60 * 60 * 1000 // 24 hours

function isStale(session: Session): boolean {
  if (session.endedAt) return false
  const start = new Date(session.startedAt)
  return Date.now() - start.getTime() > STALE_THRESHOLD_MS
}

const staleCount = computed(() => sessions.value.filter(isStale).length)

async function cleanupStaleSessions() {
  confirm.require({
    message: t('sessionsList.confirm.cleanup.message', { count: staleCount.value }),
    header: t('sessionsList.confirm.cleanup.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        cleaningUp.value = true
        const result = await sessionsApi.cleanup('24h')
        await loadSessions()
        toast.add({
          severity: 'success',
          summary: t('sessionsList.toasts.cleanedUp.summary'),
          detail: t('sessionsList.toasts.cleanedUp.detail', { count: result.cleaned }),
          life: 5000
        })
      } catch (err) {
        console.error('Failed to clean up stale sessions:', err)
        toast.add({
          severity: 'error',
          summary: t('sessionsList.toasts.cleanupFailed.summary'),
          detail: t('sessionsList.toasts.cleanupFailed.detail'),
          life: 5000
        })
      } finally {
        cleaningUp.value = false
      }
    }
  })
}

function clearFilters() {
  filterUserId.value = ''
  filterActive.value = { label: t('sessionsList.filters.status.all'), value: null }
}
</script>

<template>
  <div class="space-y-6">
    <ConfirmDialog />

    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('sessionsList.title') }}</h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('sessionsList.subtitle') }}</p>
      </div>
      <div class="flex gap-2">
        <Button
          v-if="staleCount > 0"
          :loading="cleaningUp"
          :disabled="cleaningUp"
          icon="pi pi-broom"
          :label="t('sessionsList.actions.cleanupStale', { count: staleCount })"
          severity="warn"
          @click="cleanupStaleSessions"
        />
        <Button
          :loading="loading"
          :disabled="loading"
          icon="pi pi-refresh"
          :label="t('sessionsList.actions.refresh')"
          @click="loadSessions"
        />
      </div>
    </div>

    <!-- Filters -->
    <Card>
      <template #content>
        <h2 class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-3">{{ t('sessionsList.filters.heading') }}</h2>
        <div class="flex flex-wrap gap-4 items-end">
          <div class="flex-1 min-w-[200px]">
            <label class="block text-xs text-surface-500 mb-1">{{ t('sessionsList.filters.userIdLabel') }}</label>
            <InputText
              v-model="filterUserId"
              :placeholder="t('sessionsList.filters.userIdPlaceholder')"
              :aria-label="t('sessionsList.filters.userIdLabel')"
              class="w-full"
            />
          </div>
          <div class="min-w-[150px]">
            <label class="block text-xs text-surface-500 mb-1">{{ t('sessionsList.filters.statusLabel') }}</label>
            <Select
              v-model="filterActive"
              :options="statusOptions"
              optionLabel="label"
              :aria-label="t('sessionsList.filters.statusLabel')"
              class="w-full"
            />
          </div>
          <Button
            :label="t('sessionsList.filters.clearFilters')"
            text
            size="small"
            @click="clearFilters"
          />
        </div>
      </template>
    </Card>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Sessions List -->
    <div v-else-if="sessions.length" class="space-y-4">
      <Card v-for="session in sessions" :key="session.id" class="hover:shadow-md transition-shadow">
        <template #content>
          <div class="flex items-start justify-between gap-4">
            <div class="flex-1 space-y-4">
              <div class="flex items-center gap-3">
                <h3 class="text-lg font-semibold text-surface-900 dark:text-surface-100">{{ session.labTemplateId }}</h3>
                <Tag :value="session.status" :severity="getStatusSeverity(session.status)" />
                <Tag v-if="isStale(session)" :value="t('sessionsList.fields.staleTag')" severity="danger" />
              </div>
              <p class="text-sm text-surface-500 font-mono">{{ session.id }}</p>

              <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                <div>
                  <span class="text-surface-500">{{ t('sessionsList.fields.user') }}:</span>
                  <span class="ml-1 font-medium text-surface-900 dark:text-surface-100">{{ session.userId }}</span>
                </div>
                <div>
                  <span class="text-surface-500">{{ t('sessionsList.fields.pod') }}:</span>
                  <span class="ml-1 font-mono text-xs text-surface-900 dark:text-surface-100">{{ session.podId.slice(0, 8) }}...</span>
                </div>
                <div>
                  <span class="text-surface-500">{{ t('sessionsList.fields.started') }}:</span>
                  <span class="ml-1 text-surface-900 dark:text-surface-100">{{ formatDate(session.startedAt) }}</span>
                </div>
                <div>
                  <span class="text-surface-500">{{ t('sessionsList.fields.duration') }}:</span>
                  <span class="ml-1 text-surface-900 dark:text-surface-100">{{ formatDurationFromTimestamps(session.startedAt, session.endedAt) }}</span>
                </div>
              </div>

              <!-- Score Progress -->
              <div>
                <div class="flex items-center justify-between text-sm mb-2">
                  <span class="text-surface-600 dark:text-surface-400">{{ t('sessionsList.fields.scoreProgress') }}</span>
                  <span class="font-medium text-surface-900 dark:text-surface-100">
                    {{ t('sessionsList.fields.scoreValue', { earned: session.earnedPoints, total: session.maxPoints, percent: session.percentage.toFixed(0) }) }}
                  </span>
                </div>
                <ProgressBar :value="session.percentage" :showValue="false" />
                <div class="text-xs text-surface-500 mt-1">
                  <span v-if="session.passed" class="text-green-600 dark:text-green-400 font-medium">{{ t('sessionsList.fields.passed') }}</span>
                  <span v-else-if="session.status === 'completed'">{{ t('sessionsList.fields.notPassed') }}</span>
                </div>
              </div>
            </div>

            <!-- Actions -->
            <div class="flex flex-col gap-2">
              <Button
                icon="pi pi-eye"
                :label="t('sessionsList.actions.view')"
                severity="secondary"
                size="small"
                @click="viewSession(session.id)"
              />
              <Button
                v-if="session.status === 'active'"
                :loading="endingSession === session.id"
                :disabled="endingSession === session.id"
                icon="pi pi-stop-circle"
                :label="t('sessionsList.actions.end')"
                severity="warn"
                size="small"
                @click="endSession(session.id)"
              />
              <Button
                :loading="deletingSession === session.id"
                :disabled="deletingSession === session.id"
                icon="pi pi-trash"
                :label="t('sessionsList.actions.delete')"
                severity="danger"
                size="small"
                @click="deleteSession(session.id)"
              />
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- Empty state -->
    <Card v-else>
      <template #content>
        <div class="text-center py-12">
          <i class="pi pi-clipboard text-5xl text-surface-400 mb-4" />
          <h3 class="text-lg font-medium text-surface-900 dark:text-surface-100 mb-1">{{ t('sessionsList.empty.title') }}</h3>
          <p class="text-surface-500">
            {{ filterUserId || filterActive.value !== null ? t('sessionsList.empty.hintFiltered') : t('sessionsList.empty.hintEmpty') }}
          </p>
        </div>
      </template>
    </Card>
  </div>
</template>
