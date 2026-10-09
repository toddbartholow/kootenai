<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserAchievement } from '@/api'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'

const { t, te } = useI18n()

/** Localized tier name for a known tier, or the raw string as fallback. */
function tierLabel(tier: string): string {
  const key = `achievementTier.${tier}`
  return te(key) ? t(key) : tier
}

const props = defineProps<{
  visible: boolean
  achievements: UserAchievement[]
}>()

const emit = defineEmits<{
  close: []
}>()

// Current achievement index for multi-achievement display
const currentIndex = ref(0)

// Reset index when modal opens with new achievements
watch(
  () => props.visible,
  visible => {
    if (visible) {
      currentIndex.value = 0
    }
  },
)

const currentAchievement = computed(() => props.achievements[currentIndex.value]?.achievement)
const hasMultiple = computed(() => props.achievements.length > 1)
const totalPoints = computed(() =>
  props.achievements.reduce((sum, ua) => sum + (ua.achievement?.points || 0), 0),
)

function nextAchievement() {
  if (currentIndex.value < props.achievements.length - 1) {
    currentIndex.value++
  } else {
    emit('close')
  }
}

function prevAchievement() {
  if (currentIndex.value > 0) {
    currentIndex.value--
  }
}

function getTierColor(tier: string): string {
  switch (tier) {
    case 'bronze':
      return 'text-orange-500'
    case 'silver':
      return 'text-gray-400'
    case 'gold':
      return 'text-yellow-500'
    case 'platinum':
      return 'text-blue-400'
    case 'diamond':
      return 'text-accent-500'
    default:
      return 'text-surface-500'
  }
}

function getTierBgColor(tier: string): string {
  switch (tier) {
    case 'bronze':
      return 'bg-orange-100 dark:bg-orange-900/30'
    case 'silver':
      return 'bg-gray-100 dark:bg-gray-800/50'
    case 'gold':
      return 'bg-yellow-100 dark:bg-yellow-900/30'
    case 'platinum':
      return 'bg-blue-100 dark:bg-blue-900/30'
    case 'diamond':
      return 'bg-accent-100 dark:bg-accent-900/30'
    default:
      return 'bg-surface-100 dark:bg-surface-800'
  }
}

function getTierBorderColor(tier: string): string {
  switch (tier) {
    case 'bronze':
      return 'border-orange-300 dark:border-orange-700'
    case 'silver':
      return 'border-gray-300 dark:border-gray-600'
    case 'gold':
      return 'border-yellow-400 dark:border-yellow-600'
    case 'platinum':
      return 'border-blue-300 dark:border-blue-600'
    case 'diamond':
      return 'border-accent-400 dark:border-accent-600'
    default:
      return 'border-surface-300 dark:border-surface-600'
  }
}

function getTierGlow(tier: string): string {
  switch (tier) {
    case 'bronze':
      return 'shadow-orange-300/50'
    case 'silver':
      return 'shadow-gray-300/50'
    case 'gold':
      return 'shadow-yellow-300/50'
    case 'platinum':
      return 'shadow-blue-300/50'
    case 'diamond':
      return 'shadow-accent-300/50'
    default:
      return 'shadow-surface-300/50'
  }
}

function getIconForType(type: string): string {
  switch (type) {
    case 'lab_completion':
      return 'pi pi-check-circle'
    case 'perfect_score':
      return 'pi pi-star-fill'
    case 'speed':
      return 'pi pi-bolt'
    case 'streak':
      return 'pi pi-calendar'
    case 'category':
      return 'pi pi-folder'
    case 'milestone':
      return 'pi pi-flag-fill'
    case 'special':
      return 'pi pi-gift'
    default:
      return 'pi pi-trophy'
  }
}
</script>

<template>
  <Dialog
    :visible="visible"
    @update:visible="$emit('close')"
    modal
    :closable="true"
    :style="{ width: '420px' }"
    :pt="{
      header: 'border-none pb-0',
      content: 'pt-0',
    }"
  >
    <template #header>
      <div class="w-full text-center">
        <div class="achievement-header">
          <div class="text-xs uppercase tracking-widest text-surface-500 mb-1">
            {{ t('achievementUnlock.header') }}
          </div>
          <div v-if="hasMultiple" class="text-xs text-surface-400">
            {{
              t('achievementUnlock.multiProgress', {
                current: currentIndex + 1,
                total: achievements.length,
              })
            }}
          </div>
        </div>
      </div>
    </template>

    <div class="text-center pb-4">
      <!-- Achievement Display with Animation -->
      <div v-if="currentAchievement" class="space-y-4">
        <!-- Animated Icon Container -->
        <div class="relative inline-block achievement-icon-container">
          <!-- Rotating ring animation -->
          <div
            class="absolute inset-0 achievement-ring"
            :class="getTierBorderColor(currentAchievement.tier)"
          />

          <!-- Main icon container -->
          <div
            :class="[
              'w-28 h-28 rounded-full flex items-center justify-center border-4 shadow-xl mx-auto relative z-10',
              getTierBgColor(currentAchievement.tier),
              getTierBorderColor(currentAchievement.tier),
              getTierGlow(currentAchievement.tier),
              'achievement-icon-pulse',
            ]"
          >
            <i
              :class="[
                getIconForType(currentAchievement.type),
                getTierColor(currentAchievement.tier),
                'text-5xl',
              ]"
            />
          </div>

          <!-- Sparkle animations -->
          <div class="sparkle sparkle-1">
            <i class="pi pi-sparkles text-yellow-400" />
          </div>
          <div class="sparkle sparkle-2">
            <i class="pi pi-sparkles text-yellow-300" />
          </div>
          <div class="sparkle sparkle-3">
            <i class="pi pi-sparkles text-yellow-400" />
          </div>
        </div>

        <!-- Achievement Name with fade-in -->
        <div class="achievement-text">
          <h3 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
            {{ currentAchievement.name }}
          </h3>
          <p class="text-sm text-surface-600 dark:text-surface-400 mt-2 px-4">
            {{ currentAchievement.description }}
          </p>
        </div>

        <!-- Tier Badge -->
        <div class="flex items-center justify-center gap-2">
          <span
            :class="[
              'px-4 py-1.5 rounded-md text-sm font-semibold shadow-sm',
              getTierBgColor(currentAchievement.tier),
              getTierColor(currentAchievement.tier),
            ]"
          >
            <i class="pi pi-crown mr-1" />
            {{ tierLabel(currentAchievement.tier) }}
          </span>
        </div>

        <!-- Points with bounce animation -->
        <div class="points-display">
          <div class="text-3xl font-bold text-primary-600">
            {{ t('achievementUnlock.pointsAwarded', { points: currentAchievement.points }) }}
          </div>
        </div>

        <!-- Navigation dots for multiple achievements -->
        <div v-if="hasMultiple" class="flex justify-center gap-2 pt-2">
          <button
            v-for="(_, index) in achievements"
            :key="index"
            @click="currentIndex = index"
            :class="[
              'w-2.5 h-2.5 rounded-full transition-all',
              index === currentIndex
                ? 'bg-primary-500 w-6'
                : 'bg-surface-300 dark:bg-surface-600 hover:bg-surface-400',
            ]"
          />
        </div>

        <!-- Total points summary for multiple -->
        <div
          v-if="hasMultiple && currentIndex === achievements.length - 1"
          class="mt-4 p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-xl border border-yellow-200 dark:border-yellow-700"
        >
          <div class="text-sm text-surface-500 mb-1">
            {{ t('achievementUnlock.totalPointsHeading') }}
          </div>
          <div class="text-2xl font-bold text-yellow-600">
            {{ t('achievementUnlock.totalPointsValue', { points: totalPoints }) }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="w-full flex gap-2">
        <Button
          v-if="hasMultiple && currentIndex > 0"
          @click="prevAchievement"
          icon="pi pi-arrow-left"
          severity="secondary"
          class="flex-1"
        />
        <Button
          @click="nextAchievement"
          :icon="currentIndex < achievements.length - 1 ? 'pi pi-arrow-right' : 'pi pi-thumbs-up'"
          :iconPos="currentIndex < achievements.length - 1 ? 'right' : 'left'"
          :label="
            currentIndex < achievements.length - 1
              ? t('achievementUnlock.next')
              : t('achievementUnlock.finish')
          "
          class="flex-1"
        />
      </div>
    </template>
  </Dialog>
</template>

<style scoped>
/* Icon container positioning */
.achievement-icon-container {
  width: 140px;
  height: 140px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Rotating ring animation */
.achievement-ring {
  position: absolute;
  width: 140px;
  height: 140px;
  border-radius: 50%;
  border: 2px dashed;
  opacity: 0.5;
  animation: rotate 10s linear infinite;
}

@keyframes rotate {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* Pulse animation for the icon */
@keyframes achievementPulse {
  0%,
  100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 currentColor;
  }
  50% {
    transform: scale(1.05);
    box-shadow: 0 0 25px 5px currentColor;
  }
}

.achievement-icon-pulse {
  animation: achievementPulse 2s ease-in-out infinite;
}

/* Sparkle animations */
.sparkle {
  position: absolute;
  animation: sparkle 2s ease-in-out infinite;
}

.sparkle-1 {
  top: -5px;
  right: 10px;
  animation-delay: 0s;
}

.sparkle-2 {
  top: 30px;
  left: -5px;
  animation-delay: 0.6s;
}

.sparkle-3 {
  bottom: 10px;
  right: -10px;
  animation-delay: 1.2s;
}

@keyframes sparkle {
  0%,
  100% {
    opacity: 0;
    transform: scale(0) rotate(0deg);
  }
  50% {
    opacity: 1;
    transform: scale(1.2) rotate(180deg);
  }
}

/* Text fade-in animation */
.achievement-text {
  animation: fadeInUp 0.5s ease-out;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Points bounce animation */
.points-display {
  animation: bounceIn 0.6s cubic-bezier(0.68, -0.55, 0.265, 1.55);
}

@keyframes bounceIn {
  0% {
    opacity: 0;
    transform: scale(0.3);
  }
  50% {
    transform: scale(1.1);
  }
  100% {
    opacity: 1;
    transform: scale(1);
  }
}

/* Achievement header animation */
.achievement-header {
  animation: slideDown 0.4s ease-out;
}

@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
