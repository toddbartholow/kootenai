<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import type { Pod } from '@/api'
import { usePodsStore } from '@/stores/pods'
import { getStatusSeverity } from '@/utils/status'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import { SkeletonCardGrid } from '@/components/common'
import { useConfirm } from 'primevue/useconfirm'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDateTime } = useFormatters()
const router = useRouter()
const confirm = useConfirm()
const podsStore = usePodsStore()
const { activePods, loading, error, actionLoading } = storeToRefs(podsStore)

onMounted(async () => {
  await podsStore.fetchPods()
})

async function handleRemovePod(pod: Pod) {
  confirm.require({
    message: t('pods.confirm.remove.message', { name: pod.labTemplate }),
    header: t('pods.confirm.remove.header'),
    icon: 'pi pi-info-circle',
    accept: async () => {
      await podsStore.removePod(pod.id)
    }
  })
}

async function handleDestroyPod(pod: Pod) {
  confirm.require({
    message: t('pods.confirm.destroy.message', { name: pod.name || pod.id }),
    header: t('pods.confirm.destroy.header'),
    icon: 'pi pi-exclamation-triangle',
    acceptClass: 'p-button-danger',
    accept: async () => {
      await podsStore.destroyPod(pod.id)
    }
  })
}

async function handleResetPod(pod: Pod) {
  if (!pod.vms?.length) {
    podsStore.error = { message: t('pods.errors.noVmsToReset'), type: 'validation', timestamp: new Date(), retryable: false }
    return
  }
  try {
    await podsStore.resetPod(pod.id)
  } catch {
    // Error is already set in store
  }
}

async function handleStartPod(pod: Pod) {
  await podsStore.startPod(pod.id)
}

async function handleStopPod(pod: Pod) {
  confirm.require({
    message: t('pods.confirm.stop.message', { name: pod.name || pod.id }),
    header: t('pods.confirm.stop.header'),
    icon: 'pi pi-power-off',
    accept: async () => {
      await podsStore.stopPod(pod.id)
    }
  })
}

function viewPod(pod: Pod) {
  router.push(`/pods/${pod.id}`)
}

function clearError() {
  podsStore.error = null
}
</script>

<template>
  <div class="space-y-8">
    <ConfirmDialog />

    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('pods.title') }}</h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('pods.subtitle') }}</p>
      </div>
      <div class="flex gap-2">
        <RouterLink to="/pods/create">
          <Button
            icon="pi pi-plus"
            :label="t('pods.actions.create')"
          />
        </RouterLink>
        <Button
          :loading="loading"
          :disabled="loading"
          icon="pi pi-refresh"
          :label="t('pods.actions.refresh')"
          severity="secondary"
          @click="podsStore.fetchPods()"
        />
      </div>
    </div>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="clearError">
      {{ error.message }}
    </Message>

    <!-- Loading state -->
    <SkeletonCardGrid
      v-if="loading"
      :count="4"
      columns="grid-cols-1 md:grid-cols-2"
      :show-tags="false"
      :info-lines="3"
    />

    <!-- Empty state -->
    <Card v-else-if="activePods.length === 0">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-box text-5xl text-surface-400 mb-4" />
          <h3 class="text-lg font-medium text-surface-900 dark:text-surface-100">{{ t('pods.empty.title') }}</h3>
          <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('pods.empty.hint') }}</p>
          <RouterLink to="/labs" class="inline-block mt-4">
            <Button :label="t('pods.actions.browseLabs')" icon="pi pi-arrow-right" iconPos="right" />
          </RouterLink>
        </div>
      </template>
    </Card>

    <!-- Pods grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <Card
        v-for="pod in activePods"
        :key="pod.id"
        class="cursor-pointer hover:shadow-lg transition-shadow focus-within:ring-2 focus-within:ring-primary-500"
        role="button"
        tabindex="0"
        :aria-label="t('pods.card.viewAria', { name: pod.name || pod.id })"
        @click="viewPod(pod)"
        @keydown.enter="viewPod(pod)"
        @keydown.space.prevent="viewPod(pod)"
      >
        <template #content>
          <div class="space-y-4">
            <div class="flex items-start justify-between">
              <div>
                <h3 class="font-semibold text-surface-900 dark:text-surface-100">{{ pod.name || pod.id }}</h3>
                <p class="text-sm text-surface-500">{{ pod.labTemplate }}</p>
              </div>
              <Tag :value="pod.status" :severity="getStatusSeverity(pod.status)" />
            </div>

            <div class="text-sm text-surface-600 dark:text-surface-400 space-y-1">
              <p><i class="pi pi-server mr-2" />{{ t('pods.card.vmsCount', { count: pod.vms?.length || 0 }) }}</p>
              <p><i class="pi pi-cloud mr-2" />{{ pod.platform }}</p>
              <p v-if="pod.expiresAt"><i class="pi pi-clock mr-2" />{{ t('pods.card.expiresLabel', { when: formatDateTime(pod.expiresAt) }) }}</p>
            </div>

            <div class="flex gap-2" @click.stop>
              <!-- Error state - show Remove button -->
              <template v-if="pod.status === 'error'">
                <Button
                  :loading="actionLoading === pod.id"
                  :disabled="actionLoading === pod.id"
                  icon="pi pi-times"
                  :label="t('pods.actionLabels.remove')"
                  severity="secondary"
                  size="small"
                  class="flex-1"
                  @click="handleRemovePod(pod)"
                />
              </template>
              <!-- Normal states -->
              <template v-else>
                <!-- Start button (shown when stopped) -->
                <Button
                  v-if="pod.status === 'stopped'"
                  :loading="actionLoading === pod.id"
                  :disabled="actionLoading === pod.id"
                  icon="pi pi-play"
                  :label="t('pods.actionLabels.start')"
                  severity="success"
                  size="small"
                  class="flex-1"
                  @click="handleStartPod(pod)"
                />
                <!-- Stop button (shown when running) -->
                <Button
                  v-if="pod.status === 'running'"
                  :loading="actionLoading === pod.id"
                  :disabled="actionLoading === pod.id"
                  icon="pi pi-stop"
                  :label="t('pods.actionLabels.stop')"
                  severity="warn"
                  size="small"
                  class="flex-1"
                  @click="handleStopPod(pod)"
                />
                <Button
                  :loading="actionLoading === pod.id"
                  :disabled="actionLoading === pod.id || pod.status !== 'running'"
                  icon="pi pi-refresh"
                  :label="t('pods.actionLabels.reset')"
                  severity="secondary"
                  size="small"
                  class="flex-1"
                  @click="handleResetPod(pod)"
                />
                <Button
                  :loading="actionLoading === pod.id"
                  :disabled="actionLoading === pod.id"
                  icon="pi pi-trash"
                  :label="t('pods.actionLabels.destroy')"
                  severity="danger"
                  size="small"
                  class="flex-1"
                  @click="handleDestroyPod(pod)"
                />
              </template>
            </div>
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
