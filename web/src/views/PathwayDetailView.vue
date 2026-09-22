<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { usePathwayStore } from '@/stores/pathway'
import { useAuthStore } from '@/stores/auth'
import { useModuleUnlock } from '@/composables/useModuleUnlock'
import { useFocusRestore } from '@/composables'

const { t } = useI18n()
import {
  PathwayHeader,
  PathwayProgressCard,
  PathwayLearningRoadmap,
  PathwayPrerequisites,
  CertificateModal,
} from '@/components/pathway'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import { useToast } from 'primevue/usetoast'
import { pathwaysApi, enrollmentsApi, type PathwayModule, type ModuleProgressStatus, type Certificate } from '@/api'

const router = useRouter()
const route = useRoute()
const pathwayStore = usePathwayStore()
const authStore = useAuthStore()
const toast = useToast()

// Confirmation dialog state
const showUnenrollDialog = ref(false)
useFocusRestore(showUnenrollDialog)
const publishing = ref(false)

// Certificate state
const showCertificateModal = ref(false)
const certificate = ref<Certificate | null>(null)
const loadingCertificate = ref(false)

// Check if user can manage this pathway (instructor/admin)
const canManage = computed(() => {
  return authStore.isAdmin || authStore.isInstructor
})

// Check if pathway is in draft status
const isDraft = computed(() => pathway.value?.status === 'draft')

// Get pathway slug from route
const pathwaySlug = computed(() => route.params['slug'] as string)

// Derived state from store
const pathway = computed(() => pathwayStore.pathway)
const enrollment = computed(() => pathwayStore.enrollment)
const loading = computed(() => pathwayStore.loading)
const enrolling = computed(() => pathwayStore.enrolling)
const error = computed(() => pathwayStore.errorMessage)
const isEnrolled = computed(() => pathwayStore.isEnrolled)
const isPathwayComplete = computed(() => pathwayStore.isPathwayComplete)

// Computed stats
const moduleCount = computed(() => pathway.value?.moduleCount ?? pathway.value?.modules?.length ?? 0)
const labCount = computed(() => {
  if (pathway.value?.labCount) return pathway.value.labCount
  return pathway.value?.modules?.reduce((sum, m) => sum + (m.labCount ?? m.labs?.length ?? 0), 0) ?? 0
})
const totalPoints = computed(() => {
  if (pathway.value?.totalPoints) return pathway.value.totalPoints
  return pathway.value?.modules?.reduce((sum, m) => sum + (m.totalPoints ?? 0), 0) ?? 0
})
const estimatedHours = computed(() => pathway.value?.estimatedHours ?? 0)

// Build module status map for the roadmap
const moduleStatusMap = computed(() => {
  const map = new Map<string, ModuleProgressStatus>()
  if (!pathway.value?.modules) return map

  for (const module of pathway.value.modules) {
    const status = pathwayStore.getModuleStatus(module.id)
    map.set(module.id, status)
  }

  // If not enrolled, show first module as unlocked, rest as locked
  if (!isEnrolled.value && pathway.value.modules.length > 0) {
    const sortedModules = [...pathway.value.modules].sort((a, b) => a.displayOrder - b.displayOrder)
    if (sortedModules[0]) {
      map.set(sortedModules[0].id, 'unlocked')
    }
  }

  return map
})

// Build module progress map
const moduleProgressMap = computed(() => {
  const map = new Map<string, number>()
  if (!pathway.value?.modules) return map

  for (const module of pathway.value.modules) {
    const progress = pathwayStore.getModuleProgress(module.id)
    map.set(module.id, progress)
  }
  return map
})

// Sorted modules
const sortedModules = computed(() => {
  if (!pathway.value?.modules) return []
  return [...pathway.value.modules].sort((a, b) => a.displayOrder - b.displayOrder)
})

// Module unlock logic
const modulesRef = computed(() => pathway.value?.modules ?? [])
const moduleProgressMapRef = computed(() => pathwayStore.moduleProgressMap)
const enrollmentIdRef = computed(() => enrollment.value?.id ?? null)

const { getUnlockState } = useModuleUnlock({
  modules: modulesRef,
  moduleProgressMap: moduleProgressMapRef,
  enrollmentId: enrollmentIdRef,
})

// Build unlock messages map for the roadmap
const unlockMessagesMap = computed(() => {
  const map = new Map<string, string>()
  if (!pathway.value?.modules) return map

  for (const module of pathway.value.modules) {
    const state = getUnlockState(module.id)
    if (state.isLocked && state.message) {
      map.set(module.id, state.message)
    }
  }
  return map
})

onMounted(async () => {
  await pathwayStore.fetchPathway(pathwaySlug.value)
  if (pathway.value) {
    await pathwayStore.fetchEnrollment(pathway.value.id)
  }
})

onUnmounted(() => {
  pathwayStore.reset()
})

async function handleEnroll() {
  if (!pathway.value) return
  try {
    await pathwayStore.enroll(pathway.value.id)
  } catch {
    // Error is already in store
  }
}

function handleContinue() {
  if (!enrollment.value) return

  // Find the first in-progress or unlocked module
  const nextModule = sortedModules.value.find(m => {
    const status = pathwayStore.getModuleStatus(m.id)
    return status === 'in_progress' || status === 'unlocked'
  })

  if (nextModule?.labs?.[0]) {
    // Navigate to the first lab in the module with pathway context
    router.push({
      path: `/labs/${nextModule.labs[0].labTemplateId}`,
      query: {
        enrollmentId: enrollment.value.id,
        moduleId: nextModule.id
      }
    })
  }
}

function handleUnenroll() {
  showUnenrollDialog.value = true
}

async function confirmUnenroll() {
  if (!pathway.value) return
  try {
    await pathwayStore.unenroll(pathway.value.id)
    showUnenrollDialog.value = false
  } catch {
    // Error is already in store
  }
}

async function publishPathway() {
  if (!pathway.value) return
  try {
    publishing.value = true
    await pathwaysApi.publish(pathway.value.id)
    toast.add({
      severity: 'success',
      summary: t('pathway.detail.publishedToast.summary'),
      detail: t('pathway.detail.publishedToast.detail', { name: pathway.value.name }),
      life: 3000,
    })
    // Refresh the pathway data
    await pathwayStore.fetchPathway(pathwaySlug.value)
  } catch (err) {
    console.error('Failed to publish pathway:', err)
    toast.add({
      severity: 'error',
      summary: t('pathway.detail.publishFailedToast.summary'),
      detail: err instanceof Error ? err.message : t('pathway.detail.publishFailedToast.fallback'),
      life: 5000,
    })
  } finally {
    publishing.value = false
  }
}

function handleModuleClick(module: PathwayModule) {
  if (!isEnrolled.value || !enrollment.value) {
    // Prompt to enroll
    return
  }

  // Find first lab in module and navigate with pathway context
  if (module.labs?.[0]) {
    router.push({
      path: `/labs/${module.labs[0].labTemplateId}`,
      query: {
        enrollmentId: enrollment.value.id,
        moduleId: module.id
      }
    })
  }
}

function handleLabClick(labId: string, module: PathwayModule) {
  if (!isEnrolled.value || !enrollment.value) {
    // Prompt to enroll
    return
  }
  // Navigate to lab with pathway context
  router.push({
    path: `/labs/${labId}`,
    query: {
      enrollmentId: enrollment.value.id,
      moduleId: module.id
    }
  })
}

function clearError() {
  pathwayStore.clearError()
}

async function handleViewCertificate() {
  if (!enrollment.value) return

  // If we already have the certificate cached, just show the modal
  if (certificate.value && certificate.value.enrollmentId === enrollment.value.id) {
    showCertificateModal.value = true
    return
  }

  loadingCertificate.value = true
  try {
    // Try to get existing certificate first
    if (enrollment.value.certificateIssued) {
      certificate.value = await enrollmentsApi.getCertificate(enrollment.value.id)
    } else {
      // Issue a new certificate
      certificate.value = await enrollmentsApi.issueCertificate(enrollment.value.id)
      toast.add({
        severity: 'success',
        summary: t('pathway.detail.certificateIssuedToast.summary'),
        detail: t('pathway.detail.certificateIssuedToast.detail'),
        life: 3000,
      })
    }
    showCertificateModal.value = true
  } catch (err) {
    console.error('Failed to get certificate:', err)
    toast.add({
      severity: 'error',
      summary: t('pathway.detail.certificateErrorToast.summary'),
      detail: err instanceof Error ? err.message : t('pathway.detail.certificateErrorToast.fallback'),
      life: 5000,
    })
  } finally {
    loadingCertificate.value = false
  }
}
</script>

<template>
  <div class="pathway-detail-view">
    <!-- Loading state -->
    <div v-if="loading" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Error message (when no pathway loaded) -->
    <Message v-else-if="error && !pathway" severity="error">
      {{ error }}
      <template #closeicon>
        <Button
          :label="t('pathway.detail.backAction')"
          severity="secondary"
          size="small"
          class="ml-4"
          @click="router.push('/pathways')"
        />
      </template>
    </Message>

    <template v-else-if="pathway">
      <!-- Breadcrumb and Actions -->
      <div class="flex items-center justify-between mb-6">
        <div class="flex items-center gap-2 text-sm text-surface-500">
          <router-link to="/pathways" class="hover:text-surface-700 dark:hover:text-surface-300">
            {{ t('pathway.detail.breadcrumbList') }}
          </router-link>
          <i class="pi pi-chevron-right text-xs" />
          <span class="text-surface-700 dark:text-surface-300">{{ pathway.name }}</span>
          <Tag v-if="isDraft" :value="t('pathway.detail.draftTag')" severity="warn" class="ml-2" />
        </div>

        <!-- Instructor/Admin Actions -->
        <div v-if="canManage" class="flex items-center gap-2">
          <Button
            v-if="isDraft"
            :label="t('pathway.detail.publishAction')"
            icon="pi pi-globe"
            severity="success"
            size="small"
            :loading="publishing"
            @click="publishPathway"
          />
          <Button
            :label="t('pathway.detail.editAction')"
            icon="pi pi-pencil"
            severity="secondary"
            size="small"
            @click="router.push(`/pathways/${pathway.slug || pathway.id}/edit`)"
          />
        </div>
      </div>

      <!-- Main layout: Header + Progress Card -->
      <div class="flex flex-col lg:flex-row gap-6 mb-8">
        <!-- Header section (left) -->
        <Card class="flex-1">
          <template #content>
            <PathwayHeader :pathway="pathway" />
          </template>
        </Card>

        <!-- Progress Card (right) -->
        <PathwayProgressCard
          class="lg:w-80 flex-shrink-0"
          :is-enrolled="isEnrolled"
          :progress="pathwayStore.overallProgress"
          :module-count="moduleCount"
          :completed-modules="pathwayStore.completedModules"
          :lab-count="labCount"
          :estimated-hours="estimatedHours"
          :total-points="totalPoints"
          :earned-points="pathwayStore.earnedPoints"
          :enrolling="enrolling"
          :is-complete="isPathwayComplete"
          @enroll="handleEnroll"
          @continue="handleContinue"
          @unenroll="handleUnenroll"
          @view-certificate="handleViewCertificate"
        />
      </div>

      <!-- Error for enrollment actions -->
      <Message
        v-if="error && pathway"
        severity="error"
        :closable="true"
        class="mb-6"
        @close="clearError"
      >
        {{ error }}
      </Message>

      <!-- Prerequisites -->
      <div v-if="pathway.prerequisites?.length" class="mb-6">
        <PathwayPrerequisites :prerequisites="pathway.prerequisites" />
      </div>

      <!-- Learning Roadmap -->
      <Card v-if="sortedModules.length > 0">
        <template #content>
          <PathwayLearningRoadmap
            :modules="sortedModules"
            :module-status="moduleStatusMap"
            :module-progress="moduleProgressMap"
            :unlock-messages="unlockMessagesMap"
            :is-pathway-complete="isPathwayComplete"
            @module-click="handleModuleClick"
            @lab-click="handleLabClick"
          />
        </template>
      </Card>

      <!-- Empty state -->
      <Card v-else>
        <template #content>
          <div class="text-center py-8">
            <i class="pi pi-inbox text-4xl text-surface-400 mb-4" />
            <p class="text-surface-500">{{ t('pathway.detail.emptyModules') }}</p>
          </div>
        </template>
      </Card>

      <!-- Unenroll Confirmation Dialog -->
      <ConfirmDialog
        v-model:visible="showUnenrollDialog"
        :header="t('pathway.detail.unenrollDialog.header')"
        :message="t('pathway.detail.unenrollDialog.message')"
        :accept-label="t('pathway.detail.unenrollDialog.accept')"
        :reject-label="t('pathway.detail.unenrollDialog.reject')"
        accept-severity="danger"
        @accept="confirmUnenroll"
      />

      <!-- Certificate Modal -->
      <CertificateModal
        :visible="showCertificateModal"
        :certificate="certificate"
        @close="showCertificateModal = false"
      />
    </template>
  </div>
</template>
