<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SkillProficiency } from '@/api/domains/dashboard'

const { t } = useI18n()

const props = defineProps<{
  skills: SkillProficiency[]
}>()

const SIZE = 280
const CX = SIZE / 2
const CY = SIZE / 2
const RADIUS = 100
const LABEL_R = RADIUS + 24
const RINGS = 4

const sortedSkills = computed(() =>
  [...props.skills].sort((a, b) => b.proficiency - a.proficiency).slice(0, 8)
)

const step = computed(() => (2 * Math.PI) / (sortedSkills.value.length || 1))

function pointAt(index: number, r: number): { x: number; y: number } {
  const angle = index * step.value - Math.PI / 2
  return {
    x: CX + r * Math.cos(angle),
    y: CY + r * Math.sin(angle),
  }
}

// Grid ring paths (concentric polygons)
const ringPaths = computed(() => {
  const n = sortedSkills.value.length
  if (n === 0) return []
  return Array.from({ length: RINGS }, (_, ring) => {
    const r = (RADIUS * (ring + 1)) / RINGS
    const pts = Array.from({ length: n }, (_, i) => {
      const p = pointAt(i, r)
      return `${p.x},${p.y}`
    })
    return pts.join(' ')
  })
})

// Spoke lines from center to each vertex
const spokes = computed(() => {
  const n = sortedSkills.value.length
  return Array.from({ length: n }, (_, i) => pointAt(i, RADIUS))
})

// Data polygon path
const dataPolygon = computed(() => {
  const n = sortedSkills.value.length
  if (n === 0) return ''
  const pts = sortedSkills.value.map((s, i) => {
    const val = Math.min(s.proficiency, 100) / 100
    const p = pointAt(i, RADIUS * val)
    return `${p.x},${p.y}`
  })
  return pts.join(' ')
})

// Label positions
const labels = computed(() =>
  sortedSkills.value.map((s, i) => {
    const p = pointAt(i, LABEL_R)
    return { tag: s.tag, x: p.x, y: p.y, proficiency: Math.round(s.proficiency) }
  })
)
</script>

<template>
  <div class="flex flex-col items-center">
    <svg
      v-if="sortedSkills.length > 0"
      :viewBox="`0 0 ${SIZE} ${SIZE}`"
      class="w-full max-w-[280px]"
    >
      <!-- Grid rings -->
      <polygon
        v-for="(pts, i) in ringPaths"
        :key="'ring-' + i"
        :points="pts"
        fill="none"
        class="stroke-surface-300 dark:stroke-surface-600"
        stroke-width="0.5"
      />

      <!-- Spokes -->
      <line
        v-for="(pt, i) in spokes"
        :key="'spoke-' + i"
        :x1="CX"
        :y1="CY"
        :x2="pt.x"
        :y2="pt.y"
        class="stroke-surface-300 dark:stroke-surface-600"
        stroke-width="0.5"
      />

      <!-- Data polygon -->
      <polygon
        :points="dataPolygon"
        fill="rgba(99, 102, 241, 0.25)"
        stroke="rgba(99, 102, 241, 0.85)"
        stroke-width="2"
      />

      <!-- Data points -->
      <circle
        v-for="(s, i) in sortedSkills"
        :key="'dot-' + i"
        :cx="pointAt(i, RADIUS * Math.min(s.proficiency, 100) / 100).x"
        :cy="pointAt(i, RADIUS * Math.min(s.proficiency, 100) / 100).y"
        r="3"
        fill="rgb(99, 102, 241)"
      />

      <!-- Labels -->
      <text
        v-for="label in labels"
        :key="label.tag"
        :x="label.x"
        :y="label.y"
        text-anchor="middle"
        dominant-baseline="middle"
        class="fill-surface-600 dark:fill-surface-300"
        font-size="10"
      >
        {{ label.tag }}
      </text>
    </svg>

    <div v-else class="text-sm text-surface-500 py-8">
      {{ t('dashboard.skillRadar.empty') }}
    </div>
  </div>
</template>
