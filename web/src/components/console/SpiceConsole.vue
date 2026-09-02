<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { consoleApi, type ConsoleTicket } from '@/api'
import { loggers } from '@/utils/logger'

const { t } = useI18n()

// Import spice-html5 library (types come from src/types/spice-html5.d.ts)
import { SpiceMainConn, sendCtrlAltDel, resize_helper, handle_resize } from '@spice-project/spice-html5/src/main.js'

const props = defineProps<{
  podId: string
  vmName: string
  vmId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'fullscreen'): void
  (e: 'connected'): void
  (e: 'disconnected'): void
  (e: 'error', message: string): void
}>()

const loading = ref(true)
const connecting = ref(false)
const connected = ref(false)
const error = ref<string | null>(null)
const ticket = ref<ConsoleTicket | null>(null)
const isFullscreen = ref(false)
const consoleWrapper = ref<HTMLDivElement | null>(null)
const spiceScreen = ref<HTMLDivElement | null>(null)
const messageDiv = ref<HTMLDivElement | null>(null)

// Auto-reconnect state
const reconnectAttempts = ref(0)
const maxReconnectAttempts = 5
const reconnectDelay = ref(1000)
let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
const autoReconnect = ref(true)

// SPICE connection instance
let spiceConnection: SpiceMainConn | null = null

// Build the WebSocket proxy URL (goes through our API server)
const spiceWsUrl = computed(() => {
  if (!props.podId || !props.vmName) return ''

  // Get API base URL from the build-time constant (see vite.config.ts define).
  const apiBase = __API_BASE_URL__

  // Convert http:// to ws:// or https:// to wss://
  let wsBase = apiBase.replace(/^http/, 'ws')

  // If no base URL (development with Vite proxy), use current host
  if (!wsBase) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    wsBase = `${protocol}//${window.location.host}`
  }

  return `${wsBase}/api/v1/pods/${props.podId}/vms/${props.vmName}/spice`
})

async function loadTicket() {
  loading.value = true
  error.value = null

  try {
    // Get console ticket from API (request SPICE type)
    ticket.value = await consoleApi.getTicket(props.podId, props.vmName, 'spice')

    if (ticket.value.type !== 'spice') {
      throw new Error(t('console.spice.notSupported', { type: ticket.value.type }))
    }

    loading.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('console.spice.accessFailed')
    loading.value = false
    emit('error', error.value)
  }
}

function connectSpice() {
  if (!ticket.value || !spiceScreen.value) {
    error.value = t('console.spice.containerNotReady')
    return
  }

  if (spiceConnection) {
    disconnectSpice()
  }

  connecting.value = true
  error.value = null

  try {
    const wsUrl = spiceWsUrl.value
    const password = ticket.value.password

    loggers.console.debug('SPICE connecting to:', {}, wsUrl)

    // Create SPICE connection
    spiceConnection = new SpiceMainConn({
      uri: wsUrl,
      password: password,
      screen_id: spiceScreen.value.id,
      message_id: messageDiv.value?.id,
      onerror: handleSpiceError,
      onsuccess: handleSpiceSuccess,
      onagent: handleSpiceAgent,
    })

    loggers.console.debug('SPICE connection initiated')
  } catch (err) {
    loggers.console.error('SPICE connection error', err)
    error.value = err instanceof Error ? err.message : t('console.error.failedToConnect')
    connecting.value = false
    emit('error', error.value)
  }
}

function handleSpiceError(e: Error | undefined) {
  loggers.console.error('SPICE error', e)

  connecting.value = false
  connected.value = false

  if (e !== undefined) {
    if (e.message === 'Permission denied.') {
      // Password authentication failed - don't auto-reconnect
      error.value = t('console.spice.authFailed')
      autoReconnect.value = false
    } else {
      error.value = e.message || t('console.error.connectionLost')
    }
  } else {
    error.value = t('console.spice.connectionClosed')
  }

  emit('disconnected')
  if (error.value) {
    emit('error', error.value)
  }

  // Attempt auto-reconnect
  attemptReconnect()
}

function attemptReconnect() {
  if (!autoReconnect.value) return
  if (reconnectAttempts.value >= maxReconnectAttempts) {
    loggers.console.warn('SPICE max reconnect attempts reached')
    error.value = t('console.error.maxAttempts', { max: maxReconnectAttempts })
    return
  }

  reconnectAttempts.value++
  const delay = reconnectDelay.value * Math.pow(1.5, reconnectAttempts.value - 1)
  loggers.console.debug(`SPICE reconnecting in ${delay}ms (attempt ${reconnectAttempts.value}/${maxReconnectAttempts})`)

  reconnectTimeout = setTimeout(() => {
    if (ticket.value) {
      connectSpice()
    } else {
      loadTicket().then(() => {
        if (ticket.value) connectSpice()
      })
    }
  }, delay)
}

function cancelReconnect() {
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout)
    reconnectTimeout = null
  }
  autoReconnect.value = false
}

function handleSpiceSuccess(msg?: string) {
  loggers.console.debug('SPICE connected successfully', {}, msg)
  connecting.value = false
  connected.value = true
  // Reset reconnect state on successful connection
  reconnectAttempts.value = 0
  autoReconnect.value = true
  error.value = null
  emit('connected')
}

function handleSpiceAgent(sc: unknown) {
  loggers.console.debug('SPICE agent connected')

  // Store reference for resize. The spice-html5 resize helpers read
  // `window.spice_connection` directly (see the vendor's handle_resize).
  // The type is declared globally in env.d.ts.
  window.spice_connection = sc

  // Set up resize handling
  window.addEventListener('resize', handle_resize)
  resize_helper(sc as SpiceMainConn)
}

function disconnectSpice() {
  if (spiceConnection) {
    loggers.console.debug('SPICE disconnecting')
    try {
      spiceConnection.stop()
    } catch (e) {
      loggers.console.warn('SPICE error during disconnect', {}, e)
    }
    spiceConnection = null
  }

  // Clean up resize handler
  window.removeEventListener('resize', handle_resize)
  window.spice_connection = undefined

  connected.value = false
  connecting.value = false
  emit('disconnected')
}

function handleSendCtrlAltDel() {
  if (spiceConnection && connected.value) {
    loggers.console.debug('SPICE sending Ctrl+Alt+Del')
    sendCtrlAltDel(spiceConnection)
  }
}

function toggleFullscreen() {
  if (!consoleWrapper.value) return

  if (!document.fullscreenElement) {
    consoleWrapper.value.requestFullscreen()
    isFullscreen.value = true
  } else {
    document.exitFullscreen()
    isFullscreen.value = false
  }
  emit('fullscreen')
}

function handleClose() {
  disconnectSpice()
  emit('close')
}

function handleRetry() {
  disconnectSpice()
  loadTicket().then(() => {
    if (ticket.value) {
      connectSpice()
    }
  })
}

// Handle fullscreen change events
function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement

  // Trigger resize when exiting fullscreen
  if (spiceConnection && connected.value) {
    setTimeout(() => {
      handle_resize()
    }, 100)
  }
}

// Download SPICE config file for native SPICE client (fallback option)
function downloadSpiceConfig() {
  if (!ticket.value) return

  const host = ticket.value.host.split(':')[0]
  const spiceConfig = `[virt-viewer]
type=spice
host=${host}
port=${ticket.value.port || ''}
tls-port=${ticket.value.tlsPort || ''}
password=${ticket.value.password || ''}
delete-this-file=1
fullscreen=0
title=${props.vmName}
toggle-fullscreen=shift+f11
release-cursor=shift+f12
secure-attention=ctrl+alt+end
`

  const blob = new Blob([spiceConfig], { type: 'application/x-virt-viewer' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.vmName}.vv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)

  // Load ticket and auto-connect
  loadTicket().then(() => {
    if (ticket.value && ticket.value.type === 'spice') {
      // Small delay to ensure DOM is ready
      setTimeout(connectSpice, 100)
    }
  })
})

onUnmounted(() => {
  cancelReconnect()
  disconnectSpice()
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <div ref="consoleWrapper" class="spice-console flex flex-col h-full bg-gray-900 rounded-lg overflow-hidden">
    <!-- Header -->
    <div class="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
      <div class="flex items-center space-x-3">
        <div
          class="w-3 h-3 rounded-full"
          :class="{
            'bg-green-500': connected,
            'bg-yellow-500 animate-pulse': connecting,
            'bg-red-500': error && !connecting,
            'bg-gray-500': !connected && !connecting && !error
          }"
        ></div>
        <span class="text-white font-medium">{{ vmName }}</span>
        <span class="text-gray-400 text-sm">
          {{ connected ? t('console.status.connected') : connecting ? t('console.status.connecting') : reconnectAttempts > 0 ? t('console.status.reconnecting', { current: reconnectAttempts, max: maxReconnectAttempts }) : t('console.spice.title') }}
        </span>
      </div>
      <div class="flex items-center space-x-2">
        <!-- Send Ctrl+Alt+Del button -->
        <button
          v-if="connected"
          @click="handleSendCtrlAltDel"
          class="px-3 py-1 text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
          :title="t('console.actions.sendCtrlAltDel')"
        >
          {{ t('console.actions.ctrlAltDel') }}
        </button>
        <!-- Download SPICE file button -->
        <button
          v-if="ticket"
          @click="downloadSpiceConfig"
          class="px-3 py-1 text-xs bg-gray-600 text-white rounded hover:bg-gray-500 transition-colors"
          :title="t('console.spice.downloadNativeTitle')"
        >
          {{ t('console.spice.downloadVv') }}
        </button>
        <!-- Reconnect button -->
        <button
          v-if="!connected && !connecting && ticket"
          @click="connectSpice"
          class="px-3 py-1 text-xs bg-indigo-600 text-white rounded hover:bg-indigo-700 transition-colors"
          :title="t('console.spice.reconnectTitle')"
        >
          {{ t('console.actions.connect') }}
        </button>
        <!-- Fullscreen button -->
        <button
          @click="toggleFullscreen"
          class="p-2 text-gray-400 hover:text-white transition-colors"
          :title="t('console.actions.toggleFullscreen')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path v-if="!isFullscreen" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <!-- Close button -->
        <button
          @click="handleClose"
          class="p-2 text-gray-400 hover:text-white transition-colors"
          :title="t('console.actions.closeConsole')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Console Content -->
    <div class="flex-1 relative min-h-[400px]">
      <!-- Loading State -->
      <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-gray-900 z-10">
        <div class="text-center">
          <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-500 mx-auto mb-4"></div>
          <p class="text-gray-400">{{ t('console.spice.loading') }}</p>
        </div>
      </div>

      <!-- Error State (overlay) -->
      <div v-if="error && !connecting" class="absolute inset-0 flex items-center justify-center bg-gray-900/90 z-10">
        <div class="text-center max-w-md px-4">
          <div class="text-red-500 mb-4">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <p class="text-white font-medium mb-2">{{ t('console.error.connectionFailed') }}</p>
          <p class="text-gray-400 text-sm mb-4">{{ error }}</p>
          <div class="space-y-2">
            <button
              @click="handleRetry"
              class="w-full px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
            >
              {{ t('console.actions.retryConnection') }}
            </button>
            <button
              v-if="ticket"
              @click="downloadSpiceConfig"
              class="w-full px-4 py-2 border border-gray-600 text-gray-300 rounded-lg hover:bg-gray-800 transition-colors"
            >
              {{ t('console.spice.downloadNative') }}
            </button>
          </div>
        </div>
      </div>

      <!-- SPICE Screen Container -->
      <div
        id="spice-screen"
        ref="spiceScreen"
        class="spice-screen absolute inset-0 bg-black"
        :class="{ 'opacity-50': connecting }"
      ></div>

      <!-- Message div for spice-html5 status messages -->
      <div
        id="spice-message"
        ref="messageDiv"
        class="spice-message absolute bottom-4 left-4 right-4 text-center text-gray-400 text-sm"
      ></div>

      <!-- Connecting overlay -->
      <div v-if="connecting" class="absolute inset-0 flex items-center justify-center bg-gray-900/50 z-5">
        <div class="text-center">
          <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-indigo-500 mx-auto mb-2"></div>
          <p class="text-gray-300 text-sm">{{ t('console.spice.establishing') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.spice-console {
  min-height: 500px;
}

.spice-screen {
  /* The spice-html5 library will create canvas elements inside this div */
}

/* Style for spice-html5 canvas elements */
:deep(.spice-screen canvas) {
  display: block;
  margin: 0 auto;
}

/* Override spice-html5 default message styles */
:deep(.spice-message) {
  padding: 8px;
  border-radius: 4px;
}

:deep(.spice-messages-info) {
  color: #60a5fa;
}

:deep(.spice-messages-warning) {
  color: #fbbf24;
}

:deep(.spice-messages-error) {
  color: #f87171;
}
</style>
