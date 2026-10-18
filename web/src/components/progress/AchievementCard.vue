<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Achievement } from '@/types/progress'
import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDate } = useFormatters()

defineProps<{
  achievement: Achievement
  size?: 'sm' | 'md' | 'lg'
}>()

// PrimeIcons, which this app already depends on and uses in several hundred
// other places. Emoji render differently on every platform, cannot be
// recoloured to match the locked/unlocked states, and are announced literally
// by screen readers, so the achievement's own label carries the meaning.
const iconMap: Record<string, string> = {
  trophy: 'pi-trophy',
  fire: 'pi-wave-pulse',
  shield: 'pi-shield',
  star: 'pi-star-fill',
  lightning: 'pi-bolt',
  moon: 'pi-moon',
  target: 'pi-bullseye',
  book: 'pi-book',
  crown: 'pi-crown',
  detective: 'pi-search',
  medal: 'pi-verified',
  rocket: 'pi-send',
  lock: 'pi-lock',
  key: 'pi-key',
  flag: 'pi-flag-fill',
}
</script>

<template>
  <div
    class="relative rounded-xl border transition-all"
    :class="{
      'p-3': size === 'sm',
      'p-4': size === 'md' || !size,
      'p-6': size === 'lg',
      'bg-yellow-50 dark:bg-amber-900/40 border-yellow-300 dark:border-amber-500/50':
        achievement.unlocked,
      'bg-surface-50 dark:bg-surface-800 border-surface-200 dark:border-surface-700':
        !achievement.unlocked,
    }"
  >
    <!-- Unlocked badge -->
    <div v-if="achievement.unlocked" class="absolute -top-2 -right-2">
      <span class="flex h-5 w-5">
        <span
          class="relative inline-flex rounded-full h-5 w-5 bg-yellow-500 items-center justify-center"
        >
          <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
            <path
              fill-rule="evenodd"
              d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
              clip-rule="evenodd"
            />
          </svg>
        </span>
      </span>
    </div>

    <div class="flex items-center gap-3">
      <!-- Icon -->
      <div
        class="flex-shrink-0 flex items-center justify-center rounded-full"
        :class="{
          'w-10 h-10 text-2xl': size === 'sm',
          'w-14 h-14 text-3xl': size === 'md' || !size,
          'w-20 h-20 text-5xl': size === 'lg',
          'bg-yellow-200 dark:bg-amber-600 shadow-md': achievement.unlocked,
          'bg-surface-200 dark:bg-surface-700 text-surface-500 dark:text-surface-400':
            !achievement.unlocked,
        }"
      >
        <i :class="['pi', iconMap[achievement.icon] || 'pi-star']" aria-hidden="true"></i>
      </div>

      <!-- Content -->
      <div class="flex-1 min-w-0">
        <h4
          class="font-semibold truncate"
          :class="{
            'text-sm': size === 'sm',
            'text-base': size === 'md' || !size,
            'text-lg': size === 'lg',
            'text-amber-800 dark:text-amber-200': achievement.unlocked,
            'text-surface-700 dark:text-surface-100': !achievement.unlocked,
          }"
        >
          {{ achievement.name }}
        </h4>
        <p
          class="truncate"
          :class="{
            'text-xs': size === 'sm',
            'text-sm': size === 'md' || !size || size === 'lg',
            'text-amber-700 dark:text-amber-300/90': achievement.unlocked,
            'text-surface-500 dark:text-surface-400': !achievement.unlocked,
          }"
        >
          {{ achievement.description }}
        </p>

        <!-- Progress bar for incomplete achievements -->
        <div v-if="!achievement.unlocked && achievement.progress !== undefined" class="mt-2">
          <div class="flex justify-between text-xs text-surface-500 dark:text-surface-400 mb-1">
            <span>{{ t('achievementCard.progressLabel') }}</span>
            <span>{{ achievement.progress }} / {{ achievement.progressMax }}</span>
          </div>
          <div class="w-full bg-surface-200 dark:bg-surface-700 rounded-full h-1.5">
            <div
              class="bg-amber-500 dark:bg-amber-500 h-1.5 rounded-full transition-all"
              :style="{
                width: `${(achievement.progress / (achievement.progressMax || 1)) * 100}%`,
              }"
            ></div>
          </div>
        </div>

        <!-- Points -->
        <div class="flex items-center gap-2 mt-1">
          <span
            class="text-xs font-medium"
            :class="{
              'text-amber-600 dark:text-amber-400': achievement.unlocked,
              'text-surface-500 dark:text-surface-400': !achievement.unlocked,
            }"
          >
            {{ t('achievementCard.pointsEarned', { points: achievement.points }) }}
          </span>
          <span
            v-if="achievement.unlockedAt"
            class="text-xs text-surface-500 dark:text-surface-400"
          >
            {{ formatDate(achievement.unlockedAt) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
