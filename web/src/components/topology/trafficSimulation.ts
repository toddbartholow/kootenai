/**
 * Traffic Flow Simulation
 * Generates mock traffic data for topology visualization
 */

import type { TopologyVM } from '@/api'

export type TrafficIntensity = 'none' | 'low' | 'medium' | 'high'
export type TrafficDirection = 'upload' | 'download' | 'bidirectional'

export interface TrafficFlow {
  id: string
  edgeId: string
  fromNode: string
  toNode: string
  intensity: TrafficIntensity
  direction: TrafficDirection
  particleCount: number
  speed: number // seconds per animation cycle
}

export interface TrafficParticle {
  id: string
  flowId: string
  offset: number // 0-1 position along path
  size: number
  opacity: number
  color: string
}

// Particle configuration by intensity - tuned for visual appeal
const intensityConfig: Record<TrafficIntensity, { particles: number; speed: number; size: number }> = {
  none: { particles: 0, speed: 0, size: 0 },
  low: { particles: 1, speed: 5, size: 3 },
  medium: { particles: 3, speed: 3, size: 3.5 },
  high: { particles: 5, speed: 2, size: 4 },
}

/**
 * Generate mock traffic flows based on VM status
 * Running VMs generate traffic, stopped VMs don't
 */
export function generateMockTraffic(vms: TopologyVM[]): TrafficFlow[] {
  const flows: TrafficFlow[] = []

  vms.forEach(vm => {
    // Only running VMs generate traffic
    if (vm.status !== 'running') return

    vm.networks.forEach(network => {
      const edgeId = `edge-${vm.name}-${network.segment}`
      const intensity = getRandomIntensity(vm)
      const config = intensityConfig[intensity]

      if (intensity === 'none') return

      flows.push({
        id: `flow-${vm.name}-${network.segment}`,
        edgeId,
        fromNode: `vm-${vm.name}`,
        toNode: `seg-${network.segment}`,
        intensity,
        direction: getRandomDirection(),
        particleCount: config.particles,
        speed: config.speed,
      })
    })
  })

  return flows
}

/**
 * Generate particles for a set of traffic flows
 */
export function generateParticles(flows: TrafficFlow[]): TrafficParticle[] {
  const particles: TrafficParticle[] = []

  flows.forEach(flow => {
    const config = intensityConfig[flow.intensity]

    for (let i = 0; i < flow.particleCount; i++) {
      // Stagger particles evenly along the path
      const offset = flow.particleCount > 1 ? i / flow.particleCount : 0

      particles.push({
        id: `particle-${flow.id}-${i}`,
        flowId: flow.id,
        offset,
        size: config.size,
        opacity: 0.7,
        color: getParticleColor(flow.direction),
      })
    }
  })

  return particles
}

/**
 * Get random traffic intensity based on VM characteristics
 */
function getRandomIntensity(vm: TopologyVM): TrafficIntensity {
  const name = vm.name.toLowerCase()

  // Firewalls/routers typically have high traffic
  if (name.includes('firewall') || name.includes('router') || name.includes('pfsense')) {
    return weightedRandom(['medium', 'high', 'high'], 'high')
  }

  // Security/monitoring VMs have medium traffic
  if (name.includes('security') || name.includes('siem') || name.includes('monitor')) {
    return weightedRandom(['low', 'medium', 'medium'], 'medium')
  }

  // Attack hosts might have bursts
  if (name.includes('kali') || name.includes('attacker')) {
    return weightedRandom(['none', 'low', 'medium', 'high'], 'low')
  }

  // Default: always show some traffic for running VMs
  return weightedRandom(['low', 'low', 'medium'], 'low')
}

/**
 * Get random traffic direction
 */
function getRandomDirection(): TrafficDirection {
  const directions: TrafficDirection[] = ['upload', 'download', 'bidirectional']
  const index = Math.floor(Math.random() * directions.length)
  return directions[index] ?? 'bidirectional'
}

/**
 * Get particle color based on direction
 * Colors chosen for good visibility in both light and dark modes
 */
function getParticleColor(direction: TrafficDirection): string {
  switch (direction) {
    case 'upload':
      return '#06b6d4' // cyan-500 - outbound traffic
    case 'download':
      return '#10b981' // emerald-500 - inbound traffic
    case 'bidirectional':
      return '#3b82f6' // blue-500 - two-way traffic
  }
}

/**
 * Weighted random selection from array
 */
function weightedRandom<T>(items: T[], fallback: T): T {
  const index = Math.floor(Math.random() * items.length)
  return items[index] ?? fallback
}

/**
 * Periodically update traffic flows (call this on interval to simulate changes)
 */
export function updateTrafficFlows(currentFlows: TrafficFlow[], vms: TopologyVM[]): TrafficFlow[] {
  // 20% chance to regenerate traffic patterns
  if (Math.random() < 0.2) {
    return generateMockTraffic(vms)
  }

  // Otherwise, just slightly vary existing flows
  return currentFlows.map(flow => {
    // 30% chance to change intensity slightly
    if (Math.random() < 0.3) {
      const intensities: ('low' | 'medium' | 'high')[] = ['low', 'medium', 'high']
      const currentIndex = intensities.indexOf(flow.intensity as 'low' | 'medium' | 'high')
      if (currentIndex >= 0) {
        // Move up or down by 1, staying in bounds
        const newIndex = Math.max(0, Math.min(intensities.length - 1, currentIndex + (Math.random() > 0.5 ? 1 : -1)))
        const newIntensity: TrafficIntensity = intensities[newIndex] ?? 'medium'
        const config = intensityConfig[newIntensity]
        return {
          ...flow,
          intensity: newIntensity,
          particleCount: config.particles,
          speed: config.speed,
        }
      }
    }
    return flow
  })
}
