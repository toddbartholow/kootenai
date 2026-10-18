<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { type ConsoleTicket } from '@/api'
import { loggers } from '@/utils/logger'

const { t } = useI18n()

// Import spice-html5 library
import { SpiceMainConn, sendCtrlAltDel, resize_helper, handle_resize } from '@spice-project/spice-html5/src/main.js'

const props = defineProps<{
  vmid: number
  ticket: ConsoleTicket
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

// Connection states
const connectionStatus = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('disconnected')
const errorMessage = ref('')
const windowRef = ref<HTMLDivElement | null>(null)
const spiceScreen = ref<HTMLDivElement | null>(null)
const messageDiv = ref<HTMLDivElement | null>(null)

// Window position, size, dragging, and resizing
const windowPosition = ref({ x: 50, y: 50 })
const windowSize = ref({ width: 900, height: 650 })
const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })
const isResizing = ref(false)
const resizeDirection = ref('')
const resizeStart = ref({ x: 0, y: 0, width: 0, height: 0, left: 0, top: 0 })

// Toolbar state
const toolbarCollapsed = ref(false)
const showInfoPanel = ref(false)

// SPICE connection instance
let spiceConnection: SpiceMainConn | null = null

// Build WebSocket URL for our SPICE proxy
const wsProxyUrl = computed(() => {
  if (!props.ticket || props.ticket.type !== 'spice') return ''

  // Use the SPICE proxy endpoint through our backend
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  const vmid = props.vmid
  const node = props.ticket.node

  // Direct Proxmox VM access through our WebSocket proxy
  return `${protocol}//${host}/api/v1/proxmox/vms/${vmid}/spice?node=${node}`
})

// Dragging functions
function startDrag(e: MouseEvent) {
  if (!windowRef.value) return
  isDragging.value = true
  const rect = windowRef.value.getBoundingClientRect()
  dragOffset.value = {
    x: e.clientX - rect.left,
    y: e.clientY - rect.top
  }
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
}

function onDrag(e: MouseEvent) {
  if (!isDragging.value) return
  windowPosition.value = {
    x: Math.max(0, Math.min(window.innerWidth - 200, e.clientX - dragOffset.value.x)),
    y: Math.max(0, Math.min(window.innerHeight - 100, e.clientY - dragOffset.value.y))
  }
}

function stopDrag() {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}

// Resize functions
function startResize(e: MouseEvent, direction: string) {
  e.preventDefault()
  e.stopPropagation()
  isResizing.value = true
  resizeDirection.value = direction
  resizeStart.value = {
    x: e.clientX,
    y: e.clientY,
    width: windowSize.value.width,
    height: windowSize.value.height,
    left: windowPosition.value.x,
    top: windowPosition.value.y
  }
  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
}

function onResize(e: MouseEvent) {
  if (!isResizing.value) return

  const dx = e.clientX - resizeStart.value.x
  const dy = e.clientY - resizeStart.value.y
  const minWidth = 400
  const minHeight = 300
  const dir = resizeDirection.value

  let newWidth = resizeStart.value.width
  let newHeight = resizeStart.value.height
  let newLeft = resizeStart.value.left
  let newTop = resizeStart.value.top

  // Handle horizontal resize
  if (dir.includes('e')) {
    newWidth = Math.max(minWidth, resizeStart.value.width + dx)
  }
  if (dir.includes('w')) {
    const proposedWidth = resizeStart.value.width - dx
    if (proposedWidth >= minWidth) {
      newWidth = proposedWidth
      newLeft = resizeStart.value.left + dx
    }
  }

  // Handle vertical resize
  if (dir.includes('s')) {
    newHeight = Math.max(minHeight, resizeStart.value.height + dy)
  }
  if (dir.includes('n')) {
    const proposedHeight = resizeStart.value.height - dy
    if (proposedHeight >= minHeight) {
      newHeight = proposedHeight
      newTop = resizeStart.value.top + dy
    }
  }

  windowSize.value = { width: newWidth, height: newHeight }
  windowPosition.value = { x: Math.max(0, newLeft), y: Math.max(0, newTop) }
}

function stopResize() {
  isResizing.value = false
  resizeDirection.value = ''
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
}

function connect() {
  if (!wsProxyUrl.value) {
    errorMessage.value = t('console.spice.noProxy')
    connectionStatus.value = 'error'
    return
  }

  if (!spiceScreen.value) {
    errorMessage.value = t('console.spice.containerNotReady')
    connectionStatus.value = 'error'
    return
  }

  if (spiceConnection) {
    disconnect()
  }

  connectionStatus.value = 'connecting'
  errorMessage.value = ''

  try {
    const password = props.ticket.password || ''

    loggers.console.debug('SPICE connecting to:', {}, wsProxyUrl.value)

    // Create SPICE connection
    spiceConnection = new SpiceMainConn({
      uri: wsProxyUrl.value,
      password: password,
      screen_id: spiceScreen.value.id,
      message_id: messageDiv.value?.id,
      onerror: handleSpiceError,
      onsuccess: handleSpiceSuccess,
      onagent: handleSpiceAgent,
    })

    loggers.console.debug('SPICE connection initiated')
  } catch (err: unknown) {
    loggers.console.error('SPICE connection error', err)
    errorMessage.value = err instanceof Error ? err.message : t('console.error.failedToConnect')
    connectionStatus.value = 'error'
  }
}

function handleSpiceError(e: Error | undefined) {
  loggers.console.error('SPICE error', e)

  connectionStatus.value = 'error'

  if (e !== undefined) {
    if (e.message === 'Permission denied.') {
      errorMessage.value = t('console.spice.authFailed')
    } else {
      errorMessage.value = e.message || t('console.error.failedToConnect')
    }
  } else {
    errorMessage.value = t('console.spice.connectionClosed')
  }
}

function handleSpiceSuccess(msg?: string) {
  loggers.console.debug('SPICE connected successfully', {}, msg)
  connectionStatus.value = 'connected'
}

function handleSpiceAgent(sc: unknown) {
  loggers.console.debug('SPICE agent connected')

  // Store reference for resize (see env.d.ts for the global declaration).
  window.spice_connection = sc

  // Set up resize handling
  window.addEventListener('resize', handle_resize)
  resize_helper(sc as SpiceMainConn)
}

function disconnect() {
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

  connectionStatus.value = 'disconnected'
}

function handleSendCtrlAltDel() {
  if (spiceConnection && connectionStatus.value === 'connected') {
    loggers.console.debug('SPICE sending Ctrl+Alt+Del')
    sendCtrlAltDel(spiceConnection)
  }
}

function toggleFullscreen() {
  if (windowRef.value) {
    if (document.fullscreenElement) {
      document.exitFullscreen()
    } else {
      windowRef.value.requestFullscreen()
    }
  }
}

// Handle fullscreen change events
function onFullscreenChange() {
  // Trigger resize when exiting fullscreen
  if (spiceConnection && connectionStatus.value === 'connected') {
    setTimeout(() => {
      handle_resize()
    }, 100)
  }
}

// Download SPICE config file for native SPICE client (fallback option)
function downloadSpiceConfig() {
  if (!props.ticket) return

  const host = props.ticket.host.split(':')[0]
  const spiceConfig = `[virt-viewer]
type=spice
host=${host}
port=${props.ticket.port || ''}
tls-port=${props.ticket.tlsPort || ''}
password=${props.ticket.password || ''}
delete-this-file=1
fullscreen=0
title=VM ${props.vmid}
toggle-fullscreen=shift+f11
release-cursor=shift+f12
secure-attention=ctrl+alt+end
`

  const blob = new Blob([spiceConfig], { type: 'application/x-virt-viewer' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `vm-${props.vmid}.vv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// Build the Proxmox SPICE console URL as fallback
const proxmoxConsoleUrl = computed(() => {
  if (!props.ticket || props.ticket.type !== 'spice') return ''
  const host = props.ticket.host
  const vmid = props.ticket.vmid || props.vmid
  const node = props.ticket.node
  return `https://${host}/?console=spice&vmid=${vmid}&node=${node}`
})

function openExternalConsole() {
  const width = 1024
  const height = 768
  const left = (window.screen.width - width) / 2
  const top = (window.screen.height - height) / 2
  window.open(
    proxmoxConsoleUrl.value,
    `spice_${props.vmid}`,
    `width=${width},height=${height},left=${left},top=${top},menubar=no,toolbar=no,location=no,status=no`
  )
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)

  // Center window on mount using windowSize
  windowPosition.value = {
    x: Math.max(50, (window.innerWidth - windowSize.value.width) / 2),
    y: Math.max(50, (window.innerHeight - windowSize.value.height) / 2)
  }

  // Auto-connect when component mounts (with small delay for DOM)
  setTimeout(connect, 100)
})

onUnmounted(() => {
  disconnect()
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
})

// Reconnect if ticket changes
watch(() => props.ticket, () => {
  disconnect()
  setTimeout(connect, 100)
})
</script>

<template>
  <!-- Draggable and resizable floating window -->
  <div
    ref="windowRef"
    class="spice-window fixed bg-gray-900 rounded-lg shadow-2xl border border-gray-700 overflow-hidden z-50 flex flex-col"
    :style="{
      left: windowPosition.x + 'px',
      top: windowPosition.y + 'px',
      width: windowSize.width + 'px',
      height: windowSize.height + 'px',
      maxWidth: 'calc(100vw - 50px)',
      maxHeight: 'calc(100vh - 50px)',
    }"
  >
    <!-- Resize handles -->
    <!-- Edges -->
    <div class="resize-handle resize-n" @mousedown="(e) => startResize(e, 'n')"></div>
    <div class="resize-handle resize-s" @mousedown="(e) => startResize(e, 's')"></div>
    <div class="resize-handle resize-e" @mousedown="(e) => startResize(e, 'e')"></div>
    <div class="resize-handle resize-w" @mousedown="(e) => startResize(e, 'w')"></div>
    <!-- Corners -->
    <div class="resize-handle resize-nw" @mousedown="(e) => startResize(e, 'nw')"></div>
    <div class="resize-handle resize-ne" @mousedown="(e) => startResize(e, 'ne')"></div>
    <div class="resize-handle resize-sw" @mousedown="(e) => startResize(e, 'sw')"></div>
    <div class="resize-handle resize-se" @mousedown="(e) => startResize(e, 'se')"></div>

    <!-- Title bar (draggable) -->
    <div
      class="flex items-center justify-between px-3 py-2 bg-gray-800 border-b border-gray-700 cursor-move select-none"
      @mousedown="startDrag"
    >
      <div class="flex items-center gap-2">
        <span class="text-orange-400 text-sm font-medium">SPICE</span>
        <span class="text-white font-medium text-sm">VM {{ vmid }}</span>
        <span
          :class="[
            'w-2 h-2 rounded-full',
            connectionStatus === 'connected' ? 'bg-green-500' :
            connectionStatus === 'connecting' ? 'bg-yellow-500 animate-pulse' :
            connectionStatus === 'error' ? 'bg-red-500' :
            'bg-gray-500'
          ]"
          :title="connectionStatus"
        />
      </div>

      <div class="flex items-center gap-1">
        <!-- Reconnect button -->
        <button
          v-if="connectionStatus === 'disconnected' || connectionStatus === 'error'"
          @click.stop="connect"
          class="p-1.5 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
          :title="t('console.actions.reconnect')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
        <!-- Fullscreen button -->
        <button
          @click.stop="toggleFullscreen"
          class="p-1.5 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
          :title="t('console.actions.toggleFullscreen')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        </button>
        <!-- Close button -->
        <button
          @click.stop="emit('close')"
          class="p-1.5 text-gray-400 hover:text-white hover:bg-red-600 rounded transition-colors"
          :title="t('console.actions.close')"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Main content area - uses remaining height after title bar -->
    <div class="flex flex-1 min-h-0">
      <!-- Left sidebar toolbar (Proxmox-style) -->
      <div
        class="flex flex-col bg-gray-800 border-r border-gray-700 py-2 flex-shrink-0"
        :class="toolbarCollapsed ? 'w-8' : 'w-12'"
      >
        <!-- Collapse toggle -->
        <button
          @click="toolbarCollapsed = !toolbarCollapsed"
          class="mx-auto mb-2 p-1 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
          :title="toolbarCollapsed ? t('console.actions.expandToolbar') : t('console.actions.collapseToolbar')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 transition-transform"
            :class="toolbarCollapsed ? 'rotate-180' : ''"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>

        <div v-if="!toolbarCollapsed" class="flex flex-col items-center gap-1 px-1">
          <!-- SPICE logo/icon -->
          <div class="text-orange-400 font-bold text-xs mb-2 writing-vertical">
            SPICE
          </div>

          <!-- Ctrl+Alt+Del -->
          <button
            @click="handleSendCtrlAltDel"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :disabled="connectionStatus !== 'connected'"
            :title="t('console.actions.ctrlAltDel')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
            </svg>
          </button>

          <div class="border-t border-gray-600 w-8 my-2" />

          <!-- Fullscreen toggle -->
          <button
            @click="toggleFullscreen"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :title="t('console.actions.toggleFullscreen')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
          </button>

          <!-- Download .vv file -->
          <button
            @click="downloadSpiceConfig"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :title="t('console.spice.downloadVvTitle')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>

          <!-- Settings/Info (opens connection details) -->
          <button
            @click="showInfoPanel = !showInfoPanel"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :title="t('console.spice.connectionInfo')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </button>

          <!-- Disconnect/Power -->
          <button
            v-if="connectionStatus === 'connected'"
            @click="disconnect"
            class="w-10 h-8 bg-gray-700 text-red-400 hover:bg-red-600 hover:text-white rounded transition-colors flex items-center justify-center"
            :title="t('console.actions.disconnect')"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Console display area -->
      <div class="flex-1 relative min-w-0">
        <!-- SPICE Screen Container -->
        <div
          :id="'spice-screen-' + vmid"
          ref="spiceScreen"
          class="spice-screen bg-black w-full h-full"
        >
          <!-- Connecting overlay -->
          <div
            v-if="connectionStatus === 'connecting'"
            class="absolute inset-0 flex flex-col items-center justify-center bg-black bg-opacity-75 z-10"
          >
            <div class="animate-spin w-8 h-8 border-2 border-orange-500 border-t-transparent rounded-full mb-3"></div>
            <span class="text-gray-400">{{ t('console.spice.connecting') }}</span>
          </div>

          <!-- Error/Fallback overlay -->
          <div
            v-if="connectionStatus === 'error' || connectionStatus === 'disconnected'"
            class="absolute inset-0 flex flex-col items-center justify-center bg-black bg-opacity-90 p-6 z-10"
          >
            <div v-if="errorMessage" class="text-red-400 mb-4 text-center">{{ errorMessage }}</div>

            <div class="text-gray-400 text-center mb-6">
              <p class="mb-2">{{ t('console.spice.disconnectedTitle') }}</p>
              <p class="text-sm">{{ t('console.spice.disconnectedHint') }}</p>
            </div>

            <div class="flex gap-3 flex-wrap justify-center">
              <button
                @click="connect"
                class="px-4 py-2 bg-orange-600 hover:bg-orange-700 text-white rounded transition-colors"
              >
                {{ t('console.actions.reconnect') }}
              </button>
              <button
                @click="downloadSpiceConfig"
                class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
              >
                {{ t('console.spice.downloadVv') }}
              </button>
              <button
                @click="openExternalConsole"
                class="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
              >
                {{ t('console.actions.openProxmox') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Message div for spice-html5 status messages -->
        <div
          :id="'spice-message-' + vmid"
          ref="messageDiv"
          class="spice-message absolute bottom-4 left-4 right-4 text-center text-gray-400 text-sm"
        ></div>

        <!-- Settings/Info panel -->
        <div
          v-if="showInfoPanel"
          class="absolute bottom-0 left-0 right-0 bg-gray-800 border-t border-gray-700 p-3 text-sm"
        >
          <div class="flex justify-between items-start">
            <div class="space-y-1 text-gray-400">
              <div><span class="text-gray-500">{{ t('console.labels.host') }}</span> {{ ticket.host }}</div>
              <div><span class="text-gray-500">{{ t('console.labels.node') }}</span> {{ ticket.node }}</div>
              <div><span class="text-gray-500">{{ t('console.labels.vmid') }}</span> {{ vmid }}</div>
              <div><span class="text-gray-500">{{ t('console.labels.port') }}</span> {{ ticket.port || ticket.tlsPort || 'N/A' }}</div>
              <div><span class="text-gray-500">{{ t('console.labels.status') }}</span>
                <span :class="connectionStatus === 'connected' ? 'text-green-400' : 'text-red-400'">
                  {{ connectionStatus }}
                </span>
              </div>
            </div>
            <button
              @click="showInfoPanel = false"
              class="text-gray-400 hover:text-white"
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Style for spice-html5 canvas elements */
.spice-screen {
  overflow: hidden;
  position: relative;
}

.spice-screen :deep(canvas) {
  display: block;
  margin: 0 auto;
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
}

/* Window shadow and z-index */
.spice-window {
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

/* Vertical text for SPICE logo */
.writing-vertical {
  writing-mode: vertical-rl;
  text-orientation: mixed;
}

/* Override spice-html5 default message styles */
:deep(.spice-messages-info) {
  color: #60a5fa;
}

:deep(.spice-messages-warning) {
  color: #fbbf24;
}

:deep(.spice-messages-error) {
  color: #f87171;
}

/* Resize handles */
.resize-handle {
  position: absolute;
  z-index: 100;
}

/* Edge handles */
.resize-n {
  top: 0;
  left: 8px;
  right: 8px;
  height: 6px;
  cursor: ns-resize;
}

.resize-s {
  bottom: 0;
  left: 8px;
  right: 8px;
  height: 6px;
  cursor: ns-resize;
}

.resize-e {
  top: 8px;
  right: 0;
  bottom: 8px;
  width: 6px;
  cursor: ew-resize;
}

.resize-w {
  top: 8px;
  left: 0;
  bottom: 8px;
  width: 6px;
  cursor: ew-resize;
}

/* Corner handles */
.resize-nw {
  top: 0;
  left: 0;
  width: 12px;
  height: 12px;
  cursor: nwse-resize;
}

.resize-ne {
  top: 0;
  right: 0;
  width: 12px;
  height: 12px;
  cursor: nesw-resize;
}

.resize-sw {
  bottom: 0;
  left: 0;
  width: 12px;
  height: 12px;
  cursor: nesw-resize;
}

.resize-se {
  bottom: 0;
  right: 0;
  width: 12px;
  height: 12px;
  cursor: nwse-resize;
}
</style>
