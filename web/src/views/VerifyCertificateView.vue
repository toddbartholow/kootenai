<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import ProgressSpinner from 'primevue/progressspinner'
import { enrollmentsApi, type Certificate } from '@/api'
import { formatDate } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()

// Loading and error states
const loading = ref(true)
const error = ref<string | null>(null)
const notFound = ref(false)

// Certificate data
const certificate = ref<Certificate | null>(null)

// Verification code from route
const verificationCode = ref<string>('')

// Fetch and verify certificate
async function verifyCertificate() {
  const code = route.params['code'] as string
  if (!code) {
    error.value = t('verifyCertificate.errors.noCode')
    loading.value = false
    return
  }

  verificationCode.value = code
  loading.value = true
  error.value = null
  notFound.value = false

  try {
    certificate.value = await enrollmentsApi.verifyCertificate(code)
  } catch (e: unknown) {
    console.error('Failed to verify certificate:', e)
    const axiosError = e as { response?: { status?: number } }
    if (axiosError.response?.status === 404) {
      notFound.value = true
    } else {
      error.value = t('verifyCertificate.errors.loadFailed')
    }
  } finally {
    loading.value = false
  }
}

// formatDate imported from @/utils/format

onMounted(() => {
  verifyCertificate()
})
</script>

<template>
  <div class="verify-view min-h-screen bg-surface-50 dark:bg-surface-950 py-12 px-4">
    <div class="max-w-2xl mx-auto">
      <!-- Header -->
      <div class="text-center mb-8">
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
          {{ t('verifyCertificate.title') }}
        </h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">
          {{ t('verifyCertificate.subtitle') }}
        </p>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center items-center py-12">
        <ProgressSpinner />
      </div>

      <!-- Error State -->
      <Card v-else-if="error" class="bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800">
        <template #content>
          <div class="flex items-center gap-3">
            <i class="pi pi-exclamation-triangle text-red-500 text-xl" />
            <div>
              <p class="font-medium text-red-700 dark:text-red-400">{{ error }}</p>
              <Button
                :label="t('verifyCertificate.tryAgain')"
                severity="secondary"
                size="small"
                class="mt-2"
                @click="verifyCertificate"
              />
            </div>
          </div>
        </template>
      </Card>

      <!-- Not Found State -->
      <Card v-else-if="notFound" class="text-center py-8">
        <template #content>
          <div class="flex flex-col items-center gap-4">
            <div class="w-16 h-16 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center">
              <i class="pi pi-times text-3xl text-red-500" />
            </div>
            <div>
              <h3 class="text-lg font-semibold text-surface-900 dark:text-surface-100">
                {{ t('verifyCertificate.notFound.title') }}
              </h3>
              <p class="text-surface-600 dark:text-surface-400 mt-1">
                {{ t('verifyCertificate.notFound.body', { code: verificationCode }) }}
              </p>
            </div>
          </div>
        </template>
      </Card>

      <!-- Valid Certificate -->
      <Card v-else-if="certificate" class="overflow-hidden">
        <template #content>
          <!-- Verification Badge -->
          <div class="flex justify-center mb-6">
            <div class="flex items-center gap-2 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 px-4 py-2 rounded-full">
              <i class="pi pi-check-circle" />
              <span class="font-medium">{{ t('verifyCertificate.verifiedBadge') }}</span>
            </div>
          </div>

          <!-- Certificate Preview -->
          <div class="certificate-preview p-6 rounded-xl border-2 border-amber-300 dark:border-amber-600 bg-gradient-to-br from-amber-50 to-yellow-50 dark:from-amber-900/20 dark:to-yellow-900/20 text-center mb-6">
            <!-- Decorative Header -->
            <div class="mb-4">
              <i class="pi pi-verified text-5xl text-amber-500" />
            </div>

            <!-- Title -->
            <h3 class="text-lg font-semibold text-surface-700 dark:text-surface-300 mb-1">
              {{ t('verifyCertificate.certifies') }}
            </h3>

            <!-- User Name -->
            <p class="text-2xl font-bold text-surface-900 dark:text-surface-100 mb-4">
              {{ certificate.userName }}
            </p>

            <!-- Achievement Text -->
            <p class="text-surface-600 dark:text-surface-400 mb-2">
              {{ t('verifyCertificate.hasCompleted') }}
            </p>

            <!-- Pathway Name -->
            <p class="text-xl font-bold text-amber-600 dark:text-amber-400 mb-4">
              {{ certificate.pathwayName }}
            </p>

            <!-- Score -->
            <div class="flex justify-center gap-6 mb-4">
              <div class="text-center">
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ certificate.earnedPoints }}
                </p>
                <p class="text-xs text-surface-500">{{ t('verifyCertificate.pointsEarnedLabel') }}</p>
              </div>
              <div class="text-center">
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">
                  {{ Math.round(certificate.percentage) }}%
                </p>
                <p class="text-xs text-surface-500">{{ t('verifyCertificate.scoreLabel') }}</p>
              </div>
            </div>

            <!-- Date -->
            <p class="text-sm text-surface-500">
              {{ t('verifyCertificate.completedOn', { when: formatDate(certificate.completedAt) }) }}
            </p>
          </div>

          <!-- Verification Info -->
          <div class="space-y-3">
            <div class="flex items-center justify-between p-3 rounded-lg bg-surface-100 dark:bg-surface-800">
              <div>
                <p class="text-xs text-surface-500 mb-1">{{ t('verifyCertificate.verificationCodeLabel') }}</p>
                <p class="font-mono font-semibold text-surface-900 dark:text-surface-100">
                  {{ certificate.verificationCode }}
                </p>
              </div>
              <div class="text-green-500">
                <i class="pi pi-check-circle text-xl" />
              </div>
            </div>

            <p class="text-xs text-surface-500 text-center">
              {{ t('verifyCertificate.issuedOn', { when: formatDate(certificate.issuedAt) }) }}
            </p>
          </div>
        </template>
      </Card>

      <!-- Footer -->
      <div class="text-center mt-8">
        <p class="text-sm text-surface-500">
          {{ t('verifyCertificate.footerIssuedBy') }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.verify-view {
  background: linear-gradient(135deg, var(--p-surface-50) 0%, var(--p-surface-100) 100%);
}

:deep(.dark) .verify-view {
  background: linear-gradient(135deg, var(--p-surface-950) 0%, var(--p-surface-900) 100%);
}
</style>
