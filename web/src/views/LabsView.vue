<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { labsApi, podsApi, type Lab } from '@/api'
import { formatDuration } from '@/utils/format'
import { getDifficultySeverity } from '@/utils/status'
import { useAuthStore } from '../stores/auth'
import { difficultyFilterOptions, platformFilterOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import { SkeletonCardGrid } from '@/components/common'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const labs = ref<Lab[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const launchingLab = ref<string | null>(null)

// Filter state
const searchQuery = ref('')
const selectedDifficulty = ref<string | null>(null)
const selectedPlatform = ref<string | null>(null)

// Use shared filter options
const difficultyOptions = difficultyFilterOptions
const platformOptions = platformFilterOptions

// Filtered labs based on search and filters
const filteredLabs = computed(() => {
  return labs.value.filter(lab => {
    // Search filter
    const matchesSearch = !searchQuery.value ||
      lab.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      lab.description.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      lab.tags?.some(tag => tag.toLowerCase().includes(searchQuery.value.toLowerCase()))

    // Difficulty filter
    const matchesDifficulty = !selectedDifficulty.value ||
      lab.difficulty === selectedDifficulty.value

    // Platform filter
    const matchesPlatform = !selectedPlatform.value ||
      lab.platform === selectedPlatform.value

    return matchesSearch && matchesDifficulty && matchesPlatform
  })
})

onMounted(async () => {
  try {
    const response = await labsApi.list()
    labs.value = response.labs
  } catch (err) {
    console.error('Failed to load labs:', err)
    error.value = t('labs.errors.loadFailed')
  } finally {
    loading.value = false
  }
})

function viewLab(lab: Lab) {
  router.push(`/labs/${lab.id}`)
}

async function launchLab(lab: Lab, event: Event) {
  event.stopPropagation()
  try {
    launchingLab.value = lab.id
    error.value = null

    const currentUser = authStore.user?.id || authStore.user?.email
    if (!currentUser) {
      error.value = t('labs.errors.mustBeLoggedIn')
      return
    }
    // Create a pod for this lab using the template ID
    const pod = await podsApi.create(lab.id, currentUser)

    // Navigate to the pod view
    router.push(`/pods/${pod.id}`)
  } catch (err) {
    console.error('Failed to launch lab:', err)
    error.value = t('labs.errors.launchFailed', { name: lab.name })
  } finally {
    launchingLab.value = null
  }
}

function clearFilters() {
  searchQuery.value = ''
  selectedDifficulty.value = null
  selectedPlatform.value = null
}

// formatDuration imported from @/utils/format

// getDifficultySeverity imported from @/utils/status
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('labs.title') }}</h1>
        <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('labs.subtitle') }}</p>
      </div>
      <div class="text-sm text-surface-500">
        {{ t('labs.resultCount', { shown: filteredLabs.length, total: labs.length }) }}
      </div>
    </div>

    <!-- Search and Filters -->
    <Card>
      <template #content>
        <div class="flex flex-col md:flex-row gap-4">
          <div class="flex-1">
            <span class="p-input-icon-left w-full">
              <i class="pi pi-search" aria-hidden="true" />
              <InputText
                v-model="searchQuery"
                :placeholder="t('labs.search.placeholder')"
                :aria-label="t('labs.search.ariaLabel')"
                class="w-full"
              />
            </span>
          </div>
          <Select
            v-model="selectedDifficulty"
            :options="difficultyOptions"
            optionLabel="label"
            optionValue="value"
            :placeholder="t('labs.filters.difficulty')"
            :aria-label="t('labs.filters.difficulty')"
            class="w-full md:w-48"
          />
          <Select
            v-model="selectedPlatform"
            :options="platformOptions"
            optionLabel="label"
            optionValue="value"
            :placeholder="t('labs.filters.platform')"
            :aria-label="t('labs.filters.platform')"
            class="w-full md:w-40"
          />
          <Button
            v-if="searchQuery || selectedDifficulty || selectedPlatform"
            icon="pi pi-times"
            :label="t('labs.filters.clear')"
            severity="secondary"
            text
            @click="clearFilters"
          />
        </div>
      </template>
    </Card>

    <!-- Error message -->
    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <!-- Loading state -->
    <SkeletonCardGrid v-if="loading" :count="6" :show-tags="true" />

    <!-- Empty state (no labs at all) -->
    <Card v-else-if="labs.length === 0">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-inbox text-4xl text-surface-400 mb-4" />
          <p class="text-surface-500 text-lg">{{ t('labs.empty.none.title') }}</p>
          <p class="text-surface-400 mt-2">{{ t('labs.empty.none.hint') }}</p>
        </div>
      </template>
    </Card>

    <!-- No results from filter -->
    <Card v-else-if="filteredLabs.length === 0">
      <template #content>
        <div class="text-center py-8">
          <i class="pi pi-search text-4xl text-surface-400 mb-4" />
          <p class="text-surface-500 text-lg">{{ t('labs.empty.noMatches.title') }}</p>
          <p class="text-surface-400 mt-2">{{ t('labs.empty.noMatches.hint') }}</p>
          <Button :label="t('labs.filters.clearFilters')" severity="secondary" class="mt-4" @click="clearFilters" />
        </div>
      </template>
    </Card>

    <!-- Labs grid -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6" data-pseudo-skip>
      <Card
        v-for="lab in filteredLabs"
        :key="lab.id"
        class="cursor-pointer hover:shadow-lg transition-shadow focus-within:ring-2 focus-within:ring-primary-500"
        role="button"
        tabindex="0"
        :aria-label="t('labs.card.viewDetailsAria', { name: lab.name })"
        @click="viewLab(lab)"
        @keydown.enter="viewLab(lab)"
        @keydown.space.prevent="viewLab(lab)"
      >
        <template #content>
          <div class="space-y-4">
            <div class="flex items-start justify-between">
              <h3 class="font-semibold text-surface-900 dark:text-surface-100">{{ lab.name }}</h3>
              <Tag :value="lab.difficulty || t('common.notAvailable')" :severity="getDifficultySeverity(lab.difficulty || '')" />
            </div>

            <p class="text-sm text-surface-600 dark:text-surface-400 line-clamp-2">{{ lab.description }}</p>

            <div v-if="lab.tags?.length" class="flex flex-wrap gap-1">
              <Tag
                v-for="tag in lab.tags.slice(0, 3)"
                :key="tag"
                :value="tag"
                severity="secondary"
                class="text-xs"
              />
              <Tag
                v-if="lab.tags.length > 3"
                :value="t('labs.card.moreTags', { count: lab.tags.length - 3 })"
                severity="secondary"
                class="text-xs"
              />
            </div>

            <div class="flex items-center justify-between text-sm text-surface-500">
              <span><i class="pi pi-clock mr-1" />{{ formatDuration(lab.durationMinutes) }}</span>
              <span class="capitalize"><i class="pi pi-server mr-1" />{{ lab.platform }}</span>
            </div>

            <div class="flex gap-2" @click.stop>
              <Button
                :label="t('labs.card.detailsAction')"
                icon="pi pi-info-circle"
                severity="secondary"
                size="small"
                class="flex-1"
                @click="viewLab(lab)"
              />
              <Button
                :loading="launchingLab === lab.id"
                :disabled="launchingLab === lab.id"
                :label="t('labs.card.launchAction')"
                icon="pi pi-play"
                size="small"
                class="flex-1"
                @click="launchLab(lab, $event)"
              />
            </div>
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
