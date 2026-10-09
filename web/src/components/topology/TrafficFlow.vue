<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { Network } from 'vis-network'
import type { TopologyVM } from '@/api'
import { generateMockTraffic, generateParticles, updateTrafficFlows, type TrafficFlow } from './trafficSimulation'

interface Props {
  network: Network | null
  vms: TopologyVM[]
  enabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  enabled: true,
})

// State
const flows = ref<TrafficFlow[]>([])
const canvasSize = ref({ width: 0, height: 0 })
const svgPaths = ref<Map<string, string>>(new Map())

// Generate particles from flows
const particles = computed(() => {
  if (!props.enabled || flows.value.length === 0) return []
  return generateParticles(flows.value)
})

// Calculate SVG paths from network positions
function calculatePaths() {
  if (!props.network || !props.enabled) {
    svgPaths.value = new Map()
    return
  }

  const paths = new Map<string, string>()

  try {
    const positions = props.network.getPositions()

    flows.value.forEach(flow => {
      const fromPos = positions[flow.fromNode]
      const toPos = positions[flow.toNode]

      if (!fromPos || !toPos) return

      // Use vis-network's canvasToDOM to get screen coordinates
      const fromDOM = props.network!.canvasToDOM({ x: fromPos.x, y: fromPos.y })
      const toDOM = props.network!.canvasToDOM({ x: toPos.x, y: toPos.y })

      // Create curved path
      const midX = (fromDOM.x + toDOM.x) / 2
      const midY = (fromDOM.y + toDOM.y) / 2
      const dx = toDOM.x - fromDOM.x
      const dy = toDOM.y - fromDOM.y
      const len = Math.sqrt(dx * dx + dy * dy)

      if (len < 1) return

      const curveOffset = Math.min(20, len * 0.08)
      const perpX = -dy / len * curveOffset
      const perpY = dx / len * curveOffset
      const ctrlX = midX + perpX
      const ctrlY = midY + perpY

      paths.set(flow.id, `M ${fromDOM.x} ${fromDOM.y} Q ${ctrlX} ${ctrlY} ${toDOM.x} ${toDOM.y}`)
    })
  } catch {
    // Network not ready yet
  }

  svgPaths.value = paths
}

// Update canvas size
function updateCanvasSize() {
  if (!props.network) return

  try {
    const container = (props.network as unknown as { body: { container: HTMLElement } }).body?.container
    if (container) {
      const rect = container.getBoundingClientRect()
      canvasSize.value = { width: rect.width, height: rect.height }
    }
  } catch {
    // Network not ready
  }
}

// Initialize traffic flows
function initTraffic() {
  if (props.vms.length > 0) {
    flows.value = generateMockTraffic(props.vms)
  }
}

// Get flow line color based on direction
// Colors chosen for good visibility in both light and dark modes
function getFlowColor(flow: TrafficFlow): string {
  switch (flow.direction) {
    case 'upload':
      return '#06b6d4' // cyan-500 - outbound traffic
    case 'download':
      return '#10b981' // emerald-500 - inbound traffic
    case 'bidirectional':
      return '#3b82f6' // blue-500 - two-way traffic
    default:
      return '#3b82f6'
  }
}

// Animation frame loop for smooth updates (passive - just reads state)
let animationFrameId: number | null = null
let trafficInterval: ReturnType<typeof setInterval> | null = null

function startAnimationLoop() {
  const update = () => {
    if (props.enabled && props.network) {
      updateCanvasSize()
      calculatePaths()
    }
    animationFrameId = requestAnimationFrame(update)
  }
  animationFrameId = requestAnimationFrame(update)
}

function stopAnimationLoop() {
  if (animationFrameId) {
    cancelAnimationFrame(animationFrameId)
    animationFrameId = null
  }
}

// Watch for VMs changes
watch(
  () => props.vms,
  () => initTraffic(),
  { deep: true, immediate: true }
)

// Watch for enabled toggle
watch(
  () => props.enabled,
  (enabled) => {
    if (enabled) {
      initTraffic()
      startAnimationLoop()
    } else {
      stopAnimationLoop()
    }
  }
)

// Watch for network becoming available
watch(
  () => props.network,
  (network) => {
    if (network && props.enabled) {
      // Delay to let network initialize
      setTimeout(() => {
        initTraffic()
        updateCanvasSize()
        calculatePaths()
      }, 300)
    }
  },
  { immediate: true }
)

onMounted(() => {
  if (props.enabled) {
    startAnimationLoop()
  }

  // Update traffic patterns periodically
  trafficInterval = setInterval(() => {
    if (props.enabled && props.vms.length > 0) {
      flows.value = updateTrafficFlows(flows.value, props.vms)
    }
  }, 5000)
})

onUnmounted(() => {
  stopAnimationLoop()
  if (trafficInterval) {
    clearInterval(trafficInterval)
  }
})
</script>

<template>
  <svg
    v-if="enabled && particles.length > 0 && canvasSize.width > 0"
    class="traffic-overlay"
    :width="canvasSize.width"
    :height="canvasSize.height"
    :viewBox="`0 0 ${canvasSize.width} ${canvasSize.height}`"
  >
    <defs>
      <!-- Enhanced glow filter for particles -->
      <filter id="particle-glow" x="-100%" y="-100%" width="300%" height="300%">
        <feGaussianBlur in="SourceGraphic" stdDeviation="2" result="blur1" />
        <feGaussianBlur in="SourceGraphic" stdDeviation="4" result="blur2" />
        <feMerge>
          <feMergeNode in="blur2" />
          <feMergeNode in="blur1" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>

      <!-- Subtle glow for flow lines -->
      <filter id="line-glow" x="-20%" y="-20%" width="140%" height="140%">
        <feGaussianBlur in="SourceGraphic" stdDeviation="1" result="blur" />
        <feMerge>
          <feMergeNode in="blur" />
          <feMergeNode in="SourceGraphic" />
        </feMerge>
      </filter>

      <!-- Arrow marker for direction indication -->
      <marker id="arrow-upload" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto">
        <path d="M 0 0 L 6 3 L 0 6 Z" fill="#06b6d4" opacity="0.8" />
      </marker>
      <marker id="arrow-download" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto">
        <path d="M 0 0 L 6 3 L 0 6 Z" fill="#10b981" opacity="0.8" />
      </marker>
      <marker id="arrow-bidirectional" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto">
        <path d="M 0 0 L 6 3 L 0 6 Z" fill="#3b82f6" opacity="0.8" />
      </marker>
    </defs>

    <g v-for="flow in flows" :key="flow.id" class="traffic-flow">
      <!-- Background glow line (subtle) -->
      <path
        :d="svgPaths.get(flow.id) || ''"
        fill="none"
        :stroke="getFlowColor(flow)"
        stroke-width="6"
        stroke-linecap="round"
        :opacity="flow.intensity === 'high' ? 0.15 : 0.08"
        filter="url(#line-glow)"
      />

      <!-- Animated dashed line showing flow -->
      <path
        :d="svgPaths.get(flow.id) || ''"
        fill="none"
        :stroke="getFlowColor(flow)"
        stroke-width="2"
        :class="['flow-line', `flow-speed-${flow.intensity}`]"
        :marker-mid="`url(#arrow-${flow.direction})`"
      />

      <!-- Reference path for particle animation -->
      <path
        :id="`path-${flow.id}`"
        :d="svgPaths.get(flow.id) || ''"
        fill="none"
        stroke="none"
      />

      <!-- Particle trail (fainter following particle) -->
      <circle
        v-for="particle in particles.filter(p => p.flowId === flow.id)"
        :key="`trail-${particle.id}`"
        :r="particle.size * 0.6"
        :fill="particle.color"
        :opacity="particle.opacity * 0.3"
        class="particle-trail"
      >
        <animateMotion
          :dur="`${flow.speed}s`"
          repeatCount="indefinite"
          :begin="`${-particle.offset * flow.speed - 0.1}s`"
          calcMode="linear"
        >
          <mpath :href="`#path-${flow.id}`" />
        </animateMotion>
      </circle>

      <!-- Main animated particles with glow -->
      <circle
        v-for="particle in particles.filter(p => p.flowId === flow.id)"
        :key="particle.id"
        :r="particle.size"
        :fill="particle.color"
        :opacity="particle.opacity"
        filter="url(#particle-glow)"
        class="traffic-particle"
      >
        <animateMotion
          :dur="`${flow.speed}s`"
          repeatCount="indefinite"
          :begin="`${-particle.offset * flow.speed}s`"
          calcMode="linear"
        >
          <mpath :href="`#path-${flow.id}`" />
        </animateMotion>
      </circle>
    </g>
  </svg>
</template>

<style scoped>
.traffic-overlay {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
  z-index: 5;
  overflow: visible;
}

.traffic-particle {
  will-change: transform;
  filter: url(#particle-glow);
}

.traffic-flow {
  opacity: 0.9;
  transition: opacity 0.3s ease;
}

/* Animated flowing line effect - smoother dash animation */
.flow-line {
  animation: flow-dash linear infinite;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.flow-speed-low {
  animation-duration: 3s;
  opacity: 0.4;
  stroke-dasharray: 6 8;
}

.flow-speed-medium {
  animation-duration: 1.5s;
  opacity: 0.6;
  stroke-dasharray: 8 6;
}

.flow-speed-high {
  animation-duration: 0.8s;
  opacity: 0.85;
  stroke-dasharray: 10 4;
  animation-name: flow-dash, flow-pulse;
}

@keyframes flow-dash {
  to {
    stroke-dashoffset: -24;
  }
}

/* Subtle pulse for high-traffic flows */
@keyframes flow-pulse {
  0%, 100% {
    stroke-width: 2px;
  }
  50% {
    stroke-width: 3px;
  }
}

/* Particle trail effect */
.particle-trail {
  opacity: 0.3;
}

/* Dark mode adjustments */
:global(.dark) .flow-speed-low {
  opacity: 0.5;
}

:global(.dark) .flow-speed-medium {
  opacity: 0.7;
}

:global(.dark) .flow-speed-high {
  opacity: 0.9;
}
</style>
