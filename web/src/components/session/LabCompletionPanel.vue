<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionStore } from '@/stores/session'
import { usePathwayStore } from '@/stores/pathway'
import { useNotifications } from '@/composables/useNotifications'
import { useFormatters } from '@/composables/useFormatters'
import type { SubmissionCheckpoint } from '@/api'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'

const { t, te } = useI18n()
const { formatDateTime } = useFormatters()

/** Localized tier name for a known tier, or the raw string as fallback. */
function tierLabel(tier: string): string {
  const key = `achievementTier.${tier}`
  return te(key) ? t(key) : tier
}

defineProps<{
  sessionId: string
}>()

const emit = defineEmits<{
  submit: []
  viewPod: []
  close: []
}>()

const sessionStore = useSessionStore()
const pathwayStore = usePathwayStore()
const { pathway: pathwayNotifications } = useNotifications()

// Confetti animation state
const showConfetti = ref(false)

const isSubmitting = computed(() => sessionStore.submissionStatus === 'submitting')
const isSubmitted = computed(() => sessionStore.submissionStatus === 'submitted')
const submissionResult = computed(() => sessionStore.submissionResult)

const canSubmit = computed(() => {
  return sessionStore.sessionStatus !== 'ended' &&
         sessionStore.submissionStatus === 'idle'
})

// Watch for submission success and trigger confetti
watch(() => submissionResult.value?.passed, (passed) => {
  if (passed) {
    showConfetti.value = true
    setTimeout(() => {
      showConfetti.value = false
    }, 3000)
  }
})

// Watch for module completion and show toast notifications
watch(() => sessionStore.moduleCompletion, async (completion) => {
  if (completion) {
    // Show toast for unlocked module
    if (completion.unlockedModuleName) {
      pathwayNotifications.moduleUnlocked(completion.unlockedModuleName)
    }
    // Show toast for pathway completion and auto-issue certificate
    if (completion.pathwayCompleted) {
      pathwayNotifications.completed(completion.pathwayName)
      // Auto-issue certificate for completed pathway using store method
      // This updates the store state so the certificate is immediately available
      try {
        const cert = await pathwayStore.issueCertificateOnCompletion()
        if (cert) {
          pathwayNotifications.certificateIssued(completion.pathwayName)
        }
      } catch (err) {
        // Log but don't disrupt the user experience
        console.error('Failed to auto-issue certificate:', err)
      }
    }
  }
})

// Calculate radial progress for the gauge
const gaugeRotation = computed(() => {
  const pct = sessionStore.percentage
  return (pct / 100) * 180 // 180 degrees for half circle
})

const gaugeColor = computed(() => {
  const pct = sessionStore.percentage
  if (pct >= 80) return '#22c55e' // green
  if (pct >= 60) return '#eab308' // yellow
  if (pct >= 40) return '#f97316' // orange
  return '#ef4444' // red
})

function getCheckpointIcon(checkpoint: SubmissionCheckpoint): string {
  return checkpoint.passed ? 'pi pi-check-circle' : 'pi pi-times-circle'
}

function getCheckpointColor(checkpoint: SubmissionCheckpoint): string {
  return checkpoint.passed ? 'text-green-500' : 'text-red-500'
}

function getPassedSeverity(passed: boolean): "success" | "danger" {
  return passed ? 'success' : 'danger'
}

function getTierColor(tier: string): string {
  switch (tier) {
    case 'bronze': return 'text-orange-600'
    case 'silver': return 'text-gray-400'
    case 'gold': return 'text-yellow-500'
    case 'platinum': return 'text-blue-400'
    case 'diamond': return 'text-purple-500'
    default: return 'text-surface-500'
  }
}

function getTierBg(tier: string): string {
  switch (tier) {
    case 'bronze': return 'bg-gradient-to-br from-orange-100 to-amber-100 dark:from-orange-900/30 dark:to-amber-900/30'
    case 'silver': return 'bg-gradient-to-br from-gray-100 to-slate-100 dark:from-gray-800/50 dark:to-slate-800/50'
    case 'gold': return 'bg-gradient-to-br from-yellow-100 to-amber-100 dark:from-yellow-900/30 dark:to-amber-900/30'
    case 'platinum': return 'bg-gradient-to-br from-blue-100 to-cyan-100 dark:from-blue-900/30 dark:to-cyan-900/30'
    case 'diamond': return 'bg-gradient-to-br from-purple-100 to-pink-100 dark:from-purple-900/30 dark:to-pink-900/30'
    default: return 'bg-surface-100 dark:bg-surface-800'
  }
}

function handleSubmit() {
  emit('submit')
}
</script>

<template>
  <Card class="relative overflow-hidden">
    <!-- Confetti overlay -->
    <div v-if="showConfetti" class="confetti-container">
      <div v-for="i in 50" :key="i" class="confetti" :style="{ '--i': i }" />
    </div>

    <template #title>
      <div class="flex items-center gap-2">
        <i class="pi pi-flag-fill text-primary-500" />
        <span>{{ t('labCompletion.cardTitle') }}</span>
      </div>
    </template>

    <template #content>
      <!-- Pre-submission state -->
      <div v-if="!isSubmitted" class="space-y-5">
        <!-- Radial Progress Gauge -->
        <div class="flex justify-center">
          <div class="relative w-40 h-24 overflow-hidden">
            <!-- Background arc -->
            <div class="absolute inset-0 rounded-t-full border-8 border-surface-200 dark:border-surface-700"
                 style="border-bottom: none;" />
            <!-- Progress arc -->
            <div
              class="absolute inset-0 rounded-t-full border-8 transition-all duration-500"
              :style="{
                borderColor: gaugeColor,
                borderBottom: 'none',
                clipPath: `polygon(50% 100%, 50% 0%, ${50 + 50 * Math.cos((180 - gaugeRotation) * Math.PI / 180)}% ${100 - 100 * Math.sin((180 - gaugeRotation) * Math.PI / 180)}%, 50% 100%)`,
                transform: `rotate(${-90 + gaugeRotation}deg)`,
                transformOrigin: '50% 100%'
              }"
            />
            <!-- Center content -->
            <div class="absolute bottom-0 left-1/2 transform -translate-x-1/2 text-center">
              <div class="text-3xl font-bold" :style="{ color: gaugeColor }">
                {{ sessionStore.percentage }}%
              </div>
              <div class="text-xs text-surface-500 -mt-1">{{ t('labCompletion.progressGauge.label') }}</div>
            </div>
          </div>
        </div>

        <!-- Stats Cards -->
        <div class="grid grid-cols-2 gap-3">
          <div class="p-3 bg-surface-50 dark:bg-surface-800 rounded-lg text-center">
            <div class="text-xl font-bold text-surface-900 dark:text-surface-100">
              {{ sessionStore.earnedPoints }}/{{ sessionStore.maxPoints }}
            </div>
            <div class="text-xs text-surface-500 flex items-center justify-center gap-1">
              <i class="pi pi-star text-yellow-500" />
              {{ t('labCompletion.stats.points') }}
            </div>
          </div>
          <div class="p-3 bg-surface-50 dark:bg-surface-800 rounded-lg text-center">
            <div class="text-xl font-bold text-surface-900 dark:text-surface-100">
              {{ sessionStore.passedCheckpoints }}/{{ sessionStore.totalCheckpoints }}
            </div>
            <div class="text-xs text-surface-500 flex items-center justify-center gap-1">
              <i class="pi pi-check-square text-green-500" />
              {{ t('labCompletion.stats.checkpoints') }}
            </div>
          </div>
        </div>

        <!-- Checkpoint Progress List -->
        <div v-if="sessionStore.checkpoints.length > 0" class="space-y-2">
          <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
            <i class="pi pi-list-check" />
            {{ t('labCompletion.checkpoints.heading') }}
          </h4>
          <div class="space-y-1.5 max-h-40 overflow-y-auto">
            <div
              v-for="checkpoint in sessionStore.checkpoints"
              :key="checkpoint.id"
              :class="[
                'flex items-center gap-2 p-2 rounded-lg text-sm transition-all',
                checkpoint.status === 'passed'
                  ? 'bg-green-50 dark:bg-green-900/20 border-l-4 border-green-500'
                  : 'bg-surface-50 dark:bg-surface-800 border-l-4 border-surface-300 dark:border-surface-600'
              ]"
            >
              <i
                :class="[
                  'pi',
                  checkpoint.status === 'passed' ? 'pi-check-circle text-green-500' : 'pi-circle text-surface-400'
                ]"
              />
              <span class="flex-1 text-surface-700 dark:text-surface-300 truncate">
                {{ checkpoint.name }}
              </span>
              <span
                :class="[
                  'text-xs font-mono',
                  checkpoint.status === 'passed' ? 'text-green-600 dark:text-green-400' : 'text-surface-400'
                ]"
              >
                {{ checkpoint.score }}/{{ checkpoint.maxScore }}
              </span>
            </div>
          </div>
        </div>

        <!-- Empty state for no checkpoints -->
        <div v-else class="text-center py-4 text-surface-500">
          <i class="pi pi-inbox text-2xl mb-2" />
          <p class="text-sm">{{ t('labCompletion.checkpoints.empty') }}</p>
        </div>

        <!-- Pass Threshold Indicator -->
        <div class="flex items-center gap-2 p-2 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-sm">
          <i class="pi pi-info-circle text-blue-500" />
          <span class="text-surface-600 dark:text-surface-400">
            {{ t('labCompletion.passThreshold.pre') }}
          </span>
        </div>

        <!-- Submit Button -->
        <Button
          @click="handleSubmit"
          :disabled="!canSubmit || isSubmitting"
          :loading="isSubmitting"
          icon="pi pi-send"
          :label="t('labCompletion.submit.button')"
          class="w-full"
          severity="success"
          size="large"
        />

        <p class="text-xs text-surface-500 text-center">
          <i class="pi pi-exclamation-triangle mr-1" />
          {{ t('labCompletion.submit.disclaimer') }}
        </p>
      </div>

      <!-- Post-submission results -->
      <div v-else-if="submissionResult" class="space-y-4">
        <!-- Result Header with celebration styling -->
        <div
          class="text-center p-6 rounded-xl relative overflow-hidden"
          :class="submissionResult.passed
            ? 'bg-gradient-to-br from-green-50 to-emerald-100 dark:from-green-900/30 dark:to-emerald-900/30'
            : 'bg-gradient-to-br from-red-50 to-orange-100 dark:from-red-900/30 dark:to-orange-900/30'"
        >
          <!-- Decorative background circles -->
          <div v-if="submissionResult.passed" class="absolute inset-0 overflow-hidden">
            <div class="absolute -top-10 -right-10 w-32 h-32 bg-green-200/30 rounded-full" />
            <div class="absolute -bottom-5 -left-5 w-24 h-24 bg-emerald-200/30 rounded-full" />
          </div>

          <div class="relative">
            <!-- Result Icon with pulse animation -->
            <div class="mb-3">
              <div
                :class="[
                  'inline-flex items-center justify-center w-20 h-20 rounded-full',
                  submissionResult.passed
                    ? 'bg-green-100 dark:bg-green-800 ring-4 ring-green-200 dark:ring-green-700 animate-pulse-once'
                    : 'bg-red-100 dark:bg-red-800 ring-4 ring-red-200 dark:ring-red-700'
                ]"
              >
                <i
                  :class="[
                    'text-4xl',
                    submissionResult.passed ? 'pi pi-check text-green-600' : 'pi pi-times text-red-600'
                  ]"
                />
              </div>
            </div>

            <h3
              class="text-2xl font-bold mb-2"
              :class="submissionResult.passed
                ? 'text-green-700 dark:text-green-400'
                : 'text-red-700 dark:text-red-400'"
            >
              {{ submissionResult.passed ? t('labCompletion.results.congrats') : t('labCompletion.results.notQuite') }}
            </h3>

            <!-- Score Display -->
            <div class="relative inline-block">
              <div
                class="text-5xl font-bold"
                :class="submissionResult.passed ? 'text-green-600' : 'text-red-600'"
              >
                {{ submissionResult.percentage }}%
              </div>
              <div class="text-sm text-surface-500 mt-1">
                {{ t('labCompletion.results.pointsSummary', { earned: submissionResult.earnedPoints, max: submissionResult.maxPoints }) }}
              </div>
            </div>

            <Tag
              :value="submissionResult.passed ? t('labCompletion.results.passed') : t('labCompletion.results.failed')"
              :severity="getPassedSeverity(submissionResult.passed)"
              class="mt-3"
            />
          </div>
        </div>

        <!-- Pass Threshold Info -->
        <div class="flex items-center justify-between text-sm p-3 bg-surface-50 dark:bg-surface-800 rounded-lg">
          <span class="text-surface-600 dark:text-surface-400 flex items-center gap-2">
            <i class="pi pi-chart-bar" />
            {{ t('labCompletion.passThreshold.label') }}
          </span>
          <span class="font-bold text-surface-900 dark:text-surface-100">
            {{ submissionResult.passThreshold }}%
          </span>
        </div>

        <!-- Checkpoint Details -->
        <div v-if="submissionResult.checkpoints?.length" class="space-y-2">
          <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
            <i class="pi pi-list" />
            {{ t('labCompletion.results.checkpointsHeading') }}
            <span class="text-xs text-surface-400">
              {{ t('labCompletion.results.checkpointsCount', {
                passed: submissionResult.checkpoints.filter(c => c.passed).length,
                total: submissionResult.checkpoints.length,
              }) }}
            </span>
          </h4>
          <div class="space-y-1 max-h-48 overflow-y-auto">
            <div
              v-for="checkpoint in submissionResult.checkpoints"
              :key="checkpoint.id"
              :class="[
                'flex items-center justify-between p-2.5 rounded-lg transition-all',
                checkpoint.passed
                  ? 'bg-green-50 dark:bg-green-900/20 border-l-4 border-green-500'
                  : 'bg-red-50 dark:bg-red-900/20 border-l-4 border-red-500'
              ]"
            >
              <div class="flex items-center gap-2 flex-1 min-w-0">
                <i :class="[getCheckpointIcon(checkpoint), getCheckpointColor(checkpoint)]" />
                <span class="text-sm text-surface-700 dark:text-surface-300 truncate">
                  {{ checkpoint.description }}
                </span>
              </div>
              <span
                class="text-sm font-mono ml-2 whitespace-nowrap"
                :class="checkpoint.passed ? 'text-green-600' : 'text-red-500'"
              >
                {{ checkpoint.earnedPoints }}/{{ checkpoint.points }}
              </span>
            </div>
          </div>
        </div>

        <!-- Module Completion Section -->
        <div v-if="sessionStore.moduleCompletion" class="space-y-2">
          <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
            <i class="pi pi-check-square text-green-500" />
            {{ t('labCompletion.module.heading') }}
          </h4>
          <div class="p-3 rounded-lg border bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-700">
            <div class="font-semibold text-surface-900 dark:text-surface-100">
              {{ sessionStore.moduleCompletion.moduleName }}
            </div>
            <div class="text-xs text-surface-500 mt-1">
              {{ t('labCompletion.module.partOf', { pathway: sessionStore.moduleCompletion.pathwayName }) }}
            </div>

            <!-- Unlocked next module -->
            <div
              v-if="sessionStore.moduleCompletion.unlockedModuleName"
              class="mt-2 pt-2 border-t border-green-200 dark:border-green-700 flex items-center gap-2"
            >
              <i class="pi pi-lock-open text-blue-500" />
              <span class="text-sm text-surface-700 dark:text-surface-300">
                {{ t('labCompletion.module.unlocked', { name: sessionStore.moduleCompletion.unlockedModuleName }) }}
              </span>
            </div>

            <!-- Pathway completed -->
            <div
              v-if="sessionStore.moduleCompletion.pathwayCompleted"
              class="mt-2 pt-2 border-t border-green-200 dark:border-green-700 flex items-center gap-2"
            >
              <i class="pi pi-trophy text-yellow-500" />
              <span class="text-sm font-semibold text-surface-700 dark:text-surface-300">
                {{ t('labCompletion.module.pathwayCompleted') }}
              </span>
            </div>
          </div>
        </div>

        <!-- New Achievements Section -->
        <div v-if="sessionStore.newAchievements.length > 0" class="space-y-2">
          <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 flex items-center gap-2">
            <i class="pi pi-star-fill text-yellow-500" />
            {{ t('labCompletion.achievementsHeading') }}
          </h4>
          <div class="space-y-2">
            <div
              v-for="userAchievement in sessionStore.newAchievements"
              :key="userAchievement.id"
              :class="[
                'flex items-center gap-3 p-3 rounded-lg border',
                getTierBg(userAchievement.achievement?.tier || 'bronze'),
                'border-yellow-200 dark:border-yellow-700'
              ]"
            >
              <div
                class="w-12 h-12 rounded-full flex items-center justify-center shadow-sm"
                :class="getTierBg(userAchievement.achievement?.tier || 'bronze')"
              >
                <i class="pi pi-trophy text-xl" :class="getTierColor(userAchievement.achievement?.tier || 'bronze')" />
              </div>
              <div class="flex-1 min-w-0">
                <div class="font-semibold text-surface-900 dark:text-surface-100 truncate">
                  {{ userAchievement.achievement?.name }}
                </div>
                <div class="text-xs text-surface-500 truncate">
                  {{ userAchievement.achievement?.description }}
                </div>
              </div>
              <div class="text-right">
                <div class="text-sm font-bold" :class="getTierColor(userAchievement.achievement?.tier || 'bronze')">
                  +{{ userAchievement.achievement?.points }}
                </div>
                <div class="text-xs text-surface-400">
                  {{ tierLabel(userAchievement.achievement?.tier || 'bronze') }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Submitted timestamp -->
        <div class="text-xs text-surface-400 text-center flex items-center justify-center gap-1">
          <i class="pi pi-clock" />
          {{ t('labCompletion.submittedAt', { when: formatDateTime(submissionResult.submittedAt) }) }}
        </div>

        <!-- Actions -->
        <div class="flex gap-2">
          <Button
            @click="emit('viewPod')"
            icon="pi pi-server"
            :label="t('labCompletion.actions.viewPod')"
            severity="secondary"
            class="flex-1"
          />
          <Button
            @click="emit('close')"
            icon="pi pi-home"
            :label="t('labCompletion.actions.done')"
            class="flex-1"
          />
        </div>
      </div>

      <!-- Error State -->
      <div v-if="sessionStore.submissionError" class="mt-4 p-3 bg-red-50 dark:bg-red-900/20 rounded-lg border border-red-200 dark:border-red-800">
        <div class="flex items-center gap-2 text-red-600 dark:text-red-400">
          <i class="pi pi-exclamation-triangle" />
          <span class="text-sm font-medium">{{ t('labCompletion.errorHeader') }}</span>
        </div>
        <p class="text-sm text-red-500 mt-1">{{ sessionStore.submissionError }}</p>
      </div>
    </template>
  </Card>
</template>

<style scoped>
/* Confetti animation */
.confetti-container {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  overflow: hidden;
  z-index: 10;
}

.confetti {
  position: absolute;
  width: 10px;
  height: 10px;
  top: -10px;
  left: calc(var(--i) * 2%);
  animation: confetti-fall 3s ease-out forwards;
  animation-delay: calc(var(--i) * 0.02s);
}

.confetti::before {
  content: '';
  display: block;
  width: 100%;
  height: 100%;
  background: hsl(calc(var(--i) * 7.2), 80%, 60%);
  transform: rotate(calc(var(--i) * 20deg));
  border-radius: 2px;
}

@keyframes confetti-fall {
  0% {
    top: -10px;
    transform: translateX(0) rotateZ(0deg);
    opacity: 1;
  }
  100% {
    top: 100%;
    transform: translateX(calc((var(--i) - 25) * 3px)) rotateZ(720deg);
    opacity: 0;
  }
}

/* Pulse once animation */
@keyframes pulse-once {
  0%, 100% {
    transform: scale(1);
    box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.4);
  }
  50% {
    transform: scale(1.05);
    box-shadow: 0 0 0 15px rgba(34, 197, 94, 0);
  }
}

.animate-pulse-once {
  animation: pulse-once 0.8s ease-out 2;
}
</style>
