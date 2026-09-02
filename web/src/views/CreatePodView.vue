<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { labsApi, podsApi, type Lab } from '@/api'
import { useAuthStore } from '../stores/auth'
import { getDifficultyColor } from '@/utils/status'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Select from '@volt/Select.vue'
import InputText from '@volt/InputText.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useToast } from 'primevue/usetoast'

const router = useRouter()
const authStore = useAuthStore()
const toast = useToast()
const { t } = useI18n()

// Form state
const selectedLab = ref<Lab | null>(null)
const owner = ref(authStore.user?.name || authStore.user?.email || '')

// Loading states
const labs = ref<Lab[]>([])
const loadingLabs = ref(true)
const creating = ref(false)
const error = ref<string | null>(null)

// Validation
const isValid = computed(() => {
  return selectedLab.value !== null && owner.value.trim().length > 0
})

// Check if user is admin (can change owner)
const isAdmin = computed(() => authStore.isAdmin)

onMounted(async () => {
  await loadLabs()
  // Set default owner to current user
  if (authStore.user) {
    owner.value = authStore.user.name || authStore.user.email || ''
  }
})

async function loadLabs() {
  try {
    loadingLabs.value = true
    error.value = null
    const response = await labsApi.list()
    labs.value = response.labs
  } catch (err) {
    console.error('Failed to load labs:', err)
    error.value = t('pods.create.errors.loadFailed')
  } finally {
    loadingLabs.value = false
  }
}

async function createPod() {
  if (!isValid.value || !selectedLab.value) return

  try {
    creating.value = true
    error.value = null

    const pod = await podsApi.create(selectedLab.value.name, owner.value.trim())

    toast.add({
      severity: 'success',
      summary: t('pods.create.toast.createdSummary'),
      detail: t('pods.create.toast.createdDetail', { id: pod.id }),
      life: 3000,
    })

    // Navigate to the pod detail page
    router.push(`/pods/${pod.id}`)
  } catch (err) {
    console.error('Failed to create pod:', err)
    error.value = err instanceof Error ? err.message : t('pods.create.errors.createFailed')
  } finally {
    creating.value = false
  }
}

function cancel() {
  router.push('/pods')
}

// Expose refs for testing
defineExpose({
  selectedLab,
  owner,
  creating,
  error,
  isValid,
})
</script>

<template>
  <div class="max-w-2xl mx-auto space-y-6">
    <!-- Header -->
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('pods.create.title') }}</h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('pods.create.subtitle') }}</p>
    </div>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading labs -->
    <div v-if="loadingLabs" class="flex justify-center py-12">
      <ProgressSpinner />
    </div>

    <!-- Form -->
    <Card v-else>
      <template #content>
        <form @submit.prevent="createPod" class="space-y-6">
          <!-- Lab Template Selection -->
          <div class="space-y-2">
            <label for="lab-select" class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pods.create.form.labLabel') }} <span class="text-red-500">*</span>
            </label>
            <Select
              id="lab-select"
              v-model="selectedLab"
              :options="labs"
              optionLabel="name"
              :placeholder="t('pods.create.form.labPlaceholder')"
              class="w-full"
              :filter="true"
              :filterPlaceholder="t('pods.create.form.labFilterPlaceholder')"
            >
              <template #option="{ option }">
                <div class="py-1">
                  <div class="font-medium">{{ option.name }}</div>
                  <div class="text-sm text-surface-500 flex items-center gap-3">
                    <span :class="getDifficultyColor(option.difficulty)">{{ option.difficulty }}</span>
                    <span>{{ t('pods.create.form.durationMinutes', { count: option.durationMinutes }) }}</span>
                    <span>{{ option.platform }}</span>
                  </div>
                </div>
              </template>
              <template #value="{ value }">
                <div v-if="value" class="flex items-center gap-2">
                  <span>{{ value.name }}</span>
                  <span class="text-sm text-surface-500">({{ value.platform }})</span>
                </div>
                <span v-else class="text-surface-400">{{ t('pods.create.form.labPlaceholder') }}</span>
              </template>
            </Select>
            <p v-if="selectedLab" class="text-sm text-surface-500">
              {{ selectedLab.description }}
            </p>
          </div>

          <!-- Owner -->
          <div class="space-y-2">
            <label for="owner-input" class="block text-sm font-medium text-surface-700 dark:text-surface-300">
              {{ t('pods.create.form.ownerLabel') }} <span class="text-red-500">*</span>
            </label>
            <InputText
              id="owner-input"
              v-model="owner"
              :placeholder="t('pods.create.form.ownerPlaceholder')"
              class="w-full"
              :disabled="!isAdmin"
            />
            <p v-if="!isAdmin" class="text-sm text-surface-500">
              {{ t('pods.create.form.ownerHint') }}
            </p>
          </div>

          <!-- Selected Lab Details -->
          <div v-if="selectedLab" class="bg-surface-100 dark:bg-surface-800 rounded-lg p-4 space-y-2">
            <h4 class="font-medium text-surface-900 dark:text-surface-100">{{ t('pods.create.details.heading') }}</h4>
            <div class="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span class="text-surface-500">{{ t('pods.create.details.platform') }}</span>
                <span class="ml-2 text-surface-900 dark:text-surface-100">{{ selectedLab.platform }}</span>
              </div>
              <div>
                <span class="text-surface-500">{{ t('pods.create.details.duration') }}</span>
                <span class="ml-2 text-surface-900 dark:text-surface-100">{{ t('pods.create.form.durationMinutes', { count: selectedLab.durationMinutes }) }}</span>
              </div>
              <div>
                <span class="text-surface-500">{{ t('pods.create.details.difficulty') }}</span>
                <span class="ml-2" :class="getDifficultyColor(selectedLab.difficulty)">{{ selectedLab.difficulty }}</span>
              </div>
              <div v-if="selectedLab.maxPoints">
                <span class="text-surface-500">{{ t('pods.create.details.maxPoints') }}</span>
                <span class="ml-2 text-surface-900 dark:text-surface-100">{{ selectedLab.maxPoints }}</span>
              </div>
            </div>
            <div v-if="selectedLab.tags?.length" class="flex flex-wrap gap-1 mt-2">
              <span
                v-for="tag in selectedLab.tags"
                :key="tag"
                class="px-2 py-0.5 text-xs bg-primary-100 dark:bg-primary-900 text-primary-700 dark:text-primary-300 rounded"
              >
                {{ tag }}
              </span>
            </div>
          </div>

          <!-- Actions -->
          <div class="flex justify-end gap-3 pt-4 border-t border-surface-200 dark:border-surface-700">
            <Button
              type="button"
              :label="t('pods.create.actions.cancel')"
              severity="secondary"
              @click="cancel"
              :disabled="creating"
            />
            <Button
              type="submit"
              :label="t('pods.create.actions.submit')"
              icon="pi pi-plus"
              :loading="creating"
              :disabled="!isValid || creating"
            />
          </div>
        </form>
      </template>
    </Card>

    <!-- Info card -->
    <Card>
      <template #content>
        <div class="flex gap-4">
          <i class="pi pi-info-circle text-2xl text-primary-500" />
          <div class="text-sm text-surface-600 dark:text-surface-400">
            <p class="font-medium text-surface-900 dark:text-surface-100 mb-1">{{ t('pods.create.info.heading') }}</p>
            <p>{{ t('pods.create.info.body') }}</p>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>
