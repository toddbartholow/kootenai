<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { consoleApi, type ConsoleTicket } from '@/api'

const { t } = useI18n()

// Declare RFB type for noVNC library loaded via script
declare global {
  interface Window {
    RFB: new (
      target: HTMLElement,
      url: string,
      options?: {
        credentials?: { password?: string }
        shared?: boolean
        repeaterID?: string
        wsProtocols?: string[]
      },
    ) => {
      addEventListener(type: string, listener: (event: CustomEvent) => void): void
      removeEventListener(type: string, listener: (event: CustomEvent) => void): void
      disconnect(): void
      sendCtrlAltDel(): void
      focus(): void
      blur(): void
      scaleViewport: boolean
      resizeSession: boolean
      viewOnly: boolean
    }
  }
}

const props = defineProps<{
  podId: string
  vmName: string
  vmId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'fullscreen'): void
}>()

const loading = ref(true)
const connecting = ref(false)
const connected = ref(false)
const error = ref<string | null>(null)
const ticket = ref<ConsoleTicket | null>(null)
const isFullscreen = ref(false)
const consoleWrapper = ref<HTMLDivElement | null>(null)
const vncContainer = ref<HTMLDivElement | null>(null)
const noVNCLoaded = ref(false)

// RFB instance (type is the instance type of the RFB constructor)
let rfb: InstanceType<typeof window.RFB> | null = null

// Build the WebSocket proxy URL (goes through our API server)
const proxyWsUrl = computed(() => {
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

  return `${wsBase}/api/v1/pods/${props.podId}/vms/${props.vmName}/vnc`
})

// Build the Proxmox noVNC console URL (fallback option)
const noVNCUrl = computed(() => {
  if (!ticket.value || ticket.value.type !== 'vnc') return ''

  const host = ticket.value.host
  const vmid = ticket.value.vmid
  const node = ticket.value.node

  return `https://${host}/?console=kvm&novnc=1&vmid=${vmid}&node=${node}&resize=scale`
})

// WebSocket URL for direct noVNC connection (for reference/debugging)
const wsUrl = computed(() => {
  if (!ticket.value || ticket.value.type !== 'vnc') return ''

  const host = ticket.value.host
  const node = ticket.value.node
  const vmid = ticket.value.vmid
  const port = ticket.value.port
  const vncTicket = encodeURIComponent(ticket.value.ticket)

  return `wss://${host}/api2/json/nodes/${node}/qemu/${vmid}/vncwebsocket?port=${port}&vncticket=${vncTicket}`
})

// Load noVNC library dynamically (reserved for future use)
async function _loadNoVNC(): Promise<void> {
  if (noVNCLoaded.value || window.RFB) {
    noVNCLoaded.value = true
    return
  }

  return new Promise((resolve, reject) => {
    // Load noVNC from CDN
    const script = document.createElement('script')
    script.src = 'https://cdn.jsdelivr.net/npm/@nicman23/noVNC@1.3.0/lib/rfb.js'
    script.type = 'module'

    // Alternative: load from unpkg
    // script.src = 'https://unpkg.com/@nicman23/noVNC@1.3.0/lib/rfb.js'

    script.onload = () => {
      noVNCLoaded.value = true
      resolve()
    }
    script.onerror = () => {
      reject(new Error('Failed to load noVNC library'))
    }

    document.head.appendChild(script)
  })
}

async function loadConsole() {
  loading.value = true
  error.value = null

  try {
    // Get console ticket from API (auto-detect type - tries VNC first, then SPICE)
    ticket.value = await consoleApi.getTicket(props.podId, props.vmName)
    loading.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('console.error.connectionFailed')
    loading.value = false
  }
}

async function _connectVNC() {
  if (!vncContainer.value) {
    error.value = t('console.spice.containerNotReady')
    return
  }

  connecting.value = true
  error.value = null

  try {
    // For now, just open in new tab since noVNC CDN loading is complex
    // The embedded console will work once the WebSocket proxy is fully tested
    window.open(noVNCUrl.value, '_blank')
    connecting.value = false
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('console.error.failedToConnect')
    connecting.value = false
  }
}

function disconnectVNC() {
  if (rfb) {
    rfb.disconnect()
    rfb = null
  }
  connected.value = false
}

function sendCtrlAltDel() {
  if (rfb && connected.value) {
    rfb.sendCtrlAltDel()
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
  disconnectVNC()
  emit('close')
}

function handleRetry() {
  disconnectVNC()
  loadConsole()
}

// Handle fullscreen change events
function onFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
}

// Copy WebSocket URL to clipboard for use with external noVNC client
async function copyWsUrl() {
  if (wsUrl.value) {
    await navigator.clipboard.writeText(wsUrl.value)
    alert(t('console.vnc.copiedDirect'))
  }
}

// Copy proxy WebSocket URL to clipboard
async function copyProxyWsUrl() {
  if (proxyWsUrl.value) {
    await navigator.clipboard.writeText(proxyWsUrl.value)
    alert(t('console.vnc.copiedProxy'))
  }
}

// Download SPICE config file for native SPICE client
function downloadSpiceConfig() {
  if (!ticket.value) return

  const spiceConfig = `[virt-viewer]
type=spice
host=${ticket.value.host.split(':')[0]}
port=${ticket.value.port}
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
  loadConsole()
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

onUnmounted(() => {
  disconnectVNC()
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<template>
  <div
    ref="consoleWrapper"
    class="vm-console flex flex-col h-full bg-gray-900 rounded-lg overflow-hidden"
  >
    <!-- Header -->
    <div class="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
      <div class="flex items-center space-x-3">
        <div
          class="w-3 h-3 rounded-full"
          :class="connected ? 'bg-green-500' : 'bg-yellow-500'"
        ></div>
        <span class="text-white font-medium">{{ vmName }}</span>
        <span class="text-gray-400 text-sm">{{
          connected ? t('console.status.connected') : 'Console'
        }}</span>
      </div>
      <div class="flex items-center space-x-2">
        <!-- Send Ctrl+Alt+Del button -->
        <button
          v-if="connected"
          @click="sendCtrlAltDel"
          class="px-3 py-1 text-xs bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
          :title="t('console.actions.sendCtrlAltDel')"
        >
          {{ t('console.actions.ctrlAltDel') }}
        </button>
        <!-- Fullscreen button -->
        <button
          @click="toggleFullscreen"
          class="p-2 text-gray-400 hover:text-white transition-colors"
          :title="t('console.actions.toggleFullscreen')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              v-if="!isFullscreen"
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
            />
            <path
              v-else
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
        <!-- Close button -->
        <button
          @click="handleClose"
          class="p-2 text-gray-400 hover:text-white transition-colors"
          :title="t('console.actions.closeConsole')"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5"
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

    <!-- Console Content -->
    <div class="flex-1 relative min-h-[400px]">
      <!-- Loading State -->
      <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-gray-900">
        <div class="text-center">
          <div
            class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-500 mx-auto mb-4"
          ></div>
          <p class="text-gray-400">{{ t('console.loading') }}</p>
        </div>
      </div>

      <!-- Error State -->
      <div v-else-if="error" class="absolute inset-0 flex items-center justify-center bg-gray-900">
        <div class="text-center max-w-md px-4">
          <div class="text-red-500 mb-4">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-12 w-12 mx-auto"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <p class="text-white font-medium mb-2">{{ t('console.error.failedToConnect') }}</p>
          <p class="text-gray-400 text-sm mb-4">{{ error }}</p>
          <button
            @click="handleRetry"
            class="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
          >
            {{ t('console.actions.retryConnection') }}
          </button>
        </div>
      </div>

      <!-- VNC Console -->
      <div v-else-if="ticket?.type === 'vnc'" class="absolute inset-0 flex flex-col bg-gray-900">
        <!-- VNC Container (for embedded console - future) -->
        <div v-if="connected" ref="vncContainer" class="flex-1 bg-black"></div>

        <!-- Connection Options -->
        <div v-else class="flex-1 flex items-center justify-center">
          <div class="text-center max-w-lg p-8">
            <div class="mb-6">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-16 w-16 mx-auto text-primary-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
            </div>

            <h3 class="text-xl font-bold text-white mb-2">{{ t('console.vnc.title') }}</h3>
            <p class="text-gray-400 mb-6">
              {{ t('console.vnc.connectDescription', { name: vmName }) }}
            </p>

            <div class="space-y-3">
              <!-- Open Proxmox Console (new tab) -->
              <a
                :href="noVNCUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center justify-center w-full px-6 py-3 bg-primary-600 text-white font-medium rounded-lg hover:bg-primary-700 transition-colors"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mr-2"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                  />
                </svg>
                {{ t('console.vnc.openNewTab') }}
              </a>

              <!-- Copy Proxy WebSocket URL -->
              <button
                @click="copyProxyWsUrl"
                class="inline-flex items-center justify-center w-full px-6 py-3 border border-gray-600 text-gray-300 font-medium rounded-lg hover:bg-gray-800 transition-colors"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mr-2"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                  />
                </svg>
                {{ t('console.vnc.copyProxyUrl') }}
              </button>

              <!-- Copy Direct WebSocket URL -->
              <button
                @click="copyWsUrl"
                class="inline-flex items-center justify-center w-full px-6 py-3 border border-gray-600 text-gray-300 font-medium rounded-lg hover:bg-gray-800 transition-colors"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-5 w-5 mr-2"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                  />
                </svg>
                {{ t('console.vnc.copyDirectUrl') }}
              </button>
            </div>

            <!-- Connection Info -->
            <div class="mt-6 p-4 bg-gray-800 rounded-lg text-left">
              <p class="text-gray-400 text-sm mb-2">{{ t('console.labels.connectionDetails') }}</p>
              <div class="space-y-1 text-sm">
                <p>
                  <span class="text-gray-500">{{ t('console.labels.type') }}</span>
                  <span class="text-white">VNC</span>
                </p>
                <p>
                  <span class="text-gray-500">{{ t('console.labels.host') }}</span>
                  <span class="text-white">{{ ticket.host }}</span>
                </p>
                <p>
                  <span class="text-gray-500">{{ t('console.labels.port') }}</span>
                  <span class="text-white">{{ ticket.port }}</span>
                </p>
                <p>
                  <span class="text-gray-500">{{ t('console.labels.node') }}</span>
                  <span class="text-white">{{ ticket.node }}</span>
                </p>
                <p>
                  <span class="text-gray-500">{{ t('console.labels.vmid') }}</span>
                  <span class="text-white">{{ ticket.vmid }}</span>
                </p>
              </div>
            </div>

            <p class="mt-4 text-xs text-gray-500">
              {{ t('console.vnc.note') }}
            </p>
          </div>
        </div>
      </div>

      <!-- SPICE Console -->
      <div
        v-else-if="ticket?.type === 'spice'"
        class="absolute inset-0 flex items-center justify-center bg-gray-900"
      >
        <div class="text-center max-w-lg p-8">
          <div class="mb-6">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-16 w-16 mx-auto text-primary-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          <h3 class="text-xl font-bold text-white mb-2">{{ t('console.spice.title') }}</h3>
          <p class="text-gray-400 mb-6">
            {{ t('console.spice.vmDescription') }}
          </p>

          <button
            @click="downloadSpiceConfig"
            class="inline-flex items-center justify-center w-full px-6 py-3 bg-primary-600 text-white font-medium rounded-lg hover:bg-primary-700 transition-colors"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 mr-2"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
              />
            </svg>
            {{ t('console.spice.downloadConnection') }}
          </button>

          <!-- Connection Info -->
          <div class="mt-4 p-4 bg-gray-800 rounded-lg text-left">
            <p class="text-gray-400 text-sm mb-2">{{ t('console.labels.connectionDetails') }}</p>
            <div class="space-y-1 text-sm">
              <p>
                <span class="text-gray-500">{{ t('console.labels.type') }}</span>
                <span class="text-white">SPICE</span>
              </p>
              <p>
                <span class="text-gray-500">{{ t('console.labels.host') }}</span>
                <span class="text-white">{{ ticket.host }}</span>
              </p>
              <p>
                <span class="text-gray-500">{{ t('console.labels.port') }}</span>
                <span class="text-white">{{ ticket.port }}</span>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vm-console {
  min-height: 500px;
}
</style>
