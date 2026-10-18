<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { type ConsoleTicket } from '@/api'
import { useVncConnection, buildWsProxyUrl } from '@/composables/useVncConnection'
import { useVncKeyboard } from '@/composables/useVncKeyboard'

const props = defineProps<{
  vmid: number
  ticket: ConsoleTicket
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

// DOM refs
const containerRef = ref<HTMLDivElement | null>(null)
const windowRef = ref<HTMLDivElement | null>(null)

// --- VNC connection via composable ---
const conn = useVncConnection({
  maxReconnectAttempts: 5,
  reconnectDelay: 1000,
})
const { connectionStatus, errorMessage } = conn

// --- Keyboard via composable ---
const keyboard = useVncKeyboard(() => conn.getRfb())
const {
  ctrlActive,
  altActive,
  sendCtrlAltDel,
  sendTab,
  sendEscape,
  sendSuper,
  toggleCtrl,
  toggleAlt,
} = keyboard

// Build WebSocket URL for our proxy
const wsProxyUrl = computed(() => {
  if (!props.ticket || props.ticket.type !== 'vnc') return ''
  return buildWsProxyUrl(props.vmid, props.ticket.node)
})

async function connect() {
  if (!wsProxyUrl.value || !containerRef.value) {
    errorMessage.value = t('console.vnc.noProxyUrl')
    connectionStatus.value = 'error'
    return
  }
  const vncPassword = props.ticket.ticket || props.ticket.password || ''
  const instance = await conn.connect(containerRef.value, wsProxyUrl.value, vncPassword)
  if (instance) {
    instance.dragViewport = false
  }
}

function disconnect() {
  conn.disconnect()
}

// --- Translated connection status for display ---
const connectionStatusText = computed(() => {
  switch (connectionStatus.value) {
    case 'connected':
      return t('console.status.connected')
    case 'connecting':
      return t('console.status.connecting')
    case 'disconnected':
      return t('console.status.disconnected')
    case 'error':
      return t('console.error.connectionFailed')
    default:
      return connectionStatus.value
  }
})

// --- Mobile detection ---
const isMobile = ref(false)
const isFullscreenMode = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth < 768 || 'ontouchstart' in window
}

// --- Window position, size, dragging, and resizing ---
const windowPosition = ref({ x: 50, y: 50 })
const windowSize = ref({ width: 900, height: 650 })
const isDragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })
const isResizing = ref(false)
const resizeDirection = ref('')
const resizeStart = ref({ x: 0, y: 0, width: 0, height: 0, left: 0, top: 0 })

const windowStyles = computed(() => {
  if (isMobile.value || isFullscreenMode.value) {
    return {
      left: '0',
      top: '0',
      width: '100vw',
      height: '100vh',
      maxWidth: '100vw',
      maxHeight: '100vh',
      borderRadius: '0',
    }
  }
  return {
    left: windowPosition.value.x + 'px',
    top: windowPosition.value.y + 'px',
    width: windowSize.value.width + 'px',
    height: windowSize.value.height + 'px',
    maxWidth: 'calc(100vw - 50px)',
    maxHeight: 'calc(100vh - 50px)',
  }
})

// Toolbar state
const toolbarCollapsed = ref(false)
const showKeyboardPanel = ref(false)

// Dragging functions with touch support
function startDrag(e: MouseEvent | TouchEvent) {
  if (!windowRef.value || isMobile.value) return
  isDragging.value = true
  const rect = windowRef.value.getBoundingClientRect()
  const clientX = 'touches' in e ? (e.touches[0]?.clientX ?? 0) : e.clientX
  const clientY = 'touches' in e ? (e.touches[0]?.clientY ?? 0) : e.clientY
  dragOffset.value = {
    x: clientX - rect.left,
    y: clientY - rect.top,
  }
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  document.addEventListener('touchmove', onDrag, { passive: false })
  document.addEventListener('touchend', stopDrag)
}

function onDrag(e: MouseEvent | TouchEvent) {
  if (!isDragging.value) return
  if ('touches' in e) e.preventDefault()
  const clientX = 'touches' in e ? (e.touches[0]?.clientX ?? 0) : e.clientX
  const clientY = 'touches' in e ? (e.touches[0]?.clientY ?? 0) : e.clientY
  windowPosition.value = {
    x: Math.max(0, Math.min(window.innerWidth - 200, clientX - dragOffset.value.x)),
    y: Math.max(0, Math.min(window.innerHeight - 100, clientY - dragOffset.value.y)),
  }
}

function stopDrag() {
  isDragging.value = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
  document.removeEventListener('touchmove', onDrag)
  document.removeEventListener('touchend', stopDrag)
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
    top: windowPosition.value.y,
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

function toggleFullscreen() {
  if (isMobile.value) {
    isFullscreenMode.value = !isFullscreenMode.value
    return
  }

  if (windowRef.value) {
    if (document.fullscreenElement) {
      document.exitFullscreen()
      isFullscreenMode.value = false
    } else {
      windowRef.value.requestFullscreen()
      isFullscreenMode.value = true
    }
  }
}

function onFullscreenChange() {
  isFullscreenMode.value = !!document.fullscreenElement
}

// Build the Proxmox noVNC console URL as fallback
const proxmoxConsoleUrl = computed(() => {
  if (!props.ticket || props.ticket.type !== 'vnc') return ''
  const host = props.ticket.host
  const vmid = props.ticket.vmid
  const node = props.ticket.node
  return `https://${host}/?console=kvm&novnc=1&vmid=${vmid}&node=${node}&resize=scale`
})

function openExternalConsole() {
  const width = 1024
  const height = 768
  const left = (window.screen.width - width) / 2
  const top = (window.screen.height - height) / 2
  window.open(
    proxmoxConsoleUrl.value,
    `vnc_${props.vmid}`,
    `width=${width},height=${height},left=${left},top=${top},menubar=no,toolbar=no,location=no,status=no`,
  )
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  document.addEventListener('fullscreenchange', onFullscreenChange)

  if (isMobile.value) {
    isFullscreenMode.value = true
    toolbarCollapsed.value = true
  } else {
    windowPosition.value = {
      x: Math.max(50, (window.innerWidth - windowSize.value.width) / 2),
      y: Math.max(50, (window.innerHeight - windowSize.value.height) / 2),
    }
  }

  connect()
})

onUnmounted(() => {
  disconnect()
  window.removeEventListener('resize', checkMobile)
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
  document.removeEventListener('touchmove', onDrag)
  document.removeEventListener('touchend', stopDrag)
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
})

// Reconnect if ticket changes
watch(
  () => props.ticket,
  () => {
    disconnect()
    connect()
  },
)
</script>

<template>
  <!-- Draggable and resizable floating window -->
  <div
    ref="windowRef"
    class="vnc-window fixed bg-gray-900 shadow-2xl border border-gray-700 overflow-hidden z-50 flex flex-col"
    :class="{ 'rounded-lg': !isMobile && !isFullscreenMode }"
    :style="windowStyles"
  >
    <!-- Resize handles (hidden on mobile) -->
    <template v-if="!isMobile && !isFullscreenMode">
      <!-- Edges -->
      <div class="resize-handle resize-n" @mousedown="e => startResize(e, 'n')"></div>
      <div class="resize-handle resize-s" @mousedown="e => startResize(e, 's')"></div>
      <div class="resize-handle resize-e" @mousedown="e => startResize(e, 'e')"></div>
      <div class="resize-handle resize-w" @mousedown="e => startResize(e, 'w')"></div>
      <!-- Corners -->
      <div class="resize-handle resize-nw" @mousedown="e => startResize(e, 'nw')"></div>
      <div class="resize-handle resize-ne" @mousedown="e => startResize(e, 'ne')"></div>
      <div class="resize-handle resize-sw" @mousedown="e => startResize(e, 'sw')"></div>
      <div class="resize-handle resize-se" @mousedown="e => startResize(e, 'se')"></div>
    </template>
    <!-- Title bar (draggable on desktop) -->
    <div
      class="flex items-center justify-between px-3 py-2 bg-gray-800 border-b border-gray-700 select-none"
      :class="{ 'cursor-move': !isMobile }"
      @mousedown="startDrag"
      @touchstart="startDrag"
    >
      <div class="flex items-center gap-2">
        <span class="text-gray-400 text-sm">noVNC</span>
        <span class="text-white font-medium text-sm">{{ ticket.vmid }} - VM {{ vmid }}</span>
        <span
          :class="[
            'w-2 h-2 rounded-full',
            connectionStatus === 'connected'
              ? 'bg-green-500'
              : connectionStatus === 'connecting'
                ? 'bg-yellow-500 animate-pulse'
                : connectionStatus === 'error'
                  ? 'bg-red-500'
                  : 'bg-gray-500',
          ]"
          :title="connectionStatusText"
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
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>
        <!-- Fullscreen button -->
        <button
          @click.stop="toggleFullscreen"
          class="p-1.5 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
          :title="t('console.actions.toggleFullscreen')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
            />
          </svg>
        </button>
        <!-- Close button -->
        <button
          @click.stop="emit('close')"
          class="p-1.5 text-gray-400 hover:text-white hover:bg-red-600 rounded transition-colors"
          :title="t('console.actions.close')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
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
          :title="
            toolbarCollapsed
              ? t('console.actions.expandToolbar')
              : t('console.actions.collapseToolbar')
          "
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 transition-transform"
            :class="toolbarCollapsed ? 'rotate-180' : ''"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </button>

        <div v-if="!toolbarCollapsed" class="flex flex-col items-center gap-1 px-1">
          <!-- noVNC logo/icon -->
          <div class="text-blue-400 font-bold text-xs mb-2 writing-vertical">
            <span class="text-blue-300">no</span><span class="text-blue-500">VNC</span>
          </div>

          <!-- Ctrl key (sticky) -->
          <button
            @click="toggleCtrl"
            :class="[
              'w-10 h-8 text-xs font-medium rounded transition-colors',
              ctrlActive ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600',
            ]"
            :disabled="connectionStatus !== 'connected'"
            title="Ctrl (sticky)"
          >
            Ctrl
          </button>

          <!-- Alt key (sticky) -->
          <button
            @click="toggleAlt"
            :class="[
              'w-10 h-8 text-xs font-medium rounded transition-colors',
              altActive ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600',
            ]"
            :disabled="connectionStatus !== 'connected'"
            title="Alt (sticky)"
          >
            Alt
          </button>

          <!-- Windows/Super key -->
          <button
            @click="sendSuper"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :disabled="connectionStatus !== 'connected'"
            title="Windows/Super key"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              viewBox="0 0 24 24"
              fill="currentColor"
            >
              <path
                d="M3 12V6.75l6-1.32v6.57H3zm17-8.25v8.25h-10V5.13l10-1.38zM3 13h6v6.57l-6-1.32V13zm17 5.25l-10-1.38V13h10v5.25z"
              />
            </svg>
          </button>

          <!-- Tab key -->
          <button
            @click="sendTab"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 text-xs font-medium rounded transition-colors flex items-center justify-center"
            :disabled="connectionStatus !== 'connected'"
            title="Tab"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 5l7 7-7 7"
              />
            </svg>
          </button>

          <!-- Escape key -->
          <button
            @click="sendEscape"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 text-xs font-medium rounded transition-colors"
            :disabled="connectionStatus !== 'connected'"
            title="Escape"
          >
            Esc
          </button>

          <!-- Ctrl+Alt+Del -->
          <button
            @click="sendCtrlAltDel"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :disabled="connectionStatus !== 'connected'"
            :title="t('console.actions.ctrlAltDel')"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"
              />
            </svg>
          </button>

          <div class="border-t border-gray-600 w-8 my-2" />

          <!-- Fullscreen toggle -->
          <button
            @click="toggleFullscreen"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :title="t('console.actions.toggleFullscreen')"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
              />
            </svg>
          </button>

          <!-- Settings/Info (opens connection details) -->
          <button
            @click="showKeyboardPanel = !showKeyboardPanel"
            class="w-10 h-8 bg-gray-700 text-gray-300 hover:bg-gray-600 rounded transition-colors flex items-center justify-center"
            :title="t('console.actions.settings')"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
              />
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
            </svg>
          </button>

          <!-- Disconnect/Power -->
          <button
            v-if="connectionStatus === 'connected'"
            @click="disconnect"
            class="w-10 h-8 bg-gray-700 text-red-400 hover:bg-red-600 hover:text-white rounded transition-colors flex items-center justify-center"
            :title="t('console.actions.disconnect')"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 10V3L4 14h7v7l9-11h-7z"
              />
            </svg>
          </button>
        </div>
      </div>

      <!-- Console display area -->
      <div class="flex-1 relative min-w-0">
        <div ref="containerRef" class="vnc-container bg-black w-full h-full">
          <!-- Connecting overlay -->
          <div
            v-if="connectionStatus === 'connecting'"
            class="absolute inset-0 flex flex-col items-center justify-center bg-black bg-opacity-75 z-10"
          >
            <div
              class="animate-spin w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full mb-3"
            ></div>
            <span class="text-gray-400">{{ t('console.status.connecting') }}</span>
          </div>

          <!-- Error/Fallback overlay -->
          <div
            v-if="connectionStatus === 'error' || connectionStatus === 'disconnected'"
            class="absolute inset-0 flex flex-col items-center justify-center bg-black bg-opacity-90 p-6 z-10"
          >
            <div v-if="errorMessage" class="text-red-400 mb-4 text-center">{{ errorMessage }}</div>

            <div class="text-gray-400 text-center mb-6">
              <p class="mb-2">{{ t('console.disconnected.title') }}</p>
              <p class="text-sm">{{ t('console.disconnected.hint') }}</p>
            </div>

            <div class="flex gap-3">
              <button
                @click="connect"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors"
              >
                {{ t('console.actions.reconnect') }}
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

        <!-- Settings/Info panel -->
        <div
          v-if="showKeyboardPanel"
          class="absolute bottom-0 left-0 right-0 bg-gray-800 border-t border-gray-700 p-3 text-sm"
        >
          <div class="flex justify-between items-start">
            <div class="space-y-1 text-gray-400">
              <div>
                <span class="text-gray-500">{{ t('console.labels.host') }}</span> {{ ticket.host }}
              </div>
              <div>
                <span class="text-gray-500">{{ t('console.labels.node') }}</span> {{ ticket.node }}
              </div>
              <div>
                <span class="text-gray-500">{{ t('console.labels.vmid') }}</span> {{ vmid }}
              </div>
              <div>
                <span class="text-gray-500">{{ t('console.labels.status') }}</span>
                <span :class="connectionStatus === 'connected' ? 'text-green-400' : 'text-red-400'">
                  {{ connectionStatusText }}
                </span>
              </div>
            </div>
            <button @click="showKeyboardPanel = false" class="text-gray-400 hover:text-white">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Ensure noVNC canvas fills the container and scales properly */
.vnc-container :deep(canvas) {
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
}

/* Ensure the container doesn't overflow */
.vnc-container {
  overflow: hidden;
  position: relative;
}

/* Window shadow and z-index */
.vnc-window {
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

/* Vertical text for noVNC logo */
.writing-vertical {
  writing-mode: vertical-rl;
  text-orientation: mixed;
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
