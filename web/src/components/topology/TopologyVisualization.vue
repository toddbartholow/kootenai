<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Network, type Options, type Node, type Edge } from 'vis-network'
import { DataSet } from 'vis-data'
import type { TopologyData, TopologyVM, NetworkSegment } from '@/api'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import TrafficFlow from './TrafficFlow.vue'

const { t } = useI18n()

// Props
interface Props {
  topology: TopologyData
  selectedVM?: string | undefined
  showTrafficFlow?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  selectedVM: undefined,
  showTrafficFlow: false, // Disabled until animation issues resolved (see GitHub issue)
})

// Traffic flow toggle state (internal, synced with prop) - disabled by default
const trafficEnabled = ref(false)

// Emits
const emit = defineEmits<{
  (e: 'vm-click', vm: TopologyVM): void
  (e: 'vm-dblclick', vm: TopologyVM): void
  (e: 'segment-click', segment: NetworkSegment): void
}>()

// Refs
const containerRef = ref<HTMLElement | null>(null)
const network = ref<Network | null>(null)
const selectedNode = ref<string | null>(null)
const hoveredNode = ref<string | null>(null)

// Track VM and segment data for lookups
const vmMap = computed(() => {
  const map = new Map<string, TopologyVM>()
  props.topology.vms.forEach(vm => map.set(`vm-${vm.name}`, vm))
  return map
})

const segmentMap = computed(() => {
  const map = new Map<string, NetworkSegment>()
  props.topology.segments.forEach(seg => map.set(`seg-${seg.name}`, seg))
  return map
})

// Status colors
const statusColors: Record<string, { bg: string; border: string; text: string }> = {
  running: { bg: '#dcfce7', border: '#22c55e', text: '#166534' },
  stopped: { bg: '#fee2e2', border: '#ef4444', text: '#991b1b' },
  starting: { bg: '#fef3c7', border: '#f59e0b', text: '#92400e' },
  stopping: { bg: '#fef3c7', border: '#f59e0b', text: '#92400e' },
  paused: { bg: '#e0e7ff', border: '#6366f1', text: '#3730a3' },
  suspended: { bg: '#e0e7ff', border: '#6366f1', text: '#3730a3' },
  error: { bg: '#fee2e2', border: '#dc2626', text: '#991b1b' },
  created: { bg: '#f3f4f6', border: '#9ca3af', text: '#374151' },
}

// Get VM icon based on template/name
function _getVMIcon(vm: TopologyVM): string {
  const name = vm.name.toLowerCase()
  const template = vm.template.toLowerCase()

  if (name.includes('firewall') || name.includes('pfsense') || template.includes('pfsense')) {
    return '\uf06d' // shield
  }
  if (name.includes('router') || template.includes('router')) {
    return '\uf0e8' // sitemap
  }
  if (name.includes('switch')) {
    return '\uf0ec' // exchange
  }
  if (name.includes('kali') || name.includes('attacker') || template.includes('kali')) {
    return '\uf21b' // user-secret
  }
  if (name.includes('windows') || template.includes('windows')) {
    return '\uf17a' // windows
  }
  if (name.includes('security') || name.includes('siem')) {
    return '\uf132' // shield
  }
  return '\uf233' // server (default)
}

// Build network data
function buildNetworkData() {
  const nodes: Node[] = []
  const edges: Edge[] = []

  // Track segment positions for layout
  const segmentCount = props.topology.segments.length
  const segmentSpacing = 300
  const vmSpacing = 180

  // Create segment nodes (network switches)
  props.topology.segments.forEach((segment, index) => {
    const x = (index - (segmentCount - 1) / 2) * segmentSpacing

    nodes.push({
      id: `seg-${segment.name}`,
      label: `${segment.name}\n${segment.subnet}`,
      x,
      y: 0,
      shape: 'box',
      color: {
        background: '#dbeafe',
        border: '#3b82f6',
        highlight: {
          background: '#bfdbfe',
          border: '#2563eb',
        },
      },
      font: {
        size: 12,
        color: '#1e40af',
        face: 'Inter, system-ui, sans-serif',
      },
      borderWidth: 2,
      margin: { top: 10, bottom: 10, left: 10, right: 10 },
      physics: false,
    })
  })

  // Group VMs by their connected segments
  const vmsBySegment = new Map<string, TopologyVM[]>()

  props.topology.vms.forEach(vm => {
    vm.networks.forEach(net => {
      if (!vmsBySegment.has(net.segment)) {
        vmsBySegment.set(net.segment, [])
      }
      vmsBySegment.get(net.segment)!.push(vm)
    })
  })

  // Position and create VM nodes
  const processedVMs = new Set<string>()

  props.topology.segments.forEach((segment, segIndex) => {
    const segmentX = (segIndex - (segmentCount - 1) / 2) * segmentSpacing
    const vmsInSegment = vmsBySegment.get(segment.name) || []

    // Filter to VMs not yet positioned, prefer VMs only in this segment
    const vmsToPosition = vmsInSegment.filter(vm => !processedVMs.has(vm.name))

    vmsToPosition.forEach((vm, vmIndex) => {
      const defaultStatus = { bg: '#f3f4f6', border: '#9ca3af', text: '#374151' }
      const status = (vm.status && statusColors[vm.status]) ? statusColors[vm.status] : defaultStatus
      const isMultiHomed = vm.networks.length > 1

      // Position: multi-homed VMs go at top, others at bottom
      const yOffset = isMultiHomed ? -150 : 150
      const vmCountInRow = vmsToPosition.filter(v => (v.networks.length > 1) === isMultiHomed).length
      const vmIndexInRow = vmsToPosition.filter((v, i) => i < vmIndex && (v.networks.length > 1) === isMultiHomed).length
      const xOffset = (vmIndexInRow - (vmCountInRow - 1) / 2) * vmSpacing

      nodes.push({
        id: `vm-${vm.name}`,
        label: `${vm.name}\n${vm.ipAddress || t('topology.vm.noIp')}`,
        x: segmentX + xOffset,
        y: yOffset,
        shape: 'box',
        color: {
          background: status!.bg,
          border: status!.border,
          highlight: {
            background: status!.bg,
            border: '#3b82f6',
          },
        },
        font: {
          size: 11,
          color: status!.text,
          face: 'Inter, system-ui, sans-serif',
          multi: 'html',
        },
        borderWidth: 2,
        borderWidthSelected: 3,
        margin: { top: 8, bottom: 8, left: 12, right: 12 },
        physics: false,
      })

      processedVMs.add(vm.name)
    })
  })

  // Create edges (connections)
  props.topology.vms.forEach(vm => {
    vm.networks.forEach(net => {
      const edge: Edge = {
        id: `edge-${vm.name}-${net.segment}`,
        from: `vm-${vm.name}`,
        to: `seg-${net.segment}`,
        color: {
          color: '#94a3b8',
          highlight: '#3b82f6',
        },
        width: 2,
        smooth: {
          enabled: true,
          type: 'cubicBezier',
          roundness: 0.5,
        },
      }
      if (net.ip) {
        edge.title = `IP: ${net.ip}`
      }
      edges.push(edge)
    })
  })

  return { nodes: new DataSet(nodes), edges: new DataSet(edges) }
}

// Network options
const networkOptions: Options = {
  physics: {
    enabled: false,
  },
  interaction: {
    hover: true,
    tooltipDelay: 200,
    zoomView: true,
    dragView: true,
    dragNodes: false,
    selectConnectedEdges: true,
  },
  nodes: {
    shadow: {
      enabled: true,
      color: 'rgba(0,0,0,0.1)',
      size: 5,
      x: 2,
      y: 2,
    },
  },
  edges: {
    shadow: false,
    selectionWidth: 2,
  },
}

// Initialize network
function initNetwork() {
  if (!containerRef.value) return

  const data = buildNetworkData()

  network.value = new Network(containerRef.value, data, networkOptions)

  // Event handlers
  network.value.on('click', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0] as string
      selectedNode.value = nodeId

      if (nodeId.startsWith('vm-')) {
        const vm = vmMap.value.get(nodeId)
        if (vm) emit('vm-click', vm)
      } else if (nodeId.startsWith('seg-')) {
        const segment = segmentMap.value.get(nodeId)
        if (segment) emit('segment-click', segment)
      }
    } else {
      selectedNode.value = null
    }
  })

  network.value.on('doubleClick', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0] as string
      if (nodeId.startsWith('vm-')) {
        const vm = vmMap.value.get(nodeId)
        if (vm) emit('vm-dblclick', vm)
      }
    }
  })

  network.value.on('hoverNode', (params) => {
    hoveredNode.value = params.node as string
  })

  network.value.on('blurNode', () => {
    hoveredNode.value = null
  })

  // Wait for stabilization then fit to view
  network.value.once('stabilized', () => {
    centerAndFit()
  })

  // Also fit after a short delay in case stabilized doesn't fire (physics disabled)
  setTimeout(() => {
    centerAndFit()
  }, 200)
}

// Center and fit the network view
function centerAndFit() {
  if (!network.value || !containerRef.value) return

  // Check if container has actual dimensions
  const rect = containerRef.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    // Container not visible yet, try again later
    setTimeout(() => centerAndFit(), 100)
    return
  }

  network.value.fit({
    animation: {
      duration: 300,
      easingFunction: 'easeInOutQuad',
    },
  })

  // Mark network as ready after fit
  setTimeout(() => {
    networkReady.value = true
    // Apply initial selection if prop was set
    if (props.selectedVM) {
      const nodeId = `vm-${props.selectedVM}`
      try {
        network.value?.selectNodes([nodeId])
        selectedNode.value = nodeId
      } catch (err) {
        console.warn('Failed to select initial node:', err)
      }
    }
  }, 400)
}

// Update network when topology changes
watch(() => props.topology, () => {
  if (network.value) {
    network.value.destroy()
    networkReady.value = false
  }
  initNetwork()
}, { deep: true })

// Track if network is ready for interactions
const networkReady = ref(false)

// Highlight selected VM from prop
watch(() => props.selectedVM, (vmName) => {
  if (network.value && vmName && networkReady.value) {
    const nodeId = `vm-${vmName}`
    try {
      network.value.selectNodes([nodeId])
      selectedNode.value = nodeId
    } catch (err) {
      // vis-network can throw errors if internal state isn't ready
      console.warn('Failed to select node:', err)
    }
  }
})

// Track if we've fitted since becoming visible
let hasFittedSinceVisible = false
let resizeObserver: ResizeObserver | null = null

// Lifecycle
onMounted(() => {
  initNetwork()

  // Watch for container becoming visible (e.g., when tab is selected)
  if (containerRef.value) {
    resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect.width > 0 && entry.contentRect.height > 0) {
          if (!hasFittedSinceVisible) {
            hasFittedSinceVisible = true
            // Small delay to let the container settle
            setTimeout(() => centerAndFit(), 50)
          }
        } else {
          // Container hidden, reset flag
          hasFittedSinceVisible = false
        }
      }
    })
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  if (network.value) {
    network.value.destroy()
  }
  if (resizeObserver) {
    resizeObserver.disconnect()
  }
})

// Public methods
function fitToView() {
  centerAndFit()
}

function zoomIn() {
  if (network.value) {
    const scale = network.value.getScale()
    network.value.moveTo({ scale: scale * 1.2 })
  }
}

function zoomOut() {
  if (network.value) {
    const scale = network.value.getScale()
    network.value.moveTo({ scale: scale / 1.2 })
  }
}

// Export topology as image
function exportAsImage() {
  if (!containerRef.value) return

  const canvas = containerRef.value.querySelector('canvas')
  if (canvas) {
    const link = document.createElement('a')
    link.download = `topology-${props.topology.labTemplate.replace(/\s+/g, '-')}.png`
    link.href = canvas.toDataURL('image/png')
    link.click()
  }
}

// Toggle traffic flow
function toggleTraffic() {
  trafficEnabled.value = !trafficEnabled.value
}

// Expose methods and internal maps for testing
defineExpose({
  fitToView,
  zoomIn,
  zoomOut,
  exportAsImage,
  toggleTraffic,
  vmMap,
  segmentMap,
})

// Selected node info
const selectedVMInfo = computed(() => {
  if (!selectedNode.value?.startsWith('vm-')) return null
  return vmMap.value.get(selectedNode.value) || null
})

const selectedSegmentInfo = computed(() => {
  if (!selectedNode.value?.startsWith('seg-')) return null
  return segmentMap.value.get(selectedNode.value) || null
})
</script>

<template>
  <div class="topology-container">
    <!-- Toolbar -->
    <div class="topology-toolbar">
      <div class="flex items-center gap-2">
        <Button
          icon="pi pi-search-plus"
          severity="secondary"
          size="small"
          text
          rounded
          @click="zoomIn"
          v-tooltip.bottom="t('topology.toolbar.zoomIn')"
        />
        <Button
          icon="pi pi-search-minus"
          severity="secondary"
          size="small"
          text
          rounded
          @click="zoomOut"
          v-tooltip.bottom="t('topology.toolbar.zoomOut')"
        />
        <Button
          icon="pi pi-arrows-alt"
          severity="secondary"
          size="small"
          text
          rounded
          @click="fitToView"
          v-tooltip.bottom="t('topology.toolbar.fit')"
        />
        <div class="w-px h-6 bg-surface-200 dark:bg-surface-700 mx-1"></div>
        <Button
          icon="pi pi-download"
          severity="secondary"
          size="small"
          text
          rounded
          @click="exportAsImage"
          v-tooltip.bottom="t('topology.toolbar.export')"
        />
        <div class="w-px h-6 bg-surface-200 dark:bg-surface-700 mx-1"></div>
        <Button
          :icon="trafficEnabled ? 'pi pi-bolt' : 'pi pi-stop'"
          :severity="trafficEnabled ? 'info' : 'secondary'"
          size="small"
          text
          rounded
          @click="toggleTraffic"
          v-tooltip.bottom="trafficEnabled ? t('topology.toolbar.hideTraffic') : t('topology.toolbar.showTraffic')"
        />
      </div>

      <!-- Legend -->
      <div class="flex items-center gap-4 text-xs">
        <div class="flex items-center gap-1">
          <span class="w-3 h-3 rounded border-2 bg-green-100 border-green-500"></span>
          <span class="text-surface-600 dark:text-surface-400">{{ t('topology.legend.running') }}</span>
        </div>
        <div class="flex items-center gap-1">
          <span class="w-3 h-3 rounded border-2 bg-red-100 border-red-500"></span>
          <span class="text-surface-600 dark:text-surface-400">{{ t('topology.legend.stopped') }}</span>
        </div>
        <div class="flex items-center gap-1">
          <span class="w-3 h-3 rounded border-2 bg-yellow-100 border-yellow-500"></span>
          <span class="text-surface-600 dark:text-surface-400">{{ t('topology.legend.starting') }}</span>
        </div>
        <div class="flex items-center gap-1">
          <span class="w-3 h-3 rounded border-2 bg-blue-100 border-blue-500"></span>
          <span class="text-surface-600 dark:text-surface-400">{{ t('topology.legend.network') }}</span>
        </div>
      </div>
    </div>

    <!-- Network visualization -->
    <div ref="containerRef" class="topology-canvas"></div>

    <!-- Traffic flow overlay -->
    <TrafficFlow
      :network="network"
      :vms="topology.vms"
      :enabled="trafficEnabled"
    />

    <!-- Info panel (shown when node is selected) -->
    <Transition name="slide-up">
      <div v-if="selectedVMInfo" class="topology-info-panel">
        <div class="flex items-start justify-between mb-2">
          <div>
            <h4 class="font-semibold text-surface-900 dark:text-surface-100">
              {{ selectedVMInfo.name }}
            </h4>
            <p class="text-xs text-surface-500">{{ selectedVMInfo.platformId }}</p>
          </div>
          <Tag
            :value="selectedVMInfo.status"
            :severity="selectedVMInfo.status === 'running' ? 'success' : selectedVMInfo.status === 'stopped' ? 'danger' : 'warn'"
          />
        </div>
        <div class="grid grid-cols-2 gap-2 text-sm">
          <div>
            <span class="text-surface-500">{{ t('topology.vm.ipLabel') }}</span>
            <span class="ml-1 font-mono text-surface-900 dark:text-surface-100">
              {{ selectedVMInfo.ipAddress || t('topology.vm.ipFallback') }}
            </span>
          </div>
          <div>
            <span class="text-surface-500">{{ t('topology.vm.templateLabel') }}</span>
            <span class="ml-1 text-surface-900 dark:text-surface-100">
              {{ selectedVMInfo.template }}
            </span>
          </div>
          <div>
            <span class="text-surface-500">{{ t('topology.vm.cpuLabel') }}</span>
            <span class="ml-1 text-surface-900 dark:text-surface-100">
              {{ t('topology.vm.cpuValue', { count: selectedVMInfo.resources.cpu }) }}
            </span>
          </div>
          <div>
            <span class="text-surface-500">{{ t('topology.vm.memoryLabel') }}</span>
            <span class="ml-1 text-surface-900 dark:text-surface-100">
              {{ t('topology.vm.memoryValue', { gb: (selectedVMInfo.resources.memory / 1024).toFixed(1) }) }}
            </span>
          </div>
        </div>
        <div class="mt-2 pt-2 border-t border-surface-200 dark:border-surface-700">
          <span class="text-xs text-surface-500">{{ t('topology.vm.networksLabel') }}</span>
          <div class="flex flex-wrap gap-1 mt-1">
            <Tag
              v-for="net in selectedVMInfo.networks"
              :key="net.segment"
              :value="net.ip ? t('topology.vm.networkTagWithIp', { segment: net.segment, ip: net.ip }) : t('topology.vm.networkTag', { segment: net.segment })"
              severity="info"
            />
          </div>
        </div>
        <p class="text-xs text-surface-400 mt-2">{{ t('topology.vm.doubleClickHint') }}</p>
      </div>
    </Transition>

    <Transition name="slide-up">
      <div v-if="selectedSegmentInfo && !selectedVMInfo" class="topology-info-panel">
        <h4 class="font-semibold text-surface-900 dark:text-surface-100 mb-2">
          {{ selectedSegmentInfo.name }}
        </h4>
        <div class="grid grid-cols-2 gap-2 text-sm">
          <div>
            <span class="text-surface-500">{{ t('topology.segment.subnetLabel') }}</span>
            <span class="ml-1 font-mono text-surface-900 dark:text-surface-100">
              {{ selectedSegmentInfo.subnet }}
            </span>
          </div>
          <div>
            <span class="text-surface-500">{{ t('topology.segment.vlanLabel') }}</span>
            <span class="ml-1 text-surface-900 dark:text-surface-100">
              {{ selectedSegmentInfo.vlan }}
            </span>
          </div>
          <div v-if="selectedSegmentInfo.gateway">
            <span class="text-surface-500">{{ t('topology.segment.gatewayLabel') }}</span>
            <span class="ml-1 font-mono text-surface-900 dark:text-surface-100">
              {{ selectedSegmentInfo.gateway }}
            </span>
          </div>
          <div>
            <span class="text-surface-500">{{ t('topology.segment.dhcpLabel') }}</span>
            <span class="ml-1 text-surface-900 dark:text-surface-100">
              {{ selectedSegmentInfo.dhcp ? t('topology.segment.dhcpEnabled') : t('topology.segment.dhcpDisabled') }}
            </span>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.topology-container {
  @apply relative w-full h-full min-h-[400px] bg-surface-50 dark:bg-surface-900 rounded-lg border border-surface-200 dark:border-surface-700 overflow-hidden;
}

.topology-toolbar {
  @apply absolute top-0 left-0 right-0 z-10 flex items-center justify-between px-3 py-2 bg-white/90 dark:bg-surface-800/90 backdrop-blur-sm border-b border-surface-200 dark:border-surface-700;
}

.topology-canvas {
  @apply w-full h-full;
}

.topology-info-panel {
  @apply absolute bottom-0 left-0 right-0 z-10 p-4 bg-white dark:bg-surface-800 border-t border-surface-200 dark:border-surface-700;
}

/* Transitions */
.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}
</style>
