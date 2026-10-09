<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PodVM, ConsoleTicket } from '@/api'
import api from '@/api/config'
import { loggers } from '@/utils/logger'
import type RFB from '@novnc/novnc/lib/rfb.js'
import {
  useVncConnection,
  buildWsProxyUrl,
  type UseVncConnectionReturn,
} from '@/composables/useVncConnection'
import { useVncKeyboard } from '@/composables/useVncKeyboard'

const { t } = useI18n()

const props = defineProps<{
  vm: PodVM
  allVms?: PodVM[] // All VMs in pod for tab display
  podId?: string // Pod ID for VM actions
}>()

const emit = defineEmits<{
  (e: 'open-floating'): void
  (e: 'vm-selected', vm: PodVM): void
}>()

// Console container refs
const consoleWrapperRef = ref<HTMLDivElement | null>(null)
const containerRefs = ref<Map<string, HTMLDivElement>>(new Map())

// Connection cache — one useVncConnection instance per VM, keyed by platformId.
// Keeps connections alive across tab switches.
// Use a plain Map (not reactive) — reactivity comes from each conn's internal refs.
// The selectedVmId ref triggers recomputation of currentConnection.
interface CachedConnection {
  conn: UseVncConnectionReturn
  ticket: ConsoleTicket | null
  attempts: number
}
const connectionCache = new Map<string, CachedConnection>()
// Trigger reactivity on VM switch — increment to force recomputation
const cacheVersion = ref(0)

// Current VM's connection state (computed from cache)
const currentConnection = computed(() => {
  cacheVersion.value // dependency for reactivity on cache changes
  return connectionCache.get(props.vm.platformId)
})
const connectionStatus = computed(
  () => currentConnection.value?.conn.connectionStatus.value || 'disconnected',
)
const errorMessage = computed(() => currentConnection.value?.conn.errorMessage.value || '')

const isSecureContext = ref(window.isSecureContext)
const isDemoMode = ref(false)
const maxConnectionAttempts = 3

// --- Keyboard via composable (operates on the current VM's RFB) ---
function getCurrentRfb(): RFB | null | undefined {
  return currentConnection.value?.conn.getRfb()
}
const keyboard = useVncKeyboard(getCurrentRfb)
const { ctrlActive: ctrlPressed, altActive: altPressed, toggleCtrl, toggleAlt } = keyboard

// Wrap keyboard methods to add notifications
function sendCtrlAltDel() {
  if (connectionStatus.value === 'connected') {
    keyboard.sendCtrlAltDel()
    showNotification(t('console.inline.toast.ctrlAltDelSent'), 'info')
  }
}

function sendTab() {
  if (connectionStatus.value === 'connected') {
    keyboard.sendTab()
  }
}

function sendEscape() {
  if (connectionStatus.value === 'connected') {
    keyboard.sendEscape()
  }
}

// Fullscreen state
const isFullscreen = ref(false)

// VM actions menu state
const activeMenuIndex = ref<number | null>(null)
const actionLoading = ref<string | null>(null)

// Notification state
const notification = ref<{ message: string; type: 'info' | 'success' | 'error' } | null>(null)

// Set container ref for a VM
function setContainerRef(platformId: string, el: HTMLDivElement | null) {
  if (el) {
    containerRefs.value.set(platformId, el)
  } else {
    containerRefs.value.delete(platformId)
  }
}

// Check if we have multiple VMs to show tabs
const showTabs = computed(() => props.allVms && props.allVms.length > 1)

// Get status class for VM dot indicator
function getStatusDotClass(vm: PodVM): string {
  if (vm.status === 'running') return 'bg-green-500 ring-2 ring-green-500/30'
  if (vm.status === 'stopped') return 'bg-red-500'
  if (vm.status === 'starting' || vm.status === 'stopping') return 'bg-yellow-500 animate-pulse'
  return 'bg-gray-500'
}

// Check if VM is currently selected
function isVMSelected(vm: PodVM): boolean {
  return vm.platformId === props.vm.platformId
}

async function fetchTicketForVM(platformId: string): Promise<ConsoleTicket | null> {
  try {
    const vmid = parseInt(platformId, 10)
    const response = await api.get<ConsoleTicket>(`/proxmox/vms/${vmid}/console`, {
      params: { type: 'vnc' },
    })
    const ticketData = response.data

    if (ticketData?.host === 'demo' || ticketData?.ticket === 'demo') {
      isDemoMode.value = true
      return null
    }
    return ticketData
  } catch (err) {
    const status =
      err && typeof err === 'object' && 'response' in err
        ? (err as { response?: { status?: number } }).response?.status
        : undefined
    if (status === 404 || status === 503) {
      isDemoMode.value = true
      return null
    }
    loggers.console.error('Failed to fetch ticket', err)
    return null
  }
}

async function connectVM(platformId: string) {
  // Check if already connected or connecting
  const existing = connectionCache.get(platformId)
  if (existing) {
    const status = existing.conn.connectionStatus.value
    if (status === 'connected' || status === 'connecting') {
      if (status === 'connected') existing.conn.focus()
      return
    }
  }

  const container = containerRefs.value.get(platformId)
  if (!container) {
    loggers.console.warn('Container not found for VM', { platformId })
    return
  }

  // Get or create cache entry
  let cached = connectionCache.get(platformId)
  if (!cached) {
    const ticketData = await fetchTicketForVM(platformId)
    if (!ticketData) {
      const conn = useVncConnection()
      conn.connectionStatus.value = 'error'
      conn.errorMessage.value = t('console.inline.demoNotAvailable')
      connectionCache.set(platformId, { conn, ticket: null, attempts: 0 })
      cacheVersion.value++
      return
    }
    const conn = useVncConnection({
      onStatusChange: status => {
        const entry = connectionCache.get(platformId)
        if (!entry) return
        if (status === 'disconnected') {
          entry.attempts++
          if (entry.attempts >= maxConnectionAttempts) {
            conn.connectionStatus.value = 'error'
            conn.errorMessage.value = t('console.inline.notAvailable')
          }
        } else if (status === 'connected') {
          entry.attempts = 0
        }
        cacheVersion.value++
      },
    })
    cached = { conn, ticket: ticketData, attempts: 0 }
    connectionCache.set(platformId, cached)
    cacheVersion.value++
  }

  if (!cached.ticket) return

  const wsUrl = buildWsProxyUrl(platformId, cached.ticket.node)
  const vncPassword = cached.ticket.ticket || cached.ticket.password || ''
  await cached.conn.connect(container, wsUrl, vncPassword)
}

function disconnectAll() {
  for (const [, cached] of connectionCache) {
    cached.conn.disconnect()
  }
  connectionCache.clear()
}

function focusCurrentVM() {
  const cached = connectionCache.get(props.vm.platformId)
  if (cached?.conn.isConnected.value) {
    cached.conn.focus()
  }
}

// Fullscreen methods
function toggleFullscreen() {
  if (!consoleWrapperRef.value) return

  if (!document.fullscreenElement) {
    consoleWrapperRef.value
      .requestFullscreen()
      .then(() => {
        isFullscreen.value = true
      })
      .catch(err => {
        loggers.console.error('Failed to enter fullscreen', err)
      })
  } else {
    document.exitFullscreen().then(() => {
      isFullscreen.value = false
    })
  }
}

function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
}

// VM tab selection - no disconnect needed, just switch visibility
function selectVM(vm: PodVM) {
  if (vm.platformId !== props.vm.platformId) {
    activeMenuIndex.value = null
    emit('vm-selected', vm)
    // Focus will happen after the prop change via watch
  }
}

// VM actions menu
function toggleMenu(event: Event, index: number) {
  event.stopPropagation()
  activeMenuIndex.value = activeMenuIndex.value === index ? null : index
}

function closeMenus() {
  activeMenuIndex.value = null
}

// VM power actions
async function vmAction(vm: PodVM, action: 'start' | 'stop' | 'reboot' | 'shutdown') {
  activeMenuIndex.value = null
  const vmid = vm.platformId
  actionLoading.value = `${vmid}-${action}`

  showNotification(t('console.inline.toast.actionInProgress', { action, name: vm.name }), 'info')

  try {
    await api.post(`/proxmox/vms/${vmid}/${action}`)
    showNotification(
      t('console.inline.toast.actionInitiated', { name: vm.name, action }),
      'success',
    )
  } catch (err) {
    // Prefer the backend's `.error` reason over Axios's generic message,
    // falling back to a plain Error message for non-axios failures.
    let reason = 'Unknown error'
    if (err && typeof err === 'object') {
      const body = (err as { response?: { data?: { error?: string } } }).response?.data
      if (body?.error) reason = body.error
      else if (err instanceof Error) reason = err.message
    }
    showNotification(
      t('console.inline.toast.actionFailed', { action, name: vm.name, reason }),
      'error',
    )
  } finally {
    actionLoading.value = null
  }
}

// Notification helper
function showNotification(message: string, type: 'info' | 'success' | 'error') {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

onMounted(async () => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  document.addEventListener('click', closeMenus)

  // Wait for DOM to be ready with containers
  await nextTick()

  // Connect to current VM
  await connectVM(props.vm.platformId)
})

onUnmounted(() => {
  // Disconnect all cached connections
  disconnectAll()
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('click', closeMenus)
})

// When VM selection changes, just focus the new VM (connection is cached)
watch(
  () => props.vm.platformId,
  async newPlatformId => {
    await nextTick()

    // Connect if not already connected
    await connectVM(newPlatformId)

    // Focus the RFB instance
    focusCurrentVM()
  },
)
</script>

<template>
  <div
    ref="consoleWrapperRef"
    class="enhanced-console flex flex-col h-full bg-gray-900 rounded-lg overflow-hidden"
  >
    <!-- VM Tabs (only if multiple VMs) -->
    <div
      v-if="showTabs"
      class="vm-tabs flex gap-1 px-2 pt-2 pb-0 bg-gray-800 border-b border-gray-700 overflow-x-auto"
    >
      <button
        v-for="(vmItem, idx) in allVms"
        :key="vmItem.platformId"
        @click="vmItem.status === 'running' ? selectVM(vmItem) : null"
        :disabled="vmItem.status !== 'running'"
        :class="[
          'vm-tab flex items-center gap-2 px-3 py-2 rounded-t-lg border border-b-0 text-sm font-medium transition-all min-w-[100px] relative',
          isVMSelected(vmItem)
            ? 'bg-gray-900 border-primary-500 text-white'
            : vmItem.status === 'running'
              ? 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600 cursor-pointer'
              : 'bg-gray-800 border-gray-700 text-gray-500 cursor-not-allowed opacity-60',
        ]"
      >
        <span :class="['w-2 h-2 rounded-full flex-shrink-0', getStatusDotClass(vmItem)]" />
        <span class="truncate">{{ vmItem.name }}</span>

        <!-- VM Actions Menu Button -->
        <button
          v-if="vmItem.status === 'running' || vmItem.status === 'stopped'"
          @click.stop="toggleMenu($event, idx)"
          class="ml-auto p-1 rounded hover:bg-gray-600 text-gray-400 hover:text-white"
          :title="t('console.inline.vmActions')"
        >
          <svg class="w-4 h-4" viewBox="0 0 16 16" fill="currentColor">
            <circle cx="8" cy="2.5" r="1.5" />
            <circle cx="8" cy="8" r="1.5" />
            <circle cx="8" cy="13.5" r="1.5" />
          </svg>
        </button>

        <!-- Actions Dropdown Menu -->
        <div
          v-if="activeMenuIndex === idx"
          class="absolute top-full right-0 mt-1 bg-gray-700 border border-gray-600 rounded-lg shadow-xl z-50 min-w-[140px] overflow-hidden"
          @click.stop
        >
          <button
            v-if="vmItem.status === 'stopped'"
            @click="vmAction(vmItem, 'start')"
            class="flex items-center gap-2 w-full px-3 py-2 text-left text-sm text-green-400 hover:bg-gray-600"
          >
            <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M8 5v14l11-7z" />
            </svg>
            {{ t('console.inline.start') }}
          </button>
          <button
            v-if="vmItem.status === 'running'"
            @click="vmAction(vmItem, 'reboot')"
            class="flex items-center gap-2 w-full px-3 py-2 text-left text-sm text-gray-300 hover:bg-gray-600"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M1 4v6h6M23 20v-6h-6" />
              <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15" />
            </svg>
            {{ t('console.inline.reboot') }}
          </button>
          <div v-if="vmItem.status === 'running'" class="border-t border-gray-600 my-1"></div>
          <button
            v-if="vmItem.status === 'running'"
            @click="vmAction(vmItem, 'shutdown')"
            class="flex items-center gap-2 w-full px-3 py-2 text-left text-sm text-orange-400 hover:bg-gray-600"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M18.36 6.64a9 9 0 1 1-12.73 0M12 2v10" />
            </svg>
            {{ t('console.inline.shutdown') }}
          </button>
          <button
            v-if="vmItem.status === 'running'"
            @click="vmAction(vmItem, 'stop')"
            class="flex items-center gap-2 w-full px-3 py-2 text-left text-sm text-red-400 hover:bg-gray-600"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <rect x="3" y="3" width="18" height="18" rx="2" />
            </svg>
            {{ t('console.inline.forceStop') }}
          </button>
        </div>
      </button>
    </div>

    <!-- Keyboard Toolbar -->
    <div class="toolbar flex items-center gap-1 px-2 py-1.5 bg-gray-800 border-b border-gray-700">
      <!-- Modifier Keys -->
      <button
        @click="toggleCtrl"
        :class="[
          'px-2 py-1 text-xs font-medium rounded transition-colors',
          ctrlPressed ? 'bg-primary-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600',
        ]"
        :title="t('console.inline.ctrlSticky')"
      >
        Ctrl
      </button>
      <button
        @click="toggleAlt"
        :class="[
          'px-2 py-1 text-xs font-medium rounded transition-colors',
          altPressed ? 'bg-primary-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600',
        ]"
        :title="t('console.inline.altSticky')"
      >
        Alt
      </button>

      <div class="w-px h-5 bg-gray-600 mx-1"></div>

      <!-- Direct Keys -->
      <button
        @click="sendTab"
        :disabled="connectionStatus !== 'connected'"
        class="px-2 py-1 text-xs font-medium rounded bg-gray-700 text-gray-300 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
        :title="t('console.inline.sendTab')"
      >
        Tab
      </button>
      <button
        @click="sendEscape"
        :disabled="connectionStatus !== 'connected'"
        class="px-2 py-1 text-xs font-medium rounded bg-gray-700 text-gray-300 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed"
        :title="t('console.inline.sendEscape')"
      >
        Esc
      </button>
      <button
        @click="sendCtrlAltDel"
        :disabled="connectionStatus !== 'connected'"
        class="px-2 py-1 text-xs font-medium rounded bg-red-700 text-white hover:bg-red-600 disabled:opacity-50 disabled:cursor-not-allowed"
        :title="t('console.inline.sendCtrlAltDel')"
      >
        Ctrl+Alt+Del
      </button>

      <div class="flex-1"></div>

      <!-- Fullscreen -->
      <button
        @click="toggleFullscreen"
        class="p-1.5 rounded bg-gray-700 text-gray-300 hover:bg-gray-600"
        :title="
          isFullscreen ? t('console.inline.exitFullscreen') : t('console.inline.enterFullscreen')
        "
      >
        <svg
          v-if="!isFullscreen"
          class="w-4 h-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
          />
        </svg>
        <svg
          v-else
          class="w-4 h-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            d="M8 4v4m0 0H4m4 0L3 3m13 5V4m0 4h4m-4 0l5-5M8 20v-4m0 0H4m4 0l-5 5m13-5v4m0-4h4m-4 0l5 5"
          />
        </svg>
      </button>

      <!-- Float button -->
      <button
        @click="emit('open-floating')"
        class="p-1.5 rounded bg-gray-700 text-gray-300 hover:bg-gray-600"
        :title="t('console.inline.openFloating')"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
        </svg>
      </button>

      <!-- Connection Status -->
      <div class="flex items-center gap-2 ml-2 pl-2 border-l border-gray-600">
        <span
          :class="[
            'w-2 h-2 rounded-full',
            connectionStatus === 'connected'
              ? 'bg-green-500'
              : connectionStatus === 'connecting'
                ? 'bg-yellow-500 animate-pulse'
                : 'bg-red-500',
          ]"
        />
        <span class="text-xs text-gray-400">
          {{
            connectionStatus === 'connected'
              ? t('console.status.connected')
              : connectionStatus === 'connecting'
                ? t('console.status.connecting')
                : t('console.status.disconnected')
          }}
        </span>
        <button
          v-if="connectionStatus !== 'connected' && connectionStatus !== 'connecting'"
          @click="connectVM(vm.platformId)"
          class="px-2 py-0.5 text-xs bg-primary-600 hover:bg-primary-700 text-white rounded"
        >
          {{ t('console.actions.reconnect') }}
        </button>
      </div>
    </div>

    <!-- VNC Canvas Containers - one per VM, shown/hidden based on selection -->
    <div class="relative flex-1 min-h-0">
      <!-- Container for each VM that has been visited -->
      <template v-for="vmItem in allVms || [vm]" :key="vmItem.platformId">
        <div
          :ref="el => setContainerRef(vmItem.platformId, el as HTMLDivElement)"
          :class="[
            'vnc-container absolute inset-0',
            vmItem.platformId === vm.platformId ? 'z-10' : 'z-0 invisible',
          ]"
          style="background: #000"
        />
      </template>

      <!-- Overlays for current VM only -->
      <!-- Connecting overlay -->
      <div
        v-if="connectionStatus === 'connecting'"
        class="absolute inset-0 flex flex-col items-center justify-center bg-black/75 z-20"
      >
        <div
          class="animate-spin w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full mb-3"
        ></div>
        <span class="text-gray-400">{{ t('console.inline.connectingTo', { name: vm.name }) }}</span>
      </div>

      <!-- Demo mode overlay -->
      <div
        v-if="isDemoMode && connectionStatus === 'error'"
        class="absolute inset-0 flex flex-col items-center justify-center bg-gray-900 z-20"
      >
        <div class="text-center max-w-md px-6">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-16 w-16 text-primary-500 mx-auto mb-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
            />
          </svg>
          <h3 class="text-xl font-semibold text-white mb-2">{{ t('console.inline.demoTitle') }}</h3>
          <p class="text-gray-400 mb-4">
            {{ t('console.inline.demoBody') }}
          </p>
          <div class="text-left bg-gray-800 rounded-lg p-4 text-sm text-gray-300">
            <p class="font-medium text-white mb-2">{{ t('console.inline.vmInfo') }}</p>
            <p>
              <span class="text-gray-500">{{ t('console.labels.name') }}</span> {{ vm.name }}
            </p>
            <p>
              <span class="text-gray-500">{{ t('console.labels.platformId') }}</span>
              {{ vm.platformId }}
            </p>
            <p>
              <span class="text-gray-500">{{ t('console.labels.status') }}</span> {{ vm.status }}
            </p>
            <p v-if="vm.ipAddress">
              <span class="text-gray-500">{{ t('console.labels.ip') }}</span> {{ vm.ipAddress }}
            </p>
          </div>
        </div>
      </div>

      <!-- Secure context warning -->
      <div
        v-else-if="
          !isSecureContext && connectionStatus !== 'connected' && connectionStatus !== 'connecting'
        "
        class="absolute inset-0 flex flex-col items-center justify-center bg-gray-900 z-20"
      >
        <div class="text-center max-w-md px-6">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-16 w-16 text-yellow-500 mx-auto mb-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <h3 class="text-xl font-semibold text-white mb-2">
            {{ t('console.inline.httpsTitle') }}
          </h3>
          <p class="text-gray-400 mb-4">
            {{ t('console.inline.httpsBody') }}
          </p>
          <button
            @click="connectVM(vm.platformId)"
            class="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded"
          >
            {{ t('console.inline.httpsTryAnyway') }}
          </button>
        </div>
      </div>

      <!-- Error/Disconnected overlay -->
      <div
        v-else-if="connectionStatus === 'error' || connectionStatus === 'disconnected'"
        class="absolute inset-0 flex flex-col items-center justify-center bg-black/90 z-20"
      >
        <div v-if="errorMessage" class="text-red-400 mb-4 text-center">{{ errorMessage }}</div>
        <p class="text-gray-400 mb-4">{{ t('console.disconnected.title') }}</p>
        <button
          @click="connectVM(vm.platformId)"
          class="px-4 py-2 bg-primary-600 hover:bg-primary-700 text-white rounded"
        >
          {{ t('console.actions.reconnect') }}
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <Transition name="slide">
      <div
        v-if="notification"
        :class="[
          'fixed top-4 right-4 px-4 py-3 rounded-lg shadow-lg z-50 text-sm font-medium',
          notification.type === 'success'
            ? 'bg-green-600 text-white'
            : notification.type === 'error'
              ? 'bg-red-600 text-white'
              : 'bg-primary-600 text-white',
        ]"
      >
        {{ notification.message }}
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* Ensure noVNC canvas fills the container */
.vnc-container :deep(canvas) {
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
}

/* VM tab styling */
.vm-tab {
  transition: all 0.2s;
}

/* Notification slide animation */
.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease;
}

.slide-enter-from {
  transform: translateX(100px);
  opacity: 0;
}

.slide-leave-to {
  transform: translateX(100px);
  opacity: 0;
}
</style>
