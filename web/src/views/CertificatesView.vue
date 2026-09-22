<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import ProgressSpinner from 'primevue/progressspinner'
import CertificateModal from '@/components/pathway/CertificateModal.vue'
import { enrollmentsApi, type Certificate } from '@/api'
import { formatDate } from '@/utils/format'

const { t } = useI18n()

// Loading and error states
const loading = ref(true)
const error = ref<string | null>(null)

// Certificates data
const certificates = ref<Certificate[]>([])

// Modal state
const selectedCertificate = ref<Certificate | null>(null)
const showModal = ref(false)

// Fetch certificates
async function fetchCertificates() {
  loading.value = true
  error.value = null
  try {
    const response = await enrollmentsApi.listCertificates()
    // Handle both array response and wrapped response
    certificates.value = Array.isArray(response)
      ? response
      : (response as { certificates: Certificate[] }).certificates || []
  } catch (e) {
    console.error('Failed to fetch certificates:', e)
    error.value = t('certificates.loadFailed')
  } finally {
    loading.value = false
  }
}

// Open certificate modal
function openCertificate(cert: Certificate) {
  selectedCertificate.value = cert
  showModal.value = true
}

// Close modal
function closeModal() {
  showModal.value = false
  selectedCertificate.value = null
}

// formatDate imported from @/utils/format

onMounted(() => {
  fetchCertificates()
})
</script>

<template>
  <div class="certificates-view p-6">
    <!-- Header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
        {{ t('certificates.title') }}
      </h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">
        {{ t('certificates.subtitle') }}
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
              :label="t('certificates.tryAgain')"
              severity="secondary"
              size="small"
              class="mt-2"
              @click="fetchCertificates"
            />
          </div>
        </div>
      </template>
    </Card>

    <!-- Empty State -->
    <Card v-else-if="certificates.length === 0" class="text-center py-12">
      <template #content>
        <div class="flex flex-col items-center gap-4">
          <div
            class="w-20 h-20 rounded-full bg-surface-100 dark:bg-surface-800 flex items-center justify-center"
          >
            <i class="pi pi-trophy text-4xl text-surface-400" />
          </div>
          <div>
            <h3 class="text-lg font-semibold text-surface-900 dark:text-surface-100">
              {{ t('certificates.empty.title') }}
            </h3>
            <p class="text-surface-600 dark:text-surface-400 mt-1 max-w-md">
              {{ t('certificates.empty.body') }}
            </p>
          </div>
          <Button
            :label="t('certificates.empty.browseAction')"
            icon="pi pi-arrow-right"
            iconPos="right"
            @click="$router.push('/pathways')"
          />
        </div>
      </template>
    </Card>

    <!-- Certificates Grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      <Card
        v-for="cert in certificates"
        :key="cert.id"
        class="cursor-pointer hover:shadow-lg transition-shadow focus-within:ring-2 focus-within:ring-primary-500"
        role="button"
        tabindex="0"
        :aria-label="t('certificates.cardAria', { pathway: cert.pathwayName })"
        @click="openCertificate(cert)"
        @keydown.enter="openCertificate(cert)"
        @keydown.space.prevent="openCertificate(cert)"
      >
        <template #content>
          <div class="flex flex-col">
            <!-- Certificate Icon -->
            <div class="flex justify-center mb-4">
              <div class="w-16 h-16 rounded-full bg-amber-400 flex items-center justify-center">
                <i class="pi pi-verified text-white text-3xl" />
              </div>
            </div>

            <!-- Pathway Name -->
            <h3 class="text-lg font-bold text-center text-surface-900 dark:text-surface-100 mb-2">
              {{ cert.pathwayName }}
            </h3>

            <!-- User Name -->
            <p class="text-center text-surface-600 dark:text-surface-400 mb-4">
              {{ cert.userName }}
            </p>

            <!-- Score & Date -->
            <div class="flex justify-center gap-6 text-sm">
              <div class="text-center">
                <p class="font-bold text-amber-600 dark:text-amber-400">
                  {{ Math.round(cert.percentage) }}%
                </p>
                <p class="text-surface-500">{{ t('certificates.scoreLabel') }}</p>
              </div>
              <div class="text-center">
                <p class="font-bold text-surface-900 dark:text-surface-100">
                  {{ cert.earnedPoints }}
                </p>
                <p class="text-surface-500">{{ t('certificates.pointsLabel') }}</p>
              </div>
            </div>

            <!-- Completion Date -->
            <p class="text-center text-xs text-surface-500 mt-4">
              {{ t('certificates.completedAt', { when: formatDate(cert.completedAt) }) }}
            </p>
          </div>
        </template>
      </Card>
    </div>

    <!-- Certificate Modal -->
    <CertificateModal :visible="showModal" :certificate="selectedCertificate" @close="closeModal" />
  </div>
</template>

<style scoped>
.certificates-view {
  max-width: 1200px;
  margin: 0 auto;
}
</style>
