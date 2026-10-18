<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const hasError = ref(false)
const errorMessage = ref('')

onErrorCaptured((err: Error) => {
  hasError.value = true
  errorMessage.value = err.message || t('errorBoundary.fallbackMessage')
  console.error('[ErrorBoundary] Caught render error:', err)
  return false // prevent propagation
})

function reload() {
  window.location.reload()
}

function goHome() {
  window.location.href = '/'
}
</script>

<template>
  <div v-if="hasError" class="flex items-center justify-center min-h-[400px] p-8">
    <div class="text-center max-w-md">
      <div class="text-4xl mb-4">&#x26A0;</div>
      <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-2">
        {{ t('errorBoundary.heading') }}
      </h2>
      <p class="text-surface-600 dark:text-surface-400 mb-6">
        {{ errorMessage }}
      </p>
      <div class="flex gap-3 justify-center">
        <button
          class="px-4 py-2 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors"
          @click="reload"
        >
          {{ t('errorBoundary.reload') }}
        </button>
        <button
          class="px-4 py-2 bg-surface-200 dark:bg-surface-700 text-surface-700 dark:text-surface-200 rounded-lg hover:bg-surface-300 dark:hover:bg-surface-600 transition-colors"
          @click="goHome"
        >
          {{ t('errorBoundary.goHome') }}
        </button>
      </div>
    </div>
  </div>
  <slot v-else />
</template>
