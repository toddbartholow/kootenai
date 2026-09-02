<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import type { LabRecommendation, PathwayRecommendation, Pathway } from '@/api'
import Card from '@volt/Card.vue'
import Tag from '@volt/Tag.vue'

defineProps<{
  labRecommendations: LabRecommendation[]
  pathwayRecommendations: PathwayRecommendation[]
  featuredPathways: Pathway[]
  itemCount: number
}>()

const { t } = useI18n()
const router = useRouter()

function navigateToLab(labSlug: string) {
  router.push(`/labs/${labSlug}`)
}

function navigateToRecommendedPathway(rec: PathwayRecommendation) {
  router.push(`/pathways/${rec.pathwaySlug}`)
}

function navigateToPathway(pathway: Pathway) {
  router.push(`/pathways/${pathway.slug}`)
}

function getRecommendationIcon(type: string): string {
  switch (type) {
    case 'continue_progress':
      return 'pi-play'
    case 'next_in_pathway':
      return 'pi-arrow-right'
    case 'new_pathway':
      return 'pi-sparkles'
    case 'popular':
      return 'pi-star'
    default:
      return 'pi-lightbulb'
  }
}

function getRecommendationColor(type: string): string {
  switch (type) {
    case 'continue_progress':
      return 'text-blue-500'
    case 'next_in_pathway':
      return 'text-green-500'
    case 'new_pathway':
      return 'text-violet-500'
    case 'popular':
      return 'text-amber-500'
    default:
      return 'text-cyan-500'
  }
}
</script>

<template>
  <div>
    <div
      v-if="labRecommendations.length === 0 && pathwayRecommendations.length === 0"
      class="text-center py-6"
    >
      <i
        class="pi pi-compass text-3xl text-surface-300 dark:text-surface-600 mb-3"
        aria-hidden="true"
      />
      <p class="text-sm text-surface-500">{{ t('dashboard.recommendations.empty') }}</p>
    </div>
    <div v-else class="space-y-3" data-pseudo-skip>
      <!-- Lab Recommendations -->
      <div
        v-for="rec in labRecommendations.slice(0, itemCount)"
        :key="rec.labTemplateId"
        role="button"
        tabindex="0"
        class="p-3 rounded-lg border border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-600 cursor-pointer transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
        :aria-label="`View lab: ${rec.labName}`"
        @click="navigateToLab(rec.labSlug)"
        @keydown.enter="navigateToLab(rec.labSlug)"
        @keydown.space.prevent="navigateToLab(rec.labSlug)"
      >
        <div class="flex items-start gap-3">
          <div
            class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
            :class="
              rec.type === 'continue_progress'
                ? 'bg-blue-100 dark:bg-blue-900/50'
                : 'bg-green-100 dark:bg-green-900/50'
            "
          >
            <i :class="['pi', getRecommendationIcon(rec.type), getRecommendationColor(rec.type)]" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 mb-1">
              <h3 class="font-medium text-surface-900 dark:text-surface-100 truncate text-base">
                {{ rec.labName }}
              </h3>
              <Tag
                v-if="rec.type === 'continue_progress'"
                :value="t('dashboard.recommendations.resumeTag')"
                severity="info"
                class="text-xs"
              />
            </div>
            <p class="text-xs text-surface-500 mb-1">{{ rec.reason }}</p>
            <div class="flex items-center gap-3 text-xs text-surface-500">
              <span v-if="rec.durationMinutes"
                ><i class="pi pi-clock mr-1" />{{ rec.durationMinutes }}m</span
              >
              <span v-if="rec.maxPoints"><i class="pi pi-star mr-1" />{{ rec.maxPoints }} pts</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Pathway Recommendations -->
      <div
        v-for="rec in pathwayRecommendations.slice(0, itemCount)"
        :key="rec.pathwayId"
        role="button"
        tabindex="0"
        class="p-3 rounded-lg border border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-600 cursor-pointer transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
        :aria-label="`View pathway: ${rec.pathwayName}`"
        @click="navigateToRecommendedPathway(rec)"
        @keydown.enter="navigateToRecommendedPathway(rec)"
        @keydown.space.prevent="navigateToRecommendedPathway(rec)"
      >
        <div class="flex items-start gap-3">
          <div
            class="w-8 h-8 rounded-lg bg-violet-100 dark:bg-violet-900/50 flex items-center justify-center shrink-0"
          >
            <i :class="['pi', rec.icon || 'pi-map', 'text-violet-500']" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 mb-1">
              <h3 class="font-medium text-surface-900 dark:text-surface-100 truncate text-base">
                {{ rec.pathwayName }}
              </h3>
              <Tag
                v-if="rec.type === 'popular'"
                :value="t('dashboard.recommendations.popularTag')"
                severity="warn"
                class="text-xs"
              />
            </div>
            <p class="text-xs text-surface-500 mb-1">{{ rec.reason }}</p>
            <div class="flex items-center gap-3 text-xs text-surface-500">
              <span v-if="rec.estimatedHours"
                ><i class="pi pi-clock mr-1" />{{ rec.estimatedHours }}h</span
              >
              <span v-if="rec.labCount"><i class="pi pi-book mr-1" />{{ rec.labCount }} labs</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Featured Pathways (fallback if no recommendations) -->
  <Card
    v-if="
      featuredPathways.length > 0 &&
      labRecommendations.length === 0 &&
      pathwayRecommendations.length === 0
    "
    class="mt-6"
  >
    <template #title>
      <div class="flex items-center gap-2">
        <i class="pi pi-sparkles text-violet-500" aria-hidden="true" />
        <span>{{ t('dashboard.recommendations.featuredTitle') }}</span>
      </div>
    </template>
    <template #content>
      <div class="space-y-3" data-pseudo-skip>
        <div
          v-for="pathway in featuredPathways"
          :key="pathway.id"
          role="button"
          tabindex="0"
          class="p-3 rounded-lg border border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-600 cursor-pointer transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500"
          :aria-label="`View pathway: ${pathway.name}`"
          @click="navigateToPathway(pathway)"
          @keydown.enter="navigateToPathway(pathway)"
          @keydown.space.prevent="navigateToPathway(pathway)"
        >
          <div class="flex items-start justify-between mb-2">
            <h3 class="font-medium text-surface-900 dark:text-surface-100 text-base">
              {{ pathway.name }}
            </h3>
            <Tag
              v-if="pathway.isFeatured"
              :value="t('dashboard.recommendations.featuredTag')"
              severity="warn"
              class="text-xs"
            />
          </div>
          <p class="text-sm text-surface-500 line-clamp-2 mb-2">
            {{ pathway.shortDescription || pathway.description }}
          </p>
          <div class="flex items-center gap-4 text-xs text-surface-500">
            <span
              ><i class="pi pi-clock mr-1" aria-hidden="true" />{{ pathway.estimatedHours }}h</span
            >
            <span><i class="pi pi-tag mr-1" aria-hidden="true" />{{ pathway.difficulty }}</span>
          </div>
        </div>
      </div>
    </template>
  </Card>
</template>
