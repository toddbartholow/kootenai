<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { podsApi, sessionsApi, topologyApi, type Pod, type TopologyData, type TopologyVM } from '@/api'
import { useAuthStore } from '../stores/auth'
import { useFocusRestore, usePodRealtime, useConsoleLauncher } from '../composables'
import { getStatusSeverity } from '@/utils/status'
import VncConsole from '../components/console/VncConsole.vue'
import TopologyVisualization from '../components/topology/TopologyVisualization.vue'
import VMListItem from '../components/pod/VMListItem.vue'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import Toast from '@volt/Toast.vue'
import Tabs from '@volt/Tabs.vue'
import TabList from '@volt/TabList.vue'
import Tab from '@volt/Tab.vue'
import TabPanels from '@volt/TabPanels.vue'
import TabPanel from '@volt/TabPanel.vue'
import { useConfirm } from 'primevue/useconfirm'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDateTime } = useFormatters()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const confirm = useConfirm()

const pod = ref<Pod | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const actionLoading = ref(false)

// Topology state
const topology = ref<TopologyData | null>(null)
const topologyLoading = ref(false)
const topologyError = ref<string | null>(null)
const activeTab = ref('0')

// VM expand/collapse state. Only one VM can be expanded at a time.
const expandedVM = ref<string | null>(null)

// Console launcher — modal visibility, ticket fetch, etc. live in the
// composable. Restoration of focus on modal close is wired through the
// composable's `visible` ref.
const consoleLauncher = useConsoleLauncher()
useFocusRestore(consoleLauncher.visible)

const podId = route.params['podId'] as string

// WebSocket lifecycle and pod/VM state merging live in the composable.
// It auto-connects on mount and auto-disconnects on unmount.
const { wsConnected } = usePodRealtime(podId, pod)

onMounted(async () => {
  await loadPod()
})

async function loadPod() {
  try {
    loading.value = true
    error.value = null
    pod.value = await podsApi.get(podId)
    // Load topology in parallel
    loadTopology()
  } catch (err) {
    console.error('Failed to load pod:', err)
    error.value = t('podDetail.errors.loadFailed')
  } finally {
    loading.value = false
  }
}

async function loadTopology() {
  try {
    topologyLoading.value = true
    topologyError.value = null
    topology.value = await topologyApi.getTopology(podId)
  } catch (err) {
    console.error('Failed to load topology:', err)
    topologyError.value = t('podDetail.errors.topologyFailed')
  } finally {
    topologyLoading.value = false
  }
}

function handleTopologyVMClick(vm: TopologyVM) {
  // Find and expand the VM in the list
  expandedVM.value = vm.name
}

function handleTopologyVMDblClick(vm: TopologyVM) {
  // Open the VNC console for a topology node when the user double-clicks it.
  const podVM = pod.value?.vms?.find(v => v.name === vm.name)
  if (podVM && (podVM.status === 'running' || podVM.status === 'created')) {
    void consoleLauncher.openConsole({
      name: podVM.name,
      platformId: podVM.platformId,
      node: 'pve',
    })
  }
}

function toggleVMExpanded(vmName: string) {
  if (expandedVM.value === vmName) {
    expandedVM.value = null
  } else {
    expandedVM.value = vmName
    // <VMSnapshotManager> loads its own snapshot list on mount.
  }
}

async function startSession() {
  if (!pod.value) return

  try {
    actionLoading.value = true
    const userId = authStore.user?.id || '00000000-0000-0000-0000-000000000001'

    // Check for pathway context from query params (set when launching from enrollment view)
    const enrollmentId = route.query['enrollmentId'] as string | undefined
    const moduleId = route.query['moduleId'] as string | undefined

    const response = await sessionsApi.create({
      podId: pod.value.id,
      userId,
      labTemplate: pod.value.labTemplate,
      // Include pathway context if available - links session progress to pathway
      ...(enrollmentId && moduleId ? { enrollmentId, moduleId } : {}),
    })
    router.push(`/session/${response.sessionId}`)
  } catch (err) {
    console.error('Failed to start session:', err)
    error.value = t('podDetail.errors.startSessionFailed')
  } finally {
    actionLoading.value = false
  }
}

async function resetPod() {
  if (!pod.value || !pod.value.vms?.length) return

  try {
    actionLoading.value = true
    for (const vm of pod.value.vms) {
      await podsApi.resetVM(pod.value.id, vm.name, 'initial')
    }
    await loadPod()
  } catch (err) {
    console.error('Failed to reset pod:', err)
    error.value = t('podDetail.errors.resetFailed')
  } finally {
    actionLoading.value = false
  }
}

function confirmDestroyPod() {
  confirm.require({
    message: t('podDetail.confirm.destroy.message'),
    header: t('podDetail.confirm.destroy.header'),
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: t('podDetail.confirm.destroy.reject') },
    acceptProps: { label: t('podDetail.confirm.destroy.accept') },
    accept: destroyPod
  })
}

async function destroyPod() {
  if (!pod.value) return

  try {
    actionLoading.value = true
    await podsApi.destroy(pod.value.id)
    router.push('/pods')
  } catch (err) {
    console.error('Failed to destroy pod:', err)
    error.value = t('podDetail.errors.destroyFailed')
  } finally {
    actionLoading.value = false
  }
}

async function startPod() {
  if (!pod.value) return

  try {
    actionLoading.value = true
    const updated = await podsApi.start(pod.value.id)
    pod.value = updated
  } catch (err) {
    console.error('Failed to start pod:', err)
    error.value = t('podDetail.errors.startFailed')
  } finally {
    actionLoading.value = false
  }
}

function confirmStopPod() {
  confirm.require({
    message: t('podDetail.confirm.stop.message'),
    header: t('podDetail.confirm.stop.header'),
    icon: 'pi pi-power-off',
    rejectProps: { label: t('podDetail.confirm.stop.reject') },
    acceptProps: { label: t('podDetail.confirm.stop.accept') },
    accept: stopPod
  })
}

async function stopPod() {
  if (!pod.value) return

  try {
    actionLoading.value = true
    const updated = await podsApi.stop(pod.value.id)
    pod.value = updated
  } catch (err) {
    console.error('Failed to stop pod:', err)
    error.value = t('podDetail.errors.stopFailed')
  } finally {
    actionLoading.value = false
  }
}

// Snapshot-driven VM resets moved into <VMSnapshotManager>. The remaining
// resetPod() below still calls podsApi.resetVM directly for the bulk
// "reset all VMs to initial" flow.

// Per-VM start/stop/suspend/resume has moved into <VMPowerControls>.
// The component manages its own loading state, toasts, and confirm dialog.
// Parent listens for `@changed` to refresh the pod and `@error` to surface
// failure messages in the view's existing error banner.

// Snapshot Create / Delete / Revert logic moved into <VMSnapshotManager>.
// Per-VM card layout lives in <VMListItem>. Console launcher (ticket
// fetch, modal state, SPICE download) lives in useConsoleLauncher.
</script>

<template>
  <div class="space-y-6">
    <ConfirmDialog />
    <Toast />

    <!-- Back button -->
    <div>
      <RouterLink to="/pods">
        <Button icon="pi pi-arrow-left" :label="t('podDetail.backLabel')" text />
      </RouterLink>
    </div>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Pod details -->
    <div v-else-if="pod" class="space-y-6">
      <Card>
        <template #content>
          <div class="flex items-start justify-between mb-4">
            <div>
              <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ pod.labTemplate }}</h1>
              <p class="text-sm text-surface-500 font-mono">{{ pod.id }}</p>
            </div>
            <div class="flex items-center gap-3">
              <!-- WebSocket Status -->
              <div class="flex items-center gap-2">
                <span
                  :class="[
                    'w-2 h-2 rounded-full',
                    wsConnected ? 'bg-green-500' : 'bg-surface-400'
                  ]"
                />
                <span class="text-xs text-surface-500">
                  {{ wsConnected ? t('podDetail.wsStatus.live') : t('podDetail.wsStatus.offline') }}
                </span>
              </div>
              <!-- Pod Status -->
              <Tag :value="pod.status" :severity="getStatusSeverity(pod.status)" />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span class="text-surface-500">{{ t('podDetail.fields.platform') }}:</span>
              <span class="ml-2 font-medium text-surface-900 dark:text-surface-100">{{ pod.platform }}</span>
            </div>
            <div>
              <span class="text-surface-500">{{ t('podDetail.fields.owner') }}:</span>
              <span class="ml-2 font-medium text-surface-900 dark:text-surface-100">{{ pod.owner }}</span>
            </div>
            <div>
              <span class="text-surface-500">{{ t('podDetail.fields.created') }}:</span>
              <span class="ml-2 font-medium text-surface-900 dark:text-surface-100">{{ formatDateTime(pod.createdAt) }}</span>
            </div>
            <div v-if="pod.expiresAt">
              <span class="text-surface-500">{{ t('podDetail.fields.expires') }}:</span>
              <span class="ml-2 font-medium text-surface-900 dark:text-surface-100">{{ formatDateTime(pod.expiresAt) }}</span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex gap-3 mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
            <!-- Start button (shown when stopped) -->
            <Button
              v-if="pod.status === 'stopped'"
              :disabled="actionLoading"
              :loading="actionLoading"
              icon="pi pi-play"
              :label="t('podDetail.actions.startPod')"
              severity="success"
              @click="startPod"
            />
            <!-- Stop button (shown when running) -->
            <Button
              v-if="pod.status === 'running'"
              :disabled="actionLoading"
              :loading="actionLoading"
              icon="pi pi-stop"
              :label="t('podDetail.actions.stopPod')"
              severity="warn"
              @click="confirmStopPod"
            />
            <Button
              :disabled="actionLoading || pod.status !== 'running'"
              :loading="actionLoading"
              icon="pi pi-flag"
              :label="t('podDetail.actions.startSession')"
              @click="startSession"
            />
            <Button
              :disabled="actionLoading || pod.status !== 'running'"
              icon="pi pi-refresh"
              :label="t('podDetail.actions.resetAllVms')"
              severity="secondary"
              @click="resetPod"
            />
            <Button
              :disabled="actionLoading"
              icon="pi pi-trash"
              :label="t('podDetail.actions.destroy')"
              severity="danger"
              @click="confirmDestroyPod"
            />
          </div>
        </template>
      </Card>

      <!-- VMs and Topology -->
      <Card>
        <template #title>
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <i class="pi pi-server text-xl" />
              {{ t('podDetail.vms.cardTitle') }}
            </div>
          </div>
        </template>
        <template #content>
          <Tabs v-model:value="activeTab">
            <TabList>
              <Tab value="0">
                <i class="pi pi-list mr-2" />
                {{ t('podDetail.vms.listTab') }}
              </Tab>
              <Tab value="1">
                <i class="pi pi-sitemap mr-2" />
                {{ t('podDetail.vms.topologyTab') }}
              </Tab>
            </TabList>
            <TabPanels>
              <!-- VM List Panel -->
              <TabPanel value="0">
                <div v-if="!pod.vms?.length" class="text-surface-500 text-center py-4">
                  {{ t('podDetail.vms.empty') }}
                </div>

                <div v-else class="space-y-4 mt-4">
                  <VMListItem
                    v-for="vm in pod.vms"
                    :key="vm.name"
                    :vm="vm"
                    :pod-id="pod.id"
                    :expanded="expandedVM === vm.name"
                    @toggle="toggleVMExpanded(vm.name)"
                    @changed="loadPod"
                    @error="(msg) => (error = msg)"
                    @open-console="(target) => consoleLauncher.openConsole(target)"
                    @download-spice="(target) => consoleLauncher.downloadSpiceFile(target)"
                  />
                </div>
              </TabPanel>

              <!-- Topology Panel -->
              <TabPanel value="1">
                <div class="mt-4">
                  <div v-if="topologyLoading" class="flex justify-center py-12">
                    <ProgressSpinner />
                  </div>
                  <Message v-else-if="topologyError" severity="warn" :closable="false">
                    {{ topologyError }}
                  </Message>
                  <div v-else-if="topology" class="h-[500px]">
                    <TopologyVisualization
                      :topology="topology"
                      :selected-v-m="expandedVM ?? undefined"
                      @vm-click="handleTopologyVMClick"
                      @vm-dblclick="handleTopologyVMDblClick"
                    />
                  </div>
                  <div v-else class="text-surface-500 text-center py-12">
                    <i class="pi pi-sitemap text-4xl mb-4 block" />
                    <p>{{ t('podDetail.vms.topologyEmpty') }}</p>
                  </div>
                </div>
              </TabPanel>
            </TabPanels>
          </Tabs>
        </template>
      </Card>
    </div>

    <!-- Not found -->
    <Card v-else>
      <template #content>
        <div class="text-center py-12">
          <i class="pi pi-inbox text-5xl text-surface-400 mb-4" />
          <p class="text-surface-500 text-lg">{{ t('podDetail.notFound.title') }}</p>
          <RouterLink to="/pods">
            <Button :label="t('podDetail.notFound.goBack')" link class="mt-2" />
          </RouterLink>
        </div>
      </template>
    </Card>

    <!-- Snapshot Create / Revert modals are inside <VMSnapshotManager>. -->

    <!-- VNC Console (floating window). All state comes from the
         useConsoleLauncher composable. -->
    <VncConsole
      v-if="consoleLauncher.visible.value && consoleLauncher.ticket.value && consoleLauncher.vm.value && consoleLauncher.ticket.value.type === 'vnc'"
      :vmid="parseInt(consoleLauncher.vm.value.id, 10)"
      :ticket="consoleLauncher.ticket.value"
      @close="consoleLauncher.closeConsole()"
    />

    <!-- Loading toast while the ticket is being fetched. -->
    <div v-if="consoleLauncher.visible.value && consoleLauncher.loading.value" class="fixed bottom-4 right-4 z-50">
      <Card class="shadow-lg">
        <template #content>
          <div class="flex items-center gap-3">
            <ProgressSpinner style="width: 20px; height: 20px" />
            <span class="text-surface-700 dark:text-surface-300">
              {{ t('podDetail.console.loadingVnc', { name: consoleLauncher.vm.value?.name ?? '' }) }}
            </span>
          </div>
        </template>
      </Card>
    </div>

    <!-- Error toast with close button. -->
    <div v-if="consoleLauncher.visible.value && consoleLauncher.error.value" class="fixed bottom-4 right-4 z-50 max-w-md">
      <Card class="shadow-lg border-red-200 dark:border-red-800">
        <template #content>
          <div class="flex items-start gap-3">
            <i class="pi pi-exclamation-triangle text-xl text-red-500" />
            <div class="flex-1">
              <p class="font-medium text-surface-900 dark:text-surface-100">{{ t('podDetail.console.connectionFailedTitle') }}</p>
              <p class="text-sm text-surface-600 dark:text-surface-400">{{ consoleLauncher.error.value }}</p>
            </div>
            <Button
              icon="pi pi-times"
              text
              rounded
              severity="secondary"
              size="small"
              @click="consoleLauncher.closeConsole()"
            />
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
