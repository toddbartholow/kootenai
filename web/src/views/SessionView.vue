<script setup lang="ts">
import { onMounted, onUnmounted, computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AssessmentPanel } from '@/components/assessment'
import { LabCompletionPanel, PodControlPanel, AchievementUnlockModal, InstructionsPanel, SnapshotPanel, QuestionsPanel, CheckpointsList, VMSelectorTabs } from '@/components/session'
import { useAssessmentStore } from '@/stores/assessment'
import { useSessionStore } from '@/stores/session'
import { useVMConsole, useSessionActions, useSessionWebSocket, useNotifications, useFocusRestore } from '@/composables'
import { podsApi, type Pod, type PodVM } from '@/api'
import { getStatusSeverity } from '@/utils/status'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Message from '@volt/Message.vue'
import Dialog from '@volt/Dialog.vue'
import VncConsole from '@/components/console/VncConsole.vue'
import InlineVncConsole from '@/components/console/InlineVncConsole.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import Toast from '@volt/Toast.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const assessmentStore = useAssessmentStore()
const sessionStore = useSessionStore()
const toast = useToast()
const confirm = useConfirm()
const notify = useNotifications()

const sessionId = computed(() => route.params['sessionId'] as string)

// Instructions panel state
const showInstructionsPanel = ref(false)
// Restore keyboard focus to the trigger when the instructions panel closes.
useFocusRestore(showInstructionsPanel)

// Side panel state
const activePanel = ref<'completion' | 'pod' | 'assessment' | 'snapshots' | 'questions'>('assessment')

// Pod data for main workspace console
const pod = ref<Pod | null>(null)
const podLoading = ref(false)

// Screen-reader progress announcement. The visual progress region below
// updates every time a WebSocket message arrives, which would cause SR
// software to re-announce numbers several times per second — noisy and
// unhelpful. We instead write to a dedicated aria-live span ONLY when
// meaningful transitions happen: a checkpoint passes, or the percentage
// crosses a 25% milestone.
const progressAnnouncement = ref('')
function pushProgressAnnouncement(text: string) {
  // Reset first so the same message fires again (screen readers only
  // announce aria-live changes, not repeats of identical text).
  progressAnnouncement.value = ''
  requestAnimationFrame(() => {
    progressAnnouncement.value = text
  })
}
watch(
  () => sessionStore.passedCheckpoints,
  (next, prev) => {
    if (prev === undefined) return // initial mount; don't announce the starting state
    if (next > prev) {
      pushProgressAnnouncement(
        `Checkpoint ${next} of ${sessionStore.totalCheckpoints} completed.`,
      )
    }
  },
)
// Announce when the percentage crosses a 25% bucket so screen-reader
// users get a sense of progress without hearing every tick.
let lastBucket = -1
watch(
  () => sessionStore.percentage,
  (pct) => {
    const bucket = Math.floor(pct / 25)
    if (bucket !== lastBucket && bucket > 0) {
      lastBucket = bucket
      pushProgressAnnouncement(`${bucket * 25} percent of lab complete.`)
    }
  },
)

// Selected VM for inline console
const selectedVM = ref<PodVM | null>(null)
const inlineConsoleActive = ref(false)

// Get pod ID and lab template from session
const podId = computed(() => sessionStore.currentSession?.podId)
const labTemplateId = computed(() => sessionStore.currentSession?.labTemplateId || '')
const labTemplateName = computed(() => sessionStore.currentSession?.labTemplateName || '')

// Use composables for extracted functionality
const {
  visible: showConsoleModal,
  vm: consoleVM,
  ticket: consoleTicket,
  loading: consoleLoading,
  openConsole: openVMConsole,
  closeConsole,
  isVNCConsole,
  getVMID,
} = useVMConsole({
  onError: (error) => {
    toast.add({
      severity: 'error',
      summary: t('session.console.connectionFailedSummary'),
      detail: error,
      life: 5000
    })
  }
})
// Restore focus to the VM-list button that opened the console on close.
useFocusRestore(showConsoleModal)

const {
  showAchievementModal,
  unlockedAchievements,
  handleEndSession: doEndSession,
  handleSubmitLab: doSubmitLab,
  closeAchievementModal,
} = useSessionActions({
  sessionStore,
  router,
  confirm,
  toast,
})

const { wsConnected, wsError, disconnect: disconnectWebSocket } = useSessionWebSocket(sessionId, {
  assessmentStore,
  sessionStore,
})

// Wrap session actions to use current sessionId
function handleEndSession() {
  doEndSession(sessionId.value)
}

function handleSubmitLab() {
  doSubmitLab(sessionId.value)
  activePanel.value = 'completion'
}

// Load pod data for inline console
async function loadPod() {
  if (!podId.value) return

  try {
    podLoading.value = true
    pod.value = await podsApi.get(podId.value)

    // Auto-select first running VM for console
    if (pod.value?.vms?.length) {
      const runningVM = pod.value.vms.find(vm => vm.status === 'running')
      if (runningVM && !selectedVM.value) {
        selectedVM.value = runningVM
      }
    }
  } catch (err) {
    console.error('Failed to load pod:', err)
    notify.error(t('session.notifications.loadPodFailed'))
  } finally {
    podLoading.value = false
  }
}

// Select VM for inline console - no toggle needed, InlineVncConsole caches connections
function selectVMForConsole(vm: PodVM) {
  selectedVM.value = vm
  // Activate console if not already active
  if (!inlineConsoleActive.value) {
    inlineConsoleActive.value = true
  }
}

// Open VM console in floating window (from PodControlPanel)
function handleOpenConsole(vm: PodVM) {
  openVMConsole(vm)
}

// Deactivate inline console when switching to floating
watch(showConsoleModal, (visible) => {
  if (visible) {
    inlineConsoleActive.value = false
  }
})

// Reload pod when podId changes
watch(podId, (newPodId) => {
  if (newPodId) {
    loadPod()
  }
})

function handleViewPod() {
  if (podId.value) {
    router.push(`/pods/${podId.value}`)
  }
}

function handleCloseCompletion() {
  router.push('/sessions')
}

// Status utilities imported from @/utils/status

// Reset to initial state (all VMs)
async function resetToInitial() {
  if (!pod.value?.id) return

  confirm.require({
    message: t('session.confirm.reset.message'),
    header: t('session.confirm.reset.header'),
    icon: 'pi pi-exclamation-triangle',
    accept: async () => {
      try {
        notify.info({
          title: t('session.notifications.resetting.title'),
          message: t('session.notifications.resetting.message'),
        })
        for (const vm of pod.value!.vms || []) {
          await podsApi.resetVM(pod.value!.id, vm.name, 'initial')
        }
        await loadPod()
        notify.success({
          title: t('session.notifications.resetComplete.title'),
          message: t('session.notifications.resetComplete.message'),
        })
      } catch (err) {
        console.error('Failed to reset VMs:', err)
        notify.error(t('session.notifications.resetFailed'))
      }
    }
  })
}

// Open full console in new window (LTI console with all features)
// Always use browser origin to go through nginx proxy (for HTTPS/SSL)
function openFullConsole() {
  if (!podId.value) return
  const consoleUrl = `${window.location.origin}/lti/console?podId=${podId.value}`
  window.open(consoleUrl, '_blank')
}

onMounted(async () => {
  // Fetch session data
  await sessionStore.fetchSession(sessionId.value)

  // Also fetch progress for checkpoints
  await assessmentStore.fetchAssessment(sessionId.value)

  // Load pod after session is loaded
  if (podId.value) {
    await loadPod()
  }
})

onUnmounted(() => {
  // Cleanup WebSocket and stores
  disconnectWebSocket()
  assessmentStore.reset()
  sessionStore.reset()
})
</script>

<template>
  <div class="space-y-6">
    <Toast />
    <ConfirmDialog />

    <!-- Header - responsive layout -->
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <Button
          icon="pi pi-arrow-left"
          text
          rounded
          :aria-label="t('session.actions.goBackAria')"
          @click="router.back()"
        />
        <div>
          <h1 class="text-xl font-bold text-surface-900 dark:text-surface-100">
            {{ labTemplateName || t('session.fallbackTitle') }}
          </h1>
          <p class="text-sm text-surface-500">
            {{ t('session.idLabel') }} <code class="bg-surface-100 dark:bg-surface-800 px-1.5 py-0.5 rounded font-mono text-xs">{{ sessionId.slice(0, 8) }}...</code>
          </p>
        </div>
      </div>
      <div class="flex gap-2 w-full sm:w-auto">
        <Button
          icon="pi pi-book"
          :label="t('session.actions.instructions')"
          class="flex-1 sm:flex-initial"
          :aria-label="t('session.actions.instructionsAria')"
          @click="showInstructionsPanel = true"
        />
        <Button
          :disabled="sessionStore.submissionStatus !== 'idle' || sessionStore.sessionStatus === 'ended'"
          :loading="sessionStore.submissionStatus === 'submitting'"
          icon="pi pi-send"
          :label="t('session.actions.submit')"
          severity="secondary"
          outlined
          class="flex-1 sm:flex-initial"
          @click="handleSubmitLab"
        />
        <Button
          :disabled="sessionStore.loading || sessionStore.sessionStatus === 'ended'"
          :loading="sessionStore.loading"
          icon="pi pi-stop-circle"
          :label="t('session.actions.end')"
          severity="secondary"
          outlined
          class="flex-1 sm:flex-initial"
          @click="handleEndSession"
        />
      </div>
    </div>

    <!-- Loading state -->
    <div v-if="sessionStore.loading && !sessionStore.currentSession" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Error state -->
    <Message v-if="sessionStore.error" severity="error" :closable="true" @close="sessionStore.clearError()">
      {{ sessionStore.error.message }}
    </Message>

    <div v-else class="flex flex-col lg:flex-row gap-6">
      <!-- Main Content Area (Lab Console/Workspace) -->
      <div class="flex-1">
        <Card>
          <template #title>
            <div class="flex items-center justify-between">
              <span>{{ t('session.workspace.title') }}</span>
              <div class="flex items-center gap-3">
                <!-- Session Status -->
                <Tag :value="sessionStore.sessionStatus" :severity="getStatusSeverity(sessionStore.sessionStatus)" />
                <!-- WebSocket Status -->
                <div
                  class="flex items-center gap-2"
                  role="status"
                  :aria-label="wsConnected ? t('session.workspace.wsStatus.onlineAria') : t('session.workspace.wsStatus.offlineAria')"
                >
                  <span
                    :class="[
                      'w-2 h-2 rounded-full',
                      wsConnected ? 'bg-green-500' : 'bg-red-500'
                    ]"
                    aria-hidden="true"
                  />
                  <span class="text-xs text-surface-500">
                    {{ wsConnected ? t('session.workspace.wsStatus.live') : t('session.workspace.wsStatus.offline') }}
                  </span>
                </div>
              </div>
            </div>
          </template>
          <template #content>
            <!--
              Visual-only progress summary. aria-live is intentionally OMITTED
              here — this region updates on every WebSocket tick, which
              would flood screen readers with repeat announcements. Meaningful
              progress transitions are announced via the sr-only region below
              ("Checkpoint N of M completed", "X percent of lab complete").
            -->
            <div
              class="grid grid-cols-3 gap-2 sm:gap-4 mb-4 p-3 sm:p-4 bg-surface-100 dark:bg-surface-800 rounded-lg"
              role="region"
              :aria-label="t('session.workspace.progress.regionLabel')"
            >
              <div class="text-center border-r border-surface-200 dark:border-surface-700">
                <div class="text-xl sm:text-2xl font-semibold text-primary-500">{{ sessionStore.percentage }}%</div>
                <div class="text-xs text-surface-500 dark:text-surface-400">{{ t('session.workspace.progress.progressCell') }}</div>
              </div>
              <div class="text-center border-r border-surface-200 dark:border-surface-700">
                <div class="text-xl sm:text-2xl font-semibold text-surface-800 dark:text-surface-100">
                  {{ sessionStore.earnedPoints }}/{{ sessionStore.maxPoints }}
                </div>
                <div class="text-xs text-surface-500 dark:text-surface-400">{{ t('session.workspace.progress.pointsCell') }}</div>
              </div>
              <div class="text-center">
                <div class="text-xl sm:text-2xl font-semibold text-surface-800 dark:text-surface-100">
                  {{ sessionStore.passedCheckpoints }}/{{ sessionStore.totalCheckpoints }}
                </div>
                <div class="text-xs text-surface-500 dark:text-surface-400">{{ t('session.workspace.progress.checksCell') }}</div>
              </div>
            </div>

            <!-- Screen-reader-only live region for progress milestones. -->
            <span class="sr-only" aria-live="polite" role="status">{{ progressAnnouncement }}</span>

            <!-- VM Selection Tabs -->
            <VMSelectorTabs
              :vms="pod?.vms ?? []"
              :selected-name="selectedVM?.name ?? null"
              :active="inlineConsoleActive"
              @select="selectVMForConsole"
            />

            <!-- Inline Console Display -->
            <div class="bg-surface-900 rounded-lg overflow-hidden min-h-100 relative">
              <!-- Loading state -->
              <div v-if="podLoading" class="absolute inset-0 flex items-center justify-center">
                <div class="text-center">
                  <ProgressSpinner style="width: 32px; height: 32px" />
                  <p class="text-surface-400 mt-2">{{ t('session.workspace.loadingVms') }}</p>
                </div>
              </div>

              <!-- No VMs state -->
              <div v-else-if="!pod?.vms?.length" class="absolute inset-0 flex items-center justify-center">
                <div class="text-center p-6">
                  <i class="pi pi-server text-4xl text-surface-500 mb-3" />
                  <p class="text-surface-400 mb-2">{{ t('session.workspace.noVms.title') }}</p>
                  <p class="text-surface-500 text-sm">{{ t('session.workspace.noVms.hint') }}</p>
                </div>
              </div>

              <!-- No VM selected state -->
              <div v-else-if="!selectedVM || !inlineConsoleActive" class="absolute inset-0 flex items-center justify-center">
                <div class="text-center p-6">
                  <i class="pi pi-desktop text-4xl text-surface-500 mb-3" />
                  <p class="text-surface-300 mb-2">{{ t('session.workspace.noVmSelected.title') }}</p>
                  <p class="text-surface-500 text-sm mb-4">{{ t('session.workspace.noVmSelected.hint') }}</p>
                  <div class="flex flex-wrap justify-center gap-2">
                    <Button
                      v-for="vm in pod?.vms?.filter(v => v.status === 'running')"
                      :key="vm.name"
                      :label="vm.name"
                      icon="pi pi-desktop"
                      severity="secondary"
                      size="small"
                      @click="selectVMForConsole(vm)"
                    />
                  </div>
                </div>
              </div>

              <!-- Inline VNC Console -->
              <div
                v-else-if="selectedVM && inlineConsoleActive && podId"
                class="w-full h-125"
              >
                <InlineVncConsole
                  :vm="selectedVM"
                  :all-vms="pod?.vms"
                  :pod-id="podId"
                  @open-floating="handleOpenConsole(selectedVM!)"
                  @vm-selected="selectVMForConsole"
                />
              </div>
            </div>

            <!-- Quick Actions -->
            <div class="mt-4 flex flex-wrap gap-3">
              <Button
                v-if="podId"
                icon="pi pi-desktop"
                :label="t('session.workspace.quickActions.fullConsole')"
                severity="primary"
                :aria-label="t('session.workspace.quickActions.fullConsoleAria')"
                @click="openFullConsole"
              />
              <Button
                :disabled="!pod?.id || podLoading"
                icon="pi pi-refresh"
                :label="t('session.workspace.quickActions.resetInitial')"
                severity="secondary"
                @click="resetToInitial"
              />
              <Button
                v-if="selectedVM && inlineConsoleActive"
                icon="pi pi-external-link"
                :label="t('session.workspace.quickActions.openInWindow')"
                severity="secondary"
                @click="handleOpenConsole(selectedVM)"
              />
            </div>

            <!-- Hint Nudge Notification -->
            <Transition name="slide-up">
              <div
                v-if="sessionStore.activeNudge"
                class="mt-4 p-4 rounded-lg border border-yellow-300 dark:border-yellow-700 bg-yellow-50 dark:bg-yellow-900/20"
                role="alert"
              >
                <div class="flex items-start gap-3">
                  <i class="pi pi-lightbulb text-yellow-500 text-xl mt-0.5 animate-pulse" aria-hidden="true" />
                  <div class="flex-1">
                    <p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
                      {{ t('session.workspace.nudge.heading', { name: sessionStore.activeNudge.checkpointName }) }}
                    </p>
                    <p class="text-xs text-yellow-600 dark:text-yellow-400 mt-1">
                      {{ t('session.workspace.nudge.body', { minutes: sessionStore.activeNudge.minutesStuck }) }}
                    </p>
                    <div class="flex gap-2 mt-3">
                      <Button
                        :label="t('session.workspace.nudge.showHint')"
                        icon="pi pi-lightbulb"
                        size="small"
                        severity="warn"
                        @click="() => { sessionStore.showCheckpointHint(sessionStore.activeNudge!.checkpointId); sessionStore.dismissNudge() }"
                      />
                      <Button
                        :label="t('session.workspace.nudge.dismiss')"
                        size="small"
                        severity="secondary"
                        text
                        @click="sessionStore.dismissNudge()"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </Transition>

            <!-- Checkpoints List -->
            <CheckpointsList
              :checkpoints="sessionStore.checkpoints"
              :revealed-hints="sessionStore.revealedCheckpointHints"
              :has-more-hints="sessionStore.hasMoreCheckpointHints"
              :next-hint-level="sessionStore.getNextCheckpointHintLevel"
              :show-next-hint="sessionStore.showCheckpointHint"
            />
          </template>
        </Card>
      </div>

      <!-- Right Sidebar with Panel Tabs -->
      <div class="lg:w-96 space-y-4">
        <!-- Panel Tabs - matching Penpot design with icons above text -->
        <div
          class="flex bg-surface-0 dark:bg-surface-900 rounded-lg border border-surface-200 dark:border-surface-700 overflow-hidden"
          role="tablist"
          :aria-label="t('session.panels.tablistAria')"
        >
          <button
            id="tab-assessment"
            role="tab"
            :aria-selected="activePanel === 'assessment'"
            aria-controls="panel-assessment"
            :class="[
              'flex-1 py-3 px-2 flex flex-col items-center gap-1 text-xs font-medium transition-colors border-b-2',
              activePanel === 'assessment'
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 border-primary-500'
                : 'text-surface-500 dark:text-surface-400 hover:bg-surface-50 dark:hover:bg-surface-800 border-transparent'
            ]"
            @click="activePanel = 'assessment'"
          >
            <i class="pi pi-check-square text-base" aria-hidden="true" />
            <span>{{ t('session.panels.assessment') }}</span>
          </button>
          <button
            id="tab-questions"
            role="tab"
            :aria-selected="activePanel === 'questions'"
            aria-controls="panel-questions"
            :class="[
              'flex-1 py-3 px-2 flex flex-col items-center gap-1 text-xs font-medium transition-colors border-b-2',
              activePanel === 'questions'
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 border-primary-500'
                : 'text-surface-500 dark:text-surface-400 hover:bg-surface-50 dark:hover:bg-surface-800 border-transparent'
            ]"
            @click="activePanel = 'questions'"
          >
            <i class="pi pi-question-circle text-base" aria-hidden="true" />
            <span>{{ t('session.panels.questions') }}</span>
          </button>
          <button
            id="tab-completion"
            role="tab"
            :aria-selected="activePanel === 'completion'"
            aria-controls="panel-completion"
            :class="[
              'flex-1 py-3 px-2 flex flex-col items-center gap-1 text-xs font-medium transition-colors border-b-2',
              activePanel === 'completion'
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 border-primary-500'
                : 'text-surface-500 dark:text-surface-400 hover:bg-surface-50 dark:hover:bg-surface-800 border-transparent'
            ]"
            @click="activePanel = 'completion'"
          >
            <i class="pi pi-flag text-base" aria-hidden="true" />
            <span>{{ t('session.panels.completion') }}</span>
          </button>
          <button
            id="tab-pod"
            role="tab"
            :aria-selected="activePanel === 'pod'"
            aria-controls="panel-pod"
            :class="[
              'flex-1 py-3 px-2 flex flex-col items-center gap-1 text-xs font-medium transition-colors border-b-2',
              activePanel === 'pod'
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 border-primary-500'
                : 'text-surface-500 dark:text-surface-400 hover:bg-surface-50 dark:hover:bg-surface-800 border-transparent'
            ]"
            @click="activePanel = 'pod'"
          >
            <i class="pi pi-server text-base" aria-hidden="true" />
            <span>{{ t('session.panels.pod') }}</span>
          </button>
          <button
            id="tab-snapshots"
            role="tab"
            :aria-selected="activePanel === 'snapshots'"
            aria-controls="panel-snapshots"
            :class="[
              'flex-1 py-3 px-2 flex flex-col items-center gap-1 text-xs font-medium transition-colors border-b-2',
              activePanel === 'snapshots'
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 border-primary-500'
                : 'text-surface-500 dark:text-surface-400 hover:bg-surface-50 dark:hover:bg-surface-800 border-transparent'
            ]"
            @click="activePanel = 'snapshots'"
          >
            <i class="pi pi-history text-base" aria-hidden="true" />
            <span>{{ t('session.panels.snapshots') }}</span>
          </button>
        </div>

        <!-- Panel Content -->
        <div
          v-if="activePanel === 'assessment'"
          id="panel-assessment"
          role="tabpanel"
          aria-labelledby="tab-assessment"
        >
          <AssessmentPanel :session-id="sessionId" />
        </div>

        <div
          v-if="activePanel === 'questions'"
          id="panel-questions"
          role="tabpanel"
          aria-labelledby="tab-questions"
        >
          <QuestionsPanel :session-id="sessionId" />
        </div>

        <div
          v-if="activePanel === 'completion'"
          id="panel-completion"
          role="tabpanel"
          aria-labelledby="tab-completion"
        >
          <LabCompletionPanel
            :session-id="sessionId"
            @submit="handleSubmitLab"
            @view-pod="handleViewPod"
            @close="handleCloseCompletion"
          />
        </div>

        <div
          v-if="activePanel === 'pod'"
          id="panel-pod"
          role="tabpanel"
          aria-labelledby="tab-pod"
        >
          <PodControlPanel
            v-if="podId"
            :pod-id="podId"
            @open-console="handleOpenConsole"
          />
          <Card v-else>
            <template #content>
              <div class="text-center text-surface-500 py-4">
                <i class="pi pi-info-circle text-2xl mb-2" />
                <p>{{ t('session.panels.noPodAssociated') }}</p>
              </div>
            </template>
          </Card>
        </div>

        <div
          v-if="activePanel === 'snapshots'"
          id="panel-snapshots"
          role="tabpanel"
          aria-labelledby="tab-snapshots"
        >
          <SnapshotPanel
            v-if="podId"
            :pod-id="podId"
          />
          <Card v-else>
            <template #content>
              <div class="text-center text-surface-500 py-4">
                <i class="pi pi-info-circle text-2xl mb-2" />
                <p>{{ t('session.panels.noPodAssociated') }}</p>
              </div>
            </template>
          </Card>
        </div>
      </div>
    </div>

    <!-- Floating VNC Console Modal -->
    <VncConsole
      v-if="showConsoleModal && consoleTicket && consoleVM && isVNCConsole()"
      :vmid="getVMID()"
      :ticket="consoleTicket"
      @close="closeConsole"
    />

    <!-- Console Loading Toast -->
    <div v-if="showConsoleModal && consoleLoading" class="fixed bottom-4 right-4 z-50">
      <Card class="shadow-lg">
        <template #content>
          <div class="flex items-center gap-3">
            <i class="pi pi-spin pi-spinner text-primary-500" />
            <span class="text-surface-700 dark:text-surface-300">
              {{ t('session.console.connectingTo', { name: consoleVM?.name ?? '' }) }}
            </span>
          </div>
        </template>
      </Card>
    </div>

    <!-- WebSocket Error Toast -->
    <Message
      v-if="wsError"
      severity="error"
      :closable="true"
      class="fixed bottom-4 right-4 max-w-md z-40"
    >
      {{ wsError }}
    </Message>

    <!-- Achievement Unlock Modal -->
    <AchievementUnlockModal
      :visible="showAchievementModal"
      :achievements="unlockedAchievements"
      @close="closeAchievementModal"
    />

    <!-- Instructions Panel Dialog -->
    <Dialog
      v-model:visible="showInstructionsPanel"
      :header="labTemplateName || t('session.instructionsDialog.fallbackHeader')"
      :style="{ width: '80vw', maxWidth: '1000px', height: '85vh' }"
      :contentStyle="{ height: 'calc(85vh - 60px)', padding: 0 }"
      modal
      :dismissableMask="true"
    >
      <!-- Guard the mount so InstructionsPanel never sees an empty
           labTemplateId (happens briefly before the session hydrates).
           The panel is also defensive about this internally, but gating
           at the mount boundary keeps the lifecycle clean. -->
      <InstructionsPanel
        v-if="labTemplateId"
        :lab-template-id="labTemplateId"
        :lab-template-name="labTemplateName"
        @close="showInstructionsPanel = false"
      />
    </Dialog>
  </div>
</template>

<style scoped>
/* Ensure noVNC canvas fills the inline container */
:deep(.vnc-container canvas) {
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
}

/* Hint nudge slide-up transition */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.3s ease;
}
.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
</style>
